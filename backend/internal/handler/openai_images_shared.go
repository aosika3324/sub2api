package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) handleParsedOpenAIImages(
	c *gin.Context,
	streamStarted *bool,
	requestStart time.Time,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	reqLog *zap.Logger,
	body []byte,
	parsed *service.OpenAIImagesRequest,
	logComponent string,
	chatCompletionsEnvelope bool,
) {
	if parsed == nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "parsed images request is required")
		return
	}
	if streamStarted == nil {
		local := false
		streamStarted = &local
	}

	requestModel := parsed.Model
	reqLog = reqLog.With(
		zap.String("model", requestModel),
		zap.Bool("stream", parsed.Stream),
		zap.Bool("multipart", parsed.Multipart),
		zap.String("capability", string(parsed.RequiredCapability)),
	)

	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, parsed.ModerationBody()); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}
	imageReleaseFunc, acquired := h.acquireImageGenerationSlot(c, *streamStarted)
	if !acquired {
		return
	}
	if imageReleaseFunc != nil {
		defer imageReleaseFunc()
	}

	setOpsRequestContext(c, requestModel, parsed.Stream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(parsed.Stream, false)))

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, requestModel)

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, parsed.Stream, streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai.images.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, *streamStarted)
		return
	}

	forwardBody := body
	if len(parsed.Body) > 0 {
		forwardBody = parsed.Body
	}
	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	requestCtx := service.WithOpenAIImageGenerationIntent(c.Request.Context())
	originalWriter := c.Writer
	var captureWriter *openAIImagesResponseCapture
	var chatStreamWriter *openAIChatImagesStreamWriter
	if chatCompletionsEnvelope {
		if parsed.Stream {
			chatStreamWriter = newOpenAIChatImagesStreamWriter(originalWriter, requestModel)
			c.Writer = chatStreamWriter
		} else {
			captureWriter = &openAIImagesResponseCapture{
				ResponseWriter: originalWriter,
				header:         make(http.Header),
			}
			c.Writer = captureWriter
		}
	}

	genResult, genErr := h.gatewayService.GenerateImages(requestCtx, service.ImageGenInput{
		OpsContext:         c,
		APIKey:             apiKey,
		Group:              apiKey.Group,
		Parsed:             parsed,
		Body:               forwardBody,
		SessionHash:        sessionHash,
		RequestModel:       requestModel,
		ChannelMappedModel: channelMapping.MappedModel,
		RequiredCapability: parsed.RequiredCapability,
		MaxAccountSwitches: h.maxAccountSwitches,
		RoutingStart:       routingStart,
		ReqLog:             reqLog,
		Hooks: service.ImageGenHooks{
			AcquireAccountSlot: func(selection *service.AccountSelectionResult, sessionHashForSlot string) (func(), bool) {
				return h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHashForSlot, selection, parsed.Stream, streamStarted, reqLog)
			},
			OnAccountSelected: func(account *service.Account) {
				setOpsSelectedAccount(c, account.ID, account.Platform)
			},
			MarkRoutingCapacityLimited: func() {
				markOpsRoutingCapacityLimited(c)
			},
			MarkRoutingCapacityLimitedIfNoAvail: func(err error) {
				markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
			},
			OnNoAvailableAccounts: func() {
				h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available compatible accounts", *streamStarted)
			},
			OnFailoverExhausted: func(failoverErr *service.UpstreamFailoverError) {
				h.handleFailoverExhausted(c, failoverErr, *streamStarted)
			},
			OnFailoverExhaustedSimple: func(statusCode int) {
				h.handleFailoverExhaustedSimple(c, statusCode, *streamStarted)
			},
			OnForwardError: func(account *service.Account, writerSizeBeforeForward int, err error) {
				upstreamErrorAlreadyCommunicated := openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
				wroteFallback := false
				if !upstreamErrorAlreadyCommunicated {
					wroteFallback = h.ensureForwardErrorResponse(c, *streamStarted)
				}
				fields := []zap.Field{
					zap.Int64("account_id", account.ID),
					zap.Bool("fallback_error_response_written", wroteFallback),
					zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
					zap.Error(err),
				}
				if shouldLogOpenAIForwardFailureAsWarn(c, wroteFallback) {
					reqLog.Warn("openai.images.forward_failed", fields...)
					return
				}
				reqLog.Error("openai.images.forward_failed", fields...)
			},
		},
	})
	if genErr != nil {
		if captureWriter != nil {
			c.Writer = originalWriter
			captureWriter.flushTo(originalWriter)
		}
		if chatStreamWriter != nil {
			c.Writer = originalWriter
			chatStreamWriter.finish()
		}
		return
	}
	if captureWriter != nil {
		c.Writer = originalWriter
		h.writeOpenAIChatImagesCompletionEnvelope(c, parsed, captureWriter.body.Bytes())
	}
	if chatStreamWriter != nil {
		c.Writer = originalWriter
		chatStreamWriter.finish()
	}

	result := genResult.Result
	account := genResult.Account

	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	requestPayloadHash := service.HashUsageRequestPayload(body)
	if parsed.Multipart {
		requestPayloadHash = service.HashUsageRequestPayload([]byte(parsed.StickySessionSeed()))
	}
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := parsed.Endpoint
	if upstreamEndpoint == "" {
		upstreamEndpoint = GetUpstreamEndpoint(c, account.Platform)
	}

	upstreamModel := ""
	if result != nil {
		upstreamModel = result.UpstreamModel
	}
	h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: requestPayloadHash,
			APIKeyService:      h.apiKeyService,
			ChannelUsageFields: channelMapping.ToUsageFields(requestModel, upstreamModel),
		}); err != nil {
			logger.L().With(
				zap.String("component", logComponent),
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Any("group_id", apiKey.GroupID),
				zap.String("model", requestModel),
				zap.Int64("account_id", account.ID),
			).Error("openai.images.record_usage_failed", zap.Error(err))
		}
	})
}

