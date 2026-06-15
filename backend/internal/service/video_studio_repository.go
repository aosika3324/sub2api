package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// Video generation status values. The MVP is a flat (no-conversation) async task
// state machine:
//
//	pending    -> row created, submit not yet returned (transient inside StartGenerate)
//	processing -> submit accepted, operation_name + account_id bound, awaiting poll
//	succeeded  -> upstream operation done with a usable video; billed once
//	failed     -> submit rejected, upstream operation errored, or swept as stale
const (
	VideoStatusPending    = "pending"
	VideoStatusProcessing = "processing"
	VideoStatusSucceeded  = "succeeded"
	VideoStatusFailed     = "failed"
)

// VideoStudioRepository handles persistence for in-app (JWT) Veo video generations.
//
// Unlike ImageStudioRepository there are no conversations: each row is one
// submitted job. The result transitions (TransitionToSucceeded / TransitionToFailed)
// are atomic and report whether the caller won the race, because read-time polling
// can fire concurrent status requests against the same still-processing row and
// only the winner must record billing.
type VideoStudioRepository interface {
	// CreateGeneration persists a new (pending) video generation built by the caller
	// and returns the saved entity with ID/timestamps populated.
	CreateGeneration(ctx context.Context, g *dbent.VideoGeneration) (*dbent.VideoGeneration, error)
	// GetGeneration fetches a single non-soft-deleted generation by ID; soft-deleted
	// rows are treated as not-found. Ownership scoping is the caller's responsibility.
	GetGeneration(ctx context.Context, id int64) (*dbent.VideoGeneration, error)
	// ListGenerations returns a page of generations owned by userID, plus the total count.
	ListGenerations(ctx context.Context, userID int64, page, size int) ([]*dbent.VideoGeneration, int, error)
	// ListGenerationsByIDs returns the (non-soft-deleted) generations among ids that
	// belong to userID. It powers the batch status-poll endpoint; ownership is enforced
	// here so a caller cannot read another user's generations by guessing IDs.
	ListGenerationsByIDs(ctx context.Context, userID int64, ids []int64) ([]*dbent.VideoGeneration, error)
	// CountActiveByUser counts the user's pending+processing generations, used as the
	// pre-submit concurrency guard.
	CountActiveByUser(ctx context.Context, userID int64) (int, error)
	// MarkSubmitted flips a pending row to processing, binding the upstream operation
	// name and account. Returns whether a row was updated (false if it was no longer
	// pending — e.g. already swept).
	MarkSubmitted(ctx context.Context, id int64, operationName string, accountID int64) (bool, error)
	// TransitionToSucceeded atomically flips a processing row to succeeded, recording
	// sample_count/duration/cost and stamping billed=true. Returns true only if this
	// call won the transition (and must therefore record billing); false means another
	// concurrent poll already completed it.
	TransitionToSucceeded(ctx context.Context, id int64, sampleCount int, durationSeconds, cost float64) (bool, error)
	// TransitionToFailed atomically flips a processing row to failed with errMsg/errCode.
	// Returns whether this call won the transition.
	TransitionToFailed(ctx context.Context, id int64, errMsg, errCode string) (bool, error)
	// DeleteGeneration soft-deletes a generation. Ownership scoping is the caller's responsibility.
	DeleteGeneration(ctx context.Context, id int64) error
	// ClearUserHistory soft-deletes all video generations owned by userID and returns
	// the number of rows affected.
	ClearUserHistory(ctx context.Context, userID int64) (int, error)
	// FailStaleGenerations marks pending/processing generations created before cutoff as
	// failed (with errMsg/errCode), reclaiming rows orphaned by a process restart or an
	// upstream operation that never completed. Returns the number of rows updated.
	FailStaleGenerations(ctx context.Context, cutoff time.Time, limit int, errMsg, errCode string) (int, error)
	// PruneExpiredGenerations soft-deletes generations older than cutoff (the retention
	// window) and returns the number of rows pruned. Videos are not stored locally, so
	// there are no files to clean up.
	PruneExpiredGenerations(ctx context.Context, cutoff time.Time, limit int) (int, error)
}
