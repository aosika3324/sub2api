package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type editableFileService interface {
	SubmitPPT(ctx context.Context, input service.EditableFileSubmitInput) (*dbent.EditableFileTask, error)
	SubmitPSD(ctx context.Context, input service.EditableFileSubmitInput) (*dbent.EditableFileTask, error)
	GetTask(ctx context.Context, id int64) (*dbent.EditableFileTask, error)
	ListTasks(ctx context.Context, userID int64, ids []int64, page, size int) (*service.EditableFileTaskPage, error)
	ListArtifacts(ctx context.Context, taskID int64) ([]*dbent.EditableFileArtifact, error)
}

type EditableFileHandler struct {
	service editableFileService
}

func NewEditableFileHandler(editableFileService *service.EditableFileService) *EditableFileHandler {
	return &EditableFileHandler{service: editableFileService}
}

type editableFileCreateRequest struct {
	Kind                string   `json:"kind"`
	GroupID             *int64   `json:"group_id"`
	ImageConversationID *int64   `json:"image_conversation_id"`
	SourceGenerationID  *int64   `json:"source_generation_id"`
	Prompt              string   `json:"prompt" binding:"required"`
	Model               string   `json:"model"`
	ClientTaskID        string   `json:"client_task_id"`
	Images              []string `json:"images"`
}

type editableFileTaskResponse struct {
	ID                  int64   `json:"id"`
	UserID              int64   `json:"user_id"`
	GroupID             *int64  `json:"group_id,omitempty"`
	APIKeyID            *int64  `json:"api_key_id,omitempty"`
	ImageConversationID *int64  `json:"image_conversation_id,omitempty"`
	SourceGenerationID  *int64  `json:"source_generation_id,omitempty"`
	AccountID           *int64  `json:"account_id,omitempty"`
	Kind                string  `json:"kind"`
	Status              string  `json:"status"`
	Prompt              string  `json:"prompt"`
	Model               string  `json:"model"`
	ClientTaskID        string  `json:"client_task_id,omitempty"`
	ChatGPTConversation string  `json:"chatgpt_conversation_id,omitempty"`
	Cost                float64 `json:"cost"`
	Error               string  `json:"error,omitempty"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	ExpiresAt           string  `json:"expires_at,omitempty"`
}

type editableFileArtifactResponse struct {
	ID            int64   `json:"id"`
	TaskID        int64   `json:"task_id"`
	UserID        int64   `json:"user_id"`
	Kind          string  `json:"kind"`
	FileName      string  `json:"file_name"`
	MimeType      string  `json:"mime_type"`
	SizeBytes     int64   `json:"size_bytes"`
	StorageKey    string  `json:"storage_key"`
	SourcePointer *string `json:"source_pointer,omitempty"`
	SHA256        string  `json:"sha256,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

func (h *EditableFileHandler) CreateTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.service == nil {
		response.InternalError(c, "Editable file service is unavailable")
		return
	}

	var req editableFileCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	input := service.EditableFileSubmitInput{
		UserID:              subject.UserID,
		GroupID:             req.GroupID,
		ImageConversationID: req.ImageConversationID,
		SourceGenerationID:  req.SourceGenerationID,
		Kind:                req.Kind,
		Prompt:              req.Prompt,
		Model:               req.Model,
		ClientTaskID:        req.ClientTaskID,
		Base64Images:        req.Images,
	}

	var (
		task *dbent.EditableFileTask
		err  error
	)
	switch strings.ToLower(strings.TrimSpace(req.Kind)) {
	case "ppt", "pptx", "powerpoint":
		task, err = h.service.SubmitPPT(c.Request.Context(), input)
	case "psd", "photoshop":
		task, err = h.service.SubmitPSD(c.Request.Context(), input)
	default:
		err = service.ErrEditableFileInvalidKind
	}
	if err != nil {
		h.respondEditableFileError(c, err)
		return
	}
	response.Accepted(c, buildEditableFileTaskResponse(task))
}

