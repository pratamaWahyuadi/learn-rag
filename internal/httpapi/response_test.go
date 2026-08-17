package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	domainerrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func perform(t *testing.T, handler func(c *gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/test", handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestErrorMapsDNSentinel(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not_found", domainerrors.ErrNotFound, http.StatusNotFound, "not_found"},
		{"rate_limited", domainerrors.ErrRateLimited, http.StatusTooManyRequests, "rate_limited"},
		{"unauthorized", domainerrors.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := perform(t, func(c *gin.Context) {
				Error(c, tc.err)
			})

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON body: %v", err)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("error.code = %q, want %q", body.Error.Code, tc.wantCode)
			}
			if body.Error.Message == "" {
				t.Error("error.message must not be empty")
			}
		})
	}
}

func TestErrorUnknownMapsToInternal(t *testing.T) {
	w := perform(t, func(c *gin.Context) {
		Error(c, errors.New("database: connection refused: 10.0.0.1"))
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error.code = %q, want internal_error", body.Error.Code)
	}
	if body.Error.Message == "database: connection refused: 10.0.0.1" {
		t.Error("internal error details must not leak to the client")
	}
}

func TestSuccess(t *testing.T) {
	w := perform(t, func(c *gin.Context) {
		Success(c, http.StatusCreated, gin.H{"id": "abc", "status": "issued"})
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	want := `{"id":"abc","status":"issued"}`
	if got := w.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}
