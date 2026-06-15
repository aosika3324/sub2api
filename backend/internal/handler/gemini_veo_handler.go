package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Veo 视频生成的异步透传 handler。
//
// 三个端点：
//   - 提交：POST /v1beta/models/veo-3.x:predictLongRunning（由 GeminiV1BetaModels 分流到 handleVeoSubmit）
//   - 轮询：GET  /v1beta/{operationName}（GeminiV1BetaGetOperation）
//   - 下载：GET  /v1beta/veo-files/proxy?account=&uri=（GeminiV1BetaProxyVeoFile）
//
// 仅支持 API Key 账号；计费在轮询返回 done:true 且成功时按视频秒数结算。

// handleVeoSubmit 处理 Veo 提交任务请求。
func (h *GatewayHandler) handleVeoSubmit(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	authSubject middleware.AuthSubject,
	modelName string,
	body []byte,
) {
	// 计费资格预检查（与 generateContent 路径一致）。
	subscription, _ := middleware.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(
		c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription,
		service.QuotaPlatform(c.Request.Context(), apiKey),
	); err != nil {
		reqLog.Info("veo.billing_eligibility_check_failed", zap.Error(err))
		status, _, message, _ := billingErrorDetails(err)
		googleError(c, status, message)
		return
	}

	// 选择一个 API Key 类型的 Gemini 账号（Veo 不需要粘性会话，但提交后需绑定 operation）。
	account, err := h.geminiCompatService.SelectAccountForAIStudioEndpoints(c.Request.Context(), apiKey.GroupID)
	if err != nil {
		markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		googleError(c, http.StatusServiceUnavailable, "No available Gemini accounts: "+err.Error())
		return
	}
	if account.Type != service.AccountTypeAPIKey {
		googleError(c, http.StatusServiceUnavailable, "Veo requires an api_key type Gemini account")
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	result, err := h.geminiCompatService.SubmitVeo(c.Request.Context(), account, modelName, body)
	if err != nil {
		reqLog.Error("veo.submit_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		googleError(c, http.StatusBadGateway, "Veo submit failed: "+err.Error())
		return
	}

	// 绑定 operation -> account，供后续轮询命中同一上游账号。
	if result.StatusCode < 400 && result.OperationName != "" {
		if err := h.geminiCompatService.BindVeoOperation(
			c.Request.Context(), apiKey.GroupID, result.OperationName, account.ID,
		); err != nil {
			reqLog.Warn("veo.bind_operation_failed",
				zap.Int64("account_id", account.ID),
				zap.String("operation", result.OperationName),
				zap.Error(err),
			)
		}
	}

	reqLog.Info("veo.submitted",
		zap.Int64("account_id", account.ID),
		zap.String("operation", result.OperationName),
		zap.Int("status", result.StatusCode),
	)
	writeVeoPassthrough(c, result.StatusCode, result.Headers, result.Body)
}

// GeminiV1BetaGetOperation 轮询 Veo 任务状态：
// GET /v1beta/{operationName}（operationName 形如 models/veo-3.x/operations/abc 或 operations/abc）
func (h *GatewayHandler) GeminiV1BetaGetOperation(c *gin.Context) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		googleError(c, http.StatusUnauthorized, "Invalid API key")
		return
	}
	authSubject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		googleError(c, http.StatusInternalServerError, "User context not found")
		return
	}

	operationName := veoOperationNameFromParams(c)
	if operationName == "" {
		googleError(c, http.StatusBadRequest, "Missing operation name")
		return
	}

	reqLog := requestLogger(c, "handler.gemini_v1beta.operation",
		zap.Int64("user_id", authSubject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.String("operation", operationName),
	)

	// 取回提交阶段绑定的账号。
	accountID, err := h.geminiCompatService.LookupVeoOperationAccount(c.Request.Context(), apiKey.GroupID, operationName)
	if err != nil || accountID <= 0 {
		reqLog.Info("veo.operation_binding_missing", zap.Error(err))
		googleError(c, http.StatusNotFound, "Operation not found or expired")
		return
	}
	account, err := h.geminiCompatService.GetVeoAccount(c.Request.Context(), accountID)
	if err != nil || account == nil {
		reqLog.Warn("veo.operation_account_load_failed", zap.Int64("account_id", accountID), zap.Error(err))
		googleError(c, http.StatusServiceUnavailable, "Bound account unavailable")
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	result, err := h.geminiCompatService.PollVeo(c.Request.Context(), account, operationName)
	if err != nil {
		reqLog.Error("veo.poll_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		googleError(c, http.StatusBadGateway, "Veo poll failed: "+err.Error())
		return
	}

	// 完成且成功时按秒计费（防重复扣费），并把视频 file URI 重写为网关代理地址。
	// API Key 账号下客户端只持有网关 key，无法直接下载带 Google api_key 的原始 URI，
	// 因此必须改写为网关代理 URL（下载时服务端注入真实 api_key）。
	if result.Done && result.Success {
		h.maybeBillVeo(c, reqLog, apiKey, authSubject, account, operationName, result)
		result.Body = service.RewriteVeoFileURIs(result.Body, func(index int) string {
			return buildVeoProxyURL(c, operationName, index)
		})
	}

	writeVeoPassthrough(c, result.StatusCode, result.Headers, result.Body)
}

// maybeBillVeo 在 Veo 任务完成且首次轮询到完成时，按视频秒数提交一次用量记录。
func (h *GatewayHandler) maybeBillVeo(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	authSubject middleware.AuthSubject,
	account *service.Account,
	operationName string,
	poll *service.VeoPollResult,
) {
	if !h.geminiCompatService.MarkVeoBilledIfFirst(c.Request.Context(), apiKey.GroupID, operationName) {
		return // 已计费
	}

	// operationName 里通常含模型名（models/veo-3.x/operations/...）；解析失败时回退 "veo"。
	model := extractVeoModelFromOperation(operationName)
	subscription, _ := middleware.GetSubscriptionFromContext(c)

	result := &service.ForwardResult{
		Model:                model,
		MediaType:            "video",
		VideoDurationSeconds: poll.DurationSeconds,
	}

	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)

	h.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
			Result:           result,
			APIKey:           apiKey,
			User:             apiKey.User,
			Account:          account,
			Subscription:     subscription,
			InboundEndpoint:  inboundEndpoint,
			UpstreamEndpoint: upstreamEndpoint,
			UserAgent:        userAgent,
			IPAddress:        clientIP,
			QuotaPlatform:    quotaPlatform,
			APIKeyService:    h.apiKeyService,
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.gemini_v1beta.operation"),
				zap.Int64("user_id", authSubject.UserID),
				zap.Int64("account_id", account.ID),
				zap.String("operation", operationName),
			).Error("veo.record_usage_failed", zap.Error(err))
		}
	})
	reqLog.Info("veo.billed",
		zap.Int64("account_id", account.ID),
		zap.Float64("duration_seconds", poll.DurationSeconds),
	)
}

