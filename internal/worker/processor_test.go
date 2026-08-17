package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/summarizer"
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

// fakeStorage implements ports.Storage.
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

// TestWorkerGracefulShutdown verifies that an in-flight job's context is NOT
// cancelled when the shutdown context fires — the worker gives the job an
// independent context so it can finish within the grace period instead of being
// cancelled mid-pipeline.
func TestWorkerGracefulShutdown(t *testing.T) {
	jobs := &fakeJobRepo{queue: []model.Job{{ID: "job-1"}}}
	transcriber := newFakeTranscriber()

	w := &Worker{
		cfg:  &config.Config{},
		log:  fakeLogger(),
		jobs: jobs,
		deps: pipelineDeps{
			log:            fakeLogger(),
			jobs:           jobs,
			videos:         &fakeVideos{},
			segments:       &fakeSegments{},
			transcripts:    &fakeTranscripts{},
			chunks:         &fakeChunks{},
			summaries:      &fakeSummaries{},
			storage:        &fakeStorage{},
			transcriber:    transcriber,
			parser:         &fakeParser{},
			embedder:       &fakeEmbedder{},
			llm:            &fakeLLM{},
			summarizer:     summarizer.New(12000),
			maxUploadBytes: 1 << 30,
		},
	}

	shutdownCtx, cancelShutdown := context.WithCancel(context.Background())
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()

	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.workerLoop(shutdownCtx, jobCtx, wake)
	}()

	wake <- struct{}{}

	// Wait until the job has been claimed and transcription is in-flight.
	deadline := time.Now().Add(5 * time.Second)
	for {
		transcriber.mu.Lock()
		inFlight := transcriber.calls > 0
		captured := transcriber.captured
		transcriber.mu.Unlock()
		if inFlight && captured != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for transcription to start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Simulate SIGTERM: cancel the shutdown context while transcription blocks.
	cancelShutdown()

	// The job's in-flight context must NOT be cancelled by shutdown.
	transcriber.mu.Lock()
	captured := transcriber.captured
	transcriber.mu.Unlock()
	if err := captured.Err(); err != nil {
		t.Fatalf("in-flight job context was cancelled during shutdown: %v", err)
	}

	// Let the transcription finish; the job should complete.
	close(transcriber.release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("workerLoop did not return after shutdown")
	}

	// The job must have reached the completed state (not failed).
	if !contains(jobs.statuses, model.JobStatusCompleted) {
		t.Fatalf("job did not complete during graceful shutdown; statuses=%v", jobs.statuses)
	}
	if contains(jobs.statuses, model.JobStatusFailed) {
		t.Fatalf("job failed during graceful shutdown; statuses=%v", jobs.statuses)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestNotifyListenerStopsOnCancellation verifies the LISTEN listener terminates
// promptly when the shutdown context is cancelled, even if it has not managed to
// connect, so Run's wg.Wait() is not blocked by it.
func TestNotifyListenerStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pool, err := pgxpool.New(ctx, "postgres://invalid:invalid@127.0.0.1:1/rag")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	w := &Worker{pool: pool, log: fakeLogger()}
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		w.notifyListener(ctx, wake)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("notifyListener did not stop on cancelled context")
	}
}
