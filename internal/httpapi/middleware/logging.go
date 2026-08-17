package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestLogger logs structured, non-sensitive request metadata. It never logs
// headers, bodies, query parameters, or the X-API-Key header (Threat #7).
type RequestLogger struct {
	logger *slog.Logger
}

// NewRequestLogger builds a RequestLogger writing to the given logger.
func NewRequestLogger(logger *slog.Logger) *RequestLogger {
	return &RequestLogger{logger: logger}
}

// Log returns a middleware that assigns a request_id and logs method, path,
// status, and duration after the request completes.
func (l *RequestLogger) Log() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Assign or reuse a request id and expose it for trace logging.
		requestID := RequestID(c)
		if requestID == "" {
			requestID = uuid.NewString()
			setRequestID(c, requestID)
		}
		// Expose the id to responses for client-side tracing.
		c.Header("X-Request-ID", requestID)

		c.Next()

		l.logger.Info("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestID,
		)
	}
}

// RequestID returns the request id stored on the context, if any.
func RequestID(c *gin.Context) string {
	v, _ := c.Get(string(ctxRequestID))
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func setRequestID(c *gin.Context, id string) { c.Set(string(ctxRequestID), id) }