// veoOperationNameFromParams 从路由参数重建完整 operation name。
// 支持两种路由：
//   - GET /v1beta/models/*modelPath          （modelPath = "{model}/operations/{id}"）
//   - GET /v1beta/operations/*operationId    （bare operation）
func veoOperationNameFromParams(c *gin.Context) string {
	// 优先解析 /models/*modelPath catch-all（Veo LRO 轮询走这条）。
	if modelPath := strings.TrimPrefix(strings.TrimSpace(c.Param("modelPath")), "/"); modelPath != "" {
		if idx := strings.Index(modelPath, "/operations/"); idx > 0 {
			model := modelPath[:idx]
			opID := strings.TrimPrefix(modelPath[idx:], "/operations/")
			if model != "" && opID != "" {
				return "models/" + model + "/operations/" + opID
			}
		}
	}
	// bare /operations/*operationId
	if opID := strings.TrimPrefix(strings.TrimSpace(c.Param("operationId")), "/"); opID != "" {
		return "operations/" + opID
	}
	return ""
}

// extractVeoModelFromOperation 从 operation name 提取模型名。
// 形如 "models/veo-3.0-generate-001/operations/abc" -> "veo-3.0-generate-001"；
// 无法解析时回退 "veo"。
func extractVeoModelFromOperation(operationName string) string {
	parts := strings.Split(strings.TrimPrefix(operationName, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "models" && service.IsVeoModel(parts[i+1]) {
			return parts[i+1]
		}
	}
	return "veo"
}

// writeVeoPassthrough 将上游响应（含改写后的 file URI）原样写回客户端。
func writeVeoPassthrough(c *gin.Context, statusCode int, headers http.Header, body []byte) {
	for k, vv := range headers {
		if strings.EqualFold(k, "Content-Length") ||
			strings.EqualFold(k, "Transfer-Encoding") ||
			strings.EqualFold(k, "Connection") {
			continue
		}
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
		}
	}
	contentType := headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(statusCode, contentType, body)
}

