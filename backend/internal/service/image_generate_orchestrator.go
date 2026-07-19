package service

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// imageGenSameAccountRetryDelay mirrors the handler-layer sameAccountRetryDelay
// (handler/failover_loop.go) used between same-account retries in the image
// generation loop. Kept identical to preserve behaviour after extraction.
const imageGenSameAccountRetryDelay = 500 * time.Millisecond

// ImageGenHooks carries the caller-owned, transport-specific side effects that
// the image generation loop must invoke at the exact points it did when the
// loop lived inline in the gin handler. The gateway/handler path wires these to
// its gin.Context-bound helpers (account slot acquisition, ops markers, error
// response writing). The JWT studio path may leave most of them nil and rely on
// the typed terminal error returned by GenerateImages instead.
//
// All hooks are optional; nil hooks are treated as no-ops. AcquireAccountSlot,
// when nil, behaves as "acquire succeeds with no release" so the loop can run
// without a concurrency seam.
type ImageGenHooks struct {
	// AcquireAccountSlot acquires the per-account concurrency slot for the
	// selected account (handler path: acquireResponsesAccountSlot). sessionHash is
	// the orchestrator's current per-iteration sticky-session anchor (already run
	// through ensureImageGenPoolModeSessionHash) so the caller binds sticky
	// sessions against the exact same key the scheduler will use next iteration.
	// It returns a release func and whether acquisition succeeded. When
	// acquisition fails the hook is responsible for having written any client
	// response; the loop then returns ErrImageGenAccountSlotUnavailable so the
	// caller stops.
	AcquireAccountSlot func(selection *AccountSelectionResult, sessionHash string) (release func(), ok bool)

	// OnAccountSelected is invoked right after a non-nil account is selected
	// (handler path: setOpsSelectedAccount).
	OnAccountSelected func(account *Account)

	// MarkRoutingCapacityLimited / MarkRoutingCapacityLimitedIfNoAvailable mirror
	// the ops markers written before the "no available accounts" responses.
	MarkRoutingCapacityLimited          func()
	MarkRoutingCapacityLimitedIfNoAvail func(err error)

	// OnNoAvailableAccounts writes the "No available compatible accounts" (503)
	// client response (handler path: handleStreamingAwareError).
	OnNoAvailableAccounts func()

	// OnFailoverExhausted / OnFailoverExhaustedSimple write the terminal failover
	// responses (handler path: handleFailoverExhausted / *Simple).
	OnFailoverExhausted       func(failoverErr *UpstreamFailoverError)
	OnFailoverExhaustedSimple func(statusCode int)

	// OnForwardError writes the generic forward-failure client response for a
	// non-failover, non-upstream-user error (handler path: the trailing branch
	// using ensureForwardErrorResponse + shouldLogOpenAIForwardFailureAsWarn).
	OnForwardError func(account *Account, writerSizeBeforeForward int, err error)

	// OnUpstreamUserError handles the OpenAIImagesUpstreamError branch
	// (handler path: logs and returns; response already written by ForwardImages).
	OnUpstreamUserError func(account *Account, upstreamErr *OpenAIImagesUpstreamError)
}

// ImageGenInput carries everything the account-selection + forward + failover
// loop needs. It intentionally does not require a *gin.Context in its method
// signature: OpsContext is an optional carrier used only to thread ops/response
// writes through the (nil-tolerant) ForwardImages and ops helpers on the gateway
// handler path. The studio path passes a nil OpsContext.
type ImageGenInput struct {
	// OpsContext is the gin context for the gateway handler path (ops markers,
	// response body via ForwardImages). nil on the JWT studio path.
	OpsContext *gin.Context

	APIKey *APIKey
	Group  *Group

	// Parsed is the parsed images request; Body is the raw request body forwarded
	// upstream (parsed.Body when not separately supplied).
	Parsed *OpenAIImagesRequest
	Body   []byte

	// SessionHash is the (possibly empty) sticky-session anchor. It may be
	// rewritten per-iteration for pool-mode accounts, mirroring the handler.
	SessionHash string

	// RequestModel / ChannelMappedModel mirror the handler's requestModel and
	// channelMapping.MappedModel passed to ForwardImages.
	RequestModel       string
	ChannelMappedModel string

	RequiredCapability OpenAIImagesCapability

	// MaxAccountSwitches is the failover switch cap (handler: h.maxAccountSwitches).
	MaxAccountSwitches int

	// RoutingStart anchors the OpsRoutingLatencyMs measurement. The gateway
	// handler starts this clock before user-slot acquisition + billing eligibility
	// (matching the original inline loop). Zero value falls back to "now".
	RoutingStart time.Time

	// ReqLog is the request-scoped logger; nil falls back to the package logger.
	ReqLog *zap.Logger

	// Hooks wires caller-owned, transport-specific side effects (see ImageGenHooks).
	Hooks ImageGenHooks
}

