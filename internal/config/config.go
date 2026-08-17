// Package config provides centralized environment-based configuration loading.
package config

import (
	"log"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the API server and worker,
// mirroring the variables declared in deploy/env.example.
type Config struct {
	// Core
	DatabaseURL       string
	ServerPort        string
	WorkerConcurrency int

	// Cloudflare R2 object storage
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2PublicEndpoint  string

	// External providers
	GroqAPIKey          string
	LlamaParseAPIKey    string
	CloudflareAccountID string
	CloudflareAPIToken  string
	DeepSeekAPIKey      string

	// Behavior tuning
	UploadURLTTLMinutes int
	MaxUploadBytes      int64
	EmbeddingBatchSize  int
	SummaryMaxTokens    int
	QueryResultK        int
}

// Load reads configuration from the environment, applies safe defaults, and
// fatally exits when a required variable is missing or an invalid numeric value
// is provided.
//
// Required: DATABASE_URL.
// Defaults: SERVER_PORT=8080, WORKER_CONCURRENCY=3, UPLOAD_URL_TTL_MINUTES=10,
// MAX_UPLOAD_BYTES=2147483648, EMBEDDING_BATCH_SIZE=16,
// SUMMARY_MAX_TOKENS=12000, QUERY_RESULT_K=5.
func Load() *Config {
	cfg := &Config{
		DatabaseURL:         required("DATABASE_URL"),
		ServerPort:          getenv("SERVER_PORT", "8080"),
		WorkerConcurrency:   getenvInt("WORKER_CONCURRENCY", 3),
		R2AccountID:         os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:       os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:   os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:            os.Getenv("R2_BUCKET"),
		R2PublicEndpoint:    os.Getenv("R2_PUBLIC_ENDPOINT"),
		GroqAPIKey:          os.Getenv("GROQ_API_KEY"),
		LlamaParseAPIKey:    os.Getenv("LLAMAPARSE_API_KEY"),
		CloudflareAccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		CloudflareAPIToken:  os.Getenv("CLOUDFLARE_API_TOKEN"),
		DeepSeekAPIKey:      os.Getenv("DEEPSEEK_API_KEY"),
		UploadURLTTLMinutes: getenvInt("UPLOAD_URL_TTL_MINUTES", 10),
		MaxUploadBytes:      getenvInt64("MAX_UPLOAD_BYTES", 2147483648),
		EmbeddingBatchSize:  getenvInt("EMBEDDING_BATCH_SIZE", 16),
		SummaryMaxTokens:    getenvInt("SUMMARY_MAX_TOKENS", 12000),
		QueryResultK:        getenvInt("QUERY_RESULT_K", 5),
	}

	cfg.validate()
	return cfg
}

func (c *Config) validate() {
	if c.WorkerConcurrency <= 0 {
		log.Fatalf("config: WORKER_CONCURRENCY must be > 0, got %d", c.WorkerConcurrency)
	}
	if c.EmbeddingBatchSize <= 0 {
		log.Fatalf("config: EMBEDDING_BATCH_SIZE must be > 0, got %d", c.EmbeddingBatchSize)
	}
	if c.UploadURLTTLMinutes <= 0 {
		log.Fatalf("config: UPLOAD_URL_TTL_MINUTES must be > 0, got %d", c.UploadURLTTLMinutes)
	}
	if c.MaxUploadBytes <= 0 {
		log.Fatalf("config: MAX_UPLOAD_BYTES must be > 0, got %d", c.MaxUploadBytes)
	}
}

func required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("config: required environment variable %s is not set", key)
	}
	return v
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("config: %s must be an integer, got %q", key, v)
	}
	return n
}

func getenvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Fatalf("config: %s must be an integer, got %q", key, v)
	}
	return n
}
