package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

type editableFileRepoStub struct {
	createdInput     EditableFileSubmitInput
	createdExpiresAt time.Time
	createdTask      *dbent.EditableFileTask
	addedArtifact    EditableFileArtifactInput
}

func (r *editableFileRepoStub) CreateTask(_ context.Context, input EditableFileSubmitInput, expiresAt time.Time) (*dbent.EditableFileTask, error) {
	r.createdInput = input
	r.createdExpiresAt = expiresAt
	if r.createdTask != nil {
		return r.createdTask, nil
	}
	return &dbent.EditableFileTask{
		ID:           11,
		UserID:       input.UserID,
		Kind:         input.Kind,
		Status:       EditableFileStatusQueued,
		Prompt:       input.Prompt,
		Model:        input.Model,
		ClientTaskID: input.ClientTaskID,
		ExpiresAt:    &expiresAt,
	}, nil
}

func (r *editableFileRepoStub) GetTask(_ context.Context, _ int64) (*dbent.EditableFileTask, error) {
	return nil, errors.New("not implemented")
}

func (r *editableFileRepoStub) ListTasks(_ context.Context, _ int64, _ []int64, _, _ int) ([]*dbent.EditableFileTask, int, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *editableFileRepoStub) UpdateTaskStatus(_ context.Context, _ int64, _ EditableFileTaskStatusUpdate) error {
	return nil
}

func (r *editableFileRepoStub) AddArtifact(_ context.Context, input EditableFileArtifactInput) (*dbent.EditableFileArtifact, error) {
	r.addedArtifact = input
	return &dbent.EditableFileArtifact{
		ID:         21,
		TaskID:     input.TaskID,
		UserID:     input.UserID,
		Kind:       input.Kind,
		FileName:   input.FileName,
		MimeType:   input.MimeType,
		SizeBytes:  input.SizeBytes,
		StorageKey: input.StorageKey,
		Sha256:     input.SHA256,
	}, nil
}

func (r *editableFileRepoStub) ListArtifacts(_ context.Context, _ int64) ([]*dbent.EditableFileArtifact, error) {
	return nil, errors.New("not implemented")
}

func (r *editableFileRepoStub) GetArtifact(_ context.Context, _ int64) (*dbent.EditableFileArtifact, error) {
	return nil, errors.New("not implemented")
}

func TestEditableFileServiceSubmitPPTNormalizesAndSetsRetention(t *testing.T) {
	repo := &editableFileRepoStub{}
	svc := NewEditableFileService(repo)
	now := time.Date(2026, 6, 10, 1, 2, 3, 0, time.UTC)
	svc.now = func() time.Time { return now }

	task, err := svc.SubmitPPT(context.Background(), EditableFileSubmitInput{
		UserID:       7,
		Kind:         "psd",
		Prompt:       "  make a pitch deck  ",
		Model:        " gpt-image-2 ",
		ClientTaskID: " deck-1 ",
		Base64Images: []string{" ", "data:image/png;base64,AAAA"},
	})

	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, EditableFileKindPPT, repo.createdInput.Kind)
	require.Equal(t, "make a pitch deck", repo.createdInput.Prompt)
	require.Equal(t, "gpt-image-2", repo.createdInput.Model)
	require.Equal(t, "deck-1", repo.createdInput.ClientTaskID)
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, repo.createdInput.Base64Images)
	require.Equal(t, now.Add(EditableFileRetention), repo.createdExpiresAt)
}

func TestEditableFileServiceSubmitPSDRequiresReferenceImage(t *testing.T) {
	svc := NewEditableFileService(&editableFileRepoStub{})

	_, err := svc.SubmitPSD(context.Background(), EditableFileSubmitInput{
		UserID: 7,
		Prompt: "create layered psd",
	})

	require.ErrorIs(t, err, ErrEditableFilePSDImageRequired)
}

func TestEditableFileServiceAddArtifactNormalizesKind(t *testing.T) {
	repo := &editableFileRepoStub{}
	svc := NewEditableFileService(repo)

	artifact, err := svc.AddArtifact(context.Background(), EditableFileArtifactInput{
		TaskID:     11,
		UserID:     7,
		Kind:       "archive",
		FileName:   " deck-assets.zip ",
		MimeType:   " application/zip ",
		StorageKey: " editable/11/deck-assets.zip ",
		SHA256:     " abc123 ",
	})

	require.NoError(t, err)
	require.NotNil(t, artifact)
	require.Equal(t, EditableFileArtifactZipKind, repo.addedArtifact.Kind)
	require.Equal(t, "deck-assets.zip", repo.addedArtifact.FileName)
	require.Equal(t, "application/zip", repo.addedArtifact.MimeType)
	require.Equal(t, "editable/11/deck-assets.zip", repo.addedArtifact.StorageKey)
	require.Equal(t, "abc123", repo.addedArtifact.SHA256)
}

func TestEditableFileServiceAddArtifactRequiresStorageKey(t *testing.T) {
	svc := NewEditableFileService(&editableFileRepoStub{})

	_, err := svc.AddArtifact(context.Background(), EditableFileArtifactInput{
		TaskID:   11,
		UserID:   7,
		Kind:     EditableFileArtifactPrimaryKind,
		FileName: "deck.pptx",
	})

	require.ErrorContains(t, err, "artifact storage key is required")
}
