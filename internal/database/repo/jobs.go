package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// JobRepository implements ports.JobRepository.
type JobRepository struct {
	pool *pgxpool.Pool
}

// NewJobRepository builds a JobRepository backed by pool.
func NewJobRepository(pool *pgxpool.Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

// Create inserts a job and its job_segments within a single tenant-scoped
// transaction.
func (r *JobRepository) Create(ctx context.Context, job *model.Job, segmentIDs []string) error {
	return database.WithTenantTx(ctx, r.pool, job.TenantID, func(q *queries.Queries) error {
		if _, err := q.CreateJob(ctx, queries.CreateJobParams{
			ID:             job.ID,
			TenantID:       job.TenantID,
			UploadIntentID: fromUUIDPtr(job.UploadIntentID),
			FileKey:        job.FileKey,
			Title:          job.Title,
			Kind:           job.Kind,
			Status:         job.Status,
			Stage:          job.Stage,
			ErrorMessage:   fromStrPtr(job.ErrorMessage),
			RetryCount:     int32(job.RetryCount),
			StartedAt:      fromTimePtr(job.StartedAt),
			FinishedAt:     fromTimePtr(job.FinishedAt),
			CreatedAt:      fromTime(job.CreatedAt),
			UpdatedAt:      fromTime(job.UpdatedAt),
		}); err != nil {
			return err
		}
		for _, sid := range segmentIDs {
			if err := q.AttachJobSegment(ctx, queries.AttachJobSegmentParams{
				JobID:     job.ID,
				SegmentID: sid,
				TenantID:  job.TenantID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// List returns the jobs owned by tenantID, optionally filtered by status, using
// page-based pagination (page starts at 1, limit caps the page size).
func (r *JobRepository) List(ctx context.Context, tenantID, status string, page, limit int) ([]model.Job, error) {
	var jobs []model.Job
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		rows, err := q.ListJobs(ctx, queries.ListJobsParams{
			TenantID: tenantID,
			Status:   optionalText(status),
			Offset:   int32((page - 1) * limit),
			Limit:    int32(limit),
		})
		if err != nil {
			return err
		}
		jobs = make([]model.Job, 0, len(rows))
		for _, row := range rows {
			jobs = append(jobs, mapJob(row))
		}
		return nil
	})
	return jobs, err
}

// Count returns the number of jobs owned by tenantID, optionally filtered by
// status, inside a tenant-scoped transaction.
func (r *JobRepository) Count(ctx context.Context, tenantID, status string) (int, error) {
	var total int
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		n, err := q.CountJobs(ctx, queries.CountJobsParams{
			TenantID: tenantID,
			Status:   optionalText(status),
		})
		if err != nil {
			return err
		}
		total = int(n)
		return nil
	})
	return total, err
}

// GetByID returns a job owned by tenantID, or ErrNotFound otherwise.
func (r *JobRepository) GetByID(ctx context.Context, id, tenantID string) (*model.Job, error) {
	var result *model.Job
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		row, err := q.GetJobByID(ctx, queries.GetJobByIDParams{ID: id, TenantID: tenantID})
		if err != nil {
			return err
		}
		job := mapJob(row)
		result = &job
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

// UpdateStatus sets the job status, stage, and optional error message for a
// job owned by tenantID.
func (r *JobRepository) UpdateStatus(ctx context.Context, id, tenantID, status, stage string, errorMessage *string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		_, err := q.UpdateJobStatus(ctx, queries.UpdateJobStatusParams{
			ID:           id,
			TenantID:     tenantID,
			Status:       status,
			Stage:        stage,
			ErrorMessage: fromStrPtr(errorMessage),
		})
		return err
	})
}

// Retry resets a failed job owned by tenantID back to pending.
func (r *JobRepository) Retry(ctx context.Context, id, tenantID string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		_, err := q.RetryJob(ctx, queries.RetryJobParams{ID: id, TenantID: tenantID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.ErrNotFound
			}
			return err
		}
		return nil
	})
}

// ClaimNextPending atomically claims the oldest pending job across all tenants
// using FOR UPDATE SKIP LOCKED. It deliberately bypasses tenant RLS because the
// worker must be able to pick up jobs from any tenant.
func (r *JobRepository) ClaimNextPending(ctx context.Context) (*model.Job, error) {
	q := queries.New(r.pool)
	row, err := q.ClaimNextPendingJob(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	job := mapJob(row)
	return &job, nil
}

// GetByIDAllTenants returns a job by id regardless of tenant. It bypasses tenant
// RLS like ClaimNextPending so the worker can reload a job it has claimed. It
// returns ErrNotFound when no such job exists.
func (r *JobRepository) GetByIDAllTenants(ctx context.Context, id string) (*model.Job, error) {
	q := queries.New(r.pool)
	row, err := q.GetJobByIDAllTenants(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierrors.ErrNotFound
		}
		return nil, err
	}
	job := mapJob(row)
	return &job, nil
}

// ListForRetention returns the ids and file keys of finalized jobs whose
// finished_at precedes olderThan, regardless of tenant. It deliberately bypasses
// tenant RLS because retention cleanup operates globally.
func (r *JobRepository) ListForRetention(ctx context.Context, olderThan time.Time) ([]model.RetentionJob, error) {
	q := queries.New(r.pool)
	rows, err := q.ListJobsForRetention(ctx, fromTime(olderThan))
	if err != nil {
		return nil, err
	}
	jobs := make([]model.RetentionJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, model.RetentionJob{ID: row.ID, FileKey: row.FileKey})
	}
	return jobs, nil
}

// optionalText returns a NULL pgtype.Text when s is empty, otherwise a valid
// pgtype.Text, so an empty status means "no filter".
func optionalText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// mapJob converts a sqlc Job into the domain model.
func mapJob(row queries.Job) model.Job {
	return model.Job{
		ID:             row.ID,
		TenantID:       row.TenantID,
		UploadIntentID: uuidPtr(row.UploadIntentID),
		FileKey:        row.FileKey,
		Title:          row.Title,
		Kind:           row.Kind,
		Status:         row.Status,
		Stage:          row.Stage,
		ErrorMessage:   toStrPtr(row.ErrorMessage),
		RetryCount:     int(row.RetryCount),
		StartedAt:      toTimePtr(row.StartedAt),
		FinishedAt:     toTimePtr(row.FinishedAt),
		CreatedAt:      toTime(row.CreatedAt),
		UpdatedAt:      toTime(row.UpdatedAt),
	}
}

// uuidPtr converts a pgtype.UUID to a *string UUID.
func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuidToString(u)
	return &s
}
