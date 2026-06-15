package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Video Studio: in-app (JWT) Veo video generation.
//
// Unlike the image studio (synchronous), Veo is an async long task. This service
// drives the lifecycle on behalf of a logged-in user, mirroring the image
// studio's JWT->gateway bridge (a hidden synthetic internal API key models the
// (user, group) billing identity), but persisting each job as a video_generations
// row tracked through *read-time* reconciliation rather than a background worker:
//
//	StartGenerate: preflight -> create pending row -> select account -> SubmitVeo
//	              -> bind operation_name + account_id on the row (status=processing)
//	Reconcile (on status read): poll the bound account; on done+success bill once
//	              via an atomic transition + RecordUsage; on done+error mark failed.
//	StreamVideo:  resolve the authoritative file URI server-side, proxy-stream it.
//
// Videos are NOT stored locally (MVP): the produced file is proxy-streamed on
// demand from the upstream signed URI, which expires — the UI surfaces a
// retention notice. Persisting to object storage is a deliberate V1 follow-up.
// ---------------------------------------------------------------------------

const (
	// videoStudioInboundEndpoint is the usage_log inbound endpoint tag for in-app
	// video generations (parallels imageStudioInboundEndpoint).
	videoStudioInboundEndpoint = "video_studio"

	// VideoStudioRetention is the server-side retention window for video generation
	// rows. The upstream signed file URI typically expires well before this; the
	// row is kept so the user still sees the job in history.
	VideoStudioRetention = 48 * time.Hour

	// videoStudioMaxActivePerUser bounds a single user's concurrent in-flight
	// (pending+processing) generations. Veo is expensive and slow, so this guards
	// against a user fanning out many costly jobs at once.
	videoStudioMaxActivePerUser = 3

	// videoStudioStalePendingTimeout is how long a row may stay pending/processing
	// before the sweep marks it failed. Veo jobs normally finish within minutes;
	// this reclaims rows orphaned by a crash or an operation that never completes.
	videoStudioStalePendingTimeout = 30 * time.Minute

	// videoStudioSweepInterval is the background reclaim cadence.
	videoStudioSweepInterval = 10 * time.Minute
	// videoStudioSweepTimeout bounds a single sweep pass.
	videoStudioSweepTimeout = 2 * time.Minute
	// videoStudioSweepBatchSize bounds rows touched per sweep batch.
	videoStudioSweepBatchSize = 200

	// videoStudioMaxPromptLen caps the accepted prompt length (defensive; the
	// upstream has its own limit).
	videoStudioMaxPromptLen = 5000

	// Error codes (stable, machine-readable; the frontend maps them to localized
	// messages). Raw detail stays in the row's `error` column for diagnostics.
	VideoStudioErrCodeNoAccount   = "no_account"
	VideoStudioErrCodeSubmit      = "submit_failed"
	VideoStudioErrCodeUpstream    = "upstream_error"
	VideoStudioErrCodeInterrupted = "interrupted"
)

// Error sentinels (parallel to the image studio's).
var (
	// ErrVideoStudioGroupNotAllowed is returned when the requested group is not in
	// the user's available groups.
	ErrVideoStudioGroupNotAllowed = errors.New("video studio: group not allowed for user")
	// ErrVideoStudioDisabled is returned when the group has no Veo per-second price
	// configured. Per the product decision, a configured price is what both enables
	// the feature and gates entry visibility.
	ErrVideoStudioDisabled = errors.New("video studio: video generation is not enabled for this group")
	// ErrVideoStudioBusy is returned when the user already has the max number of
	// in-flight generations.
	ErrVideoStudioBusy = errors.New("video studio: too many in-flight generations, please wait")
	// ErrVideoStudioNoAccount is returned when no usable api_key Gemini account is
	// available for the group.
	ErrVideoStudioNoAccount = errors.New("video studio: no available video account")
	// ErrVideoStudioSubmitFailed is returned when the upstream submit is rejected.
	ErrVideoStudioSubmitFailed = errors.New("video studio: upstream rejected the generation request")
	// ErrVideoStudioNotFound is returned when a generation row does not exist or is
	// not owned by the acting user.
	ErrVideoStudioNotFound = errors.New("video studio: generation not found")
	// ErrVideoStudioNotReady is returned when the video is requested before the
	// generation has succeeded.
	ErrVideoStudioNotReady = errors.New("video studio: video not ready")
	// ErrVideoStudioInvalidPrompt is returned for an empty/oversized prompt.
	ErrVideoStudioInvalidPrompt = errors.New("video studio: invalid prompt")
)

