//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// studioAPIKeyRepoStub — minimal stub for EnsureStudioAPIKey tests
// ---------------------------------------------------------------------------

// studioAPIKeyRepoStub implements APIKeyRepository for studio API key tests.
// It stores created keys in-memory and supports the idempotency lookup.
type studioAPIKeyRepoStub struct {
	// stored holds all keys created via Create.
	stored []*APIKey
	// createErr, if non-nil, is returned by Create.
	createErr error
	// findErr, if non-nil, is returned by FindInternalByUserAndGroup.
	findErr error
	// firstFindMiss, when true, forces the FIRST FindInternalByUserAndGroup call
	// to return (nil, nil) regardless of stored state, so the slow/create path is
	// exercised even when a seeded key exists. Subsequent calls behave normally.
	firstFindMiss bool
	findCalls     int
}

func (s *studioAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.createErr != nil {
		return s.createErr
	}
	key.ID = int64(len(s.stored) + 1)
	clone := *key
	s.stored = append(s.stored, &clone)
	return nil
}

func (s *studioAPIKeyRepoStub) FindInternalByUserAndGroup(_ context.Context, userID, groupID int64, name string) (*APIKey, error) {
	s.findCalls++
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.firstFindMiss && s.findCalls == 1 {
		return nil, nil
	}
	for _, k := range s.stored {
		if k.UserID == userID && k.GroupID != nil && *k.GroupID == groupID && k.Name == name && k.Internal {
			clone := *k
			return &clone, nil
		}
	}
	return nil, nil
}

func (s *studioAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, _ pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	var out []APIKey
	for _, k := range s.stored {
		if k.UserID != userID {
			continue
		}
		if filters.ExcludeInternal && k.Internal {
			continue
		}
		out = append(out, *k)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 100, Pages: 1}, nil
}

// Remaining interface methods — panic on unexpected call.

