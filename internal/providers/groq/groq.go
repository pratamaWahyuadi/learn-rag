// Package groq implements ports.Transcriber using Groq Whisper Large. File
// content, transcripts, and the API key are never logged.
package groq

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// defaultModel is used when the caller does not request a specific model.
const defaultModel = "whisper-large-v3"

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
		modelName := defaultModel
		if input.Model != "" {
			modelName = input.Model
		}

		// Build the multipart boundary once so the Content-Type header stays
		// consistent across retry attempts that each rebuild the body.
		boundary, err := newBoundary()
		if err != nil {
			return nil, fmt.Errorf("groq: generate boundary: %w", err)
		}
		contentType := "multipart/form-data; boundary=" + boundary

		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+t.apiKey)

		endpoint := t.endpoint
		if endpoint == "" {
			endpoint = transcriptionURL
		}

		body := buildMultipart(input.FilePath, input.Language, modelName, boundary)
		resp, err := t.client.DoStream(ctx, http.MethodPost, endpoint, headers, contentType, body)
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

// newBoundary returns a random, URL-safe multipart boundary.
func newBoundary() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "----ragGroqBoundary" + hex.EncodeToString(buf), nil
}

// buildMultipart returns a function that produces a fresh streaming multipart
// body for the given file using the provided boundary. Each invocation opens a
// new file handle and must be closed by the caller (the http client closes the
// returned ReadCloser after use).
func buildMultipart(filePath, language, model, boundary string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("groq: open media file: %w", err)
		}

		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		// writer.FormDataContentType() generates its own random boundary, so
		// override it to match the boundary we advertised in the header.
		_ = writer.SetBoundary(boundary)

		go func() {
			_ = pw.CloseWithError(func() error {
				defer file.Close()
				if err := writer.WriteField("model", model); err != nil {
					return err
				}
				if language != "" {
					if err := writer.WriteField("language", language); err != nil {
						return err
					}
				}
				part, err := writer.CreateFormFile("file", filepath.Base(filePath))
				if err != nil {
					return err
				}
				if _, err := io.Copy(part, file); err != nil {
					return err
				}
				return writer.Close()
			}())
		}()

		return pr, nil
	}
}