// ---------------------------------------------------------------------------
// Narrow consumer interfaces (Go idiom: accept interfaces for testability). The
// concrete *UserService / *APIKeyService / *BillingCacheService /
// *SubscriptionService / *GeminiMessagesCompatService / *GatewayService satisfy
// these.
// ---------------------------------------------------------------------------

// videoUserReader loads the acting user (and is re-read after billing to surface
// the post-deduction balance). Satisfied by *UserService.
type videoUserReader interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

// videoGroupProvider returns the groups a user is allowed to use. Satisfied by
// *APIKeyService.GetAvailableGroups.
type videoGroupProvider interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error)
}

// videoKeyEnsurer lazily provisions the hidden synthetic studio API key for
// (userID, groupID). Satisfied by *APIKeyService.EnsureStudioAPIKey (shared with
// the image studio — the same internal key models the in-app billing identity).
type videoKeyEnsurer interface {
	EnsureStudioAPIKey(ctx context.Context, userID, groupID int64) (*APIKey, error)
}

// videoEligibilityChecker is the pre-flight balance/quota gate. Satisfied by
// *BillingCacheService.CheckBillingEligibility.
type videoEligibilityChecker interface {
	CheckBillingEligibility(ctx context.Context, user *User, apiKey *APIKey, group *Group, subscription *UserSubscription, platform string) error
}

// videoSubscriptionResolver loads the active subscription for a (user, group)
// pair. Satisfied by *SubscriptionService.
type videoSubscriptionResolver interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}

// veoClient is the upstream Veo port: account selection, submit, poll, and file
// proxying. Satisfied by *GeminiMessagesCompatService.
type veoClient interface {
	SelectAccountForAIStudioEndpoints(ctx context.Context, groupID *int64) (*Account, error)
	GetVeoAccount(ctx context.Context, accountID int64) (*Account, error)
	SubmitVeo(ctx context.Context, account *Account, model string, body []byte) (*VeoSubmitResult, error)
	PollVeo(ctx context.Context, account *Account, operationName string) (*VeoPollResult, error)
	ResolveVeoFileURI(ctx context.Context, account *Account, operationName string, index int) (string, error)
	ProxyVeoFile(ctx context.Context, account *Account, fileURI string) (*http.Response, error)
}

// veoCostResolver prices a completed Veo generation the same way RecordUsage
// charges, so the persisted/displayed cost matches the deduction. Satisfied by
// *GatewayService.ComputeVeoVideoCostBreakdown.
type veoCostResolver interface {
	ComputeVeoVideoCostBreakdown(ctx context.Context, user *User, apiKey *APIKey, durationSeconds float64) *CostBreakdown
}

// veoUsageRecorder records usage + atomically deducts balance + writes usage_log.
// Satisfied by *GatewayService.RecordUsage.
type veoUsageRecorder interface {
	RecordUsage(ctx context.Context, input *RecordUsageInput) error
}

// Compile-time assertions that the concrete services satisfy the ports.
var (
	_ videoUserReader           = (*UserService)(nil)
	_ videoGroupProvider        = (*APIKeyService)(nil)
	_ videoKeyEnsurer           = (*APIKeyService)(nil)
	_ videoEligibilityChecker   = (*BillingCacheService)(nil)
	_ videoSubscriptionResolver = (*SubscriptionService)(nil)
	_ veoClient                 = (*GeminiMessagesCompatService)(nil)
	_ veoCostResolver           = (*GatewayService)(nil)
	_ veoUsageRecorder          = (*GatewayService)(nil)
)

// ---------------------------------------------------------------------------
// Public input/output types.
// ---------------------------------------------------------------------------

