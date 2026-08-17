package worker

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/summarizer"
)

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