type openAIImagesResponseCapture struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
	header http.Header
}

func (w *openAIImagesResponseCapture) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *openAIImagesResponseCapture) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *openAIImagesResponseCapture) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

func (w *openAIImagesResponseCapture) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *openAIImagesResponseCapture) WriteString(s string) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.WriteString(s)
}

func (w *openAIImagesResponseCapture) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *openAIImagesResponseCapture) Size() int {
	return w.body.Len()
}

func (w *openAIImagesResponseCapture) Written() bool {
	return w.status != 0 || w.body.Len() > 0
}

func (w *openAIImagesResponseCapture) Flush() {}

func (w *openAIImagesResponseCapture) flushTo(dst gin.ResponseWriter) {
	if dst == nil {
		return
	}
	status := w.Status()
	dst.WriteHeader(status)
	if w.body.Len() > 0 {
		_, _ = dst.Write(w.body.Bytes())
	}
}

func (h *OpenAIGatewayHandler) writeOpenAIChatImagesCompletionEnvelope(c *gin.Context, parsed *service.OpenAIImagesRequest, imagesBody []byte) {
	created := gjson.GetBytes(imagesBody, "created").Int()
	if created <= 0 {
		created = time.Now().Unix()
	}
	model := ""
	if parsed != nil {
		model = parsed.Model
	}
	content := openAIChatImagesMarkdownContent(imagesBody)
	payload := map[string]any{
		"id":      "chatcmpl-" + uuid.NewString(),
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": "stop",
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to build chat image response")
		return
	}
	c.Header("Content-Type", "application/json")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(body)
}

func openAIChatImagesMarkdownContent(imagesBody []byte) string {
	data := gjson.GetBytes(imagesBody, "data")
	if !data.IsArray() {
		return "Image generation completed."
	}
	var parts []string
	for index, item := range data.Array() {
		if b64 := item.Get("b64_json").String(); strings.TrimSpace(b64) != "" {
			parts = append(parts, "![image_"+strconv.Itoa(index+1)+"]("+openAIChatImagesMarkdownURL(item, b64)+")")
			continue
		}
		if url := strings.TrimSpace(item.Get("url").String()); url != "" {
			parts = append(parts, "![image_"+strconv.Itoa(index+1)+"]("+url+")")
		}
	}
	if len(parts) == 0 {
		return "Image generation completed."
	}
	return strings.Join(parts, "\n\n")
}

type openAIChatImagesStreamWriter struct {
	gin.ResponseWriter
	model        string
	id           string
	created      int64
	lineBuffer   string
	eventName    string
	dataLines    []string
	sentRole     bool
	finished     bool
	errorWritten bool
	passthrough  bool
	firstErr     error
}

func newOpenAIChatImagesStreamWriter(dst gin.ResponseWriter, model string) *openAIChatImagesStreamWriter {
	return &openAIChatImagesStreamWriter{
		ResponseWriter: dst,
		model:          strings.TrimSpace(model),
		id:             "chatcmpl-" + uuid.NewString(),
		created:        time.Now().Unix(),
	}
}

func (w *openAIChatImagesStreamWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *openAIChatImagesStreamWriter) Write(data []byte) (int, error) {
	w.processStreamBytes(data)
	return len(data), w.firstErr
}

func (w *openAIChatImagesStreamWriter) WriteString(s string) (int, error) {
	w.processStreamBytes([]byte(s))
	return len(s), w.firstErr
}