// VideoStudioGenerateInput is an in-app video generation request from a
// logged-in user (JWT path), already resolved to a chosen group.
type VideoStudioGenerateInput struct {
	GroupID int64
	Prompt  string
	Model   string

	// Optional request metadata, recorded on the usage log when available.
	UserAgent string
	IPAddress string
}

// VideoStudioStartResult is returned after a job is accepted and submitted. The
// caller tracks progress through GetGeneration/ListGenerations (read-time poll).
type VideoStudioStartResult struct {
	GenerationID int64
	Status       string
	Model        string
	Balance      float64
	// EstimatedCost is a best-effort pre-charge estimate (price/sec * default
	// duration); the authoritative cost is computed + persisted on completion.
	EstimatedCost float64
}

// ---------------------------------------------------------------------------
// Service.
// ---------------------------------------------------------------------------

// VideoStudioServiceDeps groups the (narrow) dependencies of VideoStudioService.
type VideoStudioServiceDeps struct {
	Users         videoUserReader
	Groups        videoGroupProvider
	KeyEnsurer    videoKeyEnsurer
	Eligibility   videoEligibilityChecker
	Subscriptions videoSubscriptionResolver
	Veo           veoClient
	CostResolver  veoCostResolver
	UsageRecord   veoUsageRecorder
	Repo          VideoStudioRepository
}

// VideoStudioService orchestrates in-app (JWT) Veo video generation: group
// validation, billing pre-flight, concurrency guard, row persistence, upstream
// submit, read-time reconciliation (poll + bill once on completion), and
// on-demand proxy streaming of the produced video.
type VideoStudioService struct {
	users         videoUserReader
	groups        videoGroupProvider
	keyEnsurer    videoKeyEnsurer
	eligibility   videoEligibilityChecker
	subscriptions videoSubscriptionResolver
	veo           veoClient
	costResolver  veoCostResolver
	usageRecord   veoUsageRecorder
	repo          VideoStudioRepository
}

// NewVideoStudioService builds the service and starts its retention/stale sweep.
func NewVideoStudioService(deps VideoStudioServiceDeps) *VideoStudioService {
	svc := &VideoStudioService{
		users:         deps.Users,
		groups:        deps.Groups,
		keyEnsurer:    deps.KeyEnsurer,
		eligibility:   deps.Eligibility,
		subscriptions: deps.Subscriptions,
		veo:           deps.Veo,
		costResolver:  deps.CostResolver,
		usageRecord:   deps.UsageRecord,
		repo:          deps.Repo,
	}
	svc.startSweeper()
	return svc
}

// ---------------------------------------------------------------------------
// StartGenerate.
// ---------------------------------------------------------------------------

