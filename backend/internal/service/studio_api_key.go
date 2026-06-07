package service

import (
	"context"
	"errors"
	"fmt"
)

// studioAPIKeyName is the sentinel name stored on the synthetic internal API key
// that the image-studio service uses to drive the image pipeline on behalf of a user.
// It is intentionally not a valid user-chosen name so it can be detected and hidden.
const studioAPIKeyName = "__image_studio__"

// EnsureStudioAPIKey returns the existing hidden image-studio API key for the
// (userID, groupID) pair, creating one atomically if it does not yet exist.
//
// The returned key has Internal=true and is therefore excluded from all
// user-facing key listings.  The plaintext key value is available on the
// returned *APIKey.Key for the duration of the call only; callers that need it
// must persist it themselves — it is not returned on subsequent calls (only the
// hashed form is stored).
//
// The method is idempotent: concurrent or repeated calls for the same
// (userID, groupID) pair will converge on a single row.
func (s *APIKeyService) EnsureStudioAPIKey(ctx context.Context, userID, groupID int64) (*APIKey, error) {
	// Fast path: return existing key without creating anything.
	existing, err := s.apiKeyRepo.FindInternalByUserAndGroup(ctx, userID, groupID, studioAPIKeyName)
	if err != nil {
		return nil, fmt.Errorf("ensure studio api key: lookup: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Slow path: generate a fresh key using the same crypto path as normal keys.
	rawKey, err := s.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("ensure studio api key: generate key: %w", err)
	}

	gid := groupID
	apiKey := &APIKey{
		UserID:   userID,
		Key:      rawKey,
		Name:     studioAPIKeyName,
		GroupID:  &gid,
		Status:   StatusActive,
		Internal: true,
	}

	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		// Idempotency: if a concurrent caller already created the key, fetch it.
		if errors.Is(err, ErrAPIKeyExists) {
			existing, lookupErr := s.apiKeyRepo.FindInternalByUserAndGroup(ctx, userID, groupID, studioAPIKeyName)
			if lookupErr != nil {
				return nil, fmt.Errorf("ensure studio api key: lookup after conflict: %w", lookupErr)
			}
			if existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("ensure studio api key: create: %w", err)
	}

	return apiKey, nil
}