func (w *openAIChatImagesStreamWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *openAIChatImagesStreamWriter) processStreamBytes(data []byte) {
	if w == nil || w.finished || len(data) == 0 {
		return
	}
	if w.passthrough {
		if _, err := w.ResponseWriter.Write(data); err != nil {
			w.setFirstErr(err)
		}
		w.Flush()
		return
	}
	if w.lineBuffer == "" && len(w.dataLines) == 0 && w.eventName == "" && !openAIChatImagesLooksLikeSSELine(string(data)) {
		if w.processNonSSEPayload(data) {
			return
		}
		if openAIChatImagesLooksLikeJSONStart(data) && !gjson.ValidBytes(data) {
			w.lineBuffer += string(data)
			return
		}
		w.passthrough = true
		if _, err := w.ResponseWriter.Write(data); err != nil {
			w.setFirstErr(err)
		}
		w.Flush()
		return
	}
	w.lineBuffer += string(data)
	for {
		idx := strings.IndexByte(w.lineBuffer, '\n')
		if idx < 0 {
			return
		}
		line := w.lineBuffer[:idx+1]
		w.lineBuffer = w.lineBuffer[idx+1:]
		w.processSSELine(strings.TrimRight(line, "\r\n"))
	}
}

func (w *openAIChatImagesStreamWriter) processNonSSEPayload(data []byte) bool {
	if w == nil || len(data) == 0 || !gjson.ValidBytes(data) {
		return false
	}
	if gjson.GetBytes(data, "error").Exists() {
		return false
	}
	if content := openAIChatImagesStreamContent(data); content != "" {
		w.writeChunk(map[string]any{"content": content}, nil)
		return true
	}
	return false
}

func (w *openAIChatImagesStreamWriter) processSSELine(line string) {
	if strings.TrimSpace(line) == "" {
		w.flushSSEEvent()
		return
	}
	if strings.HasPrefix(line, ":") {
		if _, err := io.WriteString(w.ResponseWriter, line+"\n\n"); err != nil {
			w.setFirstErr(err)
		}
		w.Flush()
		return
	}
	if strings.HasPrefix(line, "event:") {
		w.eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		return
	}
	if strings.HasPrefix(line, "data:") {
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			w.finish()
			return
		}
		w.dataLines = append(w.dataLines, data)
	}
}

func (w *openAIChatImagesStreamWriter) flushSSEEvent() {
	if w == nil || w.finished || len(w.dataLines) == 0 {
		w.eventName = ""
		w.dataLines = nil
		return
	}
	eventName := w.eventName
	dataLines := append([]string(nil), w.dataLines...)
	w.eventName = ""
	w.dataLines = nil

	joined := []byte(strings.Join(dataLines, "\n"))
	if gjson.ValidBytes(joined) {
		w.flushSSEPayload(eventName, joined)
		return
	}
	for _, line := range dataLines {
		if w.finished {
			return
		}
		w.flushSSEPayload(eventName, []byte(line))
	}
}

func (w *openAIChatImagesStreamWriter) flushSSEPayload(eventName string, data []byte) {
	if w == nil || w.finished || len(bytes.TrimSpace(data)) == 0 {
		return
	}
	if eventName == "error" || strings.EqualFold(gjson.GetBytes(data, "type").String(), "error") {
		w.writeErrorEvent(data)
		return
	}
	if content := openAIChatImagesStreamContent(data); content != "" {
		w.writeChunk(map[string]any{"content": content}, nil)
	}
}

func (w *openAIChatImagesStreamWriter) ensureStreamHeaders() {
	if w == nil || w.ResponseWriter == nil {
		return
	}
	header := w.ResponseWriter.Header()
	if strings.TrimSpace(header.Get("Content-Type")) == "" || strings.HasPrefix(strings.ToLower(header.Get("Content-Type")), "application/json") {
		header.Set("Content-Type", "text/event-stream")
	}
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
}

