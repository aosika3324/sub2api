package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Narrow consumer interfaces (handler-local) for testability. The concrete
// *service.ImageStudioService, service.ImageStudioRepository and
// service.ImageStore satisfy these.
// ---------------------------------------------------------------------------

// imageStudioGenerator runs an in-app (JWT) image generation.
type imageStudioGenerator interface {
	StartGenerate(ctx context.Context, userID int64, in service.ImageStudioGenerateInput) (*service.ImageStudioStartResult, error)
	PruneExpiredImagesThrottled()
}

// imageStudioStore reads stored image bytes back for the assets endpoint.
type imageStudioStore interface {
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, key string) error
}

// imageStudioRepo is the persistence surface the handler reads/writes directly
// (conversations CRUD, generation listing/get/delete). Generation creation +
// status transitions happen inside ImageStudioService, not here.
type imageStudioRepo interface {
	CreateConversation(ctx context.Context, userID int64, title string) (*dbent.ImageConversation, error)
	ListConversations(ctx context.Context, userID int64, page, size int) ([]*dbent.ImageConversation, int, error)
	GetConversation(ctx context.Context, id int64) (*dbent.ImageConversation, error)
	UpdateConversationTitle(ctx context.Context, id int64, title string) error
	DeleteConversation(ctx context.Context, id int64) error
	DeleteConversationCascade(ctx context.Context, id int64) ([]string, error)
	ClearUserHistory(ctx context.Context, userID int64) ([]string, error)
	GetGeneration(ctx context.Context, id int64) (*dbent.ImageGeneration, error)
	ListGenerations(ctx context.Context, userID int64, conversationID *int64, page, size int) ([]*dbent.ImageGeneration, int, error)
	DeleteGeneration(ctx context.Context, id int64) error
}

// ImageStudioHandler serves the JWT-authenticated in-app image studio endpoints.
// Every endpoint enforces the current JWT user; a user_id from the request body
// is never trusted.
type ImageStudioHandler struct {
	studio imageStudioGenerator
	repo   imageStudioRepo
	store  imageStudioStore
}

// NewImageStudioHandler creates a new ImageStudioHandler.
func NewImageStudioHandler(
	studio *service.ImageStudioService,
	repo service.ImageStudioRepository,
	store service.ImageStore,
) *ImageStudioHandler {
	return &ImageStudioHandler{
		studio: studio,
		repo:   repo,
		store:  store,
	}
}

const (
	imageStudioRoutePrefix       = "/api/v1/user/image-studio"
	imageStudioAssetsRoutePrefix = imageStudioRoutePrefix + "/assets"
	// imageStudioInputAssetsRoutePrefix serves user-provided reference images.
	// A distinct top-level segment (not assets/.../input/...) avoids gin/httprouter
	// panics that arise from mixing a param and a static segment at one depth.
	imageStudioInputAssetsRoutePrefix = imageStudioRoutePrefix + "/input-assets"

	// maxStudioInputImageBytes caps the reference image upload size (20MB), matching
	// the upstream per-part upload limit used by the images pipeline.
	maxStudioInputImageBytes = 20 << 20
	maxStudioInputImages     = 8
	imageStudioGenerateTTL   = 15 * time.Minute
)

// ---------------------------------------------------------------------------
// Request / response DTOs.
// ---------------------------------------------------------------------------

// GenerateImageRequest is the POST /generate payload. user_id is intentionally
// absent: the acting user is always the JWT subject.
type GenerateImageRequest struct {
	ConversationID *int64 `json:"conversation_id"`
	GroupID        int64  `json:"group_id" binding:"required"`
	Mode           string `json:"mode"`
	Prompt         string `json:"prompt" binding:"required"`
	Model          string `json:"model"`
	Size           string `json:"size"`
	Quality        string `json:"quality"`
	N              int    `json:"n"`
}

type generateImageResponse struct {
	GenerationID   int64    `json:"generation_id"`
	ConversationID int64    `json:"conversation_id"`
	Mode           string   `json:"mode,omitempty"`
	Images         []string `json:"images"`
	InputImages    []string `json:"input_images,omitempty"`
	Status         string   `json:"status"`
	Cost           float64  `json:"cost"`
	Balance        float64  `json:"balance"`
}

