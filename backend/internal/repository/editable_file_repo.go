package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/editablefileartifact"
	"github.com/Wei-Shaw/sub2api/ent/editablefiletask"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type editableFileRepository struct {
	client *dbent.Client
}

func NewEditableFileRepository(client *dbent.Client) service.EditableFileRepository {
	return &editableFileRepository{client: client}
}

func (r *editableFileRepository) activeTaskQuery() *dbent.EditableFileTaskQuery {
	return r.client.EditableFileTask.Query().Where(editablefiletask.DeletedAtIsNil())
}

func (r *editableFileRepository) activeArtifactQuery() *dbent.EditableFileArtifactQuery {
	return r.client.EditableFileArtifact.Query().Where(editablefileartifact.DeletedAtIsNil())
}

func (r *editableFileRepository) CreateTask(ctx context.Context, input service.EditableFileSubmitInput, expiresAt time.Time) (*dbent.EditableFileTask, error) {
	client := clientFromContext(ctx, r.client)
	b := client.EditableFileTask.Create().
		SetUserID(input.UserID).
		SetKind(input.Kind).
		SetStatus(service.EditableFileStatusQueued).
		SetPrompt(input.Prompt).
		SetModel(input.Model).
		SetClientTaskID(input.ClientTaskID).
		SetExpiresAt(expiresAt)

	if input.GroupID != nil {
		b = b.SetGroupID(*input.GroupID)
	}
	if input.APIKeyID != nil {
		b = b.SetAPIKeyID(*input.APIKeyID)
	}
	if input.ImageConversationID != nil {
		b = b.SetImageConversationID(*input.ImageConversationID)
	}
	if input.SourceGenerationID != nil {
		b = b.SetSourceGenerationID(*input.SourceGenerationID)
	}
	return b.Save(ctx)
}

func (r *editableFileRepository) GetTask(ctx context.Context, id int64) (*dbent.EditableFileTask, error) {
	return r.activeTaskQuery().
		Where(editablefiletask.IDEQ(id)).
		Only(ctx)
}

func (r *editableFileRepository) ListTasks(ctx context.Context, userID int64, ids []int64, page, size int) ([]*dbent.EditableFileTask, int, error) {
	q := r.activeTaskQuery().Where(editablefiletask.UserIDEQ(userID))
	if len(ids) > 0 {
		q = q.Where(editablefiletask.IDIn(ids...))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	if offset < 0 {
		offset = 0
	}
	items, err := q.
		Order(dbent.Desc(editablefiletask.FieldID)).
		Offset(offset).
		Limit(size).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *editableFileRepository) UpdateTaskStatus(ctx context.Context, id int64, update service.EditableFileTaskStatusUpdate) error {
	client := clientFromContext(ctx, r.client)
	u := client.EditableFileTask.UpdateOneID(id).
		SetStatus(update.Status)

	if update.AccountID != nil {
		u = u.SetAccountID(*update.AccountID)
	}
	if update.ChatGPTConversationID != "" {
		u = u.SetChatgptConversationID(update.ChatGPTConversationID)
	}
	if update.Cost > 0 {
		u = u.SetCost(update.Cost)
	}
	if update.Error != "" {
		u = u.SetError(update.Error)
	}
	_, err := u.Save(ctx)
	return err
}

func (r *editableFileRepository) AddArtifact(ctx context.Context, input service.EditableFileArtifactInput) (*dbent.EditableFileArtifact, error) {
	client := clientFromContext(ctx, r.client)
	b := client.EditableFileArtifact.Create().
		SetTaskID(input.TaskID).
		SetUserID(input.UserID).
		SetKind(input.Kind).
		SetFileName(input.FileName).
		SetMimeType(input.MimeType).
		SetSizeBytes(input.SizeBytes).
		SetStorageKey(input.StorageKey).
		SetSha256(input.SHA256)

	if input.SourcePointer != nil {
		b = b.SetSourcePointer(*input.SourcePointer)
	}
	return b.Save(ctx)
}

func (r *editableFileRepository) ListArtifacts(ctx context.Context, taskID int64) ([]*dbent.EditableFileArtifact, error) {
	return r.activeArtifactQuery().
		Where(editablefileartifact.TaskIDEQ(taskID)).
		Order(dbent.Asc(editablefileartifact.FieldID)).
		All(ctx)
}

func (r *editableFileRepository) GetArtifact(ctx context.Context, id int64) (*dbent.EditableFileArtifact, error) {
	return r.activeArtifactQuery().
		Where(editablefileartifact.IDEQ(id)).
		Only(ctx)
}
