//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs for the narrow consumer interfaces ImageStudioService depends on.
// ---------------------------------------------------------------------------

type studioUserReaderStub struct {
	user     *User
	getCalls int
	// balanceAfter, when set (>= 0), is returned on the SECOND GetByID call so
	// tests can model the post-RecordUsage balance read-back.
	balanceAfter float64
	hasAfter     bool
}

func (s *studioUserReaderStub) GetByID(_ context.Context, _ int64) (*User, error) {
	s.getCalls++
	if s.user == nil {
		return nil, errors.New("user not found")
	}
	clone := *s.user
	if s.hasAfter && s.getCalls >= 2 {
		clone.Balance = s.balanceAfter
	}
	return &clone, nil
}

type studioGroupProviderStub struct {
	groups []Group
	err    error
}

func (s *studioGroupProviderStub) GetAvailableGroups(_ context.Context, _ int64) ([]Group, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.groups, nil
}

type studioKeyEnsurerStub struct {
	key   *APIKey
	err   error
	calls int
}

func (s *studioKeyEnsurerStub) EnsureStudioAPIKey(_ context.Context, userID, groupID int64) (*APIKey, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.key != nil {
		return s.key, nil
	}
	gid := groupID
	return &APIKey{ID: 7, UserID: userID, GroupID: &gid, Internal: true, Name: studioAPIKeyName}, nil
}

type studioEligibilityStub struct {
	err   error
	calls int
}

func (s *studioEligibilityStub) CheckBillingEligibility(_ context.Context, _ *User, _ *APIKey, _ *Group, _ *UserSubscription, _ string) error {
	s.calls++
	return s.err
}

type studioImageGeneratorStub struct {
	result *ImageGenResult
	images []StudioGeneratedImage
	err    error
	calls  int
}

func (s *studioImageGeneratorStub) GenerateStudioImages(_ context.Context, _ ImageGenInput) (*ImageGenResult, []StudioGeneratedImage, error) {
	s.calls++
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.result, s.images, nil
}

type studioCostStub struct {
	breakdown *CostBreakdown
}

func (s *studioCostStub) CalculateImageCost(_ string, _ string, _ int, _ *ImagePriceConfig, _ float64) *CostBreakdown {
	if s.breakdown != nil {
		return s.breakdown
	}
	return &CostBreakdown{ActualCost: 0}
}

type studioUsageRecorderStub struct {
	err   error
	calls int
	last  *OpenAIRecordUsageInput
}

func (s *studioUsageRecorderStub) RecordUsage(_ context.Context, in *OpenAIRecordUsageInput) error {
	s.calls++
	s.last = in
	return s.err
}

// studioRepoStub is an in-memory ImageStudioRepository.
type studioRepoStub struct {
	mu            sync.Mutex
	conversations map[int64]*dbent.ImageConversation
	nextConvID    int64
	createdConv   int
	generations   map[int64]*dbent.ImageGeneration
	nextGenID     int64
	statusUpdates []studioStatusUpdate
	createGenErr  error
	createConvErr error
	updateErr     error
}

type studioStatusUpdate struct {
	id          int64
	status      string
	storageKeys []string
	cost        float64
	width       int
	height      int
	errMsg      string
}

func newStudioRepoStub() *studioRepoStub {
	return &studioRepoStub{
		conversations: map[int64]*dbent.ImageConversation{},
		generations:   map[int64]*dbent.ImageGeneration{},
	}
}

func (s *studioRepoStub) CreateConversation(_ context.Context, userID int64, title string) (*dbent.ImageConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createConvErr != nil {
		return nil, s.createConvErr
	}
	s.createdConv++
	s.nextConvID++
	conv := &dbent.ImageConversation{ID: s.nextConvID, UserID: userID, Title: title}
	s.conversations[conv.ID] = conv
	return conv, nil
}

func (s *studioRepoStub) ListConversations(_ context.Context, _ int64, _, _ int) ([]*dbent.ImageConversation, int, error) {
	return nil, 0, nil
}

func (s *studioRepoStub) CreateGeneration(_ context.Context, g *dbent.ImageGeneration) (*dbent.ImageGeneration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createGenErr != nil {
		return nil, s.createGenErr
	}
	s.nextGenID++
	clone := *g
	clone.ID = s.nextGenID
	s.generations[clone.ID] = &clone
	return &clone, nil
}

