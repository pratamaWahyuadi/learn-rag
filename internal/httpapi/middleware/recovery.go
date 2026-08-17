package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	domainerrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
)

// Recovery recovers from panics, logs the stack trace (server-side only), and
// returns a generic 500 response so internal details never reach the client.
type Recovery struct {
	logger *slog.Logger
}

// NewRecovery builds a Recovery writing to the given logger.
func NewRecovery(logger *slog.Logger) *Recovery {
	return &Recovery{logger: logger}
}

// Recover returns a middleware that catches panics and converts them into a 500.
func (r *Recovery) Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				r.logger.Error("panic_recovered",
					"error", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
					"request_id", RequestID(c),
				)
				writeError(c, domainerrors.ErrInternal)
				c.Abort()
			}
		}()
		c.Next()
	}
}
