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

// SummaryRepository implements ports.SummaryRepository.
type SummaryRepository struct {
	pool *pgxpool.Pool
}

// NewSummaryRepository builds a SummaryRepository backed by pool.
func NewSummaryRepository(pool *pgxpool.Pool) *SummaryRepository {
	return &SummaryRepository{pool: pool}
}

// Upsert inserts or updates the summary for a video based on video_id.
func (r *SummaryRepository) Upsert(ctx context.Context, summary *model.Summary) error {
	return database.WithTenantTx(ctx, r.pool, summary.TenantID, func(q *queries.Queries) error {
		_, err := q.UpsertSummary(ctx, queries.UpsertSummaryParams{
			ID:           summary.ID,
			TenantID:     summary.TenantID,
			VideoID:      summary.VideoID,
			Status:       summary.Status,
			Content:      fromStrPtr(summary.Content),
			Language:     fromStrPtr(summary.Language),
			Model:        fromStrPtr(summary.Model),
			ErrorMessage: fromStrPtr(summary.ErrorMessage),
			CreatedAt:    fromTime(summary.CreatedAt),
			UpdatedAt:    fromTime(summary.UpdatedAt),
		})
		return err
	})
}

// GetByVideoID returns the summary for a tenant-owned video, or ErrNotFound.
func (r *SummaryRepository) GetByVideoID(ctx context.Context, videoID, tenantID string) (*model.Summary, error) {
	var result *model.Summary
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		row, err := q.GetSummaryByVideoID(ctx, queries.GetSummaryByVideoIDParams{
			VideoID:  videoID,
			TenantID: tenantID,
		})
		if err != nil {
			return err
		}
		summary := &model.Summary{
			ID:           row.ID,
			TenantID:     row.TenantID,
			VideoID:      row.VideoID,
			Status:       row.Status,
			Content:      toStrPtr(row.Content),
			Language:     toStrPtr(row.Language),
			Model:        toStrPtr(row.Model),
			ErrorMessage: toStrPtr(row.ErrorMessage),
			CreatedAt:    toTime(row.CreatedAt),
			UpdatedAt:    toTime(row.UpdatedAt),
		}
		result = summary
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