// StartGenerate validates the request, creates a pending row, submits the Veo
// job upstream, and binds the returned operation to the row (status=processing).
// It returns quickly; the produced video is tracked via read-time reconciliation.
func (s *VideoStudioService) StartGenerate(ctx context.Context, userID int64, in VideoStudioGenerateInput) (*VideoStudioStartResult, error) {
	reqLog := logger.L().With(
		zap.String("component", "service.video_studio"),
		zap.Int64("user_id", userID),
		zap.Int64("group_id", in.GroupID),
	)

	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" || len([]rune(prompt)) > videoStudioMaxPromptLen {
		return nil, ErrVideoStudioInvalidPrompt
	}
	in.Prompt = prompt

	model := normalizeVeoModel(in.Model)
	in.Model = model

	// --- Step 1: load user, validate group membership + Veo enabled. ---
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("video studio: load user: %w", err)
	}
	group, err := s.resolveAllowedGroup(ctx, userID, in.GroupID)
	if err != nil {
		return nil, err
	}
	if !GroupAllowsVeoVideo(group) {
		return nil, ErrVideoStudioDisabled
	}

	// --- Step 2: per-user concurrency guard. ---
	active, err := s.repo.CountActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("video studio: count active: %w", err)
	}
	if active >= videoStudioMaxActivePerUser {
		return nil, ErrVideoStudioBusy
	}

	// --- Step 3: ensure synthetic key + hydrate, resolve subscription, eligibility. ---
	apiKey, err := s.keyEnsurer.EnsureStudioAPIKey(ctx, userID, in.GroupID)
	if err != nil {
		return nil, fmt.Errorf("video studio: ensure studio api key: %w", err)
	}
	apiKey.User = user
	apiKey.Group = group
	gid := group.ID
	apiKey.GroupID = &gid

	subscription, err := s.resolveActiveSubscription(ctx, userID, group)
	if err != nil {
		reqLog.Info("video_studio.subscription_required_not_found", zap.Error(err))
		return nil, err
	}
	if err := s.eligibility.CheckBillingEligibility(ctx, user, apiKey, group, subscription, group.Platform); err != nil {
		reqLog.Info("video_studio.billing_eligibility_failed", zap.Error(err))
		return nil, err
	}

	// --- Step 4: create the pending row. ---
	gen, err := s.repo.CreateGeneration(ctx, &dbent.VideoGeneration{
		UserID:  userID,
		GroupID: group.ID,
		Prompt:  in.Prompt,
		Model:   in.Model,
		Status:  VideoStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("video studio: create generation: %w", err)
	}
	genID := gen.ID

	// --- Step 5: select an api_key Gemini account + submit upstream. ---
	account, err := s.veo.SelectAccountForAIStudioEndpoints(ctx, &gid)
	if err != nil || account == nil {
		s.failRow(ctx, genID, "no available video account", VideoStudioErrCodeNoAccount, reqLog)
		return nil, ErrVideoStudioNoAccount
	}
	if account.Type != AccountTypeAPIKey {
		s.failRow(ctx, genID, "selected account is not api_key type", VideoStudioErrCodeNoAccount, reqLog)
		return nil, ErrVideoStudioNoAccount
	}

	body, err := buildVeoSubmitBody(in.Prompt)
	if err != nil {
		s.failRow(ctx, genID, "build submit body: "+err.Error(), VideoStudioErrCodeSubmit, reqLog)
		return nil, ErrVideoStudioSubmitFailed
	}

	submit, err := s.veo.SubmitVeo(ctx, account, in.Model, body)
	if err != nil {
		reqLog.Error("video_studio.submit_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		s.failRow(ctx, genID, "submit: "+err.Error(), VideoStudioErrCodeSubmit, reqLog)
		return nil, ErrVideoStudioSubmitFailed
	}
	if submit.StatusCode >= 400 || strings.TrimSpace(submit.OperationName) == "" {
		detail := veoUpstreamErrorDetail(submit.Body, submit.StatusCode)
		reqLog.Warn("video_studio.submit_rejected",
			zap.Int64("account_id", account.ID),
			zap.Int("status", submit.StatusCode),
			zap.String("detail", detail),
		)
		s.failRow(ctx, genID, detail, VideoStudioErrCodeSubmit, reqLog)
		return nil, ErrVideoStudioSubmitFailed
	}

	// --- Step 6: bind the operation + account to the row (pending -> processing). ---
	if ok, err := s.repo.MarkSubmitted(ctx, genID, submit.OperationName, account.ID); err != nil {
		reqLog.Error("video_studio.mark_submitted_failed", zap.Int64("generation_id", genID), zap.Error(err))
		// The upstream job is already running; surface the persistence error so the
		// caller knows tracking may be lost rather than silently dropping it.
		return nil, fmt.Errorf("video studio: mark submitted: %w", err)
	} else if !ok {
		reqLog.Warn("video_studio.mark_submitted_no_row", zap.Int64("generation_id", genID))
	}

	reqLog.Info("video_studio.submitted",
		zap.Int64("generation_id", genID),
		zap.Int64("account_id", account.ID),
		zap.String("operation", submit.OperationName),
	)

	return &VideoStudioStartResult{
		GenerationID:  genID,
		Status:        VideoStatusProcessing,
		Model:         in.Model,
		Balance:       s.readBalance(ctx, userID, user.Balance),
		EstimatedCost: s.estimateCost(ctx, user, apiKey),
	}, nil
}

// ---------------------------------------------------------------------------
// Read-time reconciliation.
// ---------------------------------------------------------------------------

// Reconcile polls the bound upstream operation for a processing generation and,
// when the operation is done, transitions the row (succeeded+billed or failed).
// It is invoked from the status-read endpoints so progress advances without a
// dedicated background worker. It returns the freshest row.
//
// Concurrent status reads can race here; the repository's atomic transitions
// ensure exactly one caller records billing. A poll/transient error leaves the
// row processing (the next read retries).
func (s *VideoStudioService) Reconcile(ctx context.Context, gen *dbent.VideoGeneration) *dbent.VideoGeneration {
	if gen == nil || gen.Status != VideoStatusProcessing {
		return gen
	}
	if strings.TrimSpace(gen.OperationName) == "" || gen.AccountID <= 0 {
		return gen
	}
	reqLog := logger.L().With(
		zap.String("component", "service.video_studio"),
		zap.Int64("generation_id", gen.ID),
		zap.Int64("user_id", gen.UserID),
		zap.String("operation", gen.OperationName),
	)

	account, err := s.veo.GetVeoAccount(ctx, gen.AccountID)
	if err != nil || account == nil {
		reqLog.Info("video_studio.reconcile_account_unavailable", zap.Error(err))
		return gen
	}

	poll, err := s.veo.PollVeo(ctx, account, gen.OperationName)
	if err != nil {
		reqLog.Info("video_studio.reconcile_poll_failed", zap.Error(err))
		return gen
	}
	if poll.StatusCode >= 400 {
		// A hard upstream error on the operation itself: fail the row.
		detail := veoUpstreamErrorDetail(poll.Body, poll.StatusCode)
		if ok, _ := s.repo.TransitionToFailed(ctx, gen.ID, detail, VideoStudioErrCodeUpstream); ok {
			reqLog.Info("video_studio.reconcile_failed_upstream", zap.Int("status", poll.StatusCode))
		}
		return s.refresh(ctx, gen)
	}
	if !poll.Done {
		return gen // still running
	}
	if !poll.Success {
		detail := veoUpstreamErrorDetail(poll.Body, poll.StatusCode)
		if ok, _ := s.repo.TransitionToFailed(ctx, gen.ID, detail, VideoStudioErrCodeUpstream); ok {
			reqLog.Info("video_studio.reconcile_failed_operation_error")
		}
		return s.refresh(ctx, gen)
	}

	// Done + success: price it, win the transition, then bill exactly once.
	sampleCount := veoSampleCount(poll.Body)
	apiKey, user, err := s.hydrateForBilling(ctx, gen)
	if err != nil {
		reqLog.Warn("video_studio.reconcile_hydrate_failed", zap.Error(err))
		// Still mark succeeded so the user can fetch the video; cost stays 0 and the
		// charge is skipped (rare; logged for follow-up).
	}

	cost := 0.0
	if apiKey != nil && s.costResolver != nil {
		if cb := s.costResolver.ComputeVeoVideoCostBreakdown(ctx, user, apiKey, poll.DurationSeconds); cb != nil {
			cost = cb.ActualCost
		}
	}

	won, err := s.repo.TransitionToSucceeded(ctx, gen.ID, sampleCount, poll.DurationSeconds, cost)
	if err != nil {
		reqLog.Error("video_studio.transition_succeeded_failed", zap.Error(err))
		return s.refresh(ctx, gen)
	}
	if won && apiKey != nil && user != nil {
		s.billCompleted(ctx, gen, apiKey, user, account, poll.DurationSeconds, reqLog)
	}
	return s.refresh(ctx, gen)
}

// billCompleted records usage (cost calc + atomic balance deduction + usage_log)
// for a just-succeeded generation. Called only by the transition winner, so the
// charge happens exactly once. A failure here is logged [CRITICAL] but does not
// fail the generation (the video already exists; balance was pre-gated).
func (s *VideoStudioService) billCompleted(
	ctx context.Context, gen *dbent.VideoGeneration, apiKey *APIKey, user *User, account *Account,
	durationSeconds float64, reqLog *zap.Logger,
) {
	if s.usageRecord == nil {
		return
	}
	subscription, _ := s.resolveActiveSubscription(ctx, gen.UserID, apiKey.Group)
	result := &ForwardResult{
		Model:                gen.Model,
		MediaType:            "video",
		VideoDurationSeconds: durationSeconds,
	}
	if err := s.usageRecord.RecordUsage(ctx, &RecordUsageInput{
		Result:          result,
		APIKey:          apiKey,
		User:            user,
		Account:         account,
		Subscription:    subscription,
		InboundEndpoint: videoStudioInboundEndpoint,
		UserAgent:       "",
		IPAddress:       "",
		QuotaPlatform:   QuotaPlatform(ctx, apiKey),
	}); err != nil {
		reqLog.Error("[CRITICAL] video_studio.record_usage_failed_after_success",
			zap.Int64("generation_id", gen.ID),
			zap.Int64("account_id", account.ID),
			zap.Float64("duration_seconds", durationSeconds),
			zap.Error(err),
		)
	}
}

// hydrateForBilling rebuilds the (apiKey, user) billing identity for a row the
// same way StartGenerate does, so RecordUsage bills against the exact same
// key/group/multiplier context.
func (s *VideoStudioService) hydrateForBilling(ctx context.Context, gen *dbent.VideoGeneration) (*APIKey, *User, error) {
	user, err := s.users.GetByID(ctx, gen.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("load user: %w", err)
	}
	group, err := s.resolveAllowedGroup(ctx, gen.UserID, gen.GroupID)
	if err != nil {
		return nil, user, fmt.Errorf("resolve group: %w", err)
	}
	apiKey, err := s.keyEnsurer.EnsureStudioAPIKey(ctx, gen.UserID, gen.GroupID)
	if err != nil {
		return nil, user, fmt.Errorf("ensure key: %w", err)
	}
	apiKey.User = user
	apiKey.Group = group
	gid := group.ID
	apiKey.GroupID = &gid
	return apiKey, user, nil
}

// ---------------------------------------------------------------------------
// StreamVideo: proxy the produced file on demand.
// ---------------------------------------------------------------------------

// StreamVideo resolves the authoritative upstream file URI for the index-th
// sample server-side (host-validated, no client-supplied URI — no SSRF) and
// returns the upstream response for the handler to stream. The caller MUST close
// the returned response body. Ownership of gen is the caller's responsibility.
func (s *VideoStudioService) StreamVideo(ctx context.Context, gen *dbent.VideoGeneration, index int) (*http.Response, error) {
	if gen == nil {
		return nil, ErrVideoStudioNotFound
	}
	if gen.Status != VideoStatusSucceeded {
		return nil, ErrVideoStudioNotReady
	}
	if index < 0 {
		index = 0
	}
	account, err := s.veo.GetVeoAccount(ctx, gen.AccountID)
	if err != nil || account == nil {
		return nil, ErrVideoStudioNoAccount
	}
	fileURI, err := s.veo.ResolveVeoFileURI(ctx, account, gen.OperationName, index)
	if err != nil {
		return nil, fmt.Errorf("video studio: resolve file uri: %w", err)
	}
	return s.veo.ProxyVeoFile(ctx, account, fileURI)
}

// ---------------------------------------------------------------------------
// Read APIs (delegate to repo; reconciliation applied to processing rows).
// ---------------------------------------------------------------------------

// GetGeneration returns one generation owned by userID, reconciling it if it is
// still processing.
func (s *VideoStudioService) GetGeneration(ctx context.Context, userID, id int64) (*dbent.VideoGeneration, error) {
	gen, err := s.repo.GetGeneration(ctx, id)
	if err != nil || gen == nil {
		return nil, ErrVideoStudioNotFound
	}
	if gen.UserID != userID {
		return nil, ErrVideoStudioNotFound
	}
	return s.Reconcile(ctx, gen), nil
}

// ListGenerations returns a page of the user's generations. Processing rows in
// the page are reconciled so the client sees fresh status without a separate poll.
func (s *VideoStudioService) ListGenerations(ctx context.Context, userID int64, page, size int) ([]*dbent.VideoGeneration, int, error) {
	items, total, err := s.repo.ListGenerations(ctx, userID, page, size)
	if err != nil {
		return nil, 0, err
	}
	for i, gen := range items {
		if gen.Status == VideoStatusProcessing {
			items[i] = s.Reconcile(ctx, gen)
		}
	}
	return items, total, nil
}

// BatchGetGenerations reconciles + returns the user's generations among ids. It
// powers the client's batch status poll; ownership is enforced in the repo.
func (s *VideoStudioService) BatchGetGenerations(ctx context.Context, userID int64, ids []int64) ([]*dbent.VideoGeneration, error) {
	items, err := s.repo.ListGenerationsByIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	for i, gen := range items {
		if gen.Status == VideoStatusProcessing {
			items[i] = s.Reconcile(ctx, gen)
		}
	}
	return items, nil
}

// DeleteGeneration soft-deletes a generation owned by userID.
func (s *VideoStudioService) DeleteGeneration(ctx context.Context, userID, id int64) error {
	gen, err := s.repo.GetGeneration(ctx, id)
	if err != nil || gen == nil || gen.UserID != userID {
		return ErrVideoStudioNotFound
	}
	return s.repo.DeleteGeneration(ctx, id)
}

// ClearHistory soft-deletes all of the user's video generations.
func (s *VideoStudioService) ClearHistory(ctx context.Context, userID int64) error {
	_, err := s.repo.ClearUserHistory(ctx, userID)
	return err
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func (s *VideoStudioService) resolveAllowedGroup(ctx context.Context, userID, groupID int64) (*Group, error) {
	groups, err := s.groups.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("video studio: load available groups: %w", err)
	}
	for i := range groups {
		if groups[i].ID == groupID {
			g := groups[i]
			return &g, nil
		}
	}
	return nil, ErrVideoStudioGroupNotAllowed
}

func (s *VideoStudioService) resolveActiveSubscription(ctx context.Context, userID int64, group *Group) (*UserSubscription, error) {
	if group == nil || !group.IsSubscriptionType() || s.subscriptions == nil {
		return nil, nil
	}
	sub, err := s.subscriptions.GetActiveSubscription(ctx, userID, group.ID)
	if err != nil {
		return nil, fmt.Errorf("video studio: resolve active subscription: %w", err)
	}
	return sub, nil
}

// estimateCost prices a not-yet-run generation using the default video duration,
// so the UI can show an approximate charge before completion. Best-effort.
func (s *VideoStudioService) estimateCost(ctx context.Context, user *User, apiKey *APIKey) float64 {
	if s.costResolver == nil {
		return 0
	}
	cb := s.costResolver.ComputeVeoVideoCostBreakdown(ctx, user, apiKey, veoDefaultVideoDurationSeconds)
	if cb == nil {
		return 0
	}
	return cb.ActualCost
}

func (s *VideoStudioService) readBalance(ctx context.Context, userID int64, fallback float64) float64 {
	fresh, err := s.users.GetByID(ctx, userID)
	if err != nil || fresh == nil {
		return fallback
	}
	return fresh.Balance
}

// failRow marks a row failed (best-effort) during the submit path. Unlike the
// reconcile transitions this is not race-sensitive (the row is still pending and
// owned by this in-flight request).
func (s *VideoStudioService) failRow(ctx context.Context, id int64, errMsg, errCode string, reqLog *zap.Logger) {
	if _, err := s.repo.TransitionToFailed(ctx, id, errMsg, errCode); err != nil {
		reqLog.Warn("video_studio.fail_row_failed", zap.Int64("generation_id", id), zap.Error(err))
	}
}

// refresh re-reads a row after a transition; on error it returns the stale copy
// so callers always have something to return.
func (s *VideoStudioService) refresh(ctx context.Context, gen *dbent.VideoGeneration) *dbent.VideoGeneration {
	fresh, err := s.repo.GetGeneration(ctx, gen.ID)
	if err != nil || fresh == nil {
		return gen
	}
	return fresh
}

// ---------------------------------------------------------------------------
// Background sweep: reclaim stale pending/processing rows + prune expired.
// ---------------------------------------------------------------------------

func (s *VideoStudioService) startSweeper() {
	if s == nil || s.repo == nil {
		return
	}
	s.sweepOnce()
	go func() {
		ticker := time.NewTicker(videoStudioSweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.sweepOnce()
		}
	}()
}

func (s *VideoStudioService) sweepOnce() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), videoStudioSweepTimeout)
		defer cancel()

		// Reclaim stale pending/processing rows (orphaned by a crash or an
		// operation that never completes).
		staleCutoff := time.Now().Add(-videoStudioStalePendingTimeout)
		for {
			n, err := s.repo.FailStaleGenerations(ctx, staleCutoff, videoStudioSweepBatchSize,
				"generation interrupted", VideoStudioErrCodeInterrupted)
			if err != nil {
				logger.LegacyPrintf("service.video_studio", "video_studio_fail_stale_failed err=%v", err)
				break
			}
			if n < videoStudioSweepBatchSize {
				break
			}
		}

		// Prune rows past the retention window (no local files to delete).
		retentionCutoff := time.Now().Add(-VideoStudioRetention)
		for {
			n, err := s.repo.PruneExpiredGenerations(ctx, retentionCutoff, videoStudioSweepBatchSize)
			if err != nil {
				logger.LegacyPrintf("service.video_studio", "video_studio_prune_expired_failed err=%v", err)
				break
			}
			if n < videoStudioSweepBatchSize {
				break
			}
		}
	}()
}

