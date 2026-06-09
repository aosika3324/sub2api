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
	"github.com/Wei-Shaw/sub2api/internal/config"
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
	last   ImageGenInput
	block  <-chan struct{}
	done   chan struct{}
}

func (s *studioImageGeneratorStub) GenerateStudioImages(_ context.Context, in ImageGenInput) (*ImageGenResult, []StudioGeneratedImage, error) {
	s.calls++
	s.last = in
	defer func() {
		if s.done != nil {
			close(s.done)
		}
	}()
	if s.block != nil {
		<-s.block
	}
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.result, s.images, nil
}

type studioCostStub struct {
	breakdown *CostBreakdown
	calls     int
	last      *OpenAIForwardResult
}

func (s *studioCostStub) ComputeImageCostBreakdown(_ context.Context, _ *User, _ *APIKey, result *OpenAIForwardResult) *CostBreakdown {
	s.calls++
	s.last = result
	if s.breakdown != nil {
		return s.breakdown
	}
	return &CostBreakdown{ActualCost: 0}
}

type studioSubscriptionStub struct {
	sub   *UserSubscription
	err   error
	calls int
}

func (s *studioSubscriptionStub) GetActiveSubscription(_ context.Context, _, _ int64) (*UserSubscription, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.sub, nil
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
	// Input-key persistence tracking (edits path).
	setInputCalls int
	lastInputKeys []string
	setInputErr   error
	getGenSignal  chan struct{}
}

type studioStatusUpdate struct {
	id          int64
	status      string
	storageKeys []string
	cost        float64
	imageCount  int
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

func (s *studioRepoStub) UpdateGenerationStatus(_ context.Context, id int64, status string, storageKeys []string, cost float64, imageCount, width, height int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		return s.updateErr
	}
	s.statusUpdates = append(s.statusUpdates, studioStatusUpdate{
		id: id, status: status, storageKeys: storageKeys, cost: cost, imageCount: imageCount, width: width, height: height, errMsg: errMsg,
	})
	if g, ok := s.generations[id]; ok {
		g.Status = status
		g.StorageKeys = storageKeys
		g.Cost = cost
		g.ImageCount = imageCount
		w, h, e := width, height, errMsg
		g.Width = &w
		g.Height = &h
		g.Error = &e
	}
	return nil
}

func (s *studioRepoStub) SetInputStorageKeys(_ context.Context, id int64, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setInputErr != nil {
		return s.setInputErr
	}
	s.setInputCalls++
	s.lastInputKeys = keys
	if g, ok := s.generations[id]; ok {
		g.InputStorageKeys = keys
	}
	return nil
}

func (s *studioRepoStub) GetGeneration(_ context.Context, id int64) (*dbent.ImageGeneration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getGenSignal != nil {
		select {
		case <-s.getGenSignal:
		default:
			close(s.getGenSignal)
		}
	}
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

func (s *studioRepoStub) GetConversation(_ context.Context, id int64) (*dbent.ImageConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.conversations[id]; ok {
		clone := *c
		return &clone, nil
	}
	return nil, errors.New("not found")
}

func (s *studioRepoStub) UpdateConversationTitle(_ context.Context, _ int64, _ string) error {
	return nil
}

func (s *studioRepoStub) DeleteConversation(_ context.Context, _ int64) error {
	return nil
}

func (s *studioRepoStub) DeleteConversationCascade(_ context.Context, _ int64) ([]string, error) {
	return nil, nil
}

func (s *studioRepoStub) ClearUserHistory(_ context.Context, _ int64) ([]string, error) {
	return nil, nil
}

func (s *studioRepoStub) DeleteGeneration(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.generations, id)
	return nil
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
	mu          sync.Mutex
	putCalls    int
	putErr      error
	keys        []string
	deletedKeys []string
	// failPutAfter, when > 0, makes the (failPutAfter+1)-th Put (0-based: the Put
	// at idx == failPutAfter) fail, exercising the partial-failure cleanup path.
	failPutAfter int
	// Input (reference) image tracking for the edits path.
	putInputCalls int
	putInputErr   error
	inputKeys     []string
}

func (s *studioStoreStub) Put(_ context.Context, userID, genID int64, idx int, _ string, _ []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return "", s.putErr
	}
	if s.failPutAfter > 0 && idx == s.failPutAfter {
		return "", errors.New("disk full at idx")
	}
	s.putCalls++
	key := studioTestKey(userID, genID, idx)
	s.keys = append(s.keys, key)
	return key, nil
}

