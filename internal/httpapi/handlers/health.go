package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
)

// Health implements GET /healthz. It is public (no auth) and reports whether
// the database connection is usable. A failing ping returns 503 in the standard
// error format.
func (h *Handler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	dbStatus := "up"
	if err := h.Pool.Ping(ctx); err != nil {
		dbStatus = "down"
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"code":    "internal_error",
				"message": "Terjadi kesalahan internal.",
			},
		})
		return
	}

	c.JSON(http.StatusOK, dto.HealthResponse{
		Status:  "ok",
		Service: "api",
		Time:    time.Now().UTC().Format(time.RFC3339),
		DB:      dbStatus,
	})
}