func (s *studioAPIKeyRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	for _, k := range s.stored {
		if k.ID == id {
			clone := *k
			return &clone, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}
func (s *studioAPIKeyRepoStub) GetKeyAndOwnerID(_ context.Context, id int64) (string, int64, bool, error) {
	for _, k := range s.stored {
		if k.ID == id {
			return k.Key, k.UserID, k.Internal, nil
		}
	}
	return "", 0, false, ErrAPIKeyNotFound
}
func (s *studioAPIKeyRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}
func (s *studioAPIKeyRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}
func (s *studioAPIKeyRepoStub) Update(context.Context, *APIKey) error {
	panic("unexpected Update call")
}
func (s *studioAPIKeyRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *studioAPIKeyRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}
func (s *studioAPIKeyRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}
func (s *studioAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}
func (s *studioAPIKeyRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	panic("unexpected ExistsByKey call")
}
func (s *studioAPIKeyRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *studioAPIKeyRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}
func (s *studioAPIKeyRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (s *studioAPIKeyRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}
func (s *studioAPIKeyRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}
func (s *studioAPIKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}
func (s *studioAPIKeyRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}
func (s *studioAPIKeyRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}
func (s *studioAPIKeyRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *studioAPIKeyRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (s *studioAPIKeyRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (s *studioAPIKeyRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

// newStudioAPIKeyService returns an APIKeyService wired with the stub repo and
// an empty config so that GenerateKey falls back to the default "sk-" prefix.
func newStudioAPIKeyService(repo *studioAPIKeyRepoStub) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: repo,
		cfg:        &config.Config{},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestEnsureStudioAPIKey_CreateOnFirstCall verifies that the first call creates
// an internal key with the correct fields.
func TestEnsureStudioAPIKey_CreateOnFirstCall(t *testing.T) {
	repo := &studioAPIKeyRepoStub{}
	svc := newStudioAPIKeyService(repo)

	const userID, groupID = int64(11), int64(22)
	key, err := svc.EnsureStudioAPIKey(context.Background(), userID, groupID)

	require.NoError(t, err)
	require.NotNil(t, key)
	require.True(t, key.Internal, "key must be marked internal")
	require.Equal(t, studioAPIKeyName, key.Name, "key name must be the sentinel constant")
	require.NotNil(t, key.GroupID, "key must have GroupID set")
	require.Equal(t, groupID, *key.GroupID)
	require.Equal(t, userID, key.UserID)
	require.Equal(t, StatusActive, key.Status)
	require.NotEmpty(t, key.Key, "plaintext key must be returned on first call")

	// Exactly one key should have been stored.
	require.Len(t, repo.stored, 1)
}

// TestEnsureStudioAPIKey_IdempotentOnSecondCall verifies that a second call
// returns the same key without inserting a duplicate row.
func TestEnsureStudioAPIKey_IdempotentOnSecondCall(t *testing.T) {
	repo := &studioAPIKeyRepoStub{}
	svc := newStudioAPIKeyService(repo)

	const userID, groupID = int64(11), int64(22)

	first, err := svc.EnsureStudioAPIKey(context.Background(), userID, groupID)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := svc.EnsureStudioAPIKey(context.Background(), userID, groupID)
	require.NoError(t, err)
	require.NotNil(t, second)

	// No duplicate rows must have been created.
	require.Len(t, repo.stored, 1, "should not insert a second row on repeat call")
	// Both calls must refer to the same logical key.
	require.Equal(t, first.ID, second.ID, "second call must return the same key ID")
	require.Equal(t, first.Name, second.Name)
	require.True(t, second.Internal)
}

// TestEnsureStudioAPIKey_DifferentGroupsDifferentKeys verifies that each
// (user, group) pair gets its own distinct key.
func TestEnsureStudioAPIKey_DifferentGroupsDifferentKeys(t *testing.T) {
	repo := &studioAPIKeyRepoStub{}
	svc := newStudioAPIKeyService(repo)

	const userID = int64(11)

	keyA, err := svc.EnsureStudioAPIKey(context.Background(), userID, 100)
	require.NoError(t, err)

	keyB, err := svc.EnsureStudioAPIKey(context.Background(), userID, 200)
	require.NoError(t, err)

	require.Len(t, repo.stored, 2, "two distinct (user,group) pairs must create two rows")
	require.NotEqual(t, keyA.ID, keyB.ID)
}

// TestEnsureStudioAPIKey_InternalKeyHiddenFromUserList verifies that the
// ExcludeInternal filter hides internal keys from user-facing key listings.
func TestEnsureStudioAPIKey_InternalKeyHiddenFromUserList(t *testing.T) {
	repo := &studioAPIKeyRepoStub{}
	svc := newStudioAPIKeyService(repo)

	const userID, groupID = int64(11), int64(22)

	// Seed one normal (non-internal) user key directly.
	normalKey := &APIKey{
		ID:       99,
		UserID:   userID,
		Key:      "sk-normal",
		Name:     "my visible key",
		Status:   StatusActive,
		Internal: false,
	}
	repo.stored = append(repo.stored, normalKey)

	// Create the studio key.
	studioKey, err := svc.EnsureStudioAPIKey(context.Background(), userID, groupID)
	require.NoError(t, err)
	require.True(t, studioKey.Internal)

	// Total stored: 1 normal + 1 internal.
	require.Len(t, repo.stored, 2)

	params := pagination.PaginationParams{Page: 1, PageSize: 100}

	// User-facing list (ExcludeInternal=true) must hide the studio key.
	userKeys, userResult, err := repo.ListByUserID(context.Background(), userID, params, APIKeyListFilters{ExcludeInternal: true})
	require.NoError(t, err)
	require.Equal(t, int64(1), userResult.Total, "user list must exclude the internal studio key")
	require.Len(t, userKeys, 1)
	require.False(t, userKeys[0].Internal)
	require.Equal(t, "my visible key", userKeys[0].Name)

	// Admin-path list (ExcludeInternal=false) must see both keys.
	allKeys, allResult, err := repo.ListByUserID(context.Background(), userID, params, APIKeyListFilters{ExcludeInternal: false})
	require.NoError(t, err)
	require.Equal(t, int64(2), allResult.Total, "admin list must include internal keys")
	require.Len(t, allKeys, 2)
}

// TestEnsureStudioAPIKey_IdempotentOnCreateConflict verifies the race-recovery
// branch: if Create returns ErrAPIKeyExists (a concurrent creator won), the
// method re-fetches and returns the existing key instead of erroring.
func TestEnsureStudioAPIKey_IdempotentOnCreateConflict(t *testing.T) {
	const userID, groupID = int64(11), int64(22)

	// Pre-seed the key the "concurrent creator" already inserted.
	gid := groupID
	existing := &APIKey{
		ID:       7,
		UserID:   userID,
		Key:      "sk-existing-studio",
		Name:     studioAPIKeyName,
		GroupID:  &gid,
		Status:   StatusActive,
		Internal: true,
	}
	// firstFindMiss forces the fast-path lookup to miss, so the slow/create path
	// runs; createErr=ErrAPIKeyExists simulates a concurrent creator winning the
	// race; the recovery lookup then returns the seeded key.
	repo := &studioAPIKeyRepoStub{
		stored:        []*APIKey{existing},
		createErr:     ErrAPIKeyExists,
		firstFindMiss: true,
	}
	svc := newStudioAPIKeyService(repo)

	key, err := svc.EnsureStudioAPIKey(context.Background(), userID, groupID)
	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, existing.ID, key.ID, "must recover the concurrently-created key")
	require.True(t, key.Internal)
}

// TestApiKeyService_GetByID_HidesInternalKey verifies that the user-facing
// GetByID path returns not-found for internal keys.
func TestApiKeyService_GetByID_HidesInternalKey(t *testing.T) {
	gid := int64(22)
	repo := &studioAPIKeyRepoStub{
		stored: []*APIKey{{
			ID:       5,
			UserID:   11,
			Key:      "sk-internal",
			Name:     studioAPIKeyName,
			GroupID:  &gid,
			Status:   StatusActive,
			Internal: true,
		}},
	}
	svc := newStudioAPIKeyService(repo)

	_, err := svc.GetByID(context.Background(), 5)
	require.ErrorIs(t, err, ErrAPIKeyNotFound, "internal key must be invisible to user GetByID")
}

// TestApiKeyService_GetByID_ReturnsNormalKey is a control: non-internal keys
// remain visible via GetByID.
func TestApiKeyService_GetByID_ReturnsNormalKey(t *testing.T) {
	repo := &studioAPIKeyRepoStub{
		stored: []*APIKey{{
			ID:       6,
			UserID:   11,
			Key:      "sk-normal",
			Name:     "visible",
			Status:   StatusActive,
			Internal: false,
		}},
	}
	svc := newStudioAPIKeyService(repo)

	key, err := svc.GetByID(context.Background(), 6)
	require.NoError(t, err)
	require.Equal(t, int64(6), key.ID)
	require.False(t, key.Internal)
}

// TestApiKeyService_Update_RejectsInternalKey verifies users cannot mutate an
// internal key: Update returns not-found and never reaches the repo Update.
func TestApiKeyService_Update_RejectsInternalKey(t *testing.T) {
	gid := int64(22)
	repo := &studioAPIKeyRepoStub{
		stored: []*APIKey{{
			ID:       5,
			UserID:   11,
			Key:      "sk-internal",
			Name:     studioAPIKeyName,
			GroupID:  &gid,
			Status:   StatusActive,
			Internal: true,
		}},
	}
	svc := newStudioAPIKeyService(repo)

	newName := "hacked"
	_, err := svc.Update(context.Background(), 5, 11, UpdateAPIKeyRequest{Name: &newName})
	require.ErrorIs(t, err, ErrAPIKeyNotFound, "internal key must not be updatable by user")
	// repo.Update panics on call; reaching here proves it was never invoked.
}

// TestApiKeyService_Delete_RejectsInternalKey verifies users cannot delete an
// internal key: Delete returns not-found and never reaches DeleteWithAudit.
func TestApiKeyService_Delete_RejectsInternalKey(t *testing.T) {
	gid := int64(22)
	repo := &studioAPIKeyRepoStub{
		stored: []*APIKey{{
			ID:       5,
			UserID:   11,
			Key:      "sk-internal",
			Name:     studioAPIKeyName,
			GroupID:  &gid,
			Status:   StatusActive,
			Internal: true,
		}},
	}
	svc := newStudioAPIKeyService(repo)

	err := svc.Delete(context.Background(), 5, 11)
	require.ErrorIs(t, err, ErrAPIKeyNotFound, "internal key must not be deletable by user")
	// repo.DeleteWithAudit panics on call; reaching here proves it was never invoked.
}
