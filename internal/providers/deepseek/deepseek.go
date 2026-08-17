// Package deepseek implements ports.LLM using the DeepSeek chat completions
// API. Prompts, transcripts, and the API key are never logged.
package deepseek

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/providers/httpclient"
)

const chatURL = "https://api.deepseek.com/chat/completions"

// Config holds the DeepSeek API key.
type Config struct {
	APIKey string
	// Endpoint overrides the default chat completions URL (for tests).
	Endpoint string
}

// LLM implements ports.LLM against DeepSeek chat completions.
type LLM struct {
	client   *httpclient.HTTPClient
	breaker  *circuitbreaker.Breaker
	apiKey   string
	endpoint string
}

// New builds a DeepSeek LLM with a 60 second timeout and 2 retries.
func New(cfg Config, cb circuitbreaker.Config) (*LLM, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("deepseek: API key is required")
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
	return &LLM{
		client:   httpclient.New(httpclient.WithTimeout(60*time.Second), httpclient.WithMaxRetries(2)),
		breaker:  circuitbreaker.New(cb),
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
	}, nil
}

// Compile-time assertion that LLM satisfies ports.LLM.
var _ ports.LLM = (*LLM)(nil)

// chatMessage mirrors the messages payload of the chat completions API.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the body sent to DeepSeek.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

// chatResponse carries the generated assistant content.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Summarize produces a summary for text given a system prompt.
func (l *LLM) Summarize(ctx context.Context, systemPrompt, text string) (string, error) {
	return l.chatCompletion(ctx, systemPrompt, text)
}

// AnswerQuery answers a question given a system prompt and user content.
func (l *LLM) AnswerQuery(ctx context.Context, systemPrompt, userContent string) (string, error) {
	return l.chatCompletion(ctx, systemPrompt, userContent)
}

// chatCompletion sends a single chat request guarded by the circuit breaker.
func (l *LLM) chatCompletion(ctx context.Context, systemPrompt, userContent string) (string, error) {
	return circuitbreaker.Execute(ctx, l.breaker, func() (string, error) {
		body := chatRequest{
			Model: "deepseek-chat",
			Messages: []chatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userContent},
			},
		}

		headers := http.Header{}
		headers.Set("Authorization", "Bearer "+l.apiKey)

		endpoint := l.endpoint
		if endpoint == "" {
			endpoint = chatURL
		}

		var out chatResponse
		if err := l.client.DoJSON(ctx, http.MethodPost, endpoint, headers, body, &out); err != nil {
			return "", fmt.Errorf("deepseek: chat request: %w", err)
		}
		if len(out.Choices) == 0 {
			return "", fmt.Errorf("deepseek: empty choices in response")
		}
		return out.Choices[0].Message.Content, nil
	})
}