func (s *studioStoreStub) PutInput(_ context.Context, userID, genID int64, idx int, _ string, _ []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putInputErr != nil {
		return "", s.putInputErr
	}
	s.putInputCalls++
	key := "input/" + studioTestKey(userID, genID, idx)
	s.inputKeys = append(s.inputKeys, key)
	return key, nil
}

func (s *studioStoreStub) Open(_ context.Context, _ string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("not implemented")
}

func (s *studioStoreStub) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

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
	subs   *studioSubscriptionStub
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
		subs:  &studioSubscriptionStub{},
		repo:  newStudioRepoStub(),
		store: &studioStoreStub{},
	}
	f.svc = NewImageStudioService(ImageStudioServiceDeps{
		Users:         f.users,
		Groups:        f.groups,
		KeyEnsurer:    f.keys,
		Eligibility:   f.elig,
		Generator:     f.gen,
		CostResolver:  f.cost,
		UsageRecord:   f.usage,
		Subscriptions: f.subs,
		Repo:          f.repo,
		Store:         f.store,
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
	require.Equal(t, 2, last.imageCount, "imageCount must equal len(images)")

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

func TestImageStudioService_StartGenerate_ReturnsPendingAndCompletesInBackground(t *testing.T) {
	f := newStudioFixture(t)
	f.users.hasAfter = true
	f.users.balanceAfter = 99.58
	releaseGeneration := make(chan struct{})
	f.gen.block = releaseGeneration

	res, err := f.svc.StartGenerate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotZero(t, res.GenerationID)
	require.NotZero(t, res.ConversationID)
	require.Equal(t, 0, f.store.putCalls, "StartGenerate returns before output images are stored")
	require.Equal(t, 0, f.usage.calls, "StartGenerate returns before billing writes")

	close(releaseGeneration)
	require.Eventually(t, func() bool {
		last, ok := f.repo.lastStatus()
		return ok && last.id == res.GenerationID && last.status == "succeeded" && last.imageCount == 2
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, 2, f.store.putCalls)
	require.Equal(t, 1, f.usage.calls)
}

func TestImageStudioService_StartGenerate_DeletedPendingDoesNotStoreOrBill(t *testing.T) {
	f := newStudioFixture(t)
	statusChecked := make(chan struct{})
	f.repo.getGenSignal = statusChecked

	limiter := &ImageConcurrencyLimiter{}
	cfg := &config.Config{}
	cfg.Gateway.ImageConcurrency.Enabled = true
	cfg.Gateway.ImageConcurrency.MaxConcurrentRequests = 1
	cfg.Gateway.ImageConcurrency.OverflowMode = config.ImageConcurrencyOverflowModeWait
	cfg.Gateway.ImageConcurrency.WaitTimeoutSeconds = 5
	cfg.Gateway.ImageConcurrency.MaxWaitingRequests = 1
	f.svc = NewImageStudioService(ImageStudioServiceDeps{
		Users:         f.users,
		Groups:        f.groups,
		KeyEnsurer:    f.keys,
		Eligibility:   f.elig,
		Generator:     f.gen,
		CostResolver:  f.cost,
		UsageRecord:   f.usage,
		Subscriptions: f.subs,
		Repo:          f.repo,
		Store:         f.store,
		Limiter:       limiter,
		Cfg:           cfg,
	})

	releaseSlot, ok := limiter.Acquire(context.Background(), true, 1, false, 0, 0)
	require.True(t, ok)
	defer releaseSlot()

	res, err := f.svc.StartGenerate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NoError(t, f.repo.DeleteGeneration(context.Background(), res.GenerationID))

	releaseSlot()
	select {
	case <-statusChecked:
	case <-time.After(time.Second):
		t.Fatal("generation status was not checked")
	}
	require.Equal(t, 0, f.gen.calls, "deleted pending job must not call upstream generator")
	require.Equal(t, 0, f.store.putCalls)
	require.Equal(t, 0, f.usage.calls)
}

func TestImageStudioService_Generate_ModeReferenceRules(t *testing.T) {
	cases := []struct {
		name string
		in   ImageStudioGenerateInput
	}{
		{
			name: "generate mode rejects references",
			in: func() ImageStudioGenerateInput {
				in := studioEditsInput(10)
				in.Mode = ImageStudioModeGenerate
				return in
			}(),
		},
		{
			name: "edit mode requires one reference",
			in: func() ImageStudioGenerateInput {
				in := studioInput(10)
				in.Mode = ImageStudioModeEdit
				return in
			}(),
		},
		{
			name: "edit mode rejects multiple references",
			in: func() ImageStudioGenerateInput {
				in := studioMultiEditsInput(10)
				in.Mode = ImageStudioModeEdit
				return in
			}(),
		},
		{
			name: "compose mode requires two references",
			in: func() ImageStudioGenerateInput {
				in := studioEditsInput(10)
				in.Mode = ImageStudioModeCompose
				return in
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStudioFixture(t)
			res, err := f.svc.Generate(context.Background(), 1, tc.in)

			require.Error(t, err)
			require.Nil(t, res)
			require.ErrorIs(t, err, ErrImageStudioInvalidMode)
			require.Equal(t, 0, f.repo.createdConv)
			require.Equal(t, 0, f.gen.calls)
			require.Equal(t, 0, f.store.putCalls)
			require.Equal(t, 0, f.usage.calls)
		})
	}
}

func TestNormalizeImageStudioMode_InferredForLegacyCallers(t *testing.T) {
	mode, err := NormalizeImageStudioMode("", 0)
	require.NoError(t, err)
	require.Equal(t, ImageStudioModeGenerate, mode)

	mode, err = NormalizeImageStudioMode("", 1)
	require.NoError(t, err)
	require.Equal(t, ImageStudioModeEdit, mode)

	mode, err = NormalizeImageStudioMode("", 2)
	require.NoError(t, err)
	require.Equal(t, ImageStudioModeCompose, mode)
}

func TestImageStudioService_Generate_Success_ExistingConversation(t *testing.T) {
	f := newStudioFixture(t)
	convID := int64(555)
	// The conversation must exist and belong to the acting user (id 1); Generate
	// now verifies ownership of a client-supplied conversation_id.
	f.repo.conversations[convID] = &dbent.ImageConversation{ID: convID, UserID: 1}
	in := studioInput(10)
	in.ConversationID = &convID

	res, err := f.svc.Generate(context.Background(), 1, in)

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, 0, f.repo.createdConv, "must reuse existing conversation")
	require.Equal(t, convID, res.ConversationID)
}

func TestImageStudioService_Generate_ForeignConversation_NotFound(t *testing.T) {
	f := newStudioFixture(t)
	convID := int64(777)
	// Conversation belongs to a different user; the acting user (id 1) must be
	// rejected with the not-found sentinel (handler maps it to 404).
	f.repo.conversations[convID] = &dbent.ImageConversation{ID: convID, UserID: 2}
	in := studioInput(10)
	in.ConversationID = &convID

	res, err := f.svc.Generate(context.Background(), 1, in)

	require.Nil(t, res)
	require.ErrorIs(t, err, ErrImageStudioConversationNotFound)
	// Nothing should have been generated, billed, or persisted.
	require.Equal(t, 0, f.gen.calls)
	require.Equal(t, 0, f.usage.calls)
}

func TestImageStudioService_Generate_MissingConversation_NotFound(t *testing.T) {
	f := newStudioFixture(t)
	missing := int64(9999)
	in := studioInput(10)
	in.ConversationID = &missing

	res, err := f.svc.Generate(context.Background(), 1, in)

	require.Nil(t, res)
	require.ErrorIs(t, err, ErrImageStudioConversationNotFound)
}

func TestImageStudioService_Generate_BillsDecodedCount_NotDataCount(t *testing.T) {
	f := newStudioFixture(t)
	// Upstream reported 2 images, but only 1 decoded successfully.
	f.gen.result = &ImageGenResult{
		Result:  &OpenAIForwardResult{ImageCount: 2, ImageSize: "2K", Model: "gpt-image-2"},
		Account: &Account{ID: 99, Platform: PlatformOpenAI},
	}
	f.gen.images = []StudioGeneratedImage{{Data: []byte("a"), ContentType: "image/png"}}

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Images, 1)
	require.Equal(t, 1, f.store.putCalls, "only the decoded image is stored")
	// Cost + usage must be computed on the actually-delivered count, not the
	// upstream data-array length, so the user is not overcharged.
	require.NotNil(t, f.cost.last)
	require.Equal(t, 1, f.cost.last.ImageCount, "cost computed on decoded count")
	require.NotNil(t, f.usage.last)
	require.NotNil(t, f.usage.last.Result)
	require.Equal(t, 1, f.usage.last.Result.ImageCount, "billed on decoded count")
	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, 1, last.imageCount, "persisted image_count matches decoded count")
}

