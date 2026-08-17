package cloudflareai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
)

func TestEmbedBatchBatchingAndOrder(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		// Return one embedding per received text, sized by call order.
		data := make([][]float32, len(req.Texts))
		for i := range req.Texts {
			data[i] = []float32{float32(n), float32(i)}
		}
		resp := embedResponse{}
		resp.Result.Data = data
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	emb, err := New(Config{
		AccountID: "acct",
		APIToken:  "tok",
		BatchSize: 2,
		Endpoint:  ts.URL,
	}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	texts := []string{"a", "b", "c", "d", "e"}
	got, err := emb.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}

	// 5 texts with batch size 2 => 3 requests (2+2+1).
	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("expected 3 batch requests, got %d", n)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 embeddings, got %d", len(got))
	}
	// The embedding of the first text should come from request 1, value [1 0].
	if len(got[0]) != 2 || got[0][0] != 1 || got[0][1] != 0 {
		t.Fatalf("unexpected first embedding: %v", got[0])
	}
	// The last text belongs to the third batch request.
	if got[4][0] != 3 {
		t.Fatalf("expected last embedding from 3rd request, got %v", got[4])
	}
}

func TestEmbedResultCountMismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embedResponse{}
		resp.Result.Data = [][]float32{{1}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	emb, err := New(Config{AccountID: "a", APIToken: "t", Endpoint: ts.URL}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := emb.EmbedBatch(context.Background(), []string{"x", "y"}); err == nil {
		t.Fatal("expected error on count mismatch")
	}
}

func TestEmbedBatchEmpty(t *testing.T) {
	emb, err := New(Config{AccountID: "a", APIToken: "t", Endpoint: "http://x"}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := emb.EmbedBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("EmbedBatch empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 embeddings, got %d", len(got))
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(Config{}, circuitbreaker.Config{}); err == nil {
		t.Fatal("expected error when credentials missing")
	}
}
