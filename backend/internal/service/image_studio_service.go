package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Public input/output types.
// ---------------------------------------------------------------------------

// ImageStudioGenerateInput is the in-app image-studio generation request issued
// by a logged-in user (JWT path), already resolved to a chosen group.
type ImageStudioGenerateInput struct {
	// GroupID is the group the user selected to drive billing + account routing.
	// It must be one of the user's available (allowed) groups.
	GroupID int64
	// ConversationID, when nil, creates a fresh conversation (title derived from
	// the prompt). When set, the generation is appended to that conversation.
	ConversationID *int64

	Prompt  string
	Model   string
	Size    string
	Quality string
	N       int

	// InputImage, when non-nil, switches the generation to the image-to-image
	// (OpenAI /v1/images/edits) path with the provided reference image.
	InputImage *StudioInputImage
	// InputImages, when non-empty, switches the generation to the image-to-image
	// path with one or more reference images. InputImage remains for older callers.
	InputImages []StudioInputImage

	// Optional request metadata, recorded on the usage log when available.
	UserAgent string
	IPAddress string
}

// StudioInputImage is a user-provided reference image for an image-to-image
// (edits) generation.
type StudioInputImage struct {
	Data        []byte
	ContentType string
	FileName    string
}

// ImageStudioGenerateResult is the successful outcome returned to the handler.
type ImageStudioGenerateResult struct {
	GenerationID   int64
	ConversationID int64
	Images         []string // storage keys (URLs are built by the handler / T9)
	InputImages    []string // input/reference storage keys (edits path only)
	Cost           float64
	Balance        float64
}

// StudioGeneratedImage is one produced image returned by the studio image
// generator port (raw bytes + content type), ready to hand to ImageStore.Put.
//
// Note: the shared *OpenAIGatewayService.GenerateImages orchestrator streams the
// upstream response body straight to its gin writer and returns only metadata
// (it carries no image bytes). Capturing those bytes for the JWT studio path is
// the concern of the concrete adapter wired in Task 9; this service depends only
// on the studioImageGenerator port below, which yields the decoded images.
type StudioGeneratedImage struct {
	Data        []byte
	ContentType string
}

// ---------------------------------------------------------------------------
// Narrow consumer interfaces (Go idiom: accept interfaces for testability).
// The concrete *UserService / *APIKeyService / *BillingCacheService /
// *BillingService / *OpenAIGatewayService satisfy these; the image generator is
// satisfied by a Task-9 adapter over GenerateImages + a capturing writer.
// ---------------------------------------------------------------------------

// studioUserReader loads the acting user (and is read again after billing to
// surface the post-deduction balance). Satisfied by *UserService.
type studioUserReader interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

// studioGroupProvider returns the groups a user is allowed to use. Satisfied by
// *APIKeyService.GetAvailableGroups (covers AllowedGroups + subscription +
// exclusivity exactly like the UI group selector).
type studioGroupProvider interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error)
}

// studioKeyEnsurer lazily provisions the hidden synthetic image-studio API key
// for (userID, groupID). Satisfied by *APIKeyService.EnsureStudioAPIKey.
type studioKeyEnsurer interface {
	EnsureStudioAPIKey(ctx context.Context, userID, groupID int64) (*APIKey, error)
}

// billingEligibilityChecker is the pre-flight balance/quota gate. Satisfied by
// *BillingCacheService.CheckBillingEligibility.
type billingEligibilityChecker interface {
	CheckBillingEligibility(ctx context.Context, user *User, apiKey *APIKey, group *Group, subscription *UserSubscription, platform string) error
}

// studioImageGenerator runs account-selection + forward + failover and returns
// the decoded produced images. See StudioGeneratedImage for why this is a
// dedicated port rather than *OpenAIGatewayService.GenerateImages directly.
type studioImageGenerator interface {
	GenerateStudioImages(ctx context.Context, in ImageGenInput) (*ImageGenResult, []StudioGeneratedImage, error)
}

// studioImageCostResolver prices the produced images using the exact same
// multiplier resolution and image-pricing path RecordUsage uses to deduct
// balance, so the reported/persisted cost equals the amount actually charged.
// Satisfied by *OpenAIGatewayService.ComputeImageCostBreakdown.
type studioImageCostResolver interface {
	ComputeImageCostBreakdown(ctx context.Context, user *User, apiKey *APIKey, result *OpenAIForwardResult) *CostBreakdown
}

