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

// VideoRepository implements ports.VideoRepository.
type VideoRepository struct {
	pool *pgxpool.Pool
}

// NewVideoRepository builds a VideoRepository backed by pool.
func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{pool: pool}
}

// Create inserts a new video row for the tenant.
func (r *VideoRepository) Create(ctx context.Context, video *model.Video) error {
	return database.WithTenantTx(ctx, r.pool, video.TenantID, func(q *queries.Queries) error {
		_, err := q.CreateVideo(ctx, queries.CreateVideoParams{
			ID:              video.ID,
			TenantID:        video.TenantID,
			JobID:           video.JobID,
			Title:           video.Title,
			Kind:            video.Kind,
			FileKey:         video.FileKey,
			Status:          video.Status,
			DurationSeconds: fromIntPtr(video.DurationSeconds),
			DeletedAt:       fromTimePtr(video.DeletedAt),
			CreatedAt:       fromTime(video.CreatedAt),
			UpdatedAt:       fromTime(video.UpdatedAt),
		})
		return err
	})
}

// GetByID returns a non-deleted video owned by tenantID, including its segment
// names. It returns ErrNotFound if the video belongs to another tenant or has
// been soft-deleted.
func (r *VideoRepository) GetByID(ctx context.Context, id, tenantID string) (*model.Video, error) {
	var result *model.Video
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		row, err := q.GetVideoByID(ctx, queries.GetVideoByIDParams{ID: id, TenantID: tenantID})
		if err != nil {
			return err
		}
		video, err := r.withSegments(ctx, q, row)
		if err != nil {
			return err
		}
		result = video
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

// List returns non-deleted videos owned by tenantID, optionally filtered by
// segment name (case-insensitive) and status, using page-based pagination.
func (r *VideoRepository) List(ctx context.Context, tenantID, segmentName, status string, page, limit int) ([]model.Video, error) {
	var videos []model.Video
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		rows, err := q.ListVideos(ctx, queries.ListVideosParams{
			TenantID:    tenantID,
			Status:      optionalText(status),
			SegmentName: optionalText(segmentName),
			Offset:      int32((page - 1) * limit),
			Limit:       int32(limit),
		})
		if err != nil {
			return err
		}
		videos = make([]model.Video, 0, len(rows))
		for _, row := range rows {
			video, err := r.withSegments(ctx, q, row)
			if err != nil {
				return err
			}
			videos = append(videos, *video)
		}
		return nil
	})
	return videos, err
}

// SoftDelete sets deleted_at=now() for a tenant-owned, non-deleted video. It
// returns ErrNotFound if no row was updated.
func (r *VideoRepository) SoftDelete(ctx context.Context, id, tenantID string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		_, err := q.SoftDeleteVideo(ctx, queries.SoftDeleteVideoParams{ID: id, TenantID: tenantID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.ErrNotFound
			}
			return err
		}
		return nil
	})
}

// UpdateStatus updates the status of a tenant-owned video.
func (r *VideoRepository) UpdateStatus(ctx context.Context, id, tenantID, status string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		_, err := q.UpdateVideoStatus(ctx, queries.UpdateVideoStatusParams{
			ID:       id,
			TenantID: tenantID,
			Status:   status,
		})
		return err
	})
}

// withSegments decorates a mapped video with its segment names using the same
// transaction/RLS context provided by q.
func (r *VideoRepository) withSegments(ctx context.Context, q *queries.Queries, row queries.Video) (*model.Video, error) {
	video := mapVideo(row)
	names, err := q.ListSegmentNamesByVideoID(ctx, queries.ListSegmentNamesByVideoIDParams{
		VideoID:  row.ID,
		TenantID: row.TenantID,
	})
	if err != nil {
		return nil, err
	}
	video.Segments = names
	return &video, nil
}

// mapVideo converts a sqlc Video into the domain model.
func mapVideo(row queries.Video) model.Video {
	return model.Video{
		ID:              row.ID,
		TenantID:        row.TenantID,
		JobID:           row.JobID,
		Title:           row.Title,
		Kind:            row.Kind,
		FileKey:         row.FileKey,
		Status:          row.Status,
		DurationSeconds: toIntPtr(row.DurationSeconds),
		DeletedAt:       toTimePtr(row.DeletedAt),
		CreatedAt:       toTime(row.CreatedAt),
		UpdatedAt:       toTime(row.UpdatedAt),
	}
}
