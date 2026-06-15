package handler

import (
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

// Status reads go THROUGH the service (not straight to a repo), because reading
// a processing row triggers read-time reconciliation (poll the upstream
// operation, bill once on completion).

const (
	// maxVideoStudioBatchIDs caps the number of generation IDs accepted by the
	// batch status endpoint, bounding the query and response size.
	maxVideoStudioBatchIDs = 100
)

// VideoStudioHandler serves the JWT-authenticated in-app video studio endpoints.
// Every endpoint enforces the current JWT user; a user_id from the request body
// is never trusted.
type VideoStudioHandler struct {
	studio *service.VideoStudioService
}

// NewVideoStudioHandler creates a new VideoStudioHandler.
func NewVideoStudioHandler(studio *service.VideoStudioService) *VideoStudioHandler {
	return &VideoStudioHandler{studio: studio}
}

// ---------------------------------------------------------------------------
// Request / response DTOs.
// ---------------------------------------------------------------------------

// generateVideoRequest is the POST /generate payload. user_id is intentionally
// absent: the acting user is always the JWT subject.
type generateVideoRequest struct {
	GroupID int64  `json:"group_id" binding:"required"`
	Prompt  string `json:"prompt" binding:"required"`
	Model   string `json:"model"`
}

type generateVideoResponse struct {
	GenerationID  int64   `json:"generation_id"`
	Status        string  `json:"status"`
	Model         string  `json:"model"`
	EstimatedCost float64 `json:"estimated_cost"`
	Balance       float64 `json:"balance"`
}

// videoGenerationResponse is the per-row status payload the client polls.
type videoGenerationResponse struct {
	ID              int64   `json:"id"`
	Prompt          string  `json:"prompt"`
	Model           string  `json:"model"`
	Status          string  `json:"status"`
	SampleCount     int     `json:"sample_count"`
	DurationSeconds float64 `json:"duration_seconds"`
	Cost            float64 `json:"cost"`
	// Videos holds ready-to-use proxy URLs for each produced sample (succeeded
	// only). Empty otherwise.
	Videos []string `json:"videos"`
	// ErrorCode is the stable machine-readable failure classifier (failed only);
	// the frontend maps it to a localized message.
	ErrorCode string `json:"error_code,omitempty"`
	CreatedAt string `json:"created_at"`
}

const videoStudioRoutePrefix = "/api/v1/user/video-studio"

// buildVideoURLs builds the proxy-stream URLs for a succeeded generation's
// samples. The bytes are fetched lazily by the browser via the stream endpoint.
func buildVideoURLs(genID int64, count int) []string {
	if count <= 0 {
		return []string{}
	}
	urls := make([]string, 0, count)
	for i := 0; i < count; i++ {
		urls = append(urls, videoStudioRoutePrefix+"/generations/"+strconv.FormatInt(genID, 10)+"/video/"+strconv.Itoa(i))
	}
	return urls
}

func toVideoGenerationResponse(gen *dbent.VideoGeneration) videoGenerationResponse {
	resp := videoGenerationResponse{
		ID:              gen.ID,
		Prompt:          gen.Prompt,
		Model:           gen.Model,
		Status:          gen.Status,
		SampleCount:     gen.SampleCount,
		DurationSeconds: gen.DurationSeconds,
		Cost:            gen.Cost,
		Videos:          []string{},
		CreatedAt:       gen.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if gen.Status == service.VideoStatusSucceeded {
		count := gen.SampleCount
		if count <= 0 {
			count = 1 // a succeeded generation always has at least one sample
		}
		resp.Videos = buildVideoURLs(gen.ID, count)
	}
	if gen.ErrorCode != nil {
		resp.ErrorCode = *gen.ErrorCode
	}
	return resp
}

// ---------------------------------------------------------------------------
// Endpoints.
// ---------------------------------------------------------------------------

// Generate handles POST /api/v1/user/video-studio/generate. It submits a Veo
// job and returns immediately with a processing generation; the client polls
// for completion.
func (h *VideoStudioHandler) Generate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req generateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		response.BadRequest(c, "prompt is required")
		return
	}

	result, err := h.studio.StartGenerate(c.Request.Context(), subject.UserID, service.VideoStudioGenerateInput{
		GroupID:   req.GroupID,
		Prompt:    req.Prompt,
		Model:     req.Model,
		UserAgent: c.GetHeader("User-Agent"),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		h.respondGenerateError(c, err)
		return
	}

	response.Success(c, generateVideoResponse{
		GenerationID:  result.GenerationID,
		Status:        result.Status,
		Model:         result.Model,
		EstimatedCost: result.EstimatedCost,
		Balance:       result.Balance,
	})
}

// ListGenerations handles GET /generations (paginated; processing rows are
// reconciled in-line so the client sees fresh status).
func (h *VideoStudioHandler) ListGenerations(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, total, err := h.studio.ListGenerations(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]videoGenerationResponse, 0, len(items))
	for _, gen := range items {
		out = append(out, toVideoGenerationResponse(gen))
	}
	response.Paginated(c, out, int64(total), page, pageSize)
}