// ---------------------------------------------------------------------------
// Veo request-body builder + response parsing.
//
// ⚠️ FIELD-GUESSING RISK SURFACE. The exact predictLongRunning request/response
// shape was authored offline without access to live Veo API docs. buildVeoSubmitBody
// emits the conventional {instances:[{prompt}], parameters:{}} shape; if the live
// API differs, adjust HERE (single chokepoint). Response parsing (duration / file
// URI / sample count) is intentionally lenient and lives in gemini_veo_service.go.
// ---------------------------------------------------------------------------

// buildVeoSubmitBody builds the predictLongRunning request body for a text
// prompt. Veo's AI Studio predict endpoint conventionally takes an `instances`
// array (each with a `prompt`) plus an optional `parameters` object. The MVP
// sends a single instance with just the prompt; duration/aspect-ratio knobs are
// a V1 follow-up and are intentionally omitted so the upstream applies defaults.
func buildVeoSubmitBody(prompt string) ([]byte, error) {
	payload := map[string]any{
		"instances": []map[string]any{
			{"prompt": prompt},
		},
		"parameters": map[string]any{},
	}
	return json.Marshal(payload)
}

// normalizeVeoModel trims/defaults the requested model. Empty defaults to a
// current Veo model; a non-Veo value is coerced to the default so the in-app
// path never submits a wrong-family model.
func normalizeVeoModel(model string) string {
	m := strings.TrimSpace(model)
	if m == "" || !IsVeoModel(m) {
		return DefaultVeoModel
	}
	return m
}