// imageUsageRecorder records usage + atomically deducts balance + writes the
// usage_log (with image fields). Satisfied by *OpenAIGatewayService.RecordUsage.
type imageUsageRecorder interface {
	RecordUsage(ctx context.Context, input *OpenAIRecordUsageInput) error
}

// studioSubscriptionResolver loads the active subscription for a (user, group)
// pair the same authoritative way the gateway auth middleware does (it gates on
// Group.IsSubscriptionType() before calling). Satisfied by *SubscriptionService.
type studioSubscriptionResolver interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}

// ---------------------------------------------------------------------------
// Typed errors.
// ---------------------------------------------------------------------------

var (
	// ErrImageStudioGroupNotAllowed is returned when the requested group is not
	// among the user's available/allowed groups.
	ErrImageStudioGroupNotAllowed = errors.New("image studio: group not allowed for user")
	// ErrImageStudioImageGenerationDisabled is returned when the (allowed) group
	// does not permit image generation.
	ErrImageStudioImageGenerationDisabled = errors.New("image studio: image generation is not enabled for this group")
	// ErrImageStudioBusy is returned when the image concurrency slot could not be
	// acquired (server too busy).
	ErrImageStudioBusy = errors.New("image studio: server busy, please retry later")
	// ErrImageStudioNoImages is returned when generation reported success but
	// produced no images to store.
	ErrImageStudioNoImages = errors.New("image studio: upstream produced no images")
	// ErrImageStudioNoAccount is returned when generation succeeded but reported no
	// account (RecordUsage requires a non-nil account).
	ErrImageStudioNoAccount = errors.New("image studio: generation returned no account")
	// ErrImageStudioConversationNotFound is returned when a client-supplied
	// conversation_id does not exist or does not belong to the acting user. The
	// handler maps it to 404 (matching the other conversation endpoints) so it
	// never leaks the existence of another user's conversation.
	ErrImageStudioConversationNotFound = errors.New("image studio: conversation not found")
)

const (
	imageStudioInboundEndpoint = "image_studio"
	imageStudioStatusPending   = "pending"
	imageStudioStatusSucceeded = "succeeded"
	imageStudioStatusFailed    = "failed"
	imageStudioTitleMaxLen     = 80
	// imageStudioMaxN caps the number of images per request, matching the OpenAI
	// images endpoints' published limit. N is clamped (not rejected) to this.
	imageStudioMaxN = 10
)

// ---------------------------------------------------------------------------
// Service.
// ---------------------------------------------------------------------------

// ImageStudioServiceDeps groups the (narrow) dependencies of ImageStudioService.
type ImageStudioServiceDeps struct {
	Users         studioUserReader
	Groups        studioGroupProvider
	KeyEnsurer    studioKeyEnsurer
	Eligibility   billingEligibilityChecker
	Generator     studioImageGenerator
	CostResolver  studioImageCostResolver
	UsageRecord   imageUsageRecorder
	Subscriptions studioSubscriptionResolver
	Repo          ImageStudioRepository
	Store         ImageStore

	// Limiter + Cfg are optional. When Limiter is non-nil the studio path
	// acquires an image-generation concurrency slot using the same config knobs
	// as the gateway handler (cfg.Gateway.ImageConcurrency). When nil, slot
	// acquisition is skipped (e.g. in unit tests).
	Limiter *ImageConcurrencyLimiter
	Cfg     *config.Config
}

// ImageStudioService orchestrates an in-app (JWT) image generation: group
// validation, billing pre-flight, concurrency slotting, conversation/generation
// persistence, generation via the shared image pipeline, local storage, and
// in-app billing via RecordUsage. It follows spec §9 ordering exactly:
// preflight -> slot -> generate -> store -> bill -> persist.
type ImageStudioService struct {
	users         studioUserReader
	groups        studioGroupProvider
	keyEnsurer    studioKeyEnsurer
	eligibility   billingEligibilityChecker
	generator     studioImageGenerator
	costResolver  studioImageCostResolver
	usageRecord   imageUsageRecorder
	subscriptions studioSubscriptionResolver
	repo          ImageStudioRepository
	store         ImageStore
	limiter       *ImageConcurrencyLimiter
	cfg           *config.Config
}

