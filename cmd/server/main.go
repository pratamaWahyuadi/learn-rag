// The rag-server binary runs the HTTP API exposing upload intents, job and
// video management, and the RAG query endpoint.
package main

import (
	"context"
	"log"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/rager"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/repo"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/handlers"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/middleware"
	"github.com/pratamaWahyuadi/learn-rag/internal/logging"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/cloudflareai"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/deepseek"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/r2"
)

func main() {
	cfg := config.Load()
	logger := logging.NewLogger()

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("server: open database: %v", err)
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
		log.Fatalf("server: init r2 storage: %v", err)
	}

	embedder, err := cloudflareai.New(cloudflareai.Config{
		AccountID: cfg.CloudflareAccountID,
		APIToken:  cfg.CloudflareAPIToken,
		BatchSize: cfg.EmbeddingBatchSize,
	}, circuitbreaker.Config{
		MaxFailures: cfg.CbMaxFailures,
		Timeout:     cfg.CbTimeout,
	})
	if err != nil {
		log.Fatalf("server: init embedder: %v", err)
	}

	llm, err := deepseek.New(deepseek.Config{APIKey: cfg.DeepSeekAPIKey}, circuitbreaker.Config{
		MaxFailures: cfg.CbMaxFailures,
		Timeout:     cfg.CbTimeout,
	})
	if err != nil {
		log.Fatalf("server: init llm: %v", err)
	}

	apiKeyRepo := repo.NewAPIKeyRepository(pool)
	uploadRepo := repo.NewUploadIntentRepository(pool)
	jobRepo := repo.NewJobRepository(pool)
	segmentRepo := repo.NewSegmentRepository(pool)
	videoRepo := repo.NewVideoRepository(pool)
	chunkRepo := repo.NewChunkRepository(pool)
	summaryRepo := repo.NewSummaryRepository(pool)
	auditRepo := repo.NewAuditLogRepository(pool)

	rag := rager.NewRAGService(embedder, llm, chunkRepo, segmentRepo)

	handler := handlers.NewHandler(
		pool,
		cfg,
		uploadRepo,
		jobRepo,
		segmentRepo,
		videoRepo,
		chunkRepo,
		summaryRepo,
		auditRepo,
		storage,
		embedder,
		llm,
		rag,
	)

	authenticator := middleware.NewAuthenticator(apiKeyRepo)
	router := httpapi.NewRouter(logger, authenticator, handler)

	logger.Info("rag-server listening", "port", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server: run: %v", err)
	}
}
