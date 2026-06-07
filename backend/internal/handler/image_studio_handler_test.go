//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs.
// ---------------------------------------------------------------------------

type studioGeneratorStub struct {
	result    *service.ImageStudioGenerateResult
	err       error
	gotUserID int64
	gotInput  service.ImageStudioGenerateInput
	calls     int
}

func (s *studioGeneratorStub) Generate(_ context.Context, userID int64, in service.ImageStudioGenerateInput) (*service.ImageStudioGenerateResult, error) {
	s.calls++
	s.gotUserID = userID
	s.gotInput = in
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type studioRepoStub struct {
	generation *dbent.ImageGeneration
	getGenErr  error
	delGenErr  error
	delGenCall int
}

func (s *studioRepoStub) CreateConversation(_ context.Context, _ int64, _ string) (*dbent.ImageConversation, error) {
	return nil, nil
}
func (s *studioRepoStub) ListConversations(_ context.Context, _ int64, _, _ int) ([]*dbent.ImageConversation, int, error) {
	return nil, 0, nil
}
func (s *studioRepoStub) GetConversation(_ context.Context, _ int64) (*dbent.ImageConversation, error) {
	return nil, nil
}
func (s *studioRepoStub) UpdateConversationTitle(_ context.Context, _ int64, _ string) error {
	return nil
}
func (s *studioRepoStub) DeleteConversation(_ context.Context, _ int64) error { return nil }
func (s *studioRepoStub) GetGeneration(_ context.Context, _ int64) (*dbent.ImageGeneration, error) {
	return s.generation, s.getGenErr
}
func (s *studioRepoStub) ListGenerations(_ context.Context, _ int64, _ *int64, _, _ int) ([]*dbent.ImageGeneration, int, error) {
	return nil, 0, nil
}
func (s *studioRepoStub) DeleteGeneration(_ context.Context, _ int64) error {
	s.delGenCall++
	return s.delGenErr
}

type studioStoreStub struct {
	data        []byte
	contentType string
	openErr     error
	openedKey   string
	deletedKeys []string
}

func (s *studioStoreStub) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.openedKey = key
	if s.openErr != nil {
		return nil, "", s.openErr
	}
	return io.NopCloser(bytes.NewReader(s.data)), s.contentType, nil
}
func (s *studioStoreStub) Delete(_ context.Context, key string) error {
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func newStudioContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
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

func setStudioAuth(c *gin.Context, userID int64) {
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})
}

// ---------------------------------------------------------------------------
// Generate.
// ---------------------------------------------------------------------------

func TestImageStudioGenerate_Unauthenticated401(t *testing.T) {
	gen := &studioGeneratorStub{}
	h := &ImageStudioHandler{studio: gen, repo: &studioRepoStub{}, store: &studioStoreStub{}}
	w, c := newStudioContext(http.MethodPost, "/generate", `{"group_id":1,"prompt":"a cat"}`)
	h.Generate(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, 0, gen.calls)
}

func TestImageStudioGenerate_MissingPrompt400(t *testing.T) {
	gen := &studioGeneratorStub{}
	h := &ImageStudioHandler{studio: gen, repo: &studioRepoStub{}, store: &studioStoreStub{}}
	w, c := newStudioContext(http.MethodPost, "/generate", `{"group_id":1}`)
	setStudioAuth(c, 7)
	h.Generate(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, 0, gen.calls)
}

func TestImageStudioGenerate_PassesJWTUserAndBuildsURLs(t *testing.T) {
	gen := &studioGeneratorStub{
		result: &service.ImageStudioGenerateResult{
			GenerationID:   42,
			ConversationID: 9,
			Images:         []string{"user_7/42/0.png", "user_7/42/1.png"},
			Cost:           0.08,
			Balance:        4.92,
		},
	}
	h := &ImageStudioHandler{studio: gen, repo: &studioRepoStub{}, store: &studioStoreStub{}}
	body := `{"conversation_id":9,"group_id":3,"prompt":"a fox","model":"gpt-image-2","size":"1024x1024","quality":"high","n":2}`
	w, c := newStudioContext(http.MethodPost, "/generate", body)
	setStudioAuth(c, 7)

	h.Generate(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, gen.calls)
	// The acting user is the JWT subject, never the body.
	require.Equal(t, int64(7), gen.gotUserID)
	require.Equal(t, int64(3), gen.gotInput.GroupID)
	require.NotNil(t, gen.gotInput.ConversationID)
	require.Equal(t, int64(9), *gen.gotInput.ConversationID)
	require.Equal(t, 2, gen.gotInput.N)

	var env struct {
		Data generateImageResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, int64(42), env.Data.GenerationID)
	require.Equal(t, int64(9), env.Data.ConversationID)
	require.Equal(t, 0.08, env.Data.Cost)
	require.Equal(t, 4.92, env.Data.Balance)
	require.Equal(t, []string{
		"/api/v1/user/image-studio/assets/42/0",
		"/api/v1/user/image-studio/assets/42/1",
	}, env.Data.Images)
}

