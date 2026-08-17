//go:build integration

// Package database_test contains integration tests that exercise the tenant
// transaction helper (WithTenantTx), the repository layer, and Row Level
// Security isolation between tenants (SR-01) against a real Postgres + pgvector
// instance.
//
// These tests require a reachable database and are gated behind the
// `integration` build tag so `make test` (go test ./...) stays green without a
// database. Run them with:
//
//	DATABASE_URL=postgres://rag:rag@localhost:5432/rag go test -tags integration ./internal/database/...
package database_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/repo"
)

// newTestPool opens a pool from DATABASE_URL or skips the test when unset.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("integration test requires DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// seedTenant inserts a tenant row directly (tenants has no RLS) and returns its id.
// The slug is suffixed with a random fragment so repeated runs never collide.
func seedTenant(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	ctx := context.Background()
	tenantID := uuid.NewString()
	slug = slug + "-" + tenantID[:8]
	_, err := pool.Exec(ctx, "INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)", tenantID, slug, slug)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	return tenantID
}

// seedVideo inserts a single job + video for the tenant and returns the video id.
func seedVideo(t *testing.T, pool *pgxpool.Pool, tenantID, fileKey, title string) string {
	t.Helper()
	ctx := context.Background()

	suffix := uuid.NewString()[:8]
	jobFileKey := fileKey + "-" + suffix + "-job"
	videoFileKey := fileKey + "-" + suffix

	jobID := uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO jobs (id, tenant_id, file_key, title, kind, status, stage)
		VALUES ($1, $2, $3, $4, 'video', 'completed', 'completed')`, jobID, tenantID, jobFileKey, title)
	if err != nil {
		t.Fatalf("seed job: %v", err)
	}

	videoID := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO videos (id, tenant_id, job_id, title, kind, file_key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'video', $5, 'completed', now(), now())`,
		videoID, tenantID, jobID, title, videoFileKey)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return videoID
}

