// Package worker implements the background job pipeline: it listens for new
// jobs via Postgres LISTEN/NOTIFY, falls back to periodic polling, claims jobs
// with FOR UPDATE SKIP LOCKED, and processes each job through download →
// transcribe/parse → chunk → embed → persist, then retires R2 files.
//
// Security note: the worker never logs file keys, transcripts, prompts,
// presigned URLs, or provider secrets — only opaque job IDs and pipeline stages.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/summarizer"
)

// Defaults for the queue loop. The retention interval lives in retention.go and
// can be overridden via the RETENTION_INTERVAL environment variable.
const (
	// claimFallbackInterval is how often the worker polls the pending queue when
	// no LISTEN notification has arrived.
	claimFallbackInterval = 60 * time.Second
	// notifyChannel is the Postgres channel fired on job insert.
	notifyChannel = "job_created"
	// wakeBufferSize bounds the number of pending wake signals buffered in
	// memory so a notification burst cannot block the listener.
	wakeBufferSize = 32
	// shutdownGrace is how long the worker lets in-flight jobs finish after the
	// shutdown context is cancelled before force-cancelling them.
	shutdownGrace = 30 * time.Second
	// listenReconnectDelay is the initial delay before re-acquiring the LISTEN
	// connection after it drops; it backs off up to listenMaxReconnectDelay.
	listenReconnectDelay = 1 * time.Second
	// listenMaxReconnectDelay caps the reconnect backoff.
	listenMaxReconnectDelay = 30 * time.Second
)

// Config carries the runtime settings and dependencies for the worker.
type Config struct {
	Cfg  *config.Config
	Pool *pgxpool.Pool
	Log  *slog.Logger

	Jobs        ports.JobRepository
	Videos      ports.VideoRepository
	Segments    ports.SegmentRepository
	Transcripts ports.TranscriptRepository
	Chunks      ports.ChunkRepository
	Summaries   ports.SummaryRepository

	Storage     ports.Storage
	Transcriber ports.Transcriber
	Parser      ports.DocumentParser
	Embedder    ports.Embedder
	LLM         ports.LLM
	Summarizer  *summarizer.Summarizer
}

// Worker runs the job processing pipeline and the R2 retention loop.
type Worker struct {
	cfg  *config.Config
	log  *slog.Logger
	pool *pgxpool.Pool
	jobs ports.JobRepository
	deps pipelineDeps
}

// New validates the configuration and returns a Worker ready to Run.
func New(c Config) (*Worker, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if c.Summarizer == nil {
		c.Summarizer = summarizer.New(c.Cfg.SummaryMaxTokens)
	}
	return &Worker{
		cfg:  c.Cfg,
		log:  c.Log,
		pool: c.Pool,
		jobs: c.Jobs,
		deps: pipelineDeps{
			log:                c.Log,
			jobs:               c.Jobs,
			videos:             c.Videos,
			segments:           c.Segments,
			transcripts:        c.Transcripts,
			chunks:             c.Chunks,
			summaries:          c.Summaries,
			storage:            c.Storage,
			transcriber:        c.Transcriber,
			parser:             c.Parser,
			embedder:           c.Embedder,
			llm:                c.LLM,
			summarizer:         c.Summarizer,
			maxUploadBytes:     c.Cfg.MaxUploadBytes,
			embeddingBatchSize: c.Cfg.EmbeddingBatchSize,
		},
	}, nil
}

// validate checks that every required dependency is present.
func (c Config) validate() error {
	if c.Cfg == nil {
		return fmt.Errorf("worker: config is required")
	}
	if c.Pool == nil {
		return fmt.Errorf("worker: pool is required")
	}
	if c.Log == nil {
		return fmt.Errorf("worker: logger is required")
	}
	for _, d := range []struct {
		name string
		ok   bool
	}{
		{"jobs", c.Jobs != nil},
		{"videos", c.Videos != nil},
		{"segments", c.Segments != nil},
		{"transcripts", c.Transcripts != nil},
		{"chunks", c.Chunks != nil},
		{"summaries", c.Summaries != nil},
		{"storage", c.Storage != nil},
		{"transcriber", c.Transcriber != nil},
		{"parser", c.Parser != nil},
		{"embedder", c.Embedder != nil},
		{"llm", c.LLM != nil},
	} {
		if !d.ok {
			return fmt.Errorf("worker: %s is required", d.name)
		}
	}
	return nil
}

