// Package worker_test fixtures: fake repositories and fake external provider
// implementations used by the worker pipeline tests. Keeping them in a dedicated
// file preserves the granular unit tests (processor_test.go, worker_test.go)
// while making the fixtures reusable for the phase-9 security/behaviour tests.
//
// No fixture makes any external network call and none depends on a database.
package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
)

// fakeJobRepo implements ports.JobRepository with a small in-memory queue.
type fakeJobRepo struct {
	mu         sync.Mutex
	queue      []model.Job
	claimCalls int
	statuses   []string
}

func (f *fakeJobRepo) Create(ctx context.Context, job *model.Job, segmentIDs []string) error {
	return nil
}
func (f *fakeJobRepo) List(ctx context.Context, tenantID, status string, page, limit int) ([]model.Job, error) {
	return nil, nil
}
func (f *fakeJobRepo) Count(ctx context.Context, tenantID, status string) (int, error) {
	return 0, nil
}
func (f *fakeJobRepo) GetByID(ctx context.Context, id, tenantID string) (*model.Job, error) {
	return nil, nil
}
func (f *fakeJobRepo) UpdateStatus(ctx context.Context, id, tenantID, status, stage string, errorMessage *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	return nil
}
func (f *fakeJobRepo) Retry(ctx context.Context, id, tenantID string) error {
	return nil
}
func (f *fakeJobRepo) ClaimNextPending(ctx context.Context) (*model.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCalls++
	if len(f.queue) == 0 {
		return nil, nil
	}
	job := f.queue[0]
	f.queue = f.queue[1:]
	return &job, nil
}
func (f *fakeJobRepo) GetByIDAllTenants(ctx context.Context, id string) (*model.Job, error) {
	return &model.Job{ID: id, TenantID: "tenant-1", FileKey: "sample.mp4", Kind: model.KindVideo}, nil
}
func (f *fakeJobRepo) ListForRetention(ctx context.Context, olderThan time.Time) ([]model.RetentionJob, error) {
	return nil, nil
}

// fakeVideos implements ports.VideoRepository.
type fakeVideos struct {
	statuses []string
}

func (f *fakeVideos) Create(ctx context.Context, video *model.Video) error { return nil }
func (f *fakeVideos) GetByID(ctx context.Context, id, tenantID string) (*model.Video, error) {
	return nil, nil
}
func (f *fakeVideos) List(ctx context.Context, tenantID, segmentName, status string, page, limit int) ([]model.Video, error) {
	return nil, nil
}
func (f *fakeVideos) Count(ctx context.Context, tenantID, segmentName, status string) (int, error) {
	return 0, nil
}
func (f *fakeVideos) SoftDelete(ctx context.Context, id, tenantID string) (*time.Time, error) {
	return nil, nil
}
func (f *fakeVideos) UpdateStatus(ctx context.Context, id, tenantID, status string) error {
	f.statuses = append(f.statuses, status)
	return nil
}
func (f *fakeVideos) DeleteByJobID(ctx context.Context, jobID, tenantID string) error { return nil }
func (f *fakeVideos) FailByJobID(ctx context.Context, jobID, tenantID string) error   { return nil }

// fakeSegments implements ports.SegmentRepository.
type fakeSegments struct{}

func (f *fakeSegments) EnsureByName(ctx context.Context, tenantID, name string) (*model.Segment, error) {
	return &model.Segment{ID: "seg-1", TenantID: tenantID, Name: name}, nil
}
func (f *fakeSegments) AttachJobSegments(ctx context.Context, jobID, tenantID string, segmentIDs []string) error {
	return nil
}
func (f *fakeSegments) AttachVideoSegments(ctx context.Context, videoID, tenantID string, segmentIDs []string) error {
	return nil
}
func (f *fakeSegments) ListNamesByJobID(ctx context.Context, jobID, tenantID string) ([]string, error) {
	return nil, nil
}
func (f *fakeSegments) ListNamesByVideoID(ctx context.Context, videoID, tenantID string) ([]string, error) {
	return nil, nil
}

// fakeTranscripts implements ports.TranscriptRepository.
type fakeTranscripts struct{}

func (f *fakeTranscripts) Upsert(ctx context.Context, transcript *model.Transcript) error { return nil }

// fakeChunks implements ports.ChunkRepository.
type fakeChunks struct{}

func (f *fakeChunks) DeleteByVideoID(ctx context.Context, videoID, tenantID string) error { return nil }
func (f *fakeChunks) Insert(ctx context.Context, chunks []model.Chunk) error              { return nil }
func (f *fakeChunks) Search(ctx context.Context, tenantID string, embedding pgvector.Vector, segmentName string, limit int) ([]model.ChunkSearchResult, error) {
	return nil, nil
}

// fakeSummaries implements ports.SummaryRepository.
type fakeSummaries struct{}

func (f *fakeSummaries) Upsert(ctx context.Context, summary *model.Summary) error { return nil }
func (f *fakeSummaries) GetByVideoID(ctx context.Context, videoID, tenantID string) (*model.Summary, error) {
	return nil, nil
}

// fakeStorage implements ports.Storage without any network access.
type fakeStorage struct{}

func (f *fakeStorage) GenerateUploadURL(ctx context.Context, fileKey, contentType string, expiresAt time.Time) (string, error) {
	return "", nil
}
func (f *fakeStorage) Download(ctx context.Context, fileKey, destPath string) error {
	// Write a small file so size/type validation passes (fileKey ends in .mp4).
	return os.WriteFile(destPath, []byte("fake media bytes for validation"), 0o644)
}
func (f *fakeStorage) Delete(ctx context.Context, fileKey string) error { return nil }

// fakeTranscriber blocks until released so a test can simulate shutdown mid-job.
type fakeTranscriber struct {
	mu       sync.Mutex
	calls    int
	captured context.Context
	release  chan struct{}
	once     sync.Once
}

func newFakeTranscriber() *fakeTranscriber {
	return &fakeTranscriber{release: make(chan struct{})}
}

func (f *fakeTranscriber) Transcribe(ctx context.Context, input ports.TranscribeInput) (*model.Transcript, error) {
	f.mu.Lock()
	f.calls++
	if f.captured == nil {
		f.captured = ctx
	}
	f.mu.Unlock()

	<-f.release
	return &model.Transcript{
		Content:  "First sentence. Second sentence. Third sentence.",
		Language: strPtr("en"),
		Model:    strPtr("whisper-test"),
	}, nil
}

// fakeParser implements ports.DocumentParser.
type fakeParser struct{}

func (f *fakeParser) Parse(ctx context.Context, filePath, contentType string) (string, error) {
	return "", nil
}

// fakeEmbedder returns one embedding per input text.
type fakeEmbedder struct{}

func (f *fakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = make([]float32, 32)
	}
	return out, nil
}

// fakeLLM implements ports.LLM.
type fakeLLM struct{}

func (f *fakeLLM) Summarize(ctx context.Context, systemPrompt, text string) (string, error) {
	return "this is a summary", nil
}
func (f *fakeLLM) AnswerQuery(ctx context.Context, systemPrompt, userContent string) (string, error) {
	return "", nil
}

// fakeLogger returns a discard logger.
func fakeLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// contains reports whether s contains v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
