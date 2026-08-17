// Package httpapi contains the HTTP API layer (handlers, router, middleware,
// and shared response helpers).
package httpapi

import (
	"github.com/gin-gonic/gin"

	domainerrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
)

// Error writes a consistent error response in the format required by the API
// Contract:
//
//	{ "error": { "code": "...", "message": "..." } }
//
// Unknown errors (anything that is not a *errors.APIError) are always returned
// as 500 internal_error so internal details never leak to the client.
func Error(c *gin.Context, err error) {
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

// Success writes a successful JSON response with the given status and body.
func Success(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

// NoContent writes an empty response with the given status.
func NoContent(c *gin.Context, status int) {
	c.Status(status)
}