// BatchGetGenerations handles GET /generations-batch?ids=1,2,3 — reconciles +
// returns the status of multiple generations in one round-trip. Ownership is
// enforced in the service/repo; unknown/foreign IDs are silently omitted.
func (h *VideoStudioHandler) BatchGetGenerations(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	ids, err := parseVideoBatchIDs(c.Query("ids"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(ids) == 0 {
		response.Success(c, gin.H{"items": []videoGenerationResponse{}})
		return
	}

	gens, err := h.studio.BatchGetGenerations(c.Request.Context(), subject.UserID, ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]videoGenerationResponse, 0, len(gens))
	for _, gen := range gens {
		out = append(out, toVideoGenerationResponse(gen))
	}
	response.Success(c, gin.H{"items": out})
}

// GetGeneration handles GET /generations/:id for progress/status polling.
func (h *VideoStudioHandler) GetGeneration(c *gin.Context) {
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

	gen, err := h.studio.GetGeneration(c.Request.Context(), subject.UserID, id)
	if err != nil {
		h.respondReadError(c, err)
		return
	}
	response.Success(c, toVideoGenerationResponse(gen))
}

// StreamVideo handles GET /generations/:id/video/:idx — proxy-streams the
// produced video file after verifying ownership. The bytes are fetched from the
// upstream signed URI on demand (not stored locally).
func (h *VideoStudioHandler) StreamVideo(c *gin.Context) {
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
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil || idx < 0 {
		response.BadRequest(c, "Invalid sample index")
		return
	}

	// Ownership + readiness are enforced in the service (GetGeneration scopes to
	// the user; StreamVideo requires succeeded).
	gen, err := h.studio.GetGeneration(c.Request.Context(), subject.UserID, id)
	if err != nil {
		h.respondReadError(c, err)
		return
	}

	resp, err := h.studio.StreamVideo(c.Request.Context(), gen, idx)
	if err != nil {
		h.respondReadError(c, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Writer.Header().Set("Content-Type", ct)
	} else {
		c.Writer.Header().Set("Content-Type", "video/mp4")
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Writer.Header().Set("Content-Length", cl)
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// DeleteGeneration handles DELETE /generations/:id (ownership-checked).
func (h *VideoStudioHandler) DeleteGeneration(c *gin.Context) {
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

	if err := h.studio.DeleteGeneration(c.Request.Context(), subject.UserID, id); err != nil {
		h.respondReadError(c, err)
		return
	}
	response.Success(c, gin.H{"message": "generation deleted"})
}

// ClearHistory handles DELETE /history for all of the user's video generations.
func (h *VideoStudioHandler) ClearHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	if err := h.studio.ClearHistory(c.Request.Context(), subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "history cleared"})
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

// parseVideoBatchIDs parses a comma-separated id list, dedupes, and caps it at
// maxVideoStudioBatchIDs. Empty entries are ignored; a non-numeric entry errors.
func parseVideoBatchIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid generation id in ids")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		if len(ids) >= maxVideoStudioBatchIDs {
			break
		}
	}
	return ids, nil
}

// respondGenerateError maps VideoStudioService.StartGenerate errors to HTTP
// codes. Billing-eligibility / subscription errors carry their own status via
// infraerrors, so response.ErrorFrom handles them; the studio's own sentinels
// are mapped explicitly.
func (h *VideoStudioHandler) respondGenerateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrVideoStudioGroupNotAllowed),
		errors.Is(err, service.ErrVideoStudioDisabled):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrVideoStudioBusy):
		response.Error(c, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, service.ErrVideoStudioInvalidPrompt):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrVideoStudioNoAccount):
		response.Error(c, http.StatusServiceUnavailable, "no available video account")
	case errors.Is(err, service.ErrVideoStudioSubmitFailed):
		response.Error(c, http.StatusBadGateway, "video generation submit failed")
	default:
		response.ErrorFrom(c, err)
	}
}

// respondReadError maps read/stream errors to HTTP codes.
func (h *VideoStudioHandler) respondReadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrVideoStudioNotFound):
		response.NotFound(c, "Generation not found")
	case errors.Is(err, service.ErrVideoStudioNotReady):
		response.Error(c, http.StatusConflict, "video not ready")
	case errors.Is(err, service.ErrVideoStudioNoAccount):
		response.Error(c, http.StatusServiceUnavailable, "video account unavailable")
	default:
		response.ErrorFrom(c, err)
	}
}