func TestImageStudioService_Generate_ClampsNToMax(t *testing.T) {
	f := newStudioFixture(t)
	in := studioInput(10)
	in.N = 1000

	res, err := f.svc.Generate(context.Background(), 1, in)

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, f.gen.last.Parsed)
	require.Equal(t, imageStudioMaxN, f.gen.last.Parsed.N, "N is clamped to the studio max before forwarding upstream")
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

// High-fix regression: a subscription-type group must resolve its active
// subscription via the authoritative resolver (not user.Subscriptions, which
// GetByID does not eager-load) and bill in subscription mode rather than being
// rejected/charged on balance.
func TestImageStudioService_Generate_SubscriptionGroup_BillsInSubscriptionMode(t *testing.T) {
	f := newStudioFixture(t)
	// Subscription-type group; user balance is 0 (would be rejected on balance).
	f.groups.groups = []Group{{
		ID: 10, Status: StatusActive, AllowImageGeneration: true,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	f.users.user = &User{ID: 1, Balance: 0, AllowedGroups: []int64{10}}
	activeSub := &UserSubscription{ID: 5, GroupID: 10, UserID: 1, Status: StatusActive}
	f.subs.sub = activeSub

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)

	// Authoritative resolver was consulted exactly once.
	require.Equal(t, 1, f.subs.calls)

	// The resolved subscription was passed to BOTH eligibility and RecordUsage,
	// matching the gateway billing contract.
	require.Equal(t, 1, f.elig.calls)
	require.NotNil(t, f.usage.last)
	require.Same(t, activeSub, f.usage.last.Subscription)
}

// A subscription-type group with no active subscription must be rejected before
// generating, surfacing the resolver's not-found error.
func TestImageStudioService_Generate_SubscriptionGroup_NoActiveSubscription_Rejected(t *testing.T) {
	f := newStudioFixture(t)
	f.groups.groups = []Group{{
		ID: 10, Status: StatusActive, AllowImageGeneration: true,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	f.subs.err = ErrSubscriptionNotFound

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
	require.Equal(t, 0, f.gen.calls, "must not generate without a valid subscription")
	require.Equal(t, 0, f.usage.calls)
}

// Cost parity: the reported/persisted cost comes from ComputeImageCostBreakdown
// (the same path RecordUsage uses to deduct), not a parallel calculator.
func TestImageStudioService_Generate_CostFromAuthoritativeResolver(t *testing.T) {
	f := newStudioFixture(t)
	f.cost.breakdown = &CostBreakdown{ActualCost: 1.23, TotalCost: 1.23, BillingMode: string(BillingModeImage)}

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, 1, f.cost.calls, "cost computed via ComputeImageCostBreakdown")
	require.NotNil(t, f.cost.last)
	require.InDelta(t, 1.23, res.Cost, 1e-9)

	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.InDelta(t, 1.23, last.cost, 1e-9, "persisted cost equals charged cost")
}

// Low-fix: a nil account from the generator must mark failed (not panic in
// RecordUsage) and must not bill.
func TestImageStudioService_Generate_NilAccount_MarksFailed_NoUsage(t *testing.T) {
	f := newStudioFixture(t)
	f.gen.result = &ImageGenResult{
		Result:  &OpenAIForwardResult{ImageCount: 2, ImageSize: "2K", Model: "gpt-image-2"},
		Account: nil, // generator returned no account
	}

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrImageStudioNoAccount)
	require.Equal(t, 0, f.store.putCalls, "must not store before the account guard")
	require.Equal(t, 0, f.usage.calls)
	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "failed", last.status)
}