type conversationResponse struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type generationResponse struct {
	ID             int64    `json:"id"`
	ConversationID int64    `json:"conversation_id"`
	GroupID        int64    `json:"group_id"`
	Mode           string   `json:"mode,omitempty"`
	Prompt         string   `json:"prompt"`
	Model          string   `json:"model"`
	Size           string   `json:"size"`
	Quality        string   `json:"quality"`
	N              int      `json:"n"`
	ImageCount     int      `json:"image_count"`
	Status         string   `json:"status"`
	Cost           float64  `json:"cost"`
	Images         []string `json:"images"`
	InputImages    []string `json:"input_images,omitempty"`
	Width          *int     `json:"width,omitempty"`
	Height         *int     `json:"height,omitempty"`
	Error          string   `json:"error,omitempty"`
	CreatedAt      string   `json:"created_at"`
	ExpiresAt      string   `json:"expires_at"`
}

type updateConversationRequest struct {
	Title string `json:"title" binding:"required"`
}

type createConversationRequest struct {
	Title string `json:"title"`
}

// ---------------------------------------------------------------------------
// Endpoints.
// ---------------------------------------------------------------------------

// Generate handles POST /api/v1/user/image-studio/generate. It accepts either a
// JSON body (text-to-image) or a multipart/form-data body carrying an "image"
// reference file (image-to-image / edits).
func (h *ImageStudioHandler) Generate(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var in service.ImageStudioGenerateInput
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		parsed, err := h.parseMultipartGenerate(c)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		in = parsed
	} else {
		var req GenerateImageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
		in = service.ImageStudioGenerateInput{
			GroupID:        req.GroupID,
			ConversationID: req.ConversationID,
			Mode:           req.Mode,
			Prompt:         req.Prompt,
			Model:          req.Model,
			Size:           req.Size,
			Quality:        req.Quality,
			N:              req.N,
		}
	}

	if strings.TrimSpace(in.Prompt) == "" {
		response.BadRequest(c, "prompt is required")
		return
	}
	in.UserAgent = c.GetHeader("User-Agent")
	in.IPAddress = c.ClientIP()

	generateCtx, cancel := context.WithTimeout(context.Background(), imageStudioGenerateTTL)
	defer cancel()

	result, err := h.studio.StartGenerate(generateCtx, subject.UserID, in)
	if err != nil {
		h.respondGenerateError(c, err)
		return
	}

	response.Success(c, generateImageResponse{
		GenerationID:   result.GenerationID,
		ConversationID: result.ConversationID,
		Mode:           result.Mode,
		Images:         []string{},
		InputImages:    buildInputAssetURLs(result.GenerationID, len(result.InputImages)),
		Status:         "pending",
		Cost:           0,
		Balance:        result.Balance,
	})
}

// parseMultipartGenerate parses a multipart/form-data /generate request into an
// ImageStudioGenerateInput, including the user-provided reference image.
func (h *ImageStudioHandler) parseMultipartGenerate(c *gin.Context) (service.ImageStudioGenerateInput, error) {
	var in service.ImageStudioGenerateInput

	groupID, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("group_id")), 10, 64)
	if err != nil || groupID <= 0 {
		return in, errors.New("group_id is required")
	}
	in.GroupID = groupID

	if raw := strings.TrimSpace(c.PostForm("conversation_id")); raw != "" {
		convID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return in, errors.New("invalid conversation_id")
		}
		in.ConversationID = &convID
	}

	in.Prompt = c.PostForm("prompt")
	in.Mode = c.PostForm("mode")
	in.Model = c.PostForm("model")
	in.Size = c.PostForm("size")
	in.Quality = c.PostForm("quality")
	if raw := strings.TrimSpace(c.PostForm("n")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return in, errors.New("invalid n")
		}
		in.N = n
	}

	form, err := c.MultipartForm()
	if err != nil {
		return in, errors.New("image file is required")
	}
	fileHeaders := form.File["image"]
	if len(fileHeaders) == 0 {
		return in, errors.New("image file is required")
	}
	if len(fileHeaders) > maxStudioInputImages {
		return in, errors.New("too many image files")
	}

	in.InputImages = make([]service.StudioInputImage, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		if fileHeader.Size > maxStudioInputImageBytes {
			return in, errors.New("image file is too large")
		}
		file, err := fileHeader.Open()
		if err != nil {
			return in, errors.New("failed to read image file")
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxStudioInputImageBytes))
		_ = file.Close()
		if readErr != nil {
			return in, errors.New("failed to read image file")
		}
		if len(data) == 0 {
			return in, errors.New("image file is empty")
		}
		in.InputImages = append(in.InputImages, service.StudioInputImage{
			Data:        data,
			ContentType: fileHeader.Header.Get("Content-Type"),
			FileName:    fileHeader.Filename,
		})
	}
	return in, nil
}

