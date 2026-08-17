package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
)

// maxContentTypeLength bounds the request content_type field size.
const maxContentTypeLength = 128

// CreateUploadIntent implements POST /api/v1/upload-intents (scope admin). It
// creates an upload intent and returns a presigned URL for direct-to-R2 upload.
func (h *Handler) CreateUploadIntent(c *gin.Context) {
	var req dto.CreateUploadIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, apierrors.ErrInvalidRequest)
		return
	}
	if len(req.ContentType) == 0 || len(req.ContentType) > maxContentTypeLength {
		writeError(c, apierrors.ErrInvalidRequest)
		return
	}

	contentType, ok := dto.AllowedContentTypes[req.ContentType]
	if !ok {
		writeError(c, apierrors.ErrUnsupportedContentType)
		return
	}

	tenantID := TenantID(c)
	intentID := uuid.NewString()
	fileKey := tenantID + "/" + intentID + "/" + uuid.NewString() + contentType.Ext
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(h.Cfg.UploadURLTTLMinutes) * time.Minute)

	intent := &model.UploadIntent{
		ID:          intentID,
		TenantID:    tenantID,
		FileKey:     fileKey,
		ContentType: req.ContentType,
		Status:      "issued",
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	}
	if err := h.Uploads.Create(c.Request.Context(), intent); err != nil {
		writeError(c, err)
		return
	}

	presignedURL, err := h.Storage.GenerateUploadURL(c.Request.Context(), fileKey, req.ContentType, expiresAt)
	if err != nil {
		writeError(c, err)
		return
	}

	_ = h.insertAudit(c, "upload_intent.create", &intentID, nil)

	c.JSON(http.StatusCreated, dto.CreateUploadIntentResponse{
		ID:           intentID,
		TenantID:     tenantID,
		FileKey:      fileKey,
		ContentType:  req.ContentType,
		Status:       "issued",
		PresignedURL: presignedURL,
		ExpiresAt:    expiresAt,
	})
}
