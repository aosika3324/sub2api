package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/imageconversation"
	"github.com/Wei-Shaw/sub2api/ent/imagegeneration"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type imageStudioRepository struct {
	client *dbent.Client
}

// NewImageStudioRepository creates a new ImageStudioRepository backed by the given ent client.
func NewImageStudioRepository(client *dbent.Client) service.ImageStudioRepository {
	return &imageStudioRepository{client: client}
}

func (r *imageStudioRepository) activeConversationQuery() *dbent.ImageConversationQuery {
	return r.client.ImageConversation.Query().Where(imageconversation.DeletedAtIsNil())
}

func (r *imageStudioRepository) activeGenerationQuery() *dbent.ImageGenerationQuery {
	return r.client.ImageGeneration.Query().Where(
		imagegeneration.DeletedAtIsNil(),
		imagegeneration.CreatedAtGTE(imageStudioRetentionCutoff()),
	)
}

func imageStudioRetentionCutoff() time.Time {
	return time.Now().Add(-service.ImageStudioRetention)
}

// CreateConversation creates a new image conversation for the given user.
func (r *imageStudioRepository) CreateConversation(ctx context.Context, userID int64, title string) (*dbent.ImageConversation, error) {
	client := clientFromContext(ctx, r.client)
	return client.ImageConversation.Create().
		SetUserID(userID).
		SetTitle(title).
		Save(ctx)
}