func TestImageStudioGenerate_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"group not allowed -> 403", service.ErrImageStudioGroupNotAllowed, http.StatusForbidden},
		{"image gen disabled -> 403", service.ErrImageStudioImageGenerationDisabled, http.StatusForbidden},
		{"busy -> 429", service.ErrImageStudioBusy, http.StatusTooManyRequests},
		{"insufficient balance -> 400", service.ErrInsufficientBalance, http.StatusBadRequest},
		{"subscription not found -> 404", service.ErrSubscriptionNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := &studioGeneratorStub{err: tc.err}
			h := &ImageStudioHandler{studio: gen, repo: &studioRepoStub{}, store: &studioStoreStub{}}
			w, c := newStudioContext(http.MethodPost, "/generate", `{"group_id":1,"prompt":"x"}`)
			setStudioAuth(c, 7)
			h.Generate(c)
			require.Equal(t, tc.want, w.Code, "error %v", tc.err)
		})
	}
}

// ---------------------------------------------------------------------------
// Assets ownership.
// ---------------------------------------------------------------------------

func TestImageStudioGetAsset_OtherUser404(t *testing.T) {
	repo := &studioRepoStub{generation: &dbent.ImageGeneration{
		ID:          5,
		UserID:      99, // owned by someone else
		StorageKeys: []string{"user_99/5/0.png"},
	}}
	store := &studioStoreStub{data: []byte("png-bytes"), contentType: "image/png"}
	h := &ImageStudioHandler{studio: &studioGeneratorStub{}, repo: repo, store: store}

	w, c := newStudioContext(http.MethodGet, "/assets/5/0", "")
	c.Params = gin.Params{{Key: "genID", Value: "5"}, {Key: "idx", Value: "0"}}
	setStudioAuth(c, 7) // requesting user 7, not the owner

	h.GetAsset(c)

	require.Equal(t, http.StatusNotFound, w.Code)
	// The store must never be opened for a non-owned asset.
	require.Empty(t, store.openedKey)
}

func TestImageStudioGetAsset_OwnerStreamsBytes(t *testing.T) {
	repo := &studioRepoStub{generation: &dbent.ImageGeneration{
		ID:          5,
		UserID:      7,
		StorageKeys: []string{"user_7/5/0.png"},
	}}
	store := &studioStoreStub{data: []byte("the-png-bytes"), contentType: "image/png"}
	h := &ImageStudioHandler{studio: &studioGeneratorStub{}, repo: repo, store: store}

	w, c := newStudioContext(http.MethodGet, "/assets/5/0", "")
	c.Params = gin.Params{{Key: "genID", Value: "5"}, {Key: "idx", Value: "0"}}
	setStudioAuth(c, 7)

	h.GetAsset(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "user_7/5/0.png", store.openedKey)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, []byte("the-png-bytes"), w.Body.Bytes())
}

func TestImageStudioGetAsset_IndexOutOfRange404(t *testing.T) {
	repo := &studioRepoStub{generation: &dbent.ImageGeneration{
		ID:          5,
		UserID:      7,
		StorageKeys: []string{"user_7/5/0.png"},
	}}
	store := &studioStoreStub{}
	h := &ImageStudioHandler{studio: &studioGeneratorStub{}, repo: repo, store: store}

	w, c := newStudioContext(http.MethodGet, "/assets/5/9", "")
	c.Params = gin.Params{{Key: "genID", Value: "5"}, {Key: "idx", Value: "9"}}
	setStudioAuth(c, 7)

	h.GetAsset(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Empty(t, store.openedKey)
}

// ---------------------------------------------------------------------------
// Delete generation ownership.
// ---------------------------------------------------------------------------

func TestImageStudioDeleteGeneration_OtherUser404(t *testing.T) {
	repo := &studioRepoStub{generation: &dbent.ImageGeneration{ID: 5, UserID: 99, StorageKeys: []string{"k"}}}
	store := &studioStoreStub{}
	h := &ImageStudioHandler{studio: &studioGeneratorStub{}, repo: repo, store: store}

	w, c := newStudioContext(http.MethodDelete, "/generations/5", "")
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setStudioAuth(c, 7)

	h.DeleteGeneration(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, 0, repo.delGenCall)
	require.Empty(t, store.deletedKeys)
}

func TestImageStudioDeleteGeneration_OwnerDeletesFilesAndRow(t *testing.T) {
	repo := &studioRepoStub{generation: &dbent.ImageGeneration{ID: 5, UserID: 7, StorageKeys: []string{"a", "b"}}}
	store := &studioStoreStub{}
	h := &ImageStudioHandler{studio: &studioGeneratorStub{}, repo: repo, store: store}

	w, c := newStudioContext(http.MethodDelete, "/generations/5", "")
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setStudioAuth(c, 7)

	h.DeleteGeneration(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.delGenCall)
	require.ElementsMatch(t, []string{"a", "b"}, store.deletedKeys)
}