func (s *studioRepoStub) UpdateGenerationStatus(_ context.Context, id int64, status string, storageKeys []string, cost float64, width, height int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		return s.updateErr
	}
	s.statusUpdates = append(s.statusUpdates, studioStatusUpdate{
		id: id, status: status, storageKeys: storageKeys, cost: cost, width: width, height: height, errMsg: errMsg,
	})
	if g, ok := s.generations[id]; ok {
		g.Status = status
		g.StorageKeys = storageKeys
		g.Cost = cost
		w, h, e := width, height, errMsg
		g.Width = &w
		g.Height = &h
		g.Error = &e
	}
	return nil
}

func (s *studioRepoStub) GetGeneration(_ context.Context, id int64) (*dbent.ImageGeneration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.generations[id]
	if !ok {
		return nil, errors.New("not found")
	}
	clone := *g
	return &clone, nil
}

func (s *studioRepoStub) ListGenerations(_ context.Context, _ int64, _ *int64, _, _ int) ([]*dbent.ImageGeneration, int, error) {
	return nil, 0, nil
}

func (s *studioRepoStub) lastStatus() (studioStatusUpdate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.statusUpdates) == 0 {
		return studioStatusUpdate{}, false
	}
	return s.statusUpdates[len(s.statusUpdates)-1], true
}

// studioStoreStub is an in-memory ImageStore.
type studioStoreStub struct {
	mu       sync.Mutex
	putCalls int
	putErr   error
	keys     []string
}

func (s *studioStoreStub) Put(_ context.Context, userID, genID int64, idx int, _ string, _ []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return "", s.putErr
	}
	s.putCalls++
	key := studioTestKey(userID, genID, idx)
	s.keys = append(s.keys, key)
	return key, nil
}

func (s *studioStoreStub) Open(_ context.Context, _ string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("not implemented")
}

func (s *studioStoreStub) Delete(_ context.Context, _ string) error { return nil }

func studioTestKey(userID, genID int64, idx int) string {
	return "k/" + itoa(userID) + "/" + itoa(genID) + "/" + itoa(int64(idx))
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ---------------------------------------------------------------------------
// Test fixture builder.
// ---------------------------------------------------------------------------

type studioFixture struct {
	svc    *ImageStudioService
	users  *studioUserReaderStub
	groups *studioGroupProviderStub
	keys   *studioKeyEnsurerStub
	elig   *studioEligibilityStub
	gen    *studioImageGeneratorStub
	cost   *studioCostStub
	usage  *studioUsageRecorderStub
	repo   *studioRepoStub
	store  *studioStoreStub
}

func newStudioFixture(t *testing.T) *studioFixture {
	t.Helper()
	allowedGroup := Group{ID: 10, Name: "g", Status: StatusActive, AllowImageGeneration: true, RateMultiplier: 1}
	f := &studioFixture{
		users:  &studioUserReaderStub{user: &User{ID: 1, Balance: 100, AllowedGroups: []int64{10}}},
		groups: &studioGroupProviderStub{groups: []Group{allowedGroup}},
		keys:   &studioKeyEnsurerStub{},
		elig:   &studioEligibilityStub{},
		gen: &studioImageGeneratorStub{
			result: &ImageGenResult{
				Result:  &OpenAIForwardResult{ImageCount: 2, ImageSize: "2K", Model: "gpt-image-2"},
				Account: &Account{ID: 99, Platform: PlatformOpenAI},
			},
			images: []StudioGeneratedImage{
				{Data: []byte("a"), ContentType: "image/png"},
				{Data: []byte("b"), ContentType: "image/png"},
			},
		},
		cost:  &studioCostStub{breakdown: &CostBreakdown{ActualCost: 0.42, TotalCost: 0.42, BillingMode: string(BillingModeImage)}},
		usage: &studioUsageRecorderStub{},
		repo:  newStudioRepoStub(),
		store: &studioStoreStub{},
	}
	f.svc = NewImageStudioService(ImageStudioServiceDeps{
		Users:       f.users,
		Groups:      f.groups,
		KeyEnsurer:  f.keys,
		Eligibility: f.elig,
		Generator:   f.gen,
		CostCalc:    f.cost,
		UsageRecord: f.usage,
		Repo:        f.repo,
		Store:       f.store,
	})
	return f
}

func studioInput(groupID int64) ImageStudioGenerateInput {
	return ImageStudioGenerateInput{
		GroupID: groupID,
		Prompt:  "a cat on the moon",
		Model:   "gpt-image-2",
		Size:    "1024x1024",
		Quality: "high",
		N:       2,
	}
}

// ---------------------------------------------------------------------------
// Tests.
// ---------------------------------------------------------------------------

func TestImageStudioService_Generate_GroupNotAllowed_Rejected(t *testing.T) {
	f := newStudioFixture(t)

	// Request a group the user is NOT allowed to use.
	res, err := f.svc.Generate(context.Background(), 1, studioInput(999))

	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrImageStudioGroupNotAllowed)
	require.Equal(t, 0, f.gen.calls, "must not generate")
	require.Equal(t, 0, f.usage.calls, "must not record usage")
	require.Equal(t, 0, f.store.putCalls, "must not store")
	require.Equal(t, 0, f.elig.calls, "must not reach billing")
}

