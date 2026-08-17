// The rag-worker binary runs the background job pipeline: it listens for new
// jobs over Postgres LISTEN/NOTIFY, claims pending jobs with SKIP LOCKED, and
// processes each through download → transcribe/parse → chunk → embed → persist,
// then periodically retires R2 source files.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/summarizer"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/repo"
	"github.com/pratamaWahyuadi/learn-rag/internal/logging"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/cloudflareai"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/deepseek"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/groq"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/llamaparse"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/r2"
	"github.com/pratamaWahyuadi/learn-rag/internal/worker"
)

func main() {
	cfg := config.Load()
	logger := logging.NewLogger()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("worker: open database: %v", err)
	}
	defer pool.Close()

	storage, err := r2.New(r2.Config{
		AccountID:       cfg.R2AccountID,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
		PublicEndpoint:  cfg.R2PublicEndpoint,
	})
	if err != nil {
		log.Fatalf("worker: init r2 storage: %v", err)
	}

	cb := circuitbreaker.Config{
		MaxFailures: cfg.CbMaxFailures,
		Timeout:     cfg.CbTimeout,
	}

	transcriber, err := groq.New(groq.Config{APIKey: cfg.GroqAPIKey}, cb)
	if err != nil {
		log.Fatalf("worker: init transcriber: %v", err)
	}

	parser, err := llamaparse.New(llamaparse.Config{APIKey: cfg.LlamaParseAPIKey}, cb)
	if err != nil {
		log.Fatalf("worker: init parser: %v", err)
	}

	embedder, err := cloudflareai.New(cloudflareai.Config{
		AccountID: cfg.CloudflareAccountID,
		APIToken:  cfg.CloudflareAPIToken,
		BatchSize: cfg.EmbeddingBatchSize,
	}, cb)
	if err != nil {
		log.Fatalf("worker: init embedder: %v", err)
	}

	llm, err := deepseek.New(deepseek.Config{APIKey: cfg.DeepSeekAPIKey}, cb)
	if err != nil {
		log.Fatalf("worker: init llm: %v", err)
	}

	jobRepo := repo.NewJobRepository(pool)
	videoRepo := repo.NewVideoRepository(pool)
	segmentRepo := repo.NewSegmentRepository(pool)
	transcriptRepo := repo.NewTranscriptRepository(pool)
	chunkRepo := repo.NewChunkRepository(pool)
	summaryRepo := repo.NewSummaryRepository(pool)

	wk, err := worker.New(worker.Config{
		Cfg:         cfg,
		Pool:        pool,
		Log:         logger,
		Jobs:        jobRepo,
		Videos:      videoRepo,
		Segments:    segmentRepo,
		Transcripts: transcriptRepo,
		Chunks:      chunkRepo,
		Summaries:   summaryRepo,
		Storage:     storage,
		Transcriber: transcriber,
		Parser:      parser,
		Embedder:    embedder,
		LLM:         llm,
		Summarizer:  summarizer.New(cfg.SummaryMaxTokens),
	})
	if err != nil {
		log.Fatalf("worker: init: %v", err)
	}

	if err := wk.Run(ctx); err != nil {
		log.Fatalf("worker: run: %v", err)
	}
}
