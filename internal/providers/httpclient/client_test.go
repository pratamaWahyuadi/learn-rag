package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type echoBody struct {
	Message string `json:"message"`
}

func TestDoJSONSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	defer ts.Close()

	client := New(WithMaxRetries(2))
	var out echoBody
	err := client.DoJSON(context.Background(), http.MethodGet, ts.URL, nil, nil, &out)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if out.Message != "ok" {
		t.Fatalf("unexpected body: %+v", out)
	}
}

func TestDoJSONRetriesOnServerError(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"recovered"}`))
	}))
	defer ts.Close()

	client := New(WithMaxRetries(2))
	var out echoBody
	err := client.DoJSON(context.Background(), http.MethodGet, ts.URL, nil, nil, &out)
	if err != nil {
		t.Fatalf("DoJSON after retries: %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 calls after 2 retries, got %d", calls)
	}
	if out.Message != "recovered" {
		t.Fatalf("unexpected body: %+v", out)
	}
}

func TestDoJSONFailsAfterRetriesExhausted(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "still down", http.StatusBadGateway)
	}))
	defer ts.Close()

	client := New(WithMaxRetries(2))
	err := client.DoJSON(context.Background(), http.MethodGet, ts.URL, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	re, ok := err.(*ResponseError)
	if !ok {
		t.Fatalf("expected *ResponseError, got %T: %v", err, err)
	}
	if re.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", re.StatusCode)
	}
	// 1 initial + 2 retries.
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 total calls, got %d", calls)
	}
}

func TestDoJSONContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the test timeout to force context cancellation.
		time.Sleep(500 * time.Millisecond)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := New(WithMaxRetries(2))
	err := client.DoJSON(ctx, http.MethodGet, ts.URL, nil, nil, nil)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestMaxRetriesDisabled(t *testing.T) {
	// Verify WithMaxRetries(0) disables retries (single attempt).
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "busy", http.StatusTooManyRequests)
	}))
	defer ts.Close()

	client := New(WithMaxRetries(0))
	err := client.DoJSON(context.Background(), http.MethodGet, ts.URL, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRequestJSONEncoding(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got echoBody
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"` + got.Message + `"}`))
	}))
	defer ts.Close()

	client := New(WithMaxRetries(2))
	var out echoBody
	err := client.DoJSON(context.Background(), http.MethodPost, ts.URL, nil, echoBody{Message: "hello"}, &out)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if out.Message != "hello" {
		t.Fatalf("unexpected echo: %s", out.Message)
	}
}

func TestDoStreamRetriesReplayBody(t *testing.T) {
	var calls, bodyProviders int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	client := New(WithMaxRetries(2))
	resp, err := client.DoStream(context.Background(), http.MethodGet, ts.URL, nil, "text/plain", func() (io.ReadCloser, error) {
		atomic.AddInt32(&bodyProviders, 1)
		return io.NopCloser(strings.NewReader("some-body")), nil
	})
	if err != nil {
		t.Fatalf("DoStream: %v", err)
	}
	defer resp.Body.Close()

	// 1 initial + 2 retries, and the body provider must be re-invoked per attempt.
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 requests, got %d", calls)
	}
	if atomic.LoadInt32(&bodyProviders) != 3 {
		t.Fatalf("expected body provider called 3 times, got %d", bodyProviders)
	}
}

func TestDoStreamNilBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	client := New(WithMaxRetries(2))
	if _, err := client.DoStream(context.Background(), http.MethodGet, ts.URL, nil, "", func() (io.ReadCloser, error) {
		return nil, nil
	}); err == nil {
		t.Fatal("expected error when body provider returns nil")
	}
}
