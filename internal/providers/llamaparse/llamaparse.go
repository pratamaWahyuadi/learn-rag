// Package llamaparse implements ports.DocumentParser using LlamaParse. File
// content and the API key are never logged. There is no OCR fallback: an
// extraction failure is returned as an error so the job is marked failed.
package llamaparse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/httpclient"
)

const (
	uploadURL = "https://api.llamacloud.com/v1/parsing/upload"
	pollURL   = "https://api.llamacloud.com/v1/parsing/job/%s"
	resultURL = "https://api.llamacloud.com/v1/parsing/job/%s/result/raw/markdown"
)

// Config holds the LlamaParse API key.
type Config struct {
	APIKey string
	// PollInterval is the delay between status polls (default 2s).
	PollInterval time.Duration
	// MaxPolls caps the number of polling attempts (default 60).
	MaxPolls int
}

// DocumentParser implements ports.DocumentParser against LlamaParse.
type DocumentParser struct {
	client    *httpclient.HTTPClient
	breaker   *circuitbreaker.Breaker
	apiKey    string
	pollEvery time.Duration
	maxPolls  int
}

// New builds a DocumentParser with the given API key and circuit breaker config.
func New(cfg Config, cb circuitbreaker.Config) (*DocumentParser, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("llamaparse: API key is required")
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.MaxPolls == 0 {
		cfg.MaxPolls = 60
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
	return &DocumentParser{
		client:    httpclient.New(),
		breaker:   circuitbreaker.New(cb),
		apiKey:    cfg.APIKey,
		pollEvery: cfg.PollInterval,
		maxPolls:  cfg.MaxPolls,
	}, nil
}

// Compile-time assertion that DocumentParser satisfies ports.DocumentParser.
var _ ports.DocumentParser = (*DocumentParser)(nil)

// Parse uploads the file, polls until extraction finishes, and returns the
// extracted text. Extraction failures return an error (no OCR fallback).
func (d *DocumentParser) Parse(ctx context.Context, filePath, _ string) (string, error) {
	return circuitbreaker.Execute(ctx, d.breaker, func() (string, error) {
		jobID, err := d.upload(ctx, filePath)
		if err != nil {
			return "", err
		}
		if err := d.waitUntilDone(ctx, jobID); err != nil {
			return "", err
		}
		return d.fetchResult(ctx, jobID)
	})
}

// upload submits the file and returns the job id.
func (d *DocumentParser) upload(ctx context.Context, filePath string) (string, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("llamaparse: read file: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(fileData); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(ctx, http.MethodPost, uploadURL, headers, buf.Bytes(), writer.FormDataContentType())
	if err != nil {
		return "", fmt.Errorf("llamaparse: upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("llamaparse: upload status %d", resp.StatusCode)
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("llamaparse: decode upload response: %w", err)
	}
	if out.ID == "" {
		return "", fmt.Errorf("llamaparse: upload returned empty job id")
	}
	return out.ID, nil
}

// waitUntilDone polls the job status until it reaches a terminal state.
func (d *DocumentParser) waitUntilDone(ctx context.Context, jobID string) error {
	for i := 0; i < d.maxPolls; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.pollEvery):
		}

		var out struct {
			Status string `json:"status"`
		}
		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+d.apiKey)
		if err := d.client.DoJSON(ctx, http.MethodGet, fmt.Sprintf(pollURL, jobID), headers, nil, &out); err != nil {
			return fmt.Errorf("llamaparse: poll: %w", err)
		}

		switch out.Status {
		case "SUCCESS":
			return nil
		case "ERROR", "CANCELED":
			return fmt.Errorf("llamaparse: job %s ended with status %q", jobID, out.Status)
		}
	}
	return fmt.Errorf("llamaparse: poll timeout for job %s", jobID)
}

// fetchResult retrieves the extracted text as markdown.
func (d *DocumentParser) fetchResult(ctx context.Context, jobID string) (string, error) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(ctx, http.MethodGet, fmt.Sprintf(resultURL, jobID), headers, nil, "")
	if err != nil {
		return "", fmt.Errorf("llamaparse: fetch result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("llamaparse: result status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llamaparse: read result: %w", err)
	}
	return string(data), nil
}
