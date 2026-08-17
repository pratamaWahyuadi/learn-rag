// Package groq implements ports.Transcriber using Groq Whisper Large. File
// content, transcripts, and the API key are never logged.
package groq

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
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/httpclient"
)

const transcriptionURL = "https://api.groq.com/openai/v1/audio/transcriptions"

// Config holds the Groq API key.
type Config struct {
	APIKey string
	// Endpoint overrides the default transcription URL (for tests).
	Endpoint string
}

// Transcriber implements ports.Transcriber against Groq Whisper Large.
type Transcriber struct {
	client   *httpclient.HTTPClient
	breaker  *circuitbreaker.Breaker
	apiKey   string
	endpoint string
}

// Compile-time assertion that Transcriber satisfies ports.Transcriber.
var _ ports.Transcriber = (*Transcriber)(nil)

// New builds a Transcriber with the given API key and circuit breaker config.
func New(cfg Config, cb circuitbreaker.Config) (*Transcriber, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("groq: API key is required")
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
	return &Transcriber{
		client:   httpclient.New(),
		breaker:  circuitbreaker.New(cb),
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
	}, nil
}

// transcriptionResponse is the subset of the Groq response we care about.
type transcriptionResponse struct {
	Text     string  `json:"text"`
	Language *string `json:"language"`
}

// Transcribe sends the media file to Groq Whisper and returns the transcript.
func (t *Transcriber) Transcribe(ctx context.Context, input ports.TranscribeInput) (*model.Transcript, error) {
	return circuitbreaker.Execute(ctx, t.breaker, func() (*model.Transcript, error) {
		body, contentType, err := buildMultipart(input)
		if err != nil {
			return nil, err
		}

		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+t.apiKey)

		endpoint := t.endpoint
		if endpoint == "" {
			endpoint = transcriptionURL
		}

		resp, err := t.client.Do(ctx, http.MethodPost, endpoint, headers, body, contentType)
		if err != nil {
			return nil, fmt.Errorf("groq: transcribe request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Drain body to allow connection reuse; body is never logged.
			_, _ = io.Copy(io.Discard, resp.Body)
			return nil, fmt.Errorf("groq: unexpected status %d", resp.StatusCode)
		}

		var out transcriptionResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, fmt.Errorf("groq: decode response: %w", err)
		}
		if out.Text == "" {
			return nil, fmt.Errorf("groq: empty transcript in response")
		}

		modelName := "whisper-large-v3"
		if input.Model != "" {
			modelName = input.Model
		}

		language := out.Language
		if language == nil && input.Language != "" {
			language = &input.Language
		}

		return &model.Transcript{
			Content:  out.Text,
			Language: language,
			Model:    &modelName,
		}, nil
	})
}

// buildMultipart assembles the Groq multipart form body from the local file.
func buildMultipart(input ports.TranscribeInput) ([]byte, string, error) {
	fileData, err := os.ReadFile(input.FilePath)
	if err != nil {
		return nil, "", fmt.Errorf("groq: read media file: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("model", "whisper-large-v3"); err != nil {
		return nil, "", err
	}
	if input.Language != "" {
		if err := writer.WriteField("language", input.Language); err != nil {
			return nil, "", err
		}
	}

	part, err := writer.CreateFormFile("file", filepath.Base(input.FilePath))
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), writer.FormDataContentType(), nil
}
