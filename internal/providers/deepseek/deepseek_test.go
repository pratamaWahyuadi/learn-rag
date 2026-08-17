package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
)

func TestSummarizeAndAnswerQuery(t *testing.T) {
	var gotBody chatRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer-text"}}]}`))
	}))
	defer ts.Close()

	llm, err := New(Config{APIKey: "test-key", Endpoint: ts.URL}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	res, err := llm.Summarize(ctx, "sys", "doc content")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if res != "answer-text" {
		t.Fatalf("unexpected summary: %q", res)
	}

	if gotBody.Model != "deepseek-chat" {
		t.Fatalf("expected model deepseek-chat, got %q", gotBody.Model)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "sys" {
		t.Fatalf("unexpected system message: %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "doc content" {
		t.Fatalf("unexpected user message: %+v", gotBody.Messages[1])
	}

	res, err = llm.AnswerQuery(ctx, "sys", "question context")
	if err != nil {
		t.Fatalf("AnswerQuery: %v", err)
	}
	if res != "answer-text" {
		t.Fatalf("unexpected answer: %q", res)
	}
}

func TestChatCompletionEmptyChoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer ts.Close()

	llm, err := New(Config{APIKey: "k", Endpoint: ts.URL}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := llm.Summarize(context.Background(), "s", "d"); err == nil {
		t.Fatal("expected error on empty choices")
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{}, circuitbreaker.Config{}); err == nil {
		t.Fatal("expected error when API key missing")
	}
}