// ListConversations returns a page of conversations owned by userID, plus the total count.
func (r *imageStudioRepository) ListConversations(ctx context.Context, userID int64, page, size int) ([]*dbent.ImageConversation, int, error) {
	q := r.activeConversationQuery().Where(imageconversation.UserIDEQ(userID))

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if offset < 0 {
		offset = 0
	}

	items, err := q.
		Order(dbent.Desc(imageconversation.FieldID)).
		Offset(offset).
		Limit(size).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetConversation fetches a single (non-soft-deleted) conversation by ID.
func (r *imageStudioRepository) GetConversation(ctx context.Context, id int64) (*dbent.ImageConversation, error) {
	return r.activeConversationQuery().
		Where(imageconversation.IDEQ(id)).
		Only(ctx)
}

// UpdateConversationTitle renames a (non-soft-deleted) conversation.
func (r *imageStudioRepository) UpdateConversationTitle(ctx context.Context, id int64, title string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ImageConversation.Update().
		Where(imageconversation.IDEQ(id), imageconversation.DeletedAtIsNil()).
		SetTitle(title).
		Save(ctx)
	return err
}

// DeleteConversation soft-deletes a conversation by stamping deleted_at.
func (r *imageStudioRepository) DeleteConversation(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ImageConversation.Update().
		Where(imageconversation.IDEQ(id), imageconversation.DeletedAtIsNil()).
		SetDeletedAt(time.Now()).
		Save(ctx)
	return err
}

// DeleteConversationCascade soft-deletes a conversation together with all of its
// (non-soft-deleted) generations in a single transaction, and returns the
// storage keys (output + input) of those generations so the caller can delete
// the underlying files. Ownership scoping is the caller's responsibility.
func (r *imageStudioRepository) DeleteConversationCascade(ctx context.Context, id int64) ([]string, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.deleteConversationCascade(ctx, tx.Client(), id)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	keys, err := r.deleteConversationCascade(txCtx, tx.Client(), id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *imageStudioRepository) deleteConversationCascade(ctx context.Context, client *dbent.Client, id int64) ([]string, error) {
	now := time.Now()

	// Collect the storage keys of the conversation's live generations BEFORE
	// deleting them, so the caller can remove the underlying files.
	gens, err := client.ImageGeneration.Query().
		Where(
			imagegeneration.ConversationIDEQ(id),
			imagegeneration.DeletedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, g := range gens {
		keys = append(keys, g.StorageKeys...)
		keys = append(keys, g.InputStorageKeys...)
	}

	// Soft-delete the child generations.
	if _, err := client.ImageGeneration.Update().
		Where(
			imagegeneration.ConversationIDEQ(id),
			imagegeneration.DeletedAtIsNil(),
		).
		SetDeletedAt(now).
		Save(ctx); err != nil {
		return nil, err
	}

	// Soft-delete the conversation itself.
	if _, err := client.ImageConversation.Update().
		Where(
			imageconversation.IDEQ(id),
			imageconversation.DeletedAtIsNil(),
		).
		SetDeletedAt(now).
		Save(ctx); err != nil {
		return nil, err
	}

	return keys, nil
}

// ClearUserHistory soft-deletes all image studio conversations and generations
// owned by userID, returning storage keys for best-effort file deletion.
func (r *imageStudioRepository) ClearUserHistory(ctx context.Context, userID int64) ([]string, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.clearUserHistory(ctx, tx.Client(), userID)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	keys, err := r.clearUserHistory(txCtx, tx.Client(), userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *imageStudioRepository) clearUserHistory(ctx context.Context, client *dbent.Client, userID int64) ([]string, error) {
	now := time.Now()
	gens, err := client.ImageGeneration.Query().
		Where(
			imagegeneration.UserIDEQ(userID),
			imagegeneration.DeletedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, g := range gens {
		keys = append(keys, g.StorageKeys...)
		keys = append(keys, g.InputStorageKeys...)
	}

	if _, err := client.ImageGeneration.Update().
		Where(
			imagegeneration.UserIDEQ(userID),
			imagegeneration.DeletedAtIsNil(),
		).
		SetDeletedAt(now).
		Save(ctx); err != nil {
		return nil, err
	}

	if _, err := client.ImageConversation.Update().
		Where(
			imageconversation.UserIDEQ(userID),
			imageconversation.DeletedAtIsNil(),
		).
		SetDeletedAt(now).
		Save(ctx); err != nil {
		return nil, err
	}

	return keys, nil
}

// DeleteGeneration soft-deletes a generation by stamping deleted_at.
func (r *imageStudioRepository) DeleteGeneration(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ImageGeneration.Update().
		Where(imagegeneration.IDEQ(id), imagegeneration.DeletedAtIsNil()).
		SetDeletedAt(time.Now()).
		Save(ctx)
	return err
}

// PruneExpiredGenerations soft-deletes image-studio generations older than the
// retention cutoff and returns their storage keys for best-effort file cleanup.
func (r *imageStudioRepository) PruneExpiredGenerations(ctx context.Context, cutoff time.Time, limit int) ([]string, int, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.pruneExpiredGenerations(ctx, tx.Client(), cutoff, limit)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, 0, err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	keys, count, err := r.pruneExpiredGenerations(txCtx, tx.Client(), cutoff, limit)
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return keys, count, nil
}

func (r *imageStudioRepository) pruneExpiredGenerations(ctx context.Context, client *dbent.Client, cutoff time.Time, limit int) ([]string, int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := time.Now()
	gens, err := client.ImageGeneration.Query().
		Where(
			imagegeneration.DeletedAtIsNil(),
			imagegeneration.CreatedAtLT(cutoff),
		).
		Order(dbent.Asc(imagegeneration.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	if len(gens) == 0 {
		return nil, 0, nil
	}

	ids := make([]int64, 0, len(gens))
	var keys []string
	for _, g := range gens {
		ids = append(ids, g.ID)
		keys = append(keys, g.StorageKeys...)
		keys = append(keys, g.InputStorageKeys...)
	}

	if _, err := client.ImageGeneration.Update().
		Where(
			imagegeneration.IDIn(ids...),
			imagegeneration.DeletedAtIsNil(),
		).
		SetDeletedAt(now).
		Save(ctx); err != nil {
		return nil, 0, err
	}
	return keys, len(gens), nil
}

// CreateGeneration persists a new image generation record built by the caller.
// The caller populates the fields on g; this method saves it and returns the
// persisted entity (with ID, timestamps, etc. set).
func (r *imageStudioRepository) CreateGeneration(ctx context.Context, g *dbent.ImageGeneration) (*dbent.ImageGeneration, error) {
	client := clientFromContext(ctx, r.client)
	b := client.ImageGeneration.Create().
		SetUserID(g.UserID).
		SetConversationID(g.ConversationID).
		SetGroupID(g.GroupID).
		SetPrompt(g.Prompt).
		SetModel(g.Model).
		SetSize(g.Size).
		SetQuality(g.Quality).
		SetN(g.N).
		SetImageCount(g.ImageCount).
		SetStatus(g.Status).
		SetCost(g.Cost)

	if len(g.StorageKeys) > 0 {
		b = b.SetStorageKeys(g.StorageKeys)
	}
	if len(g.InputStorageKeys) > 0 {
		b = b.SetInputStorageKeys(g.InputStorageKeys)
	}
	if g.Width != nil {
		b = b.SetWidth(*g.Width)
	}
	if g.Height != nil {
		b = b.SetHeight(*g.Height)
	}
	if g.Error != nil {
		b = b.SetError(*g.Error)
	}

	return b.Save(ctx)
}

// UpdateGenerationStatus updates result fields after a generation completes or fails.
func (r *imageStudioRepository) UpdateGenerationStatus(ctx context.Context, id int64, status string, storageKeys []string, cost float64, imageCount, width, height int, errMsg string) error {
	client := clientFromContext(ctx, r.client)
	u := client.ImageGeneration.UpdateOneID(id).
		SetStatus(status).
		SetCost(cost).
		SetImageCount(imageCount)

	if len(storageKeys) > 0 {
		u = u.SetStorageKeys(storageKeys)
	}
	if width > 0 {
		u = u.SetWidth(width)
	}
	if height > 0 {
		u = u.SetHeight(height)
	}
	if errMsg != "" {
		u = u.SetError(errMsg)
	}

	_, err := u.Save(ctx)
	return err
}

// SetInputStorageKeys persists the user-provided reference image storage keys
// for an edits generation. It is a no-op when keys is empty.
func (r *imageStudioRepository) SetInputStorageKeys(ctx context.Context, id int64, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	_, err := client.ImageGeneration.UpdateOneID(id).
		SetInputStorageKeys(keys).
		Save(ctx)
	return err
}

// GetGeneration fetches a single (non-soft-deleted) generation by ID.
func (r *imageStudioRepository) GetGeneration(ctx context.Context, id int64) (*dbent.ImageGeneration, error) {
	return r.activeGenerationQuery().
		Where(imagegeneration.IDEQ(id)).
		Only(ctx)
}

// ListGenerations returns a page of generations owned by userID,
// optionally scoped to a single conversation.
func (r *imageStudioRepository) ListGenerations(ctx context.Context, userID int64, conversationID *int64, page, size int) ([]*dbent.ImageGeneration, int, error) {
	q := r.activeGenerationQuery().Where(imagegeneration.UserIDEQ(userID))
	if conversationID != nil {
		q = q.Where(imagegeneration.ConversationIDEQ(*conversationID))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if offset < 0 {
		offset = 0
	}

	items, err := q.
		Order(dbent.Desc(imagegeneration.FieldID)).
		Offset(offset).
		Limit(size).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