// DefaultVeoModel is the model used when the request omits one. Exported so the
// handler/DTO can advertise it to the client.
const DefaultVeoModel = "veo-3.0-generate-001"

// GroupAllowsVeoVideo reports whether a group is configured for in-app video
// generation. Per the product decision, a configured per-second price both
// enables the feature and gates entry visibility (no separate feature flag).
func GroupAllowsVeoVideo(group *Group) bool {
	return group != nil && group.VeoVideoPricePerSecond != nil && *group.VeoVideoPricePerSecond > 0
}

// veoSampleCount counts the produced video samples in a completed poll body
// (lenient across the two known container shapes).
func veoSampleCount(body []byte) int {
	for _, container := range veoSampleContainers {
		if arr := gjson.GetBytes(body, container); arr.IsArray() {
			if n := len(arr.Array()); n > 0 {
				return n
			}
		}
	}
	return 0
}

// veoUpstreamErrorDetail extracts a human-readable error detail from an upstream
// error body, falling back to the status code. Bounded so a huge body cannot
// bloat the stored error column.
func veoUpstreamErrorDetail(body []byte, statusCode int) string {
	if msg := strings.TrimSpace(gjson.GetBytes(body, "error.message").String()); msg != "" {
		return truncateVeoError(msg)
	}
	if msg := strings.TrimSpace(gjson.GetBytes(body, "error").String()); msg != "" {
		return truncateVeoError(msg)
	}
	if statusCode > 0 {
		return fmt.Sprintf("upstream status %d", statusCode)
	}
	return "upstream error"
}

func truncateVeoError(s string) string {
	const maxLen = 480
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
