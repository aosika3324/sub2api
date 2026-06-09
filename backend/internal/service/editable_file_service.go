package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const (
	EditableFileKindPPT = "ppt"
	EditableFileKindPSD = "psd"

	EditableFileStatusQueued    = "queued"
	EditableFileStatusRunning   = "running"
	EditableFileStatusSucceeded = "succeeded"
	EditableFileStatusFailed    = "failed"

	EditableFileArtifactPrimaryKind = "primary"
	EditableFileArtifactZipKind     = "zip"
	EditableFileArtifactInputKind   = "input"

	EditableFileRetention = 7 * 24 * time.Hour
)

var (
	ErrEditableFileInvalidKind       = errors.New("editable file: invalid kind")
	ErrEditableFilePromptRequired    = errors.New("editable file: prompt is required")
	ErrEditableFilePSDImageRequired  = errors.New("editable file: psd export requires at least one reference image")
	ErrEditableFileRepositoryMissing = errors.New("editable file: repository is required")
)

type EditableFileSubmitInput struct {
	UserID              int64
	GroupID             *int64
	APIKeyID            *int64
	ImageConversationID *int64
	SourceGenerationID  *int64
	Kind                string
	Prompt              string
	Model               string
	ClientTaskID        string
	Base64Images        []string
}

type EditableFileArtifactInput struct {
	TaskID        int64
	UserID        int64
	Kind          string
	FileName      string
	MimeType      string
	SizeBytes     int64
	StorageKey    string
	SourcePointer *string
	SHA256        string
}

type EditableFileTaskStatusUpdate struct {
	Status                string
	AccountID             *int64
	ChatGPTConversationID string
	Cost                  float64
	Error                 string
}

type EditableFileTaskPage struct {
	Items []*dbent.EditableFileTask
	Total int
}

type EditableFileRepository interface {
	CreateTask(ctx context.Context, input EditableFileSubmitInput, expiresAt time.Time) (*dbent.EditableFileTask, error)
	GetTask(ctx context.Context, id int64) (*dbent.EditableFileTask, error)
	ListTasks(ctx context.Context, userID int64, ids []int64, page, size int) ([]*dbent.EditableFileTask, int, error)
	UpdateTaskStatus(ctx context.Context, id int64, update EditableFileTaskStatusUpdate) error
	AddArtifact(ctx context.Context, input EditableFileArtifactInput) (*dbent.EditableFileArtifact, error)
	ListArtifacts(ctx context.Context, taskID int64) ([]*dbent.EditableFileArtifact, error)
	GetArtifact(ctx context.Context, id int64) (*dbent.EditableFileArtifact, error)
}

type EditableFileService struct {
	repo EditableFileRepository
	now  func() time.Time
}

func NewEditableFileService(repo EditableFileRepository) *EditableFileService {
	return &EditableFileService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *EditableFileService) Submit(ctx context.Context, input EditableFileSubmitInput) (*dbent.EditableFileTask, error) {
	if s == nil || s.repo == nil {
		return nil, ErrEditableFileRepositoryMissing
	}
	normalized, err := normalizeEditableFileSubmitInput(input)
	if err != nil {
		return nil, err
	}
	expiresAt := s.now().Add(EditableFileRetention)
	return s.repo.CreateTask(ctx, normalized, expiresAt)
}

func (s *EditableFileService) SubmitPPT(ctx context.Context, input EditableFileSubmitInput) (*dbent.EditableFileTask, error) {
	input.Kind = EditableFileKindPPT
	return s.Submit(ctx, input)
}

func (s *EditableFileService) SubmitPSD(ctx context.Context, input EditableFileSubmitInput) (*dbent.EditableFileTask, error) {
	input.Kind = EditableFileKindPSD
	return s.Submit(ctx, input)
}

func (s *EditableFileService) GetTask(ctx context.Context, id int64) (*dbent.EditableFileTask, error) {
	if s == nil || s.repo == nil {
		return nil, ErrEditableFileRepositoryMissing
	}
	return s.repo.GetTask(ctx, id)
}

