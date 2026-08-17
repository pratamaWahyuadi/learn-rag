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

// SegmentRepository implements ports.SegmentRepository.
type SegmentRepository struct {
	pool *pgxpool.Pool
}

// NewSegmentRepository builds a SegmentRepository backed by pool.
func NewSegmentRepository(pool *pgxpool.Pool) *SegmentRepository {
	return &SegmentRepository{pool: pool}
}

// EnsureByName finds a segment by name for the tenant, or creates it, honoring
// the UNIQUE(tenant_id, lower(name)) constraint.
func (r *SegmentRepository) EnsureByName(ctx context.Context, tenantID, name string) (*model.Segment, error) {
	var result *model.Segment
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		row, err := q.EnsureSegmentByName(ctx, queries.EnsureSegmentByNameParams{
			ID:        newUUIDString(),
			TenantID:  tenantID,
			Name:      name,
			CreatedAt: nowTimestamptz(),
		})
		if err != nil {
			return err
		}
		result = &model.Segment{
			ID:        row.ID,
			TenantID:  row.TenantID,
			Name:      row.Name,
			CreatedAt: toTime(row.CreatedAt),
		}
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

// AttachJobSegments links a job to the given segments for the tenant.
func (r *SegmentRepository) AttachJobSegments(ctx context.Context, jobID, tenantID string, segmentIDs []string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		for _, sid := range segmentIDs {
			if err := q.AttachJobSegment(ctx, queries.AttachJobSegmentParams{
				JobID:     jobID,
				SegmentID: sid,
				TenantID:  tenantID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// AttachVideoSegments links a video to the given segments for the tenant.
func (r *SegmentRepository) AttachVideoSegments(ctx context.Context, videoID, tenantID string, segmentIDs []string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		for _, sid := range segmentIDs {
			if err := q.AttachVideoSegment(ctx, queries.AttachVideoSegmentParams{
				VideoID:   videoID,
				SegmentID: sid,
				TenantID:  tenantID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListNamesByJobID returns the segment names attached to a job for the tenant.
func (r *SegmentRepository) ListNamesByJobID(ctx context.Context, jobID, tenantID string) ([]string, error) {
	var names []string
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		var err error
		names, err = q.ListSegmentNamesByJobID(ctx, queries.ListSegmentNamesByJobIDParams{
			JobID:    jobID,
			TenantID: tenantID,
		})
		return err
	})
	return names, err
}

// ListNamesByVideoID returns the segment names attached to a video for the
// tenant.
func (r *SegmentRepository) ListNamesByVideoID(ctx context.Context, videoID, tenantID string) ([]string, error) {
	var names []string
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		var err error
		names, err = q.ListSegmentNamesByVideoID(ctx, queries.ListSegmentNamesByVideoIDParams{
			VideoID:  videoID,
			TenantID: tenantID,
		})
		return err
	})
	return names, err
}