// Compile-time assertions: the existing concrete services satisfy the studio
// ports. The studioImageGenerator port is intentionally NOT asserted here — it
// is satisfied by a capturing adapter wired in Task 9 (see StudioGeneratedImage),
// because *OpenAIGatewayService.GenerateImages returns no image bytes.
var (
	_ studioUserReader           = (*UserService)(nil)
	_ studioGroupProvider        = (*APIKeyService)(nil)
	_ studioKeyEnsurer           = (*APIKeyService)(nil)
	_ billingEligibilityChecker  = (*BillingCacheService)(nil)
	_ studioImageCostResolver    = (*OpenAIGatewayService)(nil)
	_ imageUsageRecorder         = (*OpenAIGatewayService)(nil)
	_ studioSubscriptionResolver = (*SubscriptionService)(nil)
)

// NewImageStudioService builds the service from its narrow dependencies.
func NewImageStudioService(deps ImageStudioServiceDeps) *ImageStudioService {
	return &ImageStudioService{
		users:         deps.Users,
		groups:        deps.Groups,
		keyEnsurer:    deps.KeyEnsurer,
		eligibility:   deps.Eligibility,
		generator:     deps.Generator,
		costResolver:  deps.CostResolver,
		usageRecord:   deps.UsageRecord,
		subscriptions: deps.Subscriptions,
		repo:          deps.Repo,
		store:         deps.Store,
		limiter:       deps.Limiter,
		cfg:           deps.Cfg,
	}
}

