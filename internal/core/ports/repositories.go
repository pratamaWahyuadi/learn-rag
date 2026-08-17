package ports

import (
	"context"
	"time"

	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
)

// APIKeyRepository authenticates API keys by their SHA-256 hash.
type APIKeyRepository interface {
	// GetByHash returns the API key for the given key_hash. It returns
	// ErrNotFound when the key is missing or revoked.
	GetByHash(ctx context.Context, hash string) (*model.APIKey, error)
}

// UploadIntentRepository manages direct-to-R2 upload intents.
type UploadIntentRepository interface {
	Create(ctx context.Context, intent *model.UploadIntent) error
	// GetByFileKeyForUpdate locks the intent row FOR UPDATE and only returns
	// intents owned by the given tenant.
	GetByFileKeyForUpdate(ctx context.Context, fileKey, tenantID string) (*model.UploadIntent, error)
	// Consume marks an intent as consumed for the given tenant.
	Consume(ctx context.Context, fileKey, tenantID string) error
}

// JobRepository manages the job queue.
type JobRepository interface {
	// Create inserts a job along with its job_segments in a single transaction.
	Create(ctx context.Context, job *model.Job, segmentIDs []string) error
	List(ctx context.Context, tenantID, status string, page, limit int) ([]model.Job, error)
	// Count returns the number of jobs owned by tenantID, optionally filtered by
	// status.
	Count(ctx context.Context, tenantID, status string) (int, error)
	GetByID(ctx context.Context, id, tenantID string) (*model.Job, error)
	UpdateStatus(ctx context.Context, id, tenantID, status, stage string, errorMessage *string) error
	Retry(ctx context.Context, id, tenantID string) error
	// ClaimNextPending claims the next pending job across all tenants using
	// FOR UPDATE SKIP LOCKED. It does not apply tenant RLS.
	ClaimNextPending(ctx context.Context) (*model.Job, error)
}

// SegmentRepository manages flat segment tags.
type SegmentRepository interface {
	EnsureByName(ctx context.Context, tenantID, name string) (*model.Segment, error)
	AttachJobSegments(ctx context.Context, jobID, tenantID string, segmentIDs []string) error
	AttachVideoSegments(ctx context.Context, videoID, tenantID string, segmentIDs []string) error
	ListNamesByJobID(ctx context.Context, jobID, tenantID string) ([]string, error)
	ListNamesByVideoID(ctx context.Context, videoID, tenantID string) ([]string, error)
}

// VideoRepository manages course materials.
type VideoRepository interface {
	Create(ctx context.Context, video *model.Video) error
	// GetByID returns a non-deleted video owned by the tenant.
	GetByID(ctx context.Context, id, tenantID string) (*model.Video, error)
	List(ctx context.Context, tenantID, segmentName, status string, page, limit int) ([]model.Video, error)
	// Count returns the number of non-deleted videos owned by tenantID,
	// optionally filtered by segment name and status.
	Count(ctx context.Context, tenantID, segmentName, status string) (int, error)
	// SoftDelete sets deleted_at=now() for a tenant-owned, non-deleted video and
	// returns the deletion timestamp. It returns ErrNotFound when the video is
	// missing, owned by another tenant, or already deleted.
	SoftDelete(ctx context.Context, id, tenantID string) (*time.Time, error)
	UpdateStatus(ctx context.Context, id, tenantID, status string) error
}

// TranscriptRepository persists transcription/parsing results.
type TranscriptRepository interface {
	Upsert(ctx context.Context, transcript *model.Transcript) error
}

// ChunkRepository persists and searches text chunks.
type ChunkRepository interface {
	DeleteByVideoID(ctx context.Context, videoID, tenantID string) error
	Insert(ctx context.Context, chunks []model.Chunk) error
	// Search retrieves the nearest chunks for a video owned by the tenant,
	// limited to completed, non-deleted videos.
	Search(ctx context.Context, tenantID string, embedding pgvector.Vector, segmentName string, limit int) ([]model.ChunkSearchResult, error)
}

// SummaryRepository persists per-video summaries.
type SummaryRepository interface {
	Upsert(ctx context.Context, summary *model.Summary) error
	GetByVideoID(ctx context.Context, videoID, tenantID string) (*model.Summary, error)
}

// AuditLogRepository writes internal audit records.
type AuditLogRepository interface {
	Insert(ctx context.Context, log *model.AuditLog) error
}
