package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/videogeneration"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type videoStudioRepository struct {
	client *dbent.Client
}

// NewVideoStudioRepository creates a new VideoStudioRepository backed by the given ent client.
func NewVideoStudioRepository(client *dbent.Client) service.VideoStudioRepository {
	return &videoStudioRepository{client: client}
}

func (r *videoStudioRepository) activeQuery() *dbent.VideoGenerationQuery {
	return r.client.VideoGeneration.Query().Where(
		videogeneration.DeletedAtIsNil(),
		videogeneration.CreatedAtGTE(videoStudioRetentionCutoff()),
	)
}

func videoStudioRetentionCutoff() time.Time {
	return time.Now().Add(-service.ImageStudioRetention)
}

// CreateGeneration persists a new (pending) video generation built by the caller.
func (r *videoStudioRepository) CreateGeneration(ctx context.Context, g *dbent.VideoGeneration) (*dbent.VideoGeneration, error) {
	client := clientFromContext(ctx, r.client)
	b := client.VideoGeneration.Create().
		SetUserID(g.UserID).
		SetGroupID(g.GroupID).
		SetPrompt(g.Prompt).
		SetModel(g.Model).
		SetStatus(g.Status)
	if g.OperationName != "" {
		b = b.SetOperationName(g.OperationName)
	}
	if g.AccountID != 0 {
		b = b.SetAccountID(g.AccountID)
	}
	return b.Save(ctx)
}

// GetGeneration fetches a single non-soft-deleted, non-expired generation by ID.
func (r *videoStudioRepository) GetGeneration(ctx context.Context, id int64) (*dbent.VideoGeneration, error) {
	return r.activeQuery().
		Where(videogeneration.IDEQ(id)).
		Only(ctx)
}

