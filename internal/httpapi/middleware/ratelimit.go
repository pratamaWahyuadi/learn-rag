package middleware

import (
	"github.com/gin-gonic/gin"

	domainerrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/ratelimit"
)

// RateLimiter applies a token-bucket rate limit keyed per-request, optionally
// scoped to a tenant or API key.
type RateLimiter struct {
	bucket *ratelimit.TokenBucket
	keyFn  func(c *gin.Context) string
}

// NewRateLimiter builds a RateLimiter using the provided bucket and key
// function. The key function typically returns the API key id or client IP.
func NewRateLimiter(bucket *ratelimit.TokenBucket, keyFn func(c *gin.Context) string) *RateLimiter {
	if keyFn == nil {
		keyFn = func(c *gin.Context) string { return c.ClientIP() }
	}
	return &RateLimiter{bucket: bucket, keyFn: keyFn}
}

// Limit returns a middleware that rejects requests exceeding the configured
// rate with 429.
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.bucket.Allow(rl.keyFn(c)) {
			writeError(c, domainerrors.ErrRateLimited)
			c.Abort()
			return
		}
		c.Next()
	}
}
