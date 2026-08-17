package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/pratamaWahyuadi/learn-rag/internal/ratelimit"
)

// ---- Rate limit ----

func TestRateLimiterAllowsWithinCapacity(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(2, 1)
	rl := NewRateLimiter(bucket, func(c *gin.Context) string { return "ip-1" })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rl.Limit())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimiterRejectsOverCapacity(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(1, 1)
	rl := NewRateLimiter(bucket, func(c *gin.Context) string { return "ip-1" })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rl.Limit())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i == 0 {
			if w.Code != http.StatusOK {
				t.Fatalf("1st request expected 200, got %d", w.Code)
			}
		} else if w.Code != http.StatusTooManyRequests {
			t.Fatalf("2nd request expected 429, got %d", w.Code)
		}
	}
}

// ---- Logging ----

func TestRequestLoggerLogsMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	rl := NewRequestLogger(logger)

	r := gin.New()
	r.Use(rl.Log())
	r.GET("/hello", func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	out := buf.String()
	for _, want := range []string{`"method":"GET"`, `/hello`, `"status":202`, `"request_id"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("log output missing %q: %s", want, out)
		}
	}
	// Header with the API key must not leak into logs.
	for _, want := range []string{"x-api-key", "authorization"} {
		if bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("log output must not contain %q: %s", want, out)
		}
	}
}

func TestRequestLoggerSetsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	rl := NewRequestLogger(logger)

	r := gin.New()
	r.Use(rl.Log())
	r.GET("/", func(c *gin.Context) {
		if RequestID(c) == "" {
			t.Error("request_id not set in context")
		}
		if c.Writer.Header().Get("X-Request-ID") == "" {
			t.Error("X-Request-ID header not set")
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

// ---- Recovery ----

func TestRecoveryReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	rc := NewRecovery(logger)

	r := gin.New()
	r.Use(rc.Recover())
	r.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if body := w.Body.String(); body == "" {
		t.Fatal("expected a JSON error body")
	}
}