func (s *EditableFileService) ListTasks(ctx context.Context, userID int64, ids []int64, page, size int) (*EditableFileTaskPage, error) {
	if s == nil || s.repo == nil {
		return nil, ErrEditableFileRepositoryMissing
	}
	page, size = normalizeEditableFilePagination(page, size)
	items, total, err := s.repo.ListTasks(ctx, userID, ids, page, size)
	if err != nil {
		return nil, err
	}
	return &EditableFileTaskPage{Items: items, Total: total}, nil
}

func (s *EditableFileService) MarkRunning(ctx context.Context, id int64, accountID *int64) error {
	if s == nil || s.repo == nil {
		return ErrEditableFileRepositoryMissing
	}
	return s.repo.UpdateTaskStatus(ctx, id, EditableFileTaskStatusUpdate{
		Status:    EditableFileStatusRunning,
		AccountID: accountID,
	})
}

func (s *EditableFileService) MarkSucceeded(ctx context.Context, id int64, update EditableFileTaskStatusUpdate) error {
	if s == nil || s.repo == nil {
		return ErrEditableFileRepositoryMissing
	}
	update.Status = EditableFileStatusSucceeded
	return s.repo.UpdateTaskStatus(ctx, id, update)
}

func (s *EditableFileService) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	if s == nil || s.repo == nil {
		return ErrEditableFileRepositoryMissing
	}
	return s.repo.UpdateTaskStatus(ctx, id, EditableFileTaskStatusUpdate{
		Status: EditableFileStatusFailed,
		Error:  strings.TrimSpace(errMsg),
	})
}

func (s *EditableFileService) AddArtifact(ctx context.Context, input EditableFileArtifactInput) (*dbent.EditableFileArtifact, error) {
	if s == nil || s.repo == nil {
		return nil, ErrEditableFileRepositoryMissing
	}
	input.Kind = normalizeEditableFileArtifactKind(input.Kind)
	if input.Kind == "" {
		return nil, fmt.Errorf("editable file: invalid artifact kind")
	}
	input.FileName = strings.TrimSpace(input.FileName)
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.StorageKey = strings.TrimSpace(input.StorageKey)
	input.SHA256 = strings.TrimSpace(input.SHA256)
	if input.FileName == "" {
		return nil, fmt.Errorf("editable file: artifact filename is required")
	}
	if input.StorageKey == "" {
		return nil, fmt.Errorf("editable file: artifact storage key is required")
	}
	return s.repo.AddArtifact(ctx, input)
}

func (s *EditableFileService) ListArtifacts(ctx context.Context, taskID int64) ([]*dbent.EditableFileArtifact, error) {
	if s == nil || s.repo == nil {
		return nil, ErrEditableFileRepositoryMissing
	}
	return s.repo.ListArtifacts(ctx, taskID)
}

func normalizeEditableFileSubmitInput(input EditableFileSubmitInput) (EditableFileSubmitInput, error) {
	input.Kind = normalizeEditableFileKind(input.Kind)
	if input.Kind == "" {
		return input, ErrEditableFileInvalidKind
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" {
		return input, ErrEditableFilePromptRequired
	}
	input.Model = strings.TrimSpace(input.Model)
	input.ClientTaskID = strings.TrimSpace(input.ClientTaskID)
	input.Base64Images = normalizeEditableFileBase64Images(input.Base64Images)
	if input.Kind == EditableFileKindPSD && len(input.Base64Images) == 0 {
		return input, ErrEditableFilePSDImageRequired
	}
	return input, nil
}

func normalizeEditableFileKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "ppt", "pptx", "powerpoint":
		return EditableFileKindPPT
	case "psd", "photoshop":
		return EditableFileKindPSD
	default:
		return ""
	}
}

func normalizeEditableFileArtifactKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case EditableFileArtifactPrimaryKind, "file":
		return EditableFileArtifactPrimaryKind
	case EditableFileArtifactZipKind, "archive":
		return EditableFileArtifactZipKind
	case EditableFileArtifactInputKind, "reference":
		return EditableFileArtifactInputKind
	default:
		return ""
	}
}

func normalizeEditableFileBase64Images(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeEditableFilePagination(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