// Generate runs the studio orchestration for userID. See ImageStudioService doc
// for the spec §9 ordering and the per-step mapping in the code below.
func (s *ImageStudioService) Generate(ctx context.Context, userID int64, in ImageStudioGenerateInput) (*ImageStudioGenerateResult, error) {
	reqLog := logger.L().With(
		zap.String("component", "service.image_studio"),
		zap.Int64("user_id", userID),
		zap.Int64("group_id", in.GroupID),
	)

	// --- Step 1: load user, validate group membership + image permission. ---
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("image studio: load user: %w", err)
	}

	group, err := s.resolveAllowedGroup(ctx, userID, in.GroupID)
	if err != nil {
		return nil, err
	}
	if !GroupAllowsImageGeneration(group) {
		return nil, ErrImageStudioImageGenerationDisabled
	}

	// --- Step 2: billing eligibility pre-flight (balance/quota). ---
	// The synthetic key models the (user, group) identity the image path bills
	// against; we ensure it up-front so eligibility runs against the exact same
	// key/group context the generation + RecordUsage will use.
	apiKey, err := s.keyEnsurer.EnsureStudioAPIKey(ctx, userID, in.GroupID)
	if err != nil {
		return nil, fmt.Errorf("image studio: ensure studio api key: %w", err)
	}
	// Hydrate the key with the live user + group so downstream billing/usage code
	// (which reads apiKey.User / apiKey.Group) behaves like the gateway path.
	apiKey.User = user
	apiKey.Group = group
	gid := group.ID
	apiKey.GroupID = &gid

	// Resolve the active subscription the SAME authoritative way the gateway auth
	// middleware does (gate on Group.IsSubscriptionType(), then
	// SubscriptionService.GetActiveSubscription), so subscription-mode groups bill
	// against their subscription instead of being wrongly charged/rejected on
	// balance. UserService.GetByID does not eager-load subscriptions, so we cannot
	// read them off user.Subscriptions.
	subscription, err := s.resolveActiveSubscription(ctx, userID, group)
	if err != nil {
		reqLog.Info("image_studio.subscription_required_not_found", zap.Error(err))
		return nil, err
	}
	if err := s.eligibility.CheckBillingEligibility(ctx, user, apiKey, group, subscription, group.Platform); err != nil {
		reqLog.Info("image_studio.billing_eligibility_failed", zap.Error(err))
		return nil, err
	}

	// --- Step 3: acquire an image concurrency slot (defer release). ---
	if release, ok := s.acquireSlot(ctx); !ok {
		return nil, ErrImageStudioBusy
	} else if release != nil {
		defer release()
	}

	// --- Step 4: resolve conversation + insert pending generation row. ---
	conversationID, err := s.resolveConversation(ctx, userID, in)
	if err != nil {
		return nil, err
	}

	n := in.N
	if n <= 0 {
		n = 1
	} else if n > imageStudioMaxN {
		n = imageStudioMaxN
	}
	gen, err := s.repo.CreateGeneration(ctx, &dbent.ImageGeneration{
		UserID:         userID,
		ConversationID: conversationID,
		GroupID:        group.ID,
		Prompt:         in.Prompt,
		Model:          in.Model,
		Size:           in.Size,
		Quality:        in.Quality,
		N:              n,
		Status:         imageStudioStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("image studio: create generation: %w", err)
	}
	genID := gen.ID

	// --- Step 4b: persist the user-provided reference image (edits path). ---
	// Stored BEFORE generation so the input survives even if generation fails,
	// matching the spec contract. A storage failure here aborts before billing.
	inputImages := normalizeStudioInputImages(in)
	var inputKeys []string
	if len(inputImages) > 0 {
		inputKeys = make([]string, 0, len(inputImages))
		for idx, inputImage := range inputImages {
			key, putErr := s.store.PutInput(ctx, userID, genID, idx, inputImage.ContentType, inputImage.Data)
			if putErr != nil {
				s.cleanupStoredImages(ctx, inputKeys, reqLog)
				s.markFailed(ctx, genID, putErr, reqLog)
				return nil, fmt.Errorf("image studio: store input image %d: %w", idx, putErr)
			}
			inputKeys = append(inputKeys, key)
		}
		// Persisting the input key is non-fatal: the file already exists and the
		// generation can proceed even if this write fails.
		if err := s.repo.SetInputStorageKeys(ctx, genID, inputKeys); err != nil {
			reqLog.Error("image_studio.set_input_storage_keys_failed",
				zap.Int64("generation_id", genID),
				zap.Error(err),
			)
		}
	}

	// --- Step 5: build the parsed images request + run the shared pipeline. ---
	// An input image routes through /v1/images/edits (multipart); otherwise the
	// JSON /v1/images/generations path is used.
	var parsed *OpenAIImagesRequest
	if len(inputImages) > 0 {
		editsParsed, buildErr := s.buildEditsRequest(in, n, inputImages)
		if buildErr != nil {
			s.markFailed(ctx, genID, buildErr, reqLog)
			return nil, buildErr
		}
		parsed = editsParsed
	} else {
		parsed = s.buildParsedRequest(in, n)
	}
	genResult, images, genErr := s.generator.GenerateStudioImages(ctx, ImageGenInput{
		OpsContext:         nil, // JWT studio path: no gin context (see StudioGeneratedImage doc / T9 adapter).
		APIKey:             apiKey,
		Group:              group,
		Parsed:             parsed,
		Body:               parsed.Body,
		RequestModel:       parsed.Model,
		RequiredCapability: parsed.RequiredCapability,
		ReqLog:             reqLog,
	})
	if genErr != nil {
		s.markFailed(ctx, genID, genErr, reqLog)
		return nil, genErr
	}
	if genResult == nil || genResult.Result == nil {
		err := ErrImageStudioNoImages
		s.markFailed(ctx, genID, err, reqLog)
		return nil, err
	}
	if len(images) == 0 {
		s.markFailed(ctx, genID, ErrImageStudioNoImages, reqLog)
		return nil, ErrImageStudioNoImages
	}
	// Guard: RecordUsage dereferences input.Account; a nil account from a
	// (mis)behaving generator must surface as a failed generation, not a panic.
	if genResult.Account == nil {
		s.markFailed(ctx, genID, ErrImageStudioNoAccount, reqLog)
		return nil, ErrImageStudioNoAccount
	}

	// --- Step 6a: store each produced image; collect keys. ---
	keys := make([]string, 0, len(images))
	for idx, img := range images {
		key, putErr := s.store.Put(ctx, userID, genID, idx, img.ContentType, img.Data)
		if putErr != nil {
			// Storage failure: best-effort remove the images already written for
			// this generation to avoid orphans, mark failed, do NOT bill, return.
			s.cleanupStoredImages(ctx, keys, reqLog)
			s.markFailed(ctx, genID, putErr, reqLog)
			return nil, fmt.Errorf("image studio: store image %d: %w", idx, putErr)
		}
		keys = append(keys, key)
	}

	width, height := imageStudioDimensions(genResult.Result)

	// Bill for the number of images actually decoded + stored, not the upstream
	// data-array length. A partial-decode response (some entries missing/invalid
	// b64_json) yields fewer stored images than result.ImageCount; charging the
	// original count would overcharge for undelivered images. len(images) is
	// guaranteed > 0 here, so the ImageCount > 0 billing guards still hold.
	genResult.Result.ImageCount = len(images)

	// --- Step 6b: record usage (cost calc + atomic deduct + usage_log). ---
	// Spec §9: a RecordUsage failure here (after generate + store succeed) is a
	// rare system error; balance was already gated by pre-flight, so we log
	// [CRITICAL], still mark succeeded, and still return the images.
	cost := s.computeCost(ctx, genResult.Result, user, apiKey)
	usageErr := s.usageRecord.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result:          genResult.Result,
		APIKey:          apiKey,
		User:            user,
		Account:         genResult.Account,
		Subscription:    subscription,
		InboundEndpoint: imageStudioInboundEndpoint,
		UserAgent:       in.UserAgent,
		IPAddress:       in.IPAddress,
	})
	if usageErr != nil {
		reqLog.Error("[CRITICAL] image_studio.record_usage_failed_after_generation",
			zap.Int64("generation_id", genID),
			zap.Int64("account_id", accountID(genResult.Account)),
			zap.Float64("estimated_cost", cost),
			zap.Error(usageErr),
		)
		// fall through: still persist succeeded + return images (not a free vector).
	}

	// --- Step 7: persist succeeded. ---
	if err := s.repo.UpdateGenerationStatus(ctx, genID, imageStudioStatusSucceeded, keys, cost, len(images), width, height, ""); err != nil {
		reqLog.Error("image_studio.update_generation_succeeded_failed",
			zap.Int64("generation_id", genID),
			zap.Error(err),
		)
		// The images exist and usage was (attempted to be) recorded; surface the
		// persistence error to the caller rather than silently dropping it.
		return nil, fmt.Errorf("image studio: persist succeeded generation: %w", err)
	}

	// Read balance back after deduction so the caller can show the live figure.
	balance := s.readBalance(ctx, userID, user.Balance)

	return &ImageStudioGenerateResult{
		GenerationID:   genID,
		ConversationID: conversationID,
		Images:         keys,
		InputImages:    inputKeys,
		Cost:           cost,
		Balance:        balance,
	}, nil
}