func openAIChatImagesStreamContent(data []byte) string {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return ""
	}
	if content := openAIChatImagesMarkdownContent(data); content != "Image generation completed." {
		return content
	}
	var parts []string
	item := gjson.ParseBytes(data)
	if b64 := strings.TrimSpace(item.Get("b64_json").String()); b64 != "" {
		parts = append(parts, "![image_1]("+openAIChatImagesMarkdownURL(item, b64)+")")
	} else if url := strings.TrimSpace(item.Get("url").String()); url != "" {
		parts = append(parts, "![image_1]("+url+")")
	}
	if len(parts) == 0 {
		if msg := strings.TrimSpace(item.Get("message").String()); msg != "" {
			parts = append(parts, msg)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (w *openAIChatImagesStreamWriter) writeErrorEvent(data []byte) {
	if w == nil || w.finished {
		return
	}
	w.errorWritten = true
	data = normalizeOpenAIChatImagesStreamErrorBody(data)
	w.ensureStreamHeaders()
	if _, err := fmt.Fprintf(w.ResponseWriter, "event: error\ndata: %s\n\n", data); err == nil {
		w.Flush()
	} else {
		w.setFirstErr(err)
	}
	w.finished = true
}

func (w *openAIChatImagesStreamWriter) writeChunk(delta map[string]any, finishReason *string) {
	if w == nil || w.finished {
		return
	}
	if !w.sentRole {
		w.sentRole = true
		if delta == nil {
			delta = map[string]any{}
		}
		if _, ok := delta["role"]; !ok {
			delta["role"] = "assistant"
		}
	}
	payload := map[string]any{
		"id":      w.id,
		"object":  "chat.completion.chunk",
		"created": w.created,
		"model":   w.model,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         delta,
			"finish_reason": finishReason,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		w.writeErrorEvent(buildOpenAIChatImagesStreamErrorBody(err.Error()))
		return
	}
	w.ensureStreamHeaders()
	if _, err := fmt.Fprintf(w.ResponseWriter, "data: %s\n\n", body); err == nil {
		w.Flush()
	} else {
		w.setFirstErr(err)
	}
}

func (w *openAIChatImagesStreamWriter) finish() {
	if w == nil || w.finished {
		return
	}
	if w.passthrough {
		w.Flush()
		w.finished = true
		return
	}
	if strings.TrimSpace(w.lineBuffer) != "" {
		if !openAIChatImagesLooksLikeSSELine(w.lineBuffer) {
			if w.processNonSSEPayload([]byte(w.lineBuffer)) {
				w.lineBuffer = ""
			} else {
				if _, err := io.WriteString(w.ResponseWriter, w.lineBuffer); err != nil {
					w.setFirstErr(err)
				}
				w.Flush()
				w.lineBuffer = ""
				w.finished = true
				return
			}
		} else {
			w.processSSELine(strings.TrimRight(w.lineBuffer, "\r\n"))
			w.lineBuffer = ""
		}
	}
	w.flushSSEEvent()
	if w.errorWritten || w.finished {
		return
	}
	if !w.sentRole {
		w.writeChunk(map[string]any{"content": ""}, nil)
	}
	stop := "stop"
	w.writeChunk(map[string]any{}, &stop)
	w.ensureStreamHeaders()
	if _, err := io.WriteString(w.ResponseWriter, "data: [DONE]\n\n"); err != nil {
		w.setFirstErr(err)
	} else {
		w.Flush()
	}
	w.finished = true
}

func (w *openAIChatImagesStreamWriter) setFirstErr(err error) {
	if w != nil && err != nil && w.firstErr == nil {
		w.firstErr = err
	}
}

func openAIChatImagesLooksLikeSSELine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" ||
		strings.HasPrefix(trimmed, ":") ||
		strings.HasPrefix(trimmed, "event:") ||
		strings.HasPrefix(trimmed, "data:")
}

func openAIChatImagesLooksLikeJSONStart(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	return trimmed[0] == '{' || trimmed[0] == '['
}

func openAIChatImagesMarkdownURL(item gjson.Result, b64 string) string {
	if url := strings.TrimSpace(item.Get("url").String()); url != "" {
		return url
	}
	return "data:" + openAIChatImagesMIMEType(item) + ";base64," + b64
}

func openAIChatImagesMIMEType(item gjson.Result) string {
	format := strings.ToLower(strings.TrimSpace(item.Get("output_format").String()))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(item.Get("format").String()))
	}
	format = strings.TrimPrefix(format, "image/")
	switch format {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "png", "":
		return "image/png"
	default:
		return "image/" + format
	}
}

func buildOpenAIChatImagesStreamErrorBody(message string) []byte {
	if strings.TrimSpace(message) == "" {
		message = "upstream request failed"
	}
	body := []byte(`{"error":{"type":"upstream_error","message":""}}`)
	body, _ = sjson.SetBytes(body, "error.message", message)
	return body
}

func normalizeOpenAIChatImagesStreamErrorBody(data []byte) []byte {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return buildOpenAIChatImagesStreamErrorBody("")
	}
	errObj := gjson.GetBytes(data, "error")
	if !errObj.Exists() {
		message := strings.TrimSpace(gjson.GetBytes(data, "message").String())
		return buildOpenAIChatImagesStreamErrorBody(message)
	}
	body := []byte(`{"error":{}}`)
	if errObj.IsObject() {
		body, _ = sjson.SetRawBytes(body, "error", []byte(errObj.Raw))
		return body
	}
	message := strings.TrimSpace(errObj.String())
	if message == "" {
		message = "upstream request failed"
	}
	body, _ = sjson.SetBytes(body, "error.message", message)
	body, _ = sjson.SetBytes(body, "error.type", "upstream_error")
	return body
}