// ListConversations handles GET /conversations (paginated).
func (h *ImageStudioHandler) ListConversations(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, total, err := h.repo.ListConversations(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]conversationResponse, 0, len(items))
	for _, conv := range items {
		out = append(out, toConversationResponse(conv))
	}
	response.Paginated(c, out, int64(total), page, pageSize)
}

// CreateConversation handles POST /conversations {title?}.
func (h *ImageStudioHandler) CreateConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Title is optional; tolerate an empty/absent body.
		req = createConversationRequest{}
	}

	conv, err := h.repo.CreateConversation(c.Request.Context(), subject.UserID, strings.TrimSpace(req.Title))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toConversationResponse(conv))
}

// UpdateConversation handles PATCH /conversations/:id {title}.
func (h *ImageStudioHandler) UpdateConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	var req updateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	conv, err := h.repo.GetConversation(c.Request.Context(), id)
	if err != nil {
		h.respondNotFoundOr(c, err, "Conversation not found")
		return
	}
	if conv.UserID != subject.UserID {
		response.NotFound(c, "Conversation not found")
		return
	}

	if err := h.repo.UpdateConversationTitle(c.Request.Context(), id, strings.TrimSpace(req.Title)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "conversation updated"})
}

// DeleteConversation handles DELETE /conversations/:id.
func (h *ImageStudioHandler) DeleteConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	conv, err := h.repo.GetConversation(c.Request.Context(), id)
	if err != nil {
		h.respondNotFoundOr(c, err, "Conversation not found")
		return
	}
	if conv.UserID != subject.UserID {
		response.NotFound(c, "Conversation not found")
		return
	}

	// Cascade: soft-delete the conversation AND its generations in one
	// transaction, returning the stored image keys so we can remove the files.
	keys, err := h.repo.DeleteConversationCascade(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// Best-effort file cleanup (row deletes are the source of truth; a missing
	// file must not fail the request). Mirrors DeleteGeneration's behavior.
	for _, key := range keys {
		_ = h.store.Delete(c.Request.Context(), key)
	}
	response.Success(c, gin.H{"message": "conversation deleted"})
}

// ListConversationGenerations handles GET /conversations/:id/generations (paginated).
func (h *ImageStudioHandler) ListConversationGenerations(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	// Verify the conversation belongs to the user before listing its generations.
	conv, err := h.repo.GetConversation(c.Request.Context(), id)
	if err != nil {
		h.respondNotFoundOr(c, err, "Conversation not found")
		return
	}
	if conv.UserID != subject.UserID {
		response.NotFound(c, "Conversation not found")
		return
	}

	page, pageSize := response.ParsePagination(c)
	convID := id
	items, total, err := h.repo.ListGenerations(c.Request.Context(), subject.UserID, &convID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.respondGenerations(c, items, total, page, pageSize)
}

// ListGenerations handles GET /generations (paginated gallery, all conversations).
func (h *ImageStudioHandler) ListGenerations(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, total, err := h.repo.ListGenerations(c.Request.Context(), subject.UserID, nil, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.respondGenerations(c, items, total, page, pageSize)
}

// GetGeneration handles GET /generations/:id for progress/status polling.
func (h *ImageStudioHandler) GetGeneration(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid generation ID")
		return
	}

	gen, err := h.repo.GetGeneration(c.Request.Context(), id)
	if err != nil {
		h.respondNotFoundOr(c, err, "Generation not found")
		return
	}
	if gen.UserID != subject.UserID {
		response.NotFound(c, "Generation not found")
		return
	}

	response.Success(c, toGenerationResponse(gen))
}

// ClearHistory handles DELETE /history for all image-studio conversations and generations.
func (h *ImageStudioHandler) ClearHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	keys, err := h.repo.ClearUserHistory(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	for _, key := range keys {
		_ = h.store.Delete(c.Request.Context(), key)
	}
	response.Success(c, gin.H{"message": "history cleared"})
}

// DeleteGeneration handles DELETE /generations/:id (ownership-checked; also
// removes the stored image files).
func (h *ImageStudioHandler) DeleteGeneration(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid generation ID")
		return
	}

	gen, err := h.repo.GetGeneration(c.Request.Context(), id)
	if err != nil {
		h.respondNotFoundOr(c, err, "Generation not found")
		return
	}
	if gen.UserID != subject.UserID {
		response.NotFound(c, "Generation not found")
		return
	}

	// Best-effort delete of stored image files (do not fail the request if a file
	// is already gone; the row delete is the source of truth). Output images first,
	// then user-provided reference (input) images.
	for _, key := range gen.StorageKeys {
		_ = h.store.Delete(c.Request.Context(), key)
	}
	for _, key := range gen.InputStorageKeys {
		_ = h.store.Delete(c.Request.Context(), key)
	}

	if err := h.repo.DeleteGeneration(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "generation deleted"})
}

