package config

import (
	"testing"
)

func resetEnv(t *testing.T) {
	t.Helper()
	// Clear all configuration variables so tests control the surface.
	for _, k := range []string{
		"DATABASE_URL",
		"SERVER_PORT",
		"WORKER_CONCURRENCY",
		"UPLOAD_URL_TTL_MINUTES",
		"MAX_UPLOAD_BYTES",
		"EMBEDDING_BATCH_SIZE",
		"SUMMARY_MAX_TOKENS",
		"QUERY_RESULT_K",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	resetEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/rag")

	cfg := Load()

	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/rag" {
		t.Errorf("DatabaseURL = %q, want set value", cfg.DatabaseURL)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want default 8080", cfg.ServerPort)
	}
	if cfg.WorkerConcurrency != 3 {
		t.Errorf("WorkerConcurrency = %d, want default 3", cfg.WorkerConcurrency)
	}
	if cfg.UploadURLTTLMinutes != 10 {
		t.Errorf("UploadURLTTLMinutes = %d, want default 10", cfg.UploadURLTTLMinutes)
	}
	if cfg.MaxUploadBytes != 2147483648 {
		t.Errorf("MaxUploadBytes = %d, want default 2147483648", cfg.MaxUploadBytes)
	}
	if cfg.EmbeddingBatchSize != 16 {
		t.Errorf("EmbeddingBatchSize = %d, want default 16", cfg.EmbeddingBatchSize)
	}
	if cfg.SummaryMaxTokens != 12000 {
		t.Errorf("SummaryMaxTokens = %d, want default 12000", cfg.SummaryMaxTokens)
	}
	if cfg.QueryResultK != 5 {
		t.Errorf("QueryResultK = %d, want default 5", cfg.QueryResultK)
	}
}

func TestLoadReadsExplicitValues(t *testing.T) {
	resetEnv(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/d")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("WORKER_CONCURRENCY", "5")
	t.Setenv("UPLOAD_URL_TTL_MINUTES", "15")
	t.Setenv("MAX_UPLOAD_BYTES", "1048576")
	t.Setenv("EMBEDDING_BATCH_SIZE", "8")
	t.Setenv("SUMMARY_MAX_TOKENS", "6000")
	t.Setenv("QUERY_RESULT_K", "3")

	cfg := Load()

	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want 9090", cfg.ServerPort)
	}
	if cfg.WorkerConcurrency != 5 {
		t.Errorf("WorkerConcurrency = %d, want 5", cfg.WorkerConcurrency)
	}
	if cfg.UploadURLTTLMinutes != 15 {
		t.Errorf("UploadURLTTLMinutes = %d, want 15", cfg.UploadURLTTLMinutes)
	}
	if cfg.MaxUploadBytes != 1048576 {
		t.Errorf("MaxUploadBytes = %d, want 1048576", cfg.MaxUploadBytes)
	}
	if cfg.EmbeddingBatchSize != 8 {
		t.Errorf("EmbeddingBatchSize = %d, want 8", cfg.EmbeddingBatchSize)
	}
	if cfg.SummaryMaxTokens != 6000 {
		t.Errorf("SummaryMaxTokens = %d, want 6000", cfg.SummaryMaxTokens)
	}
	if cfg.QueryResultK != 3 {
		t.Errorf("QueryResultK = %d, want 3", cfg.QueryResultK)
	}
}