// resolveAllowedGroup returns the group iff groupID is one of the user's
// available groups (membership + permission gate done by GetAvailableGroups).
func (s *ImageStudioService) resolveAllowedGroup(ctx context.Context, userID, groupID int64) (*Group, error) {
	groups, err := s.groups.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("image studio: load available groups: %w", err)
	}
	for i := range groups {
		if groups[i].ID == groupID {
			g := groups[i]
			return &g, nil
		}
	}
	return nil, ErrImageStudioGroupNotAllowed
}

// resolveConversation returns the existing conversation ID or creates a new one
// (title derived from the prompt) when none was supplied.
func (s *ImageStudioService) resolveConversation(ctx context.Context, userID int64, in ImageStudioGenerateInput) (int64, error) {
	if in.ConversationID != nil {
		// Never trust a client-supplied conversation_id: verify it exists and
		// belongs to the acting user before appending generations to it. A
		// missing/foreign id is reported as not-found (handler → 404) so it can't
		// be used to attach rows to, or probe the existence of, another user's
		// conversation.
		conv, err := s.repo.GetConversation(ctx, *in.ConversationID)
		if err != nil || conv == nil || conv.UserID != userID {
			return 0, ErrImageStudioConversationNotFound
		}
		return *in.ConversationID, nil
	}
	conv, err := s.repo.CreateConversation(ctx, userID, deriveConversationTitle(in.Prompt))
	if err != nil {
		return 0, fmt.Errorf("image studio: create conversation: %w", err)
	}
	return conv.ID, nil
}