// GetAsset handles GET /assets/:genID/:idx — streams a single stored image after
// verifying ownership.
func (h *ImageStudioHandler) GetAsset(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	genID, err := strconv.ParseInt(c.Param("genID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid generation ID")
		return
	}
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil || idx < 0 {
		response.BadRequest(c, "Invalid image index")
		return
	}

	gen, err := h.repo.GetGeneration(c.Request.Context(), genID)
	if err != nil {
		h.respondNotFoundOr(c, err, "Image not found")
		return
	}
	// Ownership gate: never serve another user's image.
	if gen.UserID != subject.UserID {
		response.NotFound(c, "Image not found")
		return
	}
	if idx >= len(gen.StorageKeys) {
		response.NotFound(c, "Image not found")
		return
	}

	key := gen.StorageKeys[idx]
	if applyStudioAssetCacheHeaders(c, gen, key) {
		return
	}

	reader, contentType, err := h.store.Open(c.Request.Context(), key)
	if err != nil {
		response.NotFound(c, "Image not found")
		return
	}
	defer func() { _ = reader.Close() }()

	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

// GetInputAsset handles GET /input-assets/:genID/:idx — streams a single stored
// user-provided reference image after verifying ownership.
func (h *ImageStudioHandler) GetInputAsset(c *gin.Context) {
	h.pruneExpiredImagesBestEffort()
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	genID, err := strconv.ParseInt(c.Param("genID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid generation ID")
		return
	}
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil || idx < 0 {
		response.BadRequest(c, "Invalid image index")
		return
	}

	gen, err := h.repo.GetGeneration(c.Request.Context(), genID)
	if err != nil {
		h.respondNotFoundOr(c, err, "Image not found")
		return
	}
	// Ownership gate: never serve another user's reference image.
	if gen.UserID != subject.UserID {
		response.NotFound(c, "Image not found")
		return
	}
	if idx >= len(gen.InputStorageKeys) {
		response.NotFound(c, "Image not found")
		return
	}

	key := gen.InputStorageKeys[idx]
	if applyStudioAssetCacheHeaders(c, gen, key) {
		return
	}

	reader, contentType, err := h.store.Open(c.Request.Context(), key)
	if err != nil {
		response.NotFound(c, "Image not found")
		return
	}
	defer func() { _ = reader.Close() }()

	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func (h *ImageStudioHandler) respondGenerations(c *gin.Context, items []*dbent.ImageGeneration, total, page, pageSize int) {
	out := make([]generationResponse, 0, len(items))
	for _, gen := range items {
		out = append(out, toGenerationResponse(gen))
	}
	response.Paginated(c, out, int64(total), page, pageSize)
}

// respondGenerateError maps ImageStudioService.Generate errors to HTTP codes.
// Billing-eligibility errors (and the wrapped subscription-not-found error) are
// infraerrors carrying their own status, so response.ErrorFrom handles them
// (insufficient balance -> 400, subscription invalid -> 403, RPM -> 429, etc.).
// The studio's own typed sentinel errors are plain errors and mapped explicitly.
func (h *ImageStudioHandler) respondGenerateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrImageStudioGroupNotAllowed),
		errors.Is(err, service.ErrImageStudioImageGenerationDisabled):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrImageStudioConversationNotFound):
		response.NotFound(c, "Conversation not found")
	case errors.Is(err, service.ErrImageStudioBusy):
		response.Error(c, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, service.ErrImageStudioInvalidMode):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrImageStudioNoImages),
		errors.Is(err, service.ErrImageStudioNoAccount):
		response.InternalError(c, "image generation produced no result")
	default:
		// Falls back to infraerrors mapping (billing eligibility, subscription not
		// found, etc.) or a generic 500 for unrecognized errors.
		response.ErrorFrom(c, err)
	}
}

// respondNotFoundOr maps ent not-found to 404, otherwise the generic error.
func (h *ImageStudioHandler) respondNotFoundOr(c *gin.Context, err error, notFoundMsg string) {
	if dbent.IsNotFound(err) {
		response.NotFound(c, notFoundMsg)
		return
	}
	response.ErrorFrom(c, err)
}

// buildAssetURLs builds ready-to-use image URLs pointing at the assets route.
func buildAssetURLs(genID int64, count int) []string {
	urls := make([]string, 0, count)
	for i := 0; i < count; i++ {
		urls = append(urls, imageStudioAssetsRoutePrefix+"/"+strconv.FormatInt(genID, 10)+"/"+strconv.Itoa(i))
	}
	return urls
}

