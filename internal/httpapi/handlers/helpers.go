package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/middleware"
)

// writeError writes a response in the shared error format:
//
//	{ "error": { "code": "...", "message": "..." } }
//
// Unknown errors (anything that is not an *APIError) are mapped to 500
// internal_error so internal details never reach the client.
func writeError(c *gin.Context, err error) {
	c.AbortWithStatusJSON(
		apierrors.HTTPStatus(err),
		gin.H{
			"error": gin.H{
				"code":    apierrors.Code(err),
				"message": apierrors.Message(err),
			},
		},
	)
}

// TenantID returns the authenticated tenant id stored on the request context.
func TenantID(c *gin.Context) string {
	return middleware.TenantID(c)
}

// insertAudit best-effort writes an internal audit log record. Audit logs are
// security-relevant but never block the primary request, so insert errors are
// intentionally ignored by callers.
func (h *Handler) insertAudit(c *gin.Context, action string, objectID *string, metadata map[string]any) error {
	keyID := middleware.APIKeyID(c)
	var actorKeyID *string
	if keyID != "" {
		actorKeyID = &keyID
	}
	return h.Audit.Insert(c.Request.Context(), &model.AuditLog{
		ID:         uuid.NewString(),
		TenantID:   TenantID(c),
		ActorKeyID: actorKeyID,
		Action:     action,
		ObjectID:   objectID,
		Metadata:   metadata,
		CreatedAt:  now(),
	})
}

// now returns the current UTC time.
func now() time.Time {
	return time.Now().UTC()
}
