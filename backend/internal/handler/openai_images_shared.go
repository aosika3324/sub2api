package handler

import (
	"bytes"
	"context"
	"encoding/json"
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
	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, parsed.ModerationBody()); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
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
	if chatCompletionsEnvelope && !parsed.Stream {
		captureWriter = &openAIImagesResponseCapture{
			ResponseWriter: originalWriter,
			header:         make(http.Header),
		}
		c.Writer = captureWriter
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
		return
	}
	if captureWriter != nil {
		c.Writer = originalWriter
		h.writeOpenAIChatImagesCompletionEnvelope(c, parsed, captureWriter.body.Bytes())
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
			parts = append(parts, "![image_"+strconv.Itoa(index+1)+"](data:image/png;base64,"+b64+")")
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
