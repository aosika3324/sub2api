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
	// CreateGeneration persists a new image generation record built by the caller.
	CreateGeneration(ctx context.Context, g *dbent.ImageGeneration) (*dbent.ImageGeneration, error)
	// UpdateGenerationStatus updates result fields after a generation completes or fails.
	UpdateGenerationStatus(ctx context.Context, id int64, status string, storageKeys []string, cost float64, width, height int, errMsg string) error
	// GetGeneration fetches a single non-soft-deleted generation by ID; soft-deleted
	// rows are treated as not-found. Ownership scoping is the caller's responsibility.
	GetGeneration(ctx context.Context, id int64) (*dbent.ImageGeneration, error)
	// ListGenerations returns a page of generations owned by userID, optionally scoped to a conversation.
	ListGenerations(ctx context.Context, userID int64, conversationID *int64, page, size int) ([]*dbent.ImageGeneration, int, error)
}