// seedCompletedVideo creates a job, a completed video, and a chunk so the
// tenant has a searchable RAG entry.
func seedCompletedVideo(t *testing.T, pool *pgxpool.Pool, tenantID, fileKey, title, chunkContent string) {
	t.Helper()
	ctx := context.Background()

	videoID := seedVideo(t, pool, tenantID, fileKey, title)

	embedding := pgvector.NewVector(make([]float32, 1024))
	chunkID := uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO chunks (id, tenant_id, video_id, chunk_index, content, embedding, created_at)
		VALUES ($1, $2, $3, 0, $4, $5, now())`, chunkID, tenantID, videoID, chunkContent, embedding)
	if err != nil {
		t.Fatalf("seed chunk: %v", err)
	}
}

// TestWithTenantTxCrossTenantRead proves a job belonging to tenant B is
// invisible inside a tenant A transaction (RLS + explicit tenant filter).
func TestWithTenantTxCrossTenantRead(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	tenantA := seedTenant(t, pool, "rls-a")
	tenantB := seedTenant(t, pool, "rls-b")
	videoB := seedVideo(t, pool, tenantB, "rls-b-file", "Video B")

	err := database.WithTenantTx(ctx, pool, tenantA, func(q *queries.Queries) error {
		_, err := q.GetVideoByID(ctx, queries.GetVideoByIDParams{ID: videoB, TenantID: tenantA})
		return err
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows reading tenant B video from tenant A, got %v", err)
	}
}

// TestSearchChunksTenantIsolation proves tenant A never retrieves tenant B
// chunks even with an identical embedding.
func TestSearchChunksTenantIsolation(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	tenantA := seedTenant(t, pool, "search-a")
	tenantB := seedTenant(t, pool, "search-b")
	seedCompletedVideo(t, pool, tenantA, "search-a-file", "Video A", "alpha content")
	seedCompletedVideo(t, pool, tenantB, "search-b-file", "Video B", "bravo content")

	chunks := repo.NewChunkRepository(pool)
	embedding := pgvector.NewVector(make([]float32, 1024))
	results, err := chunks.Search(ctx, tenantA, embedding, "", 10)
	if err != nil {
		t.Fatalf("SearchChunks: %v", err)
	}
	for _, r := range results {
		if r.TenantID != tenantA {
			t.Fatalf("returned chunk belongs to tenant %s, expected only %s", r.TenantID, tenantA)
		}
	}
}

// TestUploadIntentConsumeTenantIsolation proves an upload intent for tenant B
// cannot be consumed from tenant A.
func TestUploadIntentConsumeTenantIsolation(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	tenantA := seedTenant(t, pool, "consume-a")
	tenantB := seedTenant(t, pool, "consume-b")

	intentID := uuid.NewString()
	consumeFileKey := "consume-b-file-" + uuid.NewString()[:8]
	_, err := pool.Exec(ctx, `INSERT INTO upload_intents (id, tenant_id, file_key, content_type, status, expires_at, created_at)
		VALUES ($1, $2, $3, 'video/mp4', 'issued', now() + interval '1 hour', now())`,
		intentID, tenantB, consumeFileKey)
	if err != nil {
		t.Fatalf("seed upload intent: %v", err)
	}

	intents := repo.NewUploadIntentRepository(pool)
	if err := intents.Consume(ctx, consumeFileKey, tenantA); err == nil {
		t.Fatal("expected error consuming tenant B intent from tenant A")
	}
}

// TestRepositoriesRoundTrip exercises every tenant-scoped repository end to end
// to verify the sqlc queries and RLS wrappers work against the real schema.
func TestRepositoriesRoundTrip(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	tenantID := seedTenant(t, pool, "roundtrip-a")

	// Upload intent: create + get for update.
	uiRepo := repo.NewUploadIntentRepository(pool)
	fileKey := "roundtrip-a-file-" + uuid.NewString()[:8]
	ui := &model.UploadIntent{
		ID:          uuid.NewString(),
		TenantID:    tenantID,
		FileKey:     fileKey,
		ContentType: "video/mp4",
		Status:      "issued",
		ExpiresAt:   time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
	}
	if err := uiRepo.Create(ctx, ui); err != nil {
		t.Fatalf("create upload intent: %v", err)
	}
	if got, err := uiRepo.GetByFileKeyForUpdate(ctx, fileKey, tenantID); err != nil {
		t.Fatalf("get upload intent: %v", err)
	} else if got.ID != ui.ID {
		t.Fatalf("upload intent id mismatch: got %s want %s", got.ID, ui.ID)
	}

	// Segment: ensure + attach + list names.
	segRepo := repo.NewSegmentRepository(pool)
	seg, err := segRepo.EnsureByName(ctx, tenantID, "Web Design")
	if err != nil {
		t.Fatalf("ensure segment: %v", err)
	}
	if seg.Name != "Web Design" {
		t.Fatalf("segment name mismatch: %s", seg.Name)
	}

	// Job: create with segment, get, list segment names.
	jobRepo := repo.NewJobRepository(pool)
	job := &model.Job{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		UploadIntentID: &ui.ID,
		FileKey:        fileKey,
		Title:          "Belajar Web",
		Kind:           "video",
		Status:         "pending",
		Stage:          "queued",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := jobRepo.Create(ctx, job, []string{seg.ID}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if got, err := jobRepo.GetByID(ctx, job.ID, tenantID); err != nil {
		t.Fatalf("get job: %v", err)
	} else if got.Title != job.Title {
		t.Fatalf("job title mismatch: %s", got.Title)
	}
	names, err := segRepo.ListNamesByJobID(ctx, job.ID, tenantID)
	if err != nil {
		t.Fatalf("list segment names by job: %v", err)
	}
	if len(names) != 1 || names[0] != "Web Design" {
		t.Fatalf("unexpected job segment names: %v", names)
	}

	// Video: create, attach segments, update status, get with segments.
	videoRepo := repo.NewVideoRepository(pool)
	video := &model.Video{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		JobID:     job.ID,
		Title:     "Belajar Web",
		Kind:      "video",
		FileKey:   fileKey,
		Status:    "processing",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := videoRepo.Create(ctx, video); err != nil {
		t.Fatalf("create video: %v", err)
	}
	if err := segRepo.AttachVideoSegments(ctx, video.ID, tenantID, []string{seg.ID}); err != nil {
		t.Fatalf("attach video segments: %v", err)
	}
	if err := videoRepo.UpdateStatus(ctx, video.ID, tenantID, "completed"); err != nil {
		t.Fatalf("update video status: %v", err)
	}
	gotVideo, err := videoRepo.GetByID(ctx, video.ID, tenantID)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if gotVideo.Status != "completed" {
		t.Fatalf("video status mismatch: %s", gotVideo.Status)
	}
	if len(gotVideo.Segments) != 1 || gotVideo.Segments[0] != "Web Design" {
		t.Fatalf("unexpected video segments: %v", gotVideo.Segments)
	}

	// Chunk: insert + search.
	chunkRepo := repo.NewChunkRepository(pool)
	chunks := []model.Chunk{
		{
			ID:         uuid.NewString(),
			TenantID:   tenantID,
			VideoID:    video.ID,
			ChunkIndex: 0,
			Content:    "first sentence. second sentence. third.",
			CreatedAt:  time.Now(),
			Embedding:  pgvector.NewVector(make([]float32, 1024)),
		},
	}
	if err := chunkRepo.Insert(ctx, chunks); err != nil {
		t.Fatalf("insert chunk: %v", err)
	}
	results, err := chunkRepo.Search(ctx, tenantID, pgvector.NewVector(make([]float32, 1024)), "Web Design", 5)
	if err != nil {
		t.Fatalf("search chunks: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}

	// Summary: upsert + get.
	sumRepo := repo.NewSummaryRepository(pool)
	summary := &model.Summary{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		VideoID:   video.ID,
		Status:    "completed",
		Content:   strPtr("ringkasan"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sumRepo.Upsert(ctx, summary); err != nil {
		t.Fatalf("upsert summary: %v", err)
	}
	gotSum, err := sumRepo.GetByVideoID(ctx, video.ID, tenantID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if gotSum.Content == nil || *gotSum.Content != "ringkasan" {
		t.Fatalf("summary content mismatch: %v", gotSum.Content)
	}
}