// Low-fix: when image k (>0) fails to store, images 0..k-1 are best-effort
// deleted to avoid orphans; the generation is marked failed and not billed.
func TestImageStudioService_Generate_PartialStoreFailure_CleansUpOrphans(t *testing.T) {
	f := newStudioFixture(t)
	// 3 images; the 3rd (idx 2) fails.
	f.gen.result.Result.ImageCount = 3
	f.gen.images = []StudioGeneratedImage{
		{Data: []byte("a"), ContentType: "image/png"},
		{Data: []byte("b"), ContentType: "image/png"},
		{Data: []byte("c"), ContentType: "image/png"},
	}
	f.store.failPutAfter = 2 // Put at idx 2 fails

	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))

	require.Error(t, err)
	require.Nil(t, res)

	// Two images were written then deleted on the failure.
	require.Equal(t, 2, f.store.putCalls)
	require.Len(t, f.store.deletedKeys, 2)
	require.ElementsMatch(t, f.store.keys, f.store.deletedKeys)

	require.Equal(t, 0, f.usage.calls)
	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "failed", last.status)
}

// Concurrency slot: with a real limiter (limit 1), a held slot blocks a second
// concurrent Generate; releasing it (Generate returns) lets the next proceed.
func TestImageStudioService_Generate_ConcurrencySlotAcquireRelease(t *testing.T) {
	f := newStudioFixture(t)
	limiter := &ImageConcurrencyLimiter{}
	cfg := &config.Config{}
	cfg.Gateway.ImageConcurrency.Enabled = true
	cfg.Gateway.ImageConcurrency.MaxConcurrentRequests = 1
	cfg.Gateway.ImageConcurrency.OverflowMode = config.ImageConcurrencyOverflowModeReject

	// Rebuild the service with the limiter + cfg wired in.
	f.svc = NewImageStudioService(ImageStudioServiceDeps{
		Users:         f.users,
		Groups:        f.groups,
		KeyEnsurer:    f.keys,
		Eligibility:   f.elig,
		Generator:     f.gen,
		CostResolver:  f.cost,
		UsageRecord:   f.usage,
		Subscriptions: f.subs,
		Repo:          f.repo,
		Store:         f.store,
		Limiter:       limiter,
		Cfg:           cfg,
	})

	// Pre-occupy the single slot directly through the limiter.
	release, ok := limiter.Acquire(context.Background(), true, 1, false, 0, 0)
	require.True(t, ok)

	// While the slot is held, Generate must fail fast with the busy error and
	// must not generate.
	res, err := f.svc.Generate(context.Background(), 1, studioInput(10))
	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrImageStudioBusy)
	require.Equal(t, 0, f.gen.calls, "must not generate when slot unavailable")

	// Release the held slot; a subsequent Generate now succeeds and the slot it
	// acquired is released via defer (a second call also succeeds).
	release()

	res, err = f.svc.Generate(context.Background(), 1, studioInput(10))
	require.NoError(t, err)
	require.NotNil(t, res)

	res, err = f.svc.Generate(context.Background(), 1, studioInput(10))
	require.NoError(t, err)
	require.NotNil(t, res)
}

