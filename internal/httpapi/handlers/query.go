package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
)

const maxQuestionLength = 1000

// Query implements POST /api/v1/query (scope query). It runs the RAG service and
// returns the answer plus source references.
func (h *Handler) Query(c *gin.Context) {
	var req dto.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, apierrors.ErrInvalidRequest)
		return
	}

	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeError(c, apierrors.ErrQuestionRequired)
		return
	}
	if len(question) > maxQuestionLength {
		writeError(c, apierrors.ErrInvalidRequest)
		return
	}

	tenantID := TenantID(c)

	result, err := h.RAG.Answer(c.Request.Context(), tenantID, question, req.Segment, h.Cfg.QueryResultK)
	if err != nil {
		writeError(c, err)
		return
	}

	refs := make([]dto.Reference, 0, len(result.References))
	for _, ref := range result.References {
		refs = append(refs, dto.Reference{
			VideoID:    ref.VideoID,
			VideoTitle: ref.VideoTitle,
			ChunkIndex: ref.ChunkIndex,
			Snippet:    ref.Snippet,
			Segments:   ref.Segments,
		})
	}

	c.JSON(http.StatusOK, dto.QueryResponse{
		Answer:     result.Answer,
		References: refs,
	})
}