// buildParsedRequest assembles the same *OpenAIImagesRequest the shared pipeline
// consumes, directly from the studio params (no gin parsing needed). It mirrors
// the JSON-generations shape produced by ParseOpenAIImagesRequest:
// endpoint=/v1/images/generations, b64_json response format, native size tier
// classification, and a JSON body forwarded upstream.
func (s *ImageStudioService) buildParsedRequest(in ImageStudioGenerateInput, n int) *OpenAIImagesRequest {
	model, explicitModel := normalizeStudioPipelineModel(in.Model)
	size := strings.TrimSpace(in.Size)
	quality := strings.TrimSpace(in.Quality)

	req := &OpenAIImagesRequest{
		Endpoint:       openAIImagesGenerationsEndpoint,
		ContentType:    "application/json",
		Model:          model,
		ExplicitModel:  explicitModel,
		Prompt:         strings.TrimSpace(in.Prompt),
		N:              n,
		Size:           size,
		ExplicitSize:   size != "",
		Quality:        quality,
		ResponseFormat: "b64_json",
	}
	applyOpenAIImagesDefaults(req)
	req.SizeTier = normalizeOpenAIImageSizeTier(req.Size)
	req.HasNativeOptions = quality != ""
	req.RequiredCapability = classifyOpenAIImagesCapability(req)
	req.Body = buildOpenAIImagesJSONBody(req)
	return req
}