// ---------------------------------------------------------------------------
// Image-to-image (edits) path.
// ---------------------------------------------------------------------------

func studioEditsInput(groupID int64) ImageStudioGenerateInput {
	in := studioInput(groupID)
	in.Mode = ImageStudioModeEdit
	in.InputImage = &StudioInputImage{
		Data:        []byte("\x89PNG\r\n\x1a\nreference-bytes"),
		ContentType: "image/png",
		FileName:    "ref.png",
	}
	return in
}

func studioMultiEditsInput(groupID int64) ImageStudioGenerateInput {
	in := studioInput(groupID)
	in.Mode = ImageStudioModeCompose
	in.InputImages = []StudioInputImage{
		{
			Data:        []byte("\x89PNG\r\n\x1a\nreference-one"),
			ContentType: "image/png",
			FileName:    "ref-1.png",
		},
		{
			Data:        []byte("\xff\xd8\xffreference-two"),
			ContentType: "image/jpeg",
			FileName:    "ref-2.jpg",
		},
	}
	return in
}

// Critical round-trip: buildEditsRequest must produce a Body+ContentType pair
// that the existing multipart parser (used by ForwardImages downstream) parses
// back into the same model/prompt/size/n, multiple "image" uploads, and an
// edits request.
func TestBuildEditsRequest_RoundTripsThroughMultipartParser(t *testing.T) {
	f := newStudioFixture(t)
	in := studioMultiEditsInput(10)

	req, err := f.svc.buildEditsRequest(in, 2, in.InputImages)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, openAIImagesEditsEndpoint, req.Endpoint)
	require.True(t, req.Multipart)
	require.NotEmpty(t, req.Body)
	require.Len(t, req.Uploads, 2)

	parsed := &OpenAIImagesRequest{Endpoint: openAIImagesEditsEndpoint}
	require.NoError(t, parseOpenAIImagesMultipartRequest(req.Body, req.ContentType, parsed))

	require.Equal(t, "gpt-image-2", parsed.Model)
	require.Equal(t, "a cat on the moon", parsed.Prompt)
	require.Equal(t, "1024x1024", parsed.Size)
	require.Equal(t, 2, parsed.N)
	require.True(t, parsed.IsEdits())
	require.Len(t, parsed.Uploads, 2)
	require.Equal(t, "image", parsed.Uploads[0].FieldName)
	require.Equal(t, in.InputImages[0].Data, parsed.Uploads[0].Data)
	require.Equal(t, "image", parsed.Uploads[1].FieldName)
	require.Equal(t, in.InputImages[1].Data, parsed.Uploads[1].Data)
}