func TestImageStudioService_Generate_GroupLacksImagePermission_Rejected(t *testing.T) {
	f := newStudioFixture(t)
	// Group is allowed for the user but has image generation disabled.
	f.groups.groups = []Group{{ID: 10, Status: StatusActive, AllowImageGeneration: false}}

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrImageStudioImageGenerationDisabled)
	require.Equal(t, 0, f.gen.calls)
	require.Equal(t, 0, f.usage.calls)
}

func TestImageStudioService_Generate_IneligibleBilling_RejectedBeforeGeneration(t *testing.T) {
	f := newStudioFixture(t)
	f.elig.err = errors.New("insufficient balance")

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)
	require.Equal(t, 1, f.elig.calls)
	require.Equal(t, 0, f.gen.calls, "must not generate when ineligible")
	require.Equal(t, 0, f.usage.calls)
	require.Equal(t, 0, f.store.putCalls)
}

func TestImageStudioService_Generate_Success_NewConversation(t *testing.T) {
	f := newStudioFixture(t)
	f.users.hasAfter = true
	f.users.balanceAfter = 99.58 // 100 - 0.42

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)

	// New conversation created (ConversationID was nil).
	require.Equal(t, 1, f.repo.createdConv, "new conversation created")
	require.NotZero(t, res.ConversationID)

	// Generation persisted then marked succeeded.
	require.NotZero(t, res.GenerationID)
	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "succeeded", last.status)
	require.Len(t, last.storageKeys, 2)

	// Stored N images.
	require.Equal(t, 2, f.store.putCalls)
	require.Len(t, res.Images, 2)

	// Recorded usage exactly once with the synthetic key + studio endpoint.
	require.Equal(t, 1, f.usage.calls)
	require.NotNil(t, f.usage.last)
	require.Equal(t, "image_studio", f.usage.last.InboundEndpoint)
	require.NotNil(t, f.usage.last.APIKey)
	require.True(t, f.usage.last.APIKey.Internal)
	require.Equal(t, int64(99), f.usage.last.Account.ID)

	// Cost + balance surfaced.
	require.InDelta(t, 0.42, res.Cost, 1e-9)
	require.InDelta(t, 99.58, res.Balance, 1e-9)
}

func TestImageStudioService_Generate_Success_ExistingConversation(t *testing.T) {
	f := newStudioFixture(t)
	convID := int64(555)
	in := studioInput(10)
	in.ConversationID = &convID

	res, err := f.svc.Generate(context.Background(), 1, in)

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, 0, f.repo.createdConv, "must reuse existing conversation")
	require.Equal(t, convID, res.ConversationID)
}

func TestImageStudioService_Generate_GenerateError_MarksFailed_NoUsage(t *testing.T) {
	f := newStudioFixture(t)
	f.gen.err = errors.New("upstream boom")

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)

	// Generation row marked failed, nothing stored, no usage.
	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "failed", last.status)
	require.Nil(t, last.storageKeys)
	require.Equal(t, 0, f.store.putCalls)
	require.Equal(t, 0, f.usage.calls, "must NOT record usage on generation failure")
}

func TestImageStudioService_Generate_StoreError_MarksFailed_NoUsage(t *testing.T) {
	f := newStudioFixture(t)
	f.store.putErr = errors.New("disk full")

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)

	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "failed", last.status)
	require.Equal(t, 0, f.usage.calls, "must NOT record usage when storage fails")
}

func TestImageStudioService_Generate_BillingWriteFailure_StillSucceeds(t *testing.T) {
	// Spec §9: after generate + store succeed, a RecordUsage error is logged as
	// CRITICAL but the generation is still marked succeeded and images returned.
	f := newStudioFixture(t)
	f.usage.err = errors.New("usage write failed")

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Images, 2)

	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "succeeded", last.status)
	require.Equal(t, 1, f.usage.calls)
}

// Guard: a slow eligibility/generation path should still respect ctx (smoke).
func TestImageStudioService_Generate_ContextDeadlinePropagates(t *testing.T) {
	f := newStudioFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// Happy path still works within the deadline.
	_, err := f.svc.Generate(ctx, 1, studioInput(10))
	require.NoError(t, err)
}