// Run starts the LISTEN listener, the polling ticker, the worker pool, and the
// retention loop, and blocks until ctx is cancelled. When ctx is cancelled it
// stops claiming new jobs but lets in-flight jobs finish on an independent
// context for up to shutdownGrace before force-cancelling them.
func (w *Worker) Run(ctx context.Context) error {
	wake := make(chan struct{}, wakeBufferSize)
	var wg sync.WaitGroup

	// jobCtx governs a claimed job. It is intentionally independent of the
	// shutdown ctx so that a SIGTERM does not cancel an in-flight job in the
	// middle of the pipeline (download/transcribe/embed). In-flight jobs get up
	// to shutdownGrace to finish, then jobCancel force-cancels them.
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()

	// Retention loop is independent of the job queue.
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.retentionLoop(ctx)
	}()

	// LISTEN for newly created jobs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.notifyListener(ctx, wake)
	}()

	// Periodic fallback claiming.
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.tickerLoop(ctx, wake)
	}()

	// Worker pool. Workers claim on ctx (stop as soon as shutdown begins) but
	// process each job on the independent jobCtx so in-flight work can finish.
	concurrency := w.concurrency()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.workerLoop(ctx, jobCtx, wake)
		}()
	}

	w.log.Info("worker started", "concurrency", concurrency,
		"fallback_interval", claimFallbackInterval.String())

	<-ctx.Done()
	w.log.Info("worker shutting down; waiting for in-flight jobs", "grace", shutdownGrace.String())

	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()
	select {
	case <-wait:
		return nil
	case <-time.After(shutdownGrace):
		w.log.Warn("worker shutdown grace exceeded; cancelling in-flight jobs")
		jobCancel()
		<-wait
		return nil
	}
}

// concurrency returns the configured worker count, clamped to [1, 5].
func (w *Worker) concurrency() int {
	n := w.cfg.WorkerConcurrency
	if n <= 0 {
		n = 3
	}
	if n > 5 {
		n = 5
	}
	return n
}

// notifyListener subscribes to the job_created channel and wakes workers. If the
// LISTEN connection drops for any reason other than shutdown, it re-acquires a
// fresh connection from the pool with a bounded backoff and logs a warning so
// the silent fallback-to-polling degradation stays visible.
func (w *Worker) notifyListener(ctx context.Context, wake chan<- struct{}) {
	backoff := listenReconnectDelay
	for reconnects := 0; ; {
		if !w.listenOnce(ctx, wake) {
			return
		}
		reconnects++
		w.log.Warn("worker: notify listener down; relying on polling fallback",
			"reconnects", reconnects, "next_retry", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > listenMaxReconnectDelay {
			backoff = listenMaxReconnectDelay
		}
	}
}

// listenOnce runs one LISTEN session to completion. It returns false when ctx
// was cancelled (worker should stop entirely) and true when the connection stood
// up but later failed (caller should reconnect with a fresh connection).
func (w *Worker) listenOnce(ctx context.Context, wake chan<- struct{}) bool {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		w.log.Error("worker: acquire listen connection failed", "error", err.Error())
		return true
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		if ctx.Err() != nil {
			return false
		}
		w.log.Error("worker: listen failed", "error", err.Error())
		return true
	}

	w.log.Info("worker: notify listener established")
	pg := conn.Conn()
	for {
		n, err := pg.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return false
			}
			w.log.Error("worker: wait notification failed; reconnecting", "error", err.Error())
			return true
		}
		if n == nil || n.Payload == "" {
			continue
		}
		sendWake(ctx, wake)
	}
}

// tickerLoop periodically wakes workers to claim jobs that LISTEN may have
// missed. It sends an initial wake to drain any backlog that predates startup.
func (w *Worker) tickerLoop(ctx context.Context, wake chan<- struct{}) {
	t := time.NewTicker(claimFallbackInterval)
	defer t.Stop()

	sendWake(ctx, wake)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sendWake(ctx, wake)
		}
	}
}

// workerLoop claims and processes pending jobs until the queue is empty, then
// waits for the next wake signal. It never busy-loops: a claim that returns
// nothing simply stops the drain and blocks. Jobs are claimed using ctx (so new
// claims stop at shutdown) but run on jobCtx (so an in-flight job survives the
// shutdown signal and can finish within the grace period).
func (w *Worker) workerLoop(ctx context.Context, jobCtx context.Context, wake <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
			for {
				job, err := w.jobs.ClaimNextPending(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					w.log.Error("worker: claim job", "error", err.Error())
					time.Sleep(time.Second)
					continue
				}
				if job == nil || job.ID == "" {
					break
				}
				w.ProcessJob(jobCtx, job.ID)
				if ctx.Err() != nil {
					return
				}
			}
		}
	}
}

// sendWake performs a non-blocking send so the listener and ticker never block
// on a full buffer or block shutdown when nobody is listening.
func sendWake(ctx context.Context, wake chan<- struct{}) {
	select {
	case <-ctx.Done():
	case wake <- struct{}{}:
	default:
	}
}
