package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
)

// ListVideos implements GET /api/v1/videos (scope admin). It only returns
// non-deleted videos owned by the tenant, optionally filtered by segment name
// and status with page-based pagination.
func (h *Handler) ListVideos(c *gin.Context) {
	tenantID := TenantID(c)
	segment := c.Query("segment")
	status := c.Query("status")
	page, limit, err := pagination(c)
	if err != nil {
		writeError(c, err)
		return
	}

	videos, err := h.Videos.List(c.Request.Context(), tenantID, segment, status, page, limit)
	if err != nil {
		writeError(c, err)
		return
	}

	total, err := h.Videos.Count(c.Request.Context(), tenantID, segment, status)
	if err != nil {
		writeError(c, err)
		return
	}

	data := make([]dto.VideoResponse, 0, len(videos))
	for _, v := range videos {
		data = append(data, toVideoResponse(v))
	}

	c.JSON(http.StatusOK, dto.ListVideosResponse{Data: data, Meta: dto.Meta{Page: page, Limit: limit, Total: total}})
}

// GetVideo implements GET /api/v1/videos/:id (scope admin). It returns the video
// with its segments and optional summary. A video owned by another tenant or
// already soft-deleted is treated as 404.
func (h *Handler) GetVideo(c *gin.Context) {
	tenantID := TenantID(c)
	videoID := c.Param("id")

	video, err := h.Videos.GetByID(c.Request.Context(), videoID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}

	resp := dto.VideoDetailResponse{VideoResponse: toVideoResponse(*video)}

	summary, err := h.Summaries.GetByVideoID(c.Request.Context(), videoID, tenantID)
	if err != nil {
		if !errors.Is(err, apierrors.ErrNotFound) {
			writeError(c, err)
			return
		}
		resp.Summary = nil
	} else {
		s := toSummaryResponse(*summary)
		resp.Summary = &s
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteVideo implements DELETE /api/v1/videos/:id (scope admin). It soft-deletes
// a tenant-owned, non-deleted video, and records an audit entry. A missing or
// already-deleted video is treated as 404.
func (h *Handler) DeleteVideo(c *gin.Context) {
	tenantID := TenantID(c)
	videoID := c.Param("id")

	deletedAt, err := h.Videos.SoftDelete(c.Request.Context(), videoID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}

	_ = h.insertAudit(c, "video.delete", &videoID, nil)

	c.JSON(http.StatusOK, dto.DeleteVideoResponse{ID: videoID, DeletedAt: *deletedAt})
}