func (h *EditableFileHandler) ListTasks(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.service == nil {
		response.InternalError(c, "Editable file service is unavailable")
		return
	}

	page, size := response.ParsePagination(c)
	pageResult, err := h.service.ListTasks(c.Request.Context(), subject.UserID, nil, page, size)
	if err != nil {
		response.InternalError(c, "Failed to list editable file tasks")
		return
	}
	items := make([]editableFileTaskResponse, 0, len(pageResult.Items))
	for _, task := range pageResult.Items {
		items = append(items, buildEditableFileTaskResponse(task))
	}
	response.Paginated(c, items, int64(pageResult.Total), page, size)
}

func (h *EditableFileHandler) GetTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	task := h.loadUserTask(c, subject.UserID)
	if task == nil {
		return
	}
	response.Success(c, buildEditableFileTaskResponse(task))
}

func (h *EditableFileHandler) ListArtifacts(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	task := h.loadUserTask(c, subject.UserID)
	if task == nil {
		return
	}
	artifacts, err := h.service.ListArtifacts(c.Request.Context(), task.ID)
	if err != nil {
		response.InternalError(c, "Failed to list editable file artifacts")
		return
	}
	items := make([]editableFileArtifactResponse, 0, len(artifacts))
	for _, artifact := range artifacts {
		items = append(items, buildEditableFileArtifactResponse(artifact))
	}
	response.Success(c, items)
}

func (h *EditableFileHandler) loadUserTask(c *gin.Context, userID int64) *dbent.EditableFileTask {
	if h == nil || h.service == nil {
		response.InternalError(c, "Editable file service is unavailable")
		return nil
	}
	id, err := parseInt64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid task id")
		return nil
	}
	task, err := h.service.GetTask(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Editable file task not found")
		return nil
	}
	if task.UserID != userID {
		response.NotFound(c, "Editable file task not found")
		return nil
	}
	return task
}

func (h *EditableFileHandler) respondEditableFileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEditableFileInvalidKind):
		response.BadRequest(c, "kind must be ppt or psd")
	case errors.Is(err, service.ErrEditableFilePromptRequired):
		response.BadRequest(c, "prompt is required")
	case errors.Is(err, service.ErrEditableFilePSDImageRequired):
		response.BadRequest(c, "psd export requires at least one reference image")
	default:
		response.InternalError(c, "Failed to create editable file task")
	}
}

func buildEditableFileTaskResponse(task *dbent.EditableFileTask) editableFileTaskResponse {
	if task == nil {
		return editableFileTaskResponse{}
	}
	resp := editableFileTaskResponse{
		ID:                  task.ID,
		UserID:              task.UserID,
		GroupID:             task.GroupID,
		APIKeyID:            task.APIKeyID,
		ImageConversationID: task.ImageConversationID,
		SourceGenerationID:  task.SourceGenerationID,
		AccountID:           task.AccountID,
		Kind:                task.Kind,
		Status:              task.Status,
		Prompt:              task.Prompt,
		Model:               task.Model,
		ClientTaskID:        task.ClientTaskID,
		ChatGPTConversation: task.ChatgptConversationID,
		Cost:                task.Cost,
		CreatedAt:           task.CreatedAt.UTC().Format(timeRFC3339),
		UpdatedAt:           task.UpdatedAt.UTC().Format(timeRFC3339),
	}
	if task.Error != nil {
		resp.Error = *task.Error
	}
	if task.ExpiresAt != nil {
		resp.ExpiresAt = task.ExpiresAt.UTC().Format(timeRFC3339)
	}
	return resp
}

func buildEditableFileArtifactResponse(artifact *dbent.EditableFileArtifact) editableFileArtifactResponse {
	if artifact == nil {
		return editableFileArtifactResponse{}
	}
	return editableFileArtifactResponse{
		ID:            artifact.ID,
		TaskID:        artifact.TaskID,
		UserID:        artifact.UserID,
		Kind:          artifact.Kind,
		FileName:      artifact.FileName,
		MimeType:      artifact.MimeType,
		SizeBytes:     artifact.SizeBytes,
		StorageKey:    artifact.StorageKey,
		SourcePointer: artifact.SourcePointer,
		SHA256:        artifact.Sha256,
		CreatedAt:     artifact.CreatedAt.UTC().Format(timeRFC3339),
	}
}

func parseInt64Param(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Param(name))
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(raw, 10, 64)
}
