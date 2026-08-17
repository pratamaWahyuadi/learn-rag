// Package cloudflareai implements ports.Embedder using Cloudflare Workers AI
// BGE-M3. Input texts and the API token are never logged.
package cloudflareai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/httpclient"
)

// Config holds the Cloudflare account id and API token for Workers AI.
type Config struct {
	AccountID string
	APIToken  string
	// BatchSize caps how many texts are embedded per request (default 16).
	BatchSize int
	// Endpoint overrides the default Workers AI run URL (for tests).
	Endpoint string
}

// Embedder implements ports.Embedder against Cloudflare BGE-M3.
type Embedder struct {
	client    *httpclient.HTTPClient
	breaker   *circuitbreaker.Breaker
	accountID string
	apiToken  string
	batchSize int
	endpoint  string
}

func New(cfg Config, cb circuitbreaker.Config) (*Embedder, error) {
	if cfg.AccountID == "" || cfg.APIToken == "" {
		return nil, fmt.Errorf("cloudflareai: account id and API token are required")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 16
	}
	if cb.MaxFailures == 0 {
		cb.MaxFailures = 3
	}
	if cb.Timeout == 0 {
		cb.Timeout = 30 * time.Second
	}
	if cb.HalfOpenMaxCalls == 0 {
		cb.HalfOpenMaxCalls = 1
	}
	return &Embedder{
		client:    httpclient.New(),
		breaker:   circuitbreaker.New(cb),
		accountID: cfg.AccountID,
		apiToken:  cfg.APIToken,
		batchSize: cfg.BatchSize,
		endpoint:  cfg.Endpoint,
	}, nil
}

// Compile-time assertion that Embedder satisfies ports.Embedder.
var _ ports.Embedder = (*Embedder)(nil)

// embedRequest is the request body for the Workers AI run endpoint.
type embedRequest struct {
	Texts []string `json:"texts"`
}

// embedResponse carries the `result.data` embedding payload.
type embedResponse struct {
	Result struct {
		Data [][]float32 `json:"data"`
	} `json:"result"`
}

// EmbedBatch embeds every text, batching the calls per BatchSize.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	all := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += e.batchSize {
		end := start + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]
		batchResult, err := circuitbreaker.Execute(ctx, e.breaker, func() ([][]float32, error) {
			return e.embedOne(ctx, batch)
		})
		if err != nil {
			return nil, fmt.Errorf("cloudflareai: embed batch: %w", err)
		}
		all = append(all, batchResult...)
	}
	return all, nil
}

// embedOne embeds a single batch via one API call.
func (e *Embedder) embedOne(ctx context.Context, texts []string) ([][]float32, error) {
	endpoint := e.endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/ai/run/@cf/baai/bge-m3",
			e.accountID,
		)
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+e.apiToken)

	var out embedResponse
	if err := e.client.DoJSON(ctx, http.MethodPost, endpoint, headers, embedRequest{Texts: texts}, &out); err != nil {
		return nil, err
	}

	data := out.Result.Data
	if len(data) != len(texts) {
		return nil, fmt.Errorf("cloudflareai: expected %d embeddings, got %d", len(texts), len(data))
	}
	return data, nil
}
