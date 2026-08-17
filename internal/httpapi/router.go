// Package httpapi assembles the HTTP router with the authentication, rate
// limiting, request logging, and recovery middlewares. It never uses gin.Logger()
// because that would log request headers including secrets.
package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/handlers"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/middleware"
	"github.com/pratamaWahyuadi/learn-rag/internal/ratelimit"
)

// rate limits (token bucket capacity = burst, refill per second derived from
// the per-minute budgets specified in the API Contract).
const (
	adminQuotaPerMinute = 5.0
	queryQuotaPerMinute = 10.0
)

// NewRouter builds the Gin engine with all routes and middleware wired.
func NewRouter(logger *slog.Logger, authenticator *middleware.Authenticator, h *handlers.Handler) *gin.Engine {
	r := gin.New()
	r.Use(
		middleware.NewRecovery(logger).Recover(),
		middleware.NewRequestLogger(logger).Log(),
	)

	r.GET("/healthz", h.Health)

	api := r.Group("/api/v1")
	api.Use(authenticator.RequireKey())

	admin := api.Group("")
	admin.Use(
		middleware.RequireScope("admin"),
		newRateLimiter(adminQuotaPerMinute, adminKey).Limit(),
	)
	admin.POST("/upload-intents", h.CreateUploadIntent)
	admin.POST("/jobs", h.CreateJob)
	admin.GET("/jobs", h.ListJobs)
	admin.GET("/jobs/:id", h.GetJob)
	admin.POST("/jobs/:id/retry", h.RetryJob)
	admin.GET("/videos", h.ListVideos)
	admin.GET("/videos/:id", h.GetVideo)
	admin.DELETE("/videos/:id", h.DeleteVideo)

	query := api.Group("")
	query.Use(
		middleware.RequireScope("query"),
		newRateLimiter(queryQuotaPerMinute, queryKey).Limit(),
	)
	query.POST("/query", h.Query)

	return r
}

// adminKey keys the admin rate limit bucket by the authenticated API key id,
// falling back to the client IP when no key id is present.
func adminKey(c *gin.Context) string {
	if id := middleware.APIKeyID(c); id != "" {
		return "admin:" + id
	}
	return "admin:ip:" + c.ClientIP()
}

// queryKey keys the query rate limit bucket by the authenticated API key id,
// falling back to the client IP when no key id is present.
func queryKey(c *gin.Context) string {
	if id := middleware.APIKeyID(c); id != "" {
		return "query:" + id
	}
	return "query:ip:" + c.ClientIP()
}

// newRateLimiter builds a token bucket allowing quotaPerMinute tokens, refilled
// continuously, and wraps it in a RateLimiter middleware.
func newRateLimiter(quotaPerMinute float64, keyFn func(c *gin.Context) string) *middleware.RateLimiter {
	bucket := ratelimit.NewTokenBucket(quotaPerMinute, quotaPerMinute/60.0)
	return middleware.NewRateLimiter(bucket, keyFn)
}
