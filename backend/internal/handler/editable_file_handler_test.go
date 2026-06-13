//go:build unit

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type editableFileServiceStub struct {
	submitPPTInput service.EditableFileSubmitInput
	submitPSDInput service.EditableFileSubmitInput
	submitPPTTask  *dbent.EditableFileTask
	submitPSDTask  *dbent.EditableFileTask
	submitPPTErr   error
	submitPSDErr   error
	task           *dbent.EditableFileTask
	getErr         error
	artifacts      []*dbent.EditableFileArtifact
}

func (s *editableFileServiceStub) SubmitPPT(_ context.Context, input service.EditableFileSubmitInput) (*dbent.EditableFileTask, error) {
	s.submitPPTInput = input
	if s.submitPPTErr != nil {
		return nil, s.submitPPTErr
	}
	return s.submitPPTTask, nil
}

func (s *editableFileServiceStub) SubmitPSD(_ context.Context, input service.EditableFileSubmitInput) (*dbent.EditableFileTask, error) {
	s.submitPSDInput = input
	if s.submitPSDErr != nil {
		return nil, s.submitPSDErr
	}
	return s.submitPSDTask, nil
}

func (s *editableFileServiceStub) GetTask(_ context.Context, _ int64) (*dbent.EditableFileTask, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.task, nil
}

func (s *editableFileServiceStub) ListTasks(_ context.Context, _ int64, _ []int64, _, _ int) (*service.EditableFileTaskPage, error) {
	return &service.EditableFileTaskPage{Items: []*dbent.EditableFileTask{s.task}, Total: 1}, nil
}

func (s *editableFileServiceStub) ListArtifacts(_ context.Context, _ int64) ([]*dbent.EditableFileArtifact, error) {
	return s.artifacts, nil
}

func newEditableFileContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return w, c
}

func setEditableFileAuth(c *gin.Context, userID int64) {
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})
}

func TestEditableFileCreateTaskSubmitsPPTForJWTUser(t *testing.T) {
	now := time.Date(2026, 6, 10, 3, 0, 0, 0, time.UTC)
	expires := now.Add(service.EditableFileRetention)
	svc := &editableFileServiceStub{
		submitPPTTask: &dbent.EditableFileTask{
			ID:        42,
			UserID:    7,
			Kind:      service.EditableFileKindPPT,
			Status:    service.EditableFileStatusQueued,
			Prompt:    "make a deck",
			Model:     "gpt-image-2",
			CreatedAt: now,
			UpdatedAt: now,
			ExpiresAt: &expires,
		},
	}
	h := &EditableFileHandler{service: svc}
	w, c := newEditableFileContext(http.MethodPost, "/tasks", `{"kind":"ppt","prompt":"make a deck","model":"gpt-image-2","images":["data:image/png;base64,AAAA"]}`)
	setEditableFileAuth(c, 7)

	h.CreateTask(c)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, int64(7), svc.submitPPTInput.UserID)
	require.Equal(t, "make a deck", svc.submitPPTInput.Prompt)
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, svc.submitPPTInput.Base64Images)
	require.Equal(t, int64(42), gjson.GetBytes(w.Body.Bytes(), "data.id").Int())
	require.Equal(t, service.EditableFileStatusQueued, gjson.GetBytes(w.Body.Bytes(), "data.status").String())
}

func TestEditableFileCreateTaskPSDMissingReferenceReturns400(t *testing.T) {
	svc := &editableFileServiceStub{submitPSDErr: service.ErrEditableFilePSDImageRequired}
	h := &EditableFileHandler{service: svc}
	w, c := newEditableFileContext(http.MethodPost, "/tasks", `{"kind":"psd","prompt":"make psd"}`)
	setEditableFileAuth(c, 7)

	h.CreateTask(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, gjson.GetBytes(w.Body.Bytes(), "message").String(), "reference image")
}

func TestEditableFileGetTaskHidesOtherUsersTask(t *testing.T) {
	svc := &editableFileServiceStub{task: &dbent.EditableFileTask{ID: 42, UserID: 99}}
	h := &EditableFileHandler{service: svc}
	w, c := newEditableFileContext(http.MethodGet, "/tasks/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	setEditableFileAuth(c, 7)

	h.GetTask(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestEditableFileListArtifactsReturnsTaskArtifacts(t *testing.T) {
	now := time.Date(2026, 6, 10, 3, 0, 0, 0, time.UTC)
	svc := &editableFileServiceStub{
		task: &dbent.EditableFileTask{ID: 42, UserID: 7},
		artifacts: []*dbent.EditableFileArtifact{{
			ID:         9,
			TaskID:     42,
			UserID:     7,
			Kind:       service.EditableFileArtifactPrimaryKind,
			FileName:   "deck.pptx",
			MimeType:   "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			SizeBytes:  123,
			StorageKey: "editable/42/deck.pptx",
			CreatedAt:  now,
		}},
	}
	h := &EditableFileHandler{service: svc}
	w, c := newEditableFileContext(http.MethodGet, "/tasks/42/artifacts", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	setEditableFileAuth(c, 7)

	h.ListArtifacts(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "deck.pptx", gjson.GetBytes(w.Body.Bytes(), "data.0.file_name").String())
	require.Equal(t, "editable/42/deck.pptx", gjson.GetBytes(w.Body.Bytes(), "data.0.storage_key").String())
}

func TestEditableFileGetTaskMissingReturns404(t *testing.T) {
	svc := &editableFileServiceStub{getErr: errors.New("not found")}
	h := &EditableFileHandler{service: svc}
	w, c := newEditableFileContext(http.MethodGet, "/tasks/42", "")
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	setEditableFileAuth(c, 7)

	h.GetTask(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}