// ListGenerations returns a page of generations owned by userID, plus the total count.
func (r *videoStudioRepository) ListGenerations(ctx context.Context, userID int64, page, size int) ([]*dbent.VideoGeneration, int, error) {
	q := r.activeQuery().Where(videogeneration.UserIDEQ(userID))

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if offset < 0 {
		offset = 0
	}

	items, err := q.
		Order(dbent.Desc(videogeneration.FieldID)).
		Offset(offset).
		Limit(size).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ListGenerationsByIDs returns the (non-soft-deleted, non-expired) generations
// among ids owned by userID. Ownership is enforced in the query so a caller
// cannot read another user's generations by guessing IDs.
func (r *videoStudioRepository) ListGenerationsByIDs(ctx context.Context, userID int64, ids []int64) ([]*dbent.VideoGeneration, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.activeQuery().
		Where(
			videogeneration.UserIDEQ(userID),
			videogeneration.IDIn(ids...),
		).
		Order(dbent.Desc(videogeneration.FieldID)).
		All(ctx)
}

// CountActiveByUser counts the user's pending+processing generations, used as the
// pre-submit concurrency guard. The retention cutoff is intentionally NOT applied:
// active rows are always recent, and a stale row beyond the window would be swept
// to failed rather than counted here.
func (r *videoStudioRepository) CountActiveByUser(ctx context.Context, userID int64) (int, error) {
	return r.client.VideoGeneration.Query().
		Where(
			videogeneration.DeletedAtIsNil(),
			videogeneration.UserIDEQ(userID),
			videogeneration.StatusIn(service.VideoStatusPending, service.VideoStatusProcessing),
		).
		Count(ctx)
}

// MarkSubmitted flips a pending row to processing, binding the upstream operation
// name and account. Returns whether a row was updated.
func (r *videoStudioRepository) MarkSubmitted(ctx context.Context, id int64, operationName string, accountID int64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	n, err := client.VideoGeneration.Update().
		Where(
			videogeneration.IDEQ(id),
			videogeneration.DeletedAtIsNil(),
			videogeneration.StatusEQ(service.VideoStatusPending),
		).
		SetStatus(service.VideoStatusProcessing).
		SetOperationName(operationName).
		SetAccountID(accountID).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// TransitionToSucceeded atomically flips a processing row to succeeded. The
// status=processing guard means only the first concurrent poll wins (n==1);
// later polls observe n==0 and must not re-bill.
func (r *videoStudioRepository) TransitionToSucceeded(ctx context.Context, id int64, sampleCount int, durationSeconds, cost float64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	n, err := client.VideoGeneration.Update().
		Where(
			videogeneration.IDEQ(id),
			videogeneration.DeletedAtIsNil(),
			videogeneration.StatusEQ(service.VideoStatusProcessing),
		).
		SetStatus(service.VideoStatusSucceeded).
		SetSampleCount(sampleCount).
		SetDurationSeconds(durationSeconds).
		SetCost(cost).
		SetBilled(true).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// TransitionToFailed atomically flips a processing row to failed. Returns whether
// this call won the transition.
func (r *videoStudioRepository) TransitionToFailed(ctx context.Context, id int64, errMsg, errCode string) (bool, error) {
	client := clientFromContext(ctx, r.client)
	u := client.VideoGeneration.Update().
		Where(
			videogeneration.IDEQ(id),
			videogeneration.DeletedAtIsNil(),
			videogeneration.StatusEQ(service.VideoStatusProcessing),
		).
		SetStatus(service.VideoStatusFailed)
	if errMsg != "" {
		u = u.SetError(errMsg)
	}
	if errCode != "" {
		u = u.SetErrorCode(errCode)
	}
	n, err := u.Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// DeleteGeneration soft-deletes a generation by stamping deleted_at.
func (r *videoStudioRepository) DeleteGeneration(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.VideoGeneration.Update().
		Where(videogeneration.IDEQ(id), videogeneration.DeletedAtIsNil()).
		SetDeletedAt(time.Now()).
		Save(ctx)
	return err
}

// ClearUserHistory soft-deletes all video generations owned by userID.
func (r *videoStudioRepository) ClearUserHistory(ctx context.Context, userID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	return client.VideoGeneration.Update().
		Where(
			videogeneration.UserIDEQ(userID),
			videogeneration.DeletedAtIsNil(),
		).
		SetDeletedAt(time.Now()).
		Save(ctx)
}

// FailStaleGenerations marks pending/processing generations created before cutoff
// as failed, reclaiming rows orphaned by a restart or an operation that never
// completed. Bounded by limit per call.
func (r *videoStudioRepository) FailStaleGenerations(ctx context.Context, cutoff time.Time, limit int, errMsg, errCode string) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	client := clientFromContext(ctx, r.client)
	ids, err := client.VideoGeneration.Query().
		Where(
			videogeneration.DeletedAtIsNil(),
			videogeneration.StatusIn(service.VideoStatusPending, service.VideoStatusProcessing),
			videogeneration.CreatedAtLT(cutoff),
		).
		Order(dbent.Asc(videogeneration.FieldID)).
		Limit(limit).
		IDs(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	u := client.VideoGeneration.Update().
		Where(
			videogeneration.IDIn(ids...),
			videogeneration.DeletedAtIsNil(),
			videogeneration.StatusIn(service.VideoStatusPending, service.VideoStatusProcessing),
		).
		SetStatus(service.VideoStatusFailed)
	if errMsg != "" {
		u = u.SetError(errMsg)
	}
	if errCode != "" {
		u = u.SetErrorCode(errCode)
	}
	return u.Save(ctx)
}

// PruneExpiredGenerations soft-deletes generations older than cutoff. Videos are
// not stored locally, so there are no files to clean up.
func (r *videoStudioRepository) PruneExpiredGenerations(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	client := clientFromContext(ctx, r.client)
	ids, err := client.VideoGeneration.Query().
		Where(
			videogeneration.DeletedAtIsNil(),
			videogeneration.CreatedAtLT(cutoff),
		).
		Order(dbent.Asc(videogeneration.FieldID)).
		Limit(limit).
		IDs(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return client.VideoGeneration.Update().
		Where(
			videogeneration.IDIn(ids...),
			videogeneration.DeletedAtIsNil(),
		).
		SetDeletedAt(time.Now()).
		Save(ctx)
}