// buildInputAssetURLs builds ready-to-use URLs pointing at the input-assets route
// (user-provided reference images).
func buildInputAssetURLs(genID int64, count int) []string {
	urls := make([]string, 0, count)
	for i := 0; i < count; i++ {
		urls = append(urls, imageStudioInputAssetsRoutePrefix+"/"+strconv.FormatInt(genID, 10)+"/"+strconv.Itoa(i))
	}
	return urls
}

// applyStudioAssetCacheHeaders sets private immutable-style cache headers for a
// stored studio asset and returns true when the request was satisfied as 304.
func applyStudioAssetCacheHeaders(c *gin.Context, gen *dbent.ImageGeneration, key string) bool {
	if c == nil || gen == nil {
		return false
	}
	modified := gen.UpdatedAt
	if modified.IsZero() {
		modified = gen.CreatedAt
	}
	modified = modified.UTC().Truncate(time.Second)
	etag := fmt.Sprintf(`"studio-%d-%s"`, gen.ID, strings.ReplaceAll(key, `"`, `%22`))

	maxAge := int(service.ImageStudioRetention.Seconds())
	if !gen.CreatedAt.IsZero() {
		maxAge = int(time.Until(gen.CreatedAt.Add(service.ImageStudioRetention)).Seconds())
		if maxAge < 0 {
			maxAge = 0
		}
		if maxAge > int(service.ImageStudioRetention.Seconds()) {
			maxAge = int(service.ImageStudioRetention.Seconds())
		}
	}
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
	c.Header("ETag", etag)
	if !modified.IsZero() {
		c.Header("Last-Modified", modified.Format(http.TimeFormat))
	}

	if studioAssetETagMatches(c.GetHeader("If-None-Match"), etag) {
		c.AbortWithStatus(http.StatusNotModified)
		return true
	}
	if raw := strings.TrimSpace(c.GetHeader("If-Modified-Since")); raw != "" && !modified.IsZero() {
		if since, err := http.ParseTime(raw); err == nil && !modified.After(since.UTC()) {
			c.AbortWithStatus(http.StatusNotModified)
			return true
		}
	}
	return false
}

func (h *ImageStudioHandler) pruneExpiredImagesBestEffort() {
	if h == nil || h.studio == nil {
		return
	}
	h.studio.PruneExpiredImagesThrottled()
}

func studioAssetETagMatches(raw, etag string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if tag == "*" || tag == etag || strings.TrimPrefix(tag, "W/") == etag {
			return true
		}
	}
	return false
}

func toConversationResponse(conv *dbent.ImageConversation) conversationResponse {
	if conv == nil {
		return conversationResponse{}
	}
	return conversationResponse{
		ID:        conv.ID,
		Title:     conv.Title,
		CreatedAt: conv.CreatedAt.UTC().Format(timeRFC3339),
		UpdatedAt: conv.UpdatedAt.UTC().Format(timeRFC3339),
	}
}

func toGenerationResponse(gen *dbent.ImageGeneration) generationResponse {
	if gen == nil {
		return generationResponse{}
	}
	errMsg := ""
	if gen.Error != nil {
		errMsg = *gen.Error
	}
	return generationResponse{
		ID:             gen.ID,
		ConversationID: gen.ConversationID,
		GroupID:        gen.GroupID,
		Mode:           inferredGenerationMode(gen),
		Prompt:         gen.Prompt,
		Model:          gen.Model,
		Size:           gen.Size,
		Quality:        gen.Quality,
		N:              gen.N,
		ImageCount:     gen.ImageCount,
		Status:         gen.Status,
		Cost:           gen.Cost,
		Images:         buildAssetURLs(gen.ID, len(gen.StorageKeys)),
		InputImages:    buildInputAssetURLs(gen.ID, len(gen.InputStorageKeys)),
		Width:          gen.Width,
		Height:         gen.Height,
		Error:          errMsg,
		CreatedAt:      gen.CreatedAt.UTC().Format(timeRFC3339),
		ExpiresAt:      gen.CreatedAt.Add(service.ImageStudioRetention).UTC().Format(timeRFC3339),
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

func inferredGenerationMode(gen *dbent.ImageGeneration) string {
	if gen == nil {
		return service.ImageStudioModeGenerate
	}
	mode, err := service.NormalizeImageStudioMode("", len(gen.InputStorageKeys))
	if err != nil {
		return service.ImageStudioModeGenerate
	}
	return mode
}
