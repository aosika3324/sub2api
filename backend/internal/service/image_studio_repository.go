package service

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// ImageStudioRepository handles persistence for image conversations and generations.
type ImageStudioRepository interface {
	// CreateConversation creates a new image conversation for the given user.
	CreateConversation(ctx context.Context, userID int64, title string) (*dbent.ImageConversation, error)
	// ListConversations returns a page of conversations owned by userID, plus the total count.
	ListConversations(ctx context.Context, userID int64, page, size int) ([]*dbent.ImageConversation, int, error)
	// GetConversation fetches a single non-soft-deleted conversation by ID; soft-deleted
	// rows are treated as not-found. Ownership scoping is the caller's responsibility.
	GetConversation(ctx context.Context, id int64) (*dbent.ImageConversation, error)
	// UpdateConversationTitle renames a conversation. Ownership scoping is the caller's responsibility.
	UpdateConversationTitle(ctx context.Context, id int64, title string) error
	// DeleteConversation soft-deletes a conversation. Ownership scoping is the caller's responsibility.
	DeleteConversation(ctx context.Context, id int64) error
	// DeleteConversationCascade soft-deletes a conversation together with all of its
	// generations in one transaction and returns the storage keys (output + input)
	// of those generations so the caller can delete the underlying files. Ownership
	// scoping is the caller's responsibility.
	DeleteConversationCascade(ctx context.Context, id int64) ([]string, error)
	// CreateGeneration persists a new image generation record built by the caller.
	CreateGeneration(ctx context.Context, g *dbent.ImageGeneration) (*dbent.ImageGeneration, error)
	// UpdateGenerationStatus updates result fields after a generation completes or fails.
	UpdateGenerationStatus(ctx context.Context, id int64, status string, storageKeys []string, cost float64, imageCount, width, height int, errMsg string) error
	// SetInputStorageKeys persists the user-provided reference image storage keys
	// for an edits generation. It is a no-op when keys is empty.
	SetInputStorageKeys(ctx context.Context, id int64, keys []string) error
	// GetGeneration fetches a single non-soft-deleted generation by ID; soft-deleted
	// rows are treated as not-found. Ownership scoping is the caller's responsibility.
	GetGeneration(ctx context.Context, id int64) (*dbent.ImageGeneration, error)
	// ListGenerations returns a page of generations owned by userID, optionally scoped to a conversation.
	ListGenerations(ctx context.Context, userID int64, conversationID *int64, page, size int) ([]*dbent.ImageGeneration, int, error)
	// DeleteGeneration soft-deletes a generation. Ownership scoping is the caller's responsibility.
	DeleteGeneration(ctx context.Context, id int64) error
}
