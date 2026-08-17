package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// UploadIntentRepository implements ports.UploadIntentRepository.
type UploadIntentRepository struct {
	pool *pgxpool.Pool
}

// NewUploadIntentRepository builds an UploadIntentRepository backed by pool.
func NewUploadIntentRepository(pool *pgxpool.Pool) *UploadIntentRepository {
	return &UploadIntentRepository{pool: pool}
}

// Create inserts a new upload intent scoped to its tenant.
func (r *UploadIntentRepository) Create(ctx context.Context, intent *model.UploadIntent) error {
	return database.WithTenantTx(ctx, r.pool, intent.TenantID, func(q *queries.Queries) error {
		_, err := q.CreateUploadIntent(ctx, queries.CreateUploadIntentParams{
			ID:          intent.ID,
			TenantID:    intent.TenantID,
			FileKey:     intent.FileKey,
			ContentType: intent.ContentType,
			Status:      intent.Status,
			ExpiresAt:   fromTime(intent.ExpiresAt),
			ConsumedAt:  fromTimePtr(intent.ConsumedAt),
			CreatedAt:   fromTime(intent.CreatedAt),
		})
		return err
	})
}

// GetByFileKeyForUpdate locks the intent row FOR UPDATE, but only returns
// intents owned by tenantID. It returns ErrNotFound otherwise.
func (r *UploadIntentRepository) GetByFileKeyForUpdate(ctx context.Context, fileKey, tenantID string) (*model.UploadIntent, error) {
	var result *model.UploadIntent
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		row, err := q.GetUploadIntentByFileKeyForUpdate(ctx, queries.GetUploadIntentByFileKeyForUpdateParams{
			FileKey:  fileKey,
			TenantID: tenantID,
		})
		if err != nil {
			return err
		}
		result = mapUploadIntent(row)
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierrors.ErrNotFound
		}
		return nil, err
	}
	return result, nil
}

// Consume marks an issued intent owned by tenantID as consumed.
func (r *UploadIntentRepository) Consume(ctx context.Context, fileKey, tenantID string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		_, err := q.MarkUploadIntentConsumed(ctx, queries.MarkUploadIntentConsumedParams{
			FileKey:  fileKey,
			TenantID: tenantID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.ErrNotFound
			}
			return err
		}
		return nil
	})
}

// mapUploadIntent converts a sqlc UploadIntent into the domain model.
func mapUploadIntent(row queries.UploadIntent) *model.UploadIntent {
	return &model.UploadIntent{
		ID:          row.ID,
		TenantID:    row.TenantID,
		FileKey:     row.FileKey,
		ContentType: row.ContentType,
		Status:      row.Status,
		ExpiresAt:   toTime(row.ExpiresAt),
		ConsumedAt:  toTimePtr(row.ConsumedAt),
		CreatedAt:   toTime(row.CreatedAt),
	}
}