// buildVeoProxyURL 为第 index 个视频样本构造网关代理下载 URL（绝对地址）。
// 只编码 operation name（base64url，因含斜杠）和样本下标，不含任何上游 URI，
// 从根本上消除客户端传入 URI 造成的 SSRF 风险。
func buildVeoProxyURL(c *gin.Context, operationName string, index int) string {
	scheme := "https"
	if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") == "" {
		scheme = "http"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	op := base64.RawURLEncoding.EncodeToString([]byte(operationName))
	return fmt.Sprintf("%s://%s/v1beta/veo-files/%s/%d", scheme, c.Request.Host, op, index)
}

// GeminiV1BetaProxyVeoFile 代理下载 Veo 生成的视频文件：
// GET /v1beta/veo-files/:opToken/:index
// opToken 是 operation name 的 base64url 编码；账号由 operation 绑定再次推导（与轮询同一信任模型）。
func (h *GatewayHandler) GeminiV1BetaProxyVeoFile(c *gin.Context) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		googleError(c, http.StatusUnauthorized, "Invalid API key")
		return
	}
	authSubject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		googleError(c, http.StatusInternalServerError, "User context not found")
		return
	}

	opBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(c.Param("opToken")))
	if err != nil || len(opBytes) == 0 {
		googleError(c, http.StatusBadRequest, "Invalid operation token")
		return
	}
	operationName := string(opBytes)
	index, err := strconv.Atoi(strings.TrimSpace(c.Param("index")))
	if err != nil || index < 0 {
		googleError(c, http.StatusBadRequest, "Invalid sample index")
		return
	}

	reqLog := requestLogger(c, "handler.gemini_v1beta.veo_file",
		zap.Int64("user_id", authSubject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.String("operation", operationName),
		zap.Int("index", index),
	)

	// 与轮询相同的信任模型：从 operation 绑定推导账号，不信任 URL 里的任何上游地址。
	accountID, err := h.geminiCompatService.LookupVeoOperationAccount(c.Request.Context(), apiKey.GroupID, operationName)
	if err != nil || accountID <= 0 {
		reqLog.Info("veo.file_operation_binding_missing", zap.Error(err))
		googleError(c, http.StatusNotFound, "Operation not found or expired")
		return
	}
	account, err := h.geminiCompatService.GetVeoAccount(c.Request.Context(), accountID)
	if err != nil || account == nil {
		reqLog.Warn("veo.file_account_load_failed", zap.Int64("account_id", accountID), zap.Error(err))
		googleError(c, http.StatusServiceUnavailable, "Bound account unavailable")
		return
	}

	// 服务端取回权威 file URI（已做 host 校验），再注入 api_key 下载。
	fileURI, err := h.geminiCompatService.ResolveVeoFileURI(c.Request.Context(), account, operationName, index)
	if err != nil {
		reqLog.Warn("veo.resolve_file_uri_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		googleError(c, http.StatusNotFound, "Veo file not available: "+err.Error())
		return
	}

	resp, err := h.geminiCompatService.ProxyVeoFile(c.Request.Context(), account, fileURI)
	if err != nil {
		reqLog.Error("veo.proxy_file_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		googleError(c, http.StatusBadGateway, "Veo file download failed: "+err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// 透传内容类型/长度并流式回传，避免大文件全量驻留内存。
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Writer.Header().Set("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Writer.Header().Set("Content-Length", cl)
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		reqLog.Warn("veo.proxy_file_stream_interrupted", zap.Error(err))
	}
}
