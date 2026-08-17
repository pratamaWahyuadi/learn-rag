// Package middleware provides the HTTP middlewares protecting the /api/v1 API:
// API-key authentication with scope enforcement, token-bucket rate limiting,
// request logging, and panic recovery. It deliberately never uses gin.Logger(),
// which would log request headers including secrets.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	domainerrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

// Context keys stored on the request context by middleware.
const (
	ctxTenantID  contextKey = "rent_tenant_id"
	ctxAPIKeyID  contextKey = "rent_api_key_id"
	ctxScope     contextKey = "rent_scope"
	ctxRequestID contextKey = "rent_request_id"
)

// APIKeyStore authenticates API keys and records best-effort usage. The
// concrete *repo.APIKeyRepository satisfies this interface.
type APIKeyStore interface {
	GetByHash(ctx context.Context, hash string) (*model.APIKey, error)
	TouchLastUsed(ctx context.Context, id string) error
}

// Authenticator validates the X-API-Key header and stores identity in the
// context for downstream handlers and scope middleware.
type Authenticator struct {
	keys APIKeyStore
}

// NewAuthenticator builds an Authenticator backed by the given key store.
func NewAuthenticator(keys APIKeyStore) *Authenticator {
	return &Authenticator{keys: keys}
}

// APIKeyHeader is the header carrying the plaintext API key.
const APIKeyHeader = "X-API-Key"

// HashKey returns the SHA-256 hex digest of a plaintext API key.
func HashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// RequireKey returns a middleware that authenticates every request via the
// X-API-Key header. Requests without a valid, non-revoked key get 401.
func (a *Authenticator) RequireKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !a.authenticate(c) {
			c.Abort()
			return
		}
		c.Next()
	}
}

// authenticate resolves, validates, and stores the API key identity. On success
// it also schedules a best-effort last_used_at update. It returns false and
// writes the error response when the key is invalid.
func (a *Authenticator) authenticate(c *gin.Context) bool {
	plain := c.GetHeader(APIKeyHeader)
	if plain == "" {
		writeError(c, domainerrors.ErrUnauthorized)
		return false
	}

	key, err := a.keys.GetByHash(c.Request.Context(), HashKey(plain))
	if err != nil || key.RevokedAt != nil {
		writeError(c, domainerrors.ErrUnauthorized)
		return false
	}

	setTenantID(c, key.TenantID)
	setAPIKeyID(c, key.ID)
	setScope(c, key.Scope)

	// Best-effort usage tracking in the background; never blocks the request.
	id := key.ID
	go func() {
		_ = a.keys.TouchLastUsed(context.Background(), id)
	}()

	return true
}

// RequireScope returns a middleware that rejects requests whose stored scope
// does not match the required scope with 403.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if Scope(c) != scope {
			writeError(c, domainerrors.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// TenantID returns the authenticated tenant id stored on the context.
func TenantID(c *gin.Context) string {
	v, _ := c.Get(string(ctxTenantID))
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// APIKeyID returns the authenticated API key id stored on the context.
func APIKeyID(c *gin.Context) string {
	v, _ := c.Get(string(ctxAPIKeyID))
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Scope returns the scope stored on the context.
func Scope(c *gin.Context) string {
	v, _ := c.Get(string(ctxScope))
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func setTenantID(c *gin.Context, v string) { c.Set(string(ctxTenantID), v) }
func setAPIKeyID(c *gin.Context, v string) { c.Set(string(ctxAPIKeyID), v) }
func setScope(c *gin.Context, v string)    { c.Set(string(ctxScope), v) }

// writeError writes a response in the standard error format:
//
//	{ "error": { "code": "...", "message": "..." } }
func writeError(c *gin.Context, err error) {
	c.AbortWithStatusJSON(
		domainerrors.HTTPStatus(err),
		gin.H{
			"error": gin.H{
				"code":    domainerrors.Code(err),
				"message": domainerrors.Message(err),
			},
		},
	)
}