// ImageGenResult is the successful outcome of GenerateImages. The caller is
// responsible for recording usage from Result/Account (RecordUsage is NOT done
// inside GenerateImages).
type ImageGenResult struct {
	Result      *OpenAIForwardResult
	Account     *Account
	SwitchCount int
}

// imageGenTerminalError is returned when GenerateImages stops without a
// successful result. It is purely a signal for callers (the gateway handler has
// already written the matching client response via the hooks; the studio path
// inspects Kind). It deliberately mirrors the terminal branches of the original
// inline loop.
type imageGenTerminalError struct {
	Kind        string
	FailoverErr *UpstreamFailoverError
	Err         error
}

func (e *imageGenTerminalError) Error() string {
	if e == nil {
		return "image generation failed"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "image generation failed: " + e.Kind
}

func (e *imageGenTerminalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Terminal error kinds (stable identifiers for callers that need to branch).
const (
	imageGenKindNoAvailableAccounts = "no_available_accounts"
	imageGenKindFailoverExhausted   = "failover_exhausted"
	imageGenKindAccountSlot         = "account_slot_unavailable"
	imageGenKindUpstreamUserError   = "upstream_user_error"
	imageGenKindForwardFailed       = "forward_failed"
	imageGenKindCanceled            = "canceled"
)

// ErrImageGenAccountSlotUnavailable signals the per-account concurrency slot
// could not be acquired (the hook already wrote any client response).
var ErrImageGenAccountSlotUnavailable = errors.New("image generation account slot unavailable")

func (in ImageGenInput) reqLog() *zap.Logger {
	if in.ReqLog != nil {
		return in.ReqLog
	}
	return zap.NewNop()
}

func (in ImageGenInput) forwardBody() []byte {
	if in.Body != nil {
		return in.Body
	}
	if in.Parsed != nil {
		return in.Parsed.Body
	}
	return nil
}

// GenerateImages runs the shared image-generation orchestration: account
// selection (scheduler) -> ForwardImages -> failover/switch with a max-switch
// cap -> same-account retry for pool-mode retryable errors. It does NOT acquire
// the image-generation concurrency slot, perform billing eligibility checks,
// content moderation, or record usage — those remain the caller's responsibility.
//
// This is a faithful extraction of handler/openai_images.go's inline loop. The
// caller-specific, transport-bound side effects (per-account slot acquisition,
// ops markers, error-response writing) are invoked through in.Hooks at the exact
// same points; on the gateway handler path those hooks reproduce the original
// behaviour byte-for-byte, while ForwardImages/ops writes go through the
// (nil-tolerant) in.OpsContext.
func (s *OpenAIGatewayService) GenerateImages(ctx context.Context, in ImageGenInput) (*ImageGenResult, error) {
	reqLog := in.reqLog()
	c := in.OpsContext
	apiKey := in.APIKey
	parsed := in.Parsed
	requestModel := in.RequestModel
	body := in.forwardBody()
	routingStart := in.RoutingStart
	if routingStart.IsZero() {
		routingStart = time.Now()
	}

	maxAccountSwitches := in.MaxAccountSwitches
	sessionHash := in.SessionHash
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *UpstreamFailoverError
	var oauth429FailoverState OpenAIOAuth429FailoverState

	var groupID *int64
	if apiKey != nil {
		groupID = apiKey.GroupID
	}

	for {
		reqLog.Debug("openai.images.account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, scheduleDecision, err := s.SelectAccountWithSchedulerForImages(
			ctx,
			groupID,
			sessionHash,
			requestModel,
			failedAccountIDs,
			in.RequiredCapability,
		)
		if err != nil {
			reqLog.Warn("openai.images.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if len(failedAccountIDs) == 0 {
				if in.Hooks.MarkRoutingCapacityLimitedIfNoAvail != nil {
					in.Hooks.MarkRoutingCapacityLimitedIfNoAvail(err)
				}
				if in.Hooks.OnNoAvailableAccounts != nil {
					in.Hooks.OnNoAvailableAccounts()
				}
				return nil, &imageGenTerminalError{Kind: imageGenKindNoAvailableAccounts, Err: err}
			}
			if lastFailoverErr != nil {
				if in.Hooks.OnFailoverExhausted != nil {
					in.Hooks.OnFailoverExhausted(lastFailoverErr)
				}
			} else {
				if in.Hooks.OnFailoverExhaustedSimple != nil {
					in.Hooks.OnFailoverExhaustedSimple(502)
				}
			}
			return nil, &imageGenTerminalError{Kind: imageGenKindFailoverExhausted, FailoverErr: lastFailoverErr, Err: err}
		}
		if selection == nil || selection.Account == nil {
			if in.Hooks.MarkRoutingCapacityLimited != nil {
				in.Hooks.MarkRoutingCapacityLimited()
			}
			if in.Hooks.OnNoAvailableAccounts != nil {
				in.Hooks.OnNoAvailableAccounts()
			}
			return nil, &imageGenTerminalError{Kind: imageGenKindNoAvailableAccounts}
		}

		reqLog.Debug("openai.images.account_schedule_decision",
			zap.String("layer", scheduleDecision.Layer),
			zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
			zap.Int("top_k", scheduleDecision.TopK),
			zap.Int64("latency_ms", scheduleDecision.LatencyMs),
			zap.Float64("load_skew", scheduleDecision.LoadSkew),
		)

		account := selection.Account
		sessionHash = ensureImageGenPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai.images.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		if in.Hooks.OnAccountSelected != nil {
			in.Hooks.OnAccountSelected(account)
		}

		var accountReleaseFunc func()
		if in.Hooks.AcquireAccountSlot != nil {
			var acquired bool
			accountReleaseFunc, acquired = in.Hooks.AcquireAccountSlot(selection, sessionHash)
			if !acquired {
				return nil, &imageGenTerminalError{Kind: imageGenKindAccountSlot, Err: ErrImageGenAccountSlotUnavailable}
			}
		} else {
			// No concurrency seam: honour any release the scheduler attached.
			accountReleaseFunc = selection.ReleaseFunc
		}

		SetOpsLatencyMs(c, OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		writerSizeBeforeForward := imageGenWriterSize(c)
		result, err := func() (*OpenAIForwardResult, error) {
			defer func() {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
			}()
			return s.ForwardImages(ctx, c, account, body, parsed, in.ChannelMappedModel)
		}()
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		upstreamLatencyMs, _ := imageGenContextInt64(c, OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		SetOpsLatencyMs(c, OpsResponseLatencyMsKey, responseLatencyMs)
		if result != nil && result.FirstTokenMs != nil {
			SetOpsLatencyMs(c, OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			if result != nil && result.ImageCount > 0 {
				reqLog.Warn("openai.images.forward_partial_error_with_image_result",
					zap.Int64("account_id", account.ID),
					zap.Int("image_count", result.ImageCount),
					zap.Error(err),
				)
			} else {
				var imageUpstreamErr *OpenAIImagesUpstreamError
				if errors.As(err, &imageUpstreamErr) {
					s.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), true, nil)
					reqLog.Warn("openai.images.upstream_user_error",
						zap.Int64("account_id", account.ID),
						zap.Int("status_code", imageUpstreamErr.StatusCode),
						zap.String("error_type", imageUpstreamErr.ErrorType),
						zap.String("error_code", imageUpstreamErr.Code),
						zap.Error(err),
					)
					if in.Hooks.OnUpstreamUserError != nil {
						in.Hooks.OnUpstreamUserError(account, imageUpstreamErr)
					}
					return nil, &imageGenTerminalError{Kind: imageGenKindUpstreamUserError, Err: err}
				}
				var failoverErr *UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					s.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), false, nil)
					if failoverErr.RetryableOnSameAccount {
						retryLimit := account.GetPoolModeRetryCount()
						if sameAccountRetryCount[account.ID] < retryLimit {
							sameAccountRetryCount[account.ID]++
							reqLog.Warn("openai.images.pool_mode_same_account_retry",
								zap.Int64("account_id", account.ID),
								zap.Int("upstream_status", failoverErr.StatusCode),
								zap.Int("retry_limit", retryLimit),
								zap.Int("retry_count", sameAccountRetryCount[account.ID]),
							)
							select {
							case <-ctx.Done():
								return nil, &imageGenTerminalError{Kind: imageGenKindCanceled, Err: ctx.Err()}
							case <-time.After(imageGenSameAccountRetryDelay):
							}
							continue
						}
					}
					s.RecordOpenAIAccountSwitch()
					failedAccountIDs[account.ID] = struct{}{}
					lastFailoverErr = failoverErr
					if switchCount >= maxAccountSwitches {
						if in.Hooks.OnFailoverExhausted != nil {
							in.Hooks.OnFailoverExhausted(failoverErr)
						}
						return nil, &imageGenTerminalError{Kind: imageGenKindFailoverExhausted, FailoverErr: failoverErr, Err: err}
					}
					switchCount++
					if s.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount, &oauth429FailoverState) {
						if in.Hooks.OnFailoverExhausted != nil {
							in.Hooks.OnFailoverExhausted(failoverErr)
						}
						return nil, &imageGenTerminalError{Kind: imageGenKindFailoverExhausted, FailoverErr: failoverErr, Err: err}
					}
					reqLog.Warn("openai.images.upstream_failover_switching",
						zap.Int64("account_id", account.ID),
						zap.Int("upstream_status", failoverErr.StatusCode),
						zap.Int("switch_count", switchCount),
						zap.Int("max_switches", maxAccountSwitches),
					)
					continue
				}
				s.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), false, nil)
				if in.Hooks.OnForwardError != nil {
					in.Hooks.OnForwardError(account, writerSizeBeforeForward, err)
				} else {
					logger.L().With(
						zap.String("component", "service.openai_gateway.images"),
						zap.Int64("account_id", account.ID),
					).Warn("openai.images.forward_failed", zap.Error(err))
				}
				return nil, &imageGenTerminalError{Kind: imageGenKindForwardFailed, Err: err}
			}
		}
		if result != nil {
			if account.Type == AccountTypeOAuth {
				s.UpdateCodexUsageSnapshotFromHeaders(imageGenRequestContext(c, ctx), account.ID, result.ResponseHeaders)
			}
			s.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), true, result.FirstTokenMs)
		} else {
			s.ReportOpenAIAccountScheduleResult(account.ID, account.GetMappedModel(requestModel), true, nil)
		}

		reqLog.Debug("openai.images.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return &ImageGenResult{
			Result:      result,
			Account:     account,
			SwitchCount: switchCount,
		}, nil
	}
}

// ensureImageGenPoolModeSessionHash mirrors handler.ensureOpenAIPoolModeSessionHash.
func ensureImageGenPoolModeSessionHash(sessionHash string, account *Account) string {
	if sessionHash != "" || account == nil || !account.IsPoolMode() {
		return sessionHash
	}
	return "openai-pool-retry-" + uuid.NewString()
}

// imageGenWriterSize mirrors the handler's c.Writer.Size() probe used for
// openAIForwardErrorAlreadyCommunicated, returning 0 when there is no context.
func imageGenWriterSize(c *gin.Context) int {
	if c == nil || c.Writer == nil {
		return 0
	}
	return c.Writer.Size()
}

// imageGenContextInt64 mirrors handler.getContextInt64 for the OpsContext.
func imageGenContextInt64(c *gin.Context, key string) (int64, bool) {
	if c == nil || key == "" {
		return 0, false
	}
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

// imageGenRequestContext returns c.Request.Context() when available (matching the
// handler's UpdateCodexUsageSnapshotFromHeaders call site) else the provided ctx.
func imageGenRequestContext(c *gin.Context, fallback context.Context) context.Context {
	if c != nil && c.Request != nil {
		return c.Request.Context()
	}
	return fallback
}
