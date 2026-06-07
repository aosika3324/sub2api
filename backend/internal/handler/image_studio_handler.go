package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	Generate(ctx context.Context, userID int64, in service.ImageStudioGenerateInput) (*service.ImageStudioGenerateResult, error)
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

const imageStudioAssetsRoutePrefix = "/api/v1/user/image-studio/assets"

// ---------------------------------------------------------------------------
// Request / response DTOs.
// ---------------------------------------------------------------------------

// GenerateImageRequest is the POST /generate payload. user_id is intentionally
// absent: the acting user is always the JWT subject.
type GenerateImageRequest struct {
	ConversationID *int64 `json:"conversation_id"`
	GroupID        int64  `json:"group_id" binding:"required"`
	Prompt         string `json:"prompt" binding:"required"`
	Model          string `json:"model"`
	Size           string `json:"size"`
	Quality        string `json:"quality"`
	N              int    `json:"n"`
}

type generateImageResponse struct {
	GenerationID   int64    `json:"generation_id"`
	ConversationID int64    `json:"conversation_id"`
	Images         []string `json:"images"`
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
	Prompt         string   `json:"prompt"`
	Model          string   `json:"model"`
	Size           string   `json:"size"`
	Quality        string   `json:"quality"`
	N              int      `json:"n"`
	ImageCount     int      `json:"image_count"`
	Status         string   `json:"status"`
	Cost           float64  `json:"cost"`
	Images         []string `json:"images"`
	Width          *int     `json:"width,omitempty"`
	Height         *int     `json:"height,omitempty"`
	Error          string   `json:"error,omitempty"`
	CreatedAt      string   `json:"created_at"`
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

// Generate handles POST /api/v1/user/image-studio/generate.
func (h *ImageStudioHandler) Generate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req GenerateImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		response.BadRequest(c, "prompt is required")
		return
	}

	result, err := h.studio.Generate(c.Request.Context(), subject.UserID, service.ImageStudioGenerateInput{
		GroupID:        req.GroupID,
		ConversationID: req.ConversationID,
		Prompt:         req.Prompt,
		Model:          req.Model,
		Size:           req.Size,
		Quality:        req.Quality,
		N:              req.N,
		UserAgent:      c.GetHeader("User-Agent"),
		IPAddress:      c.ClientIP(),
	})
	if err != nil {
		h.respondGenerateError(c, err)
		return
	}

	response.Success(c, generateImageResponse{
		GenerationID:   result.GenerationID,
		ConversationID: result.ConversationID,
		Images:         buildAssetURLs(result.GenerationID, len(result.Images)),
		Cost:           result.Cost,
		Balance:        result.Balance,
	})
}

// ListConversations handles GET /conversations (paginated).
func (h *ImageStudioHandler) ListConversations(c *gin.Context) {
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

	if err := h.repo.DeleteConversation(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "conversation deleted"})
}

// ListConversationGenerations handles GET /conversations/:id/generations (paginated).
func (h *ImageStudioHandler) ListConversationGenerations(c *gin.Context) {
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
	// is already gone; the row delete is the source of truth).
	for _, key := range gen.StorageKeys {
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

	reader, contentType, err := h.store.Open(c.Request.Context(), gen.StorageKeys[idx])
	if err != nil {
		response.NotFound(c, "Image not found")
		return
	}
	defer func() { _ = reader.Close() }()

	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	c.Header("Content-Type", contentType)
	// Studio images are immutable for a given (genID, idx); cache aggressively but
	// keep them private to the authenticated user.
	c.Header("Cache-Control", "private, max-age=86400")
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
	case errors.Is(err, service.ErrImageStudioBusy):
		response.Error(c, http.StatusTooManyRequests, err.Error())
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
		Prompt:         gen.Prompt,
		Model:          gen.Model,
		Size:           gen.Size,
		Quality:        gen.Quality,
		N:              gen.N,
		ImageCount:     gen.ImageCount,
		Status:         gen.Status,
		Cost:           gen.Cost,
		Images:         buildAssetURLs(gen.ID, len(gen.StorageKeys)),
		Width:          gen.Width,
		Height:         gen.Height,
		Error:          errMsg,
		CreatedAt:      gen.CreatedAt.UTC().Format(timeRFC3339),
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