func TestBuildParsedRequest_NormalizesWorkbenchModelAliases(t *testing.T) {
	f := newStudioFixture(t)

	autoReq := f.svc.buildParsedRequest(ImageStudioGenerateInput{
		Prompt: "draw",
		Model:  "auto",
		Size:   "1024x1024",
	}, 1)
	require.Equal(t, "gpt-image-2", autoReq.Model)
	require.False(t, autoReq.ExplicitModel)

	gpt5Req := f.svc.buildParsedRequest(ImageStudioGenerateInput{
		Prompt: "draw",
		Model:  "gpt-5-3",
		Size:   "1024x1024",
	}, 1)
	require.Equal(t, "gpt-image-2", gpt5Req.Model)
	require.False(t, gpt5Req.ExplicitModel)

	codexReq := f.svc.buildParsedRequest(ImageStudioGenerateInput{
		Prompt: "draw",
		Model:  "codex-gpt-image-2",
		Size:   "1024x1024",
	}, 1)
	require.Equal(t, "gpt-image-2-codex", codexReq.Model)
	require.True(t, codexReq.ExplicitModel)
}

// An input image routes Generate through the edits endpoint, stores the input
// once, persists the input keys, returns them in the result, and bills once.
func TestGenerate_WithInputImage_RoutesToEdits_PersistsInput(t *testing.T) {
	f := newStudioFixture(t)

	res, err := f.svc.Generate(context.Background(), 1, studioEditsInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)

	// Routed through edits.
	require.Equal(t, 1, f.gen.calls)
	require.NotNil(t, f.gen.last.Parsed)
	require.Equal(t, openAIImagesEditsEndpoint, f.gen.last.Parsed.Endpoint)
	require.True(t, f.gen.last.Parsed.Multipart)

	// Input stored once and persisted.
	require.Equal(t, 1, f.store.putInputCalls)
	require.Equal(t, 1, f.repo.setInputCalls)
	require.Len(t, f.repo.lastInputKeys, 1)

	// Result surfaces the input keys.
	require.Len(t, res.InputImages, 1)
	require.Equal(t, f.repo.lastInputKeys, res.InputImages)

	// Billed exactly once (happy path).
	require.Equal(t, 1, f.usage.calls)
}

func TestGenerate_WithMultipleInputImages_RoutesToEdits_PersistsInputs(t *testing.T) {
	f := newStudioFixture(t)

	res, err := f.svc.Generate(context.Background(), 1, studioMultiEditsInput(10))

	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, 1, f.gen.calls)
	require.NotNil(t, f.gen.last.Parsed)
	require.Equal(t, openAIImagesEditsEndpoint, f.gen.last.Parsed.Endpoint)
	require.Len(t, f.gen.last.Parsed.Uploads, 2)

	require.Equal(t, 2, f.store.putInputCalls)
	require.Equal(t, 1, f.repo.setInputCalls)
	require.Len(t, f.repo.lastInputKeys, 2)
	require.Equal(t, f.repo.lastInputKeys, res.InputImages)
}

// The reference image must be persisted even when generation later fails, and
// no billing must occur.
func TestGenerate_InputStoredEvenWhenGenerationFails(t *testing.T) {
	f := newStudioFixture(t)
	f.gen.err = errors.New("upstream boom")

	res, err := f.svc.Generate(context.Background(), 1, studioEditsInput(10))

	require.Error(t, err)
	require.Nil(t, res)

	// Input image was stored + persisted before the (failed) generation.
	require.Equal(t, 1, f.store.putInputCalls)
	require.Equal(t, 1, f.repo.setInputCalls)
	require.Len(t, f.repo.lastInputKeys, 1)

	// Generation marked failed, no output stored, no billing.
	last, ok := f.repo.lastStatus()
	require.True(t, ok)
	require.Equal(t, "failed", last.status)
	require.Equal(t, 0, f.store.putCalls)
	require.Equal(t, 0, f.usage.calls, "must NOT bill when generation fails")
}