// buildEditsRequest assembles a multipart *OpenAIImagesRequest for the
// image-to-image (OpenAI /v1/images/edits) path from the studio params plus the
// user-provided reference image. The SAME multipart.Writer produces both Body
// and ContentType so the boundary in ContentType matches the bytes in Body —
// ForwardImages re-parses Body against ContentType downstream.
func (s *ImageStudioService) buildEditsRequest(in ImageStudioGenerateInput, n int, inputImages []StudioInputImage) (*OpenAIImagesRequest, error) {
	if len(inputImages) == 0 {
		return nil, fmt.Errorf("image studio: edits request requires an input image")
	}
	model, explicitModel := normalizeStudioPipelineModel(in.Model)
	size := strings.TrimSpace(in.Size)
	quality := strings.TrimSpace(in.Quality)
	prompt := strings.TrimSpace(in.Prompt)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	uploads := make([]OpenAIImagesUpload, 0, len(inputImages))

	for idx, inputImage := range inputImages {
		fileName := sanitizeEditsFileName(inputImage.FileName, idx)
		contentType := strings.TrimSpace(inputImage.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(inputImage.Data)
		}

		partHeader := textproto.MIMEHeader{}
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename=%q`, fileName))
		partHeader.Set("Content-Type", contentType)
		part, err := w.CreatePart(partHeader)
		if err != nil {
			return nil, fmt.Errorf("image studio: create multipart image part %d: %w", idx, err)
		}
		if _, err := part.Write(inputImage.Data); err != nil {
			return nil, fmt.Errorf("image studio: write multipart image part %d: %w", idx, err)
		}
		uploads = append(uploads, OpenAIImagesUpload{
			FieldName:   "image",
			FileName:    fileName,
			ContentType: contentType,
			Data:        inputImage.Data,
		})
	}

	if explicitModel {
		if err := w.WriteField("model", model); err != nil {
			return nil, fmt.Errorf("image studio: write multipart model: %w", err)
		}
	}
	if err := w.WriteField("prompt", prompt); err != nil {
		return nil, fmt.Errorf("image studio: write multipart prompt: %w", err)
	}
	if size != "" {
		if err := w.WriteField("size", size); err != nil {
			return nil, fmt.Errorf("image studio: write multipart size: %w", err)
		}
	}
	if quality != "" {
		if err := w.WriteField("quality", quality); err != nil {
			return nil, fmt.Errorf("image studio: write multipart quality: %w", err)
		}
	}
	if err := w.WriteField("n", strconv.Itoa(n)); err != nil {
		return nil, fmt.Errorf("image studio: write multipart n: %w", err)
	}
	if err := w.WriteField("response_format", "b64_json"); err != nil {
		return nil, fmt.Errorf("image studio: write multipart response_format: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("image studio: finalize multipart body: %w", err)
	}

	req := &OpenAIImagesRequest{
		Endpoint:       openAIImagesEditsEndpoint,
		ContentType:    w.FormDataContentType(),
		Multipart:      true,
		Model:          model,
		ExplicitModel:  explicitModel,
		Prompt:         prompt,
		N:              n,
		Size:           size,
		ExplicitSize:   size != "",
		Quality:        quality,
		ResponseFormat: "b64_json",
		Uploads:        uploads,
		Body:           buf.Bytes(),
	}
	applyOpenAIImagesDefaults(req)
	req.SizeTier = normalizeOpenAIImageSizeTier(req.Size)
	req.HasNativeOptions = quality != ""
	req.RequiredCapability = classifyOpenAIImagesCapability(req)
	return req, nil
}

// sanitizeEditsFileName returns a safe, non-empty filename for the multipart
// image part. It strips any path components and falls back to "reference.png".
func sanitizeEditsFileName(name string, index int) string {
	name = strings.TrimSpace(name)
	// Strip path separators (defense against header injection / traversal).
	if idx := strings.LastIndexAny(name, `/\`); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSpace(name)
	// Drop any characters that could break the Content-Disposition header.
	name = strings.Map(func(r rune) rune {
		if r == '"' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, name)
	if name == "" {
		return fmt.Sprintf("reference-%d.png", index+1)
	}
	return name
}

func normalizeStudioInputImages(in ImageStudioGenerateInput) []StudioInputImage {
	if len(in.InputImages) > 0 {
		out := make([]StudioInputImage, 0, len(in.InputImages))
		for _, img := range in.InputImages {
			if len(img.Data) > 0 {
				out = append(out, img)
			}
		}
		return out
	}
	if in.InputImage != nil && len(in.InputImage.Data) > 0 {
		return []StudioInputImage{*in.InputImage}
	}
	return nil
}

func normalizeStudioPipelineModel(model string) (string, bool) {
	trimmed := strings.TrimSpace(model)
	lower := strings.ToLower(trimmed)
	switch {
	case lower == "", lower == "auto":
		return "gpt-image-2", false
	case lower == "codex-gpt-image-2":
		return "gpt-image-2-codex", true
	case isOpenAIImageGenerationModel(lower):
		return trimmed, true
	case strings.HasPrefix(lower, "gpt-5"):
		return "gpt-image-2", false
	default:
		return trimmed, trimmed != ""
	}
}

// markFailed records a failed generation; storage/usage are intentionally not
// touched. Errors from the status write are logged, not propagated (the caller
// already has a terminal error to return).
func (s *ImageStudioService) markFailed(ctx context.Context, genID int64, cause error, reqLog *zap.Logger) {
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	if err := s.repo.UpdateGenerationStatus(ctx, genID, imageStudioStatusFailed, nil, 0, 0, 0, 0, msg); err != nil {
		reqLog.Error("image_studio.update_generation_failed_failed",
			zap.Int64("generation_id", genID),
			zap.Error(err),
		)
	}
}

// computeCost returns the figure RecordUsage will deduct, using the SAME
// multiplier resolution + image-pricing path as RecordUsage (via
// ComputeImageCostBreakdown) so the reported/persisted cost equals the charge.
func (s *ImageStudioService) computeCost(ctx context.Context, result *OpenAIForwardResult, user *User, apiKey *APIKey) float64 {
	if s.costResolver == nil || result == nil {
		return 0
	}
	breakdown := s.costResolver.ComputeImageCostBreakdown(ctx, user, apiKey, result)
	if breakdown == nil {
		return 0
	}
	return breakdown.ActualCost
}

// cleanupStoredImages best-effort removes already-stored images for a generation
// after a later Put fails, to avoid leaving orphaned files. Delete errors are
// logged, not propagated.
func (s *ImageStudioService) cleanupStoredImages(ctx context.Context, keys []string, reqLog *zap.Logger) {
	for _, key := range keys {
		if err := s.store.Delete(ctx, key); err != nil {
			reqLog.Warn("image_studio.cleanup_orphan_image_failed",
				zap.String("storage_key", key),
				zap.Error(err),
			)
		}
	}
}

// acquireSlot acquires an image-generation concurrency slot using the gateway
// config knobs. Returns (nil, true) when no limiter is configured.
func (s *ImageStudioService) acquireSlot(ctx context.Context) (func(), bool) {
	if s.limiter == nil || s.cfg == nil {
		return nil, true
	}
	ic := s.cfg.Gateway.ImageConcurrency
	wait := strings.TrimSpace(ic.OverflowMode) == config.ImageConcurrencyOverflowModeWait
	return s.limiter.Acquire(
		ctx,
		ic.Enabled,
		ic.MaxConcurrentRequests,
		wait,
		time.Duration(ic.WaitTimeoutSeconds)*time.Second,
		ic.MaxWaitingRequests,
	)
}

// resolveActiveSubscription loads the active subscription for (userID, group)
// the same authoritative way the gateway auth middleware does: only for
// subscription-type groups, via SubscriptionService.GetActiveSubscription. A
// subscription-type group with no active subscription is rejected (mirroring the
// gateway's 403). Non-subscription groups return (nil, nil) → balance billing.
func (s *ImageStudioService) resolveActiveSubscription(ctx context.Context, userID int64, group *Group) (*UserSubscription, error) {
	if group == nil || !group.IsSubscriptionType() || s.subscriptions == nil {
		return nil, nil
	}
	sub, err := s.subscriptions.GetActiveSubscription(ctx, userID, group.ID)
	if err != nil {
		// GetActiveSubscription returns ErrSubscriptionNotFound when none is
		// active; surface it (the handler maps it to 403) so subscription users
		// without a valid subscription are rejected rather than silently billed.
		return nil, fmt.Errorf("image studio: resolve active subscription: %w", err)
	}
	return sub, nil
}

// readBalance re-reads the user's balance after RecordUsage deducted it,
// falling back to the pre-deduction value if the read fails.
func (s *ImageStudioService) readBalance(ctx context.Context, userID int64, fallback float64) float64 {
	fresh, err := s.users.GetByID(ctx, userID)
	if err != nil || fresh == nil {
		return fallback
	}
	return fresh.Balance
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func deriveConversationTitle(prompt string) string {
	title := strings.TrimSpace(prompt)
	if title == "" {
		return "New image"
	}
	runes := []rune(title)
	if len(runes) > imageStudioTitleMaxLen {
		return strings.TrimSpace(string(runes[:imageStudioTitleMaxLen]))
	}
	return title
}

// imageStudioDimensions extracts width/height from the result's output size
// (e.g. "1024x1024") when present; returns (0, 0) otherwise.
func imageStudioDimensions(result *OpenAIForwardResult) (int, int) {
	if result == nil {
		return 0, 0
	}
	for _, candidate := range []string{result.ImageOutputSize, result.ImageInputSize} {
		if w, h, ok := parseImageDimensionString(candidate); ok {
			return w, h
		}
	}
	if len(result.ImageOutputSizes) > 0 {
		if w, h, ok := parseImageDimensionString(result.ImageOutputSizes[0]); ok {
			return w, h
		}
	}
	return 0, 0
}

func parseImageDimensionString(s string) (int, int, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, ok1 := atoiPositive(strings.TrimSpace(parts[0]))
	h, ok2 := atoiPositive(strings.TrimSpace(parts[1]))
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return w, h, true
}

func atoiPositive(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func accountID(a *Account) int64 {
	if a == nil {
		return 0
	}
	return a.ID
}

// buildOpenAIImagesJSONBody serializes the studio-built parsed request into the
// JSON body the upstream image-generations endpoint expects.
func buildOpenAIImagesJSONBody(req *OpenAIImagesRequest) []byte {
	if req == nil {
		return nil
	}
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"n":      req.N,
	}
	if strings.TrimSpace(req.Size) != "" {
		payload["size"] = req.Size
	}
	if strings.TrimSpace(req.Quality) != "" {
		payload["quality"] = req.Quality
	}
	if strings.TrimSpace(req.ResponseFormat) != "" {
		payload["response_format"] = req.ResponseFormat
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return body
}
