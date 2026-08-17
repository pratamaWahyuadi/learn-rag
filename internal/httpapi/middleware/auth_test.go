package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
)

var errNotFound = errors.New("not found")

// fakeKeyStore implements APIKeyStore for tests.
type fakeKeyStore struct {
	keys map[string]*model.APIKey // keyed by SHA-256 hash
}

func (f *fakeKeyStore) GetByHash(_ context.Context, hash string) (*model.APIKey, error) {
	if k, ok := f.keys[hash]; ok {
		return k, nil
	}
	return nil, errNotFound
}

func (f *fakeKeyStore) TouchLastUsed(_ context.Context, _ string) error { return nil }

func newTestAuth() (*Authenticator, *fakeKeyStore) {
	now := time.Now()
	store := &fakeKeyStore{keys: map[string]*model.APIKey{}}
	store.keys[HashKey("admin-key")] = &model.APIKey{ID: "key-admin", TenantID: "tenant-1", Scope: "admin"}
	store.keys[HashKey("query-key")] = &model.APIKey{ID: "key-query", TenantID: "tenant-1", Scope: "query"}
	store.keys[HashKey("revoked-key")] = &model.APIKey{ID: "key-revoked", TenantID: "tenant-2", Scope: "admin", RevokedAt: &now}
	return NewAuthenticator(store), store
}

func setupRouter(auth *Authenticator, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.RequireKey())
	r.GET("/protected", handler)
	return r
}

func doRequest(t *testing.T, r http.Handler, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if key != "" {
		req.Header.Set(APIKeyHeader, key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthenticateValidKey(t *testing.T) {
	auth, _ := newTestAuth()

	r := setupRouter(auth, func(c *gin.Context) {
		if TenantID(c) != "tenant-1" || APIKeyID(c) != "key-admin" || Scope(c) != "admin" {
			t.Fatalf("context values not set: tenant=%q key=%q scope=%q", TenantID(c), APIKeyID(c), Scope(c))
		}
		c.Status(http.StatusOK)
	})

	if w := doRequest(t, r, "admin-key"); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthenticateMissingKey(t *testing.T) {
	auth, _ := newTestAuth()
	r := setupRouter(auth, func(c *gin.Context) { c.Status(http.StatusOK) })

	if w := doRequest(t, r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthenticateUnknownKey(t *testing.T) {
	auth, _ := newTestAuth()
	r := setupRouter(auth, func(c *gin.Context) { c.Status(http.StatusOK) })

	if w := doRequest(t, r, "does-not-exist"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthenticateRevokedKey(t *testing.T) {
	auth, _ := newTestAuth()
	r := setupRouter(auth, func(c *gin.Context) { c.Status(http.StatusOK) })

	if w := doRequest(t, r, "revoked-key"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireScope(t *testing.T) {
	auth, _ := newTestAuth()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.RequireKey())
	admin := r.Group("/admin")
	admin.Use(RequireScope("admin"))
	admin.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })

	// query-scoped key hitting an admin endpoint → 403.
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set(APIKeyHeader, "query-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for query scope on admin route, got %d", w.Code)
	}

	// admin-scoped key → 200.
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set(APIKeyHeader, "admin-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin scope, got %d", w.Code)
	}
}

func TestHashKeyMatchesSHA256(t *testing.T) {
	// SHA-256 of "secret" (known test vector).
	const want = "2bb80d537b1da3e38bd30361aa855686bde0eacd7162fef6a25fe97bf527a25b"
	sum := sha256.Sum256([]byte("secret"))
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("sha256 mismatch: %s", got)
	}
	if got := HashKey("secret"); got != want {
		t.Fatalf("HashKey(secret) = %s, want %s", got, want)
	}
}
