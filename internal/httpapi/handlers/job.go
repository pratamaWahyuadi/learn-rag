package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
)

const (
	maxJobTitleLength    = 255
	maxSegmentsCount     = 50
	maxSegmentNameLength = 100
	currentPageDefault   = 1
	listLimitDefault     = 10
	listLimitMax         = 100
)

// CreateJob implements POST /api/v1/jobs (scope admin). It consumes an issued,
// non-expired upload intent owned by the tenant and creates a job whose kind is
// derived from the intent's content type. The whole flow runs in a single
// tenant-scoped transaction guarded by FOR UPDATE so the intent cannot be
// consumed twice concurrently.
func (h *Handler) CreateJob(c *gin.Context) {
	var req dto.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, apierrors.ErrInvalidRequest)
		return
	}
	if err := validateCreateJob(&req); err != nil {
		writeError(c, err)
		return
	}

	tenantID := TenantID(c)

	var jobResp dto.JobResponse

	err := database.WithTenantTx(c.Request.Context(), h.Pool, tenantID, func(q *queries.Queries) error {
		// Lock + validate the upload intent (ownership enforced by tenant_id).
		intent, err := q.GetUploadIntentByFileKeyForUpdate(c.Request.Context(), queries.GetUploadIntentByFileKeyForUpdateParams{
			FileKey:  req.FileKey,
			TenantID: tenantID,
		})
		if err != nil {
			return mapNoRows(err, apierrors.ErrNotFound)
		}
		if intent.Status != "issued" {
			return apierrors.ErrUploadIntentConsumed
		}
		if !time.Now().UTC().Before(intent.ExpiresAt.Time) {
			return apierrors.ErrExpiredUploadIntent
		}

		// Mark the intent consumed.
		if _, err := q.MarkUploadIntentConsumed(c.Request.Context(), queries.MarkUploadIntentConsumedParams{
			FileKey:  req.FileKey,
			TenantID: tenantID,
		}); err != nil {
			return err
		}

		// Derive the pipeline kind from the intent's content type, not the client.
		contentType, ok := dto.AllowedContentTypes[intent.ContentType]
		if !ok {
			return apierrors.ErrUnsupportedContentType
		}

		// Ensure segments exist and collect their IDs.
		segmentIDs := make([]string, 0, len(req.Segments))
		for _, name := range req.Segments {
			seg, err := q.EnsureSegmentByName(c.Request.Context(), queries.EnsureSegmentByNameParams{
				ID:        uuid.NewString(),
				TenantID:  tenantID,
				Name:      name,
				CreatedAt: toTimestamptz(time.Now().UTC()),
			})
			if err != nil {
				return err
			}
			segmentIDs = append(segmentIDs, seg.ID)
		}

		// Create the job.
		now := time.Now().UTC()
		job, err := q.CreateJob(c.Request.Context(), queries.CreateJobParams{
			ID:             uuid.NewString(),
			TenantID:       tenantID,
			UploadIntentID: toUUID(intent.ID),
			FileKey:        req.FileKey,
			Title:          req.Title,
			Kind:           contentType.Kind,
			Status:         model.JobStatusPending,
			Stage:          "queued",
			RetryCount:     0,
			CreatedAt:      toTimestamptz(now),
			UpdatedAt:      toTimestamptz(now),
		})
		if err != nil {
			return err
		}

		for _, sid := range segmentIDs {
			if err := q.AttachJobSegment(c.Request.Context(), queries.AttachJobSegmentParams{
				JobID:     job.ID,
				SegmentID: sid,
				TenantID:  tenantID,
			}); err != nil {
				return err
			}
		}

		// Collect segment names for the response.
		segmentNames, err := q.ListSegmentNamesByJobID(c.Request.Context(), queries.ListSegmentNamesByJobIDParams{
			JobID:    job.ID,
			TenantID: tenantID,
		})
		if err != nil {
			return err
		}

		jobResp = dto.JobResponse{
			ID:             job.ID,
			TenantID:       job.TenantID,
			UploadIntentID: strPtr(intent.ID),
			FileKey:        job.FileKey,
			Title:          job.Title,
			Kind:           job.Kind,
			Status:         job.Status,
			Stage:          job.Stage,
			ErrorMessage:   textPtr(job.ErrorMessage),
			RetryCount:     int(job.RetryCount),
			Segments:       segmentNames,
			StartedAt:      timePtr(job.StartedAt),
			FinishedAt:     timePtr(job.FinishedAt),
			CreatedAt:      job.CreatedAt.Time,
			UpdatedAt:      job.UpdatedAt.Time,
		}
		return nil
	})

	if err != nil {
		writeError(c, err)
		return
	}

	_ = h.insertAudit(c, "job.create", &jobResp.ID, map[string]any{"upload_intent_id": jobResp.UploadIntentID})

	c.JSON(http.StatusCreated, jobResp)
}

// ListJobs implements GET /api/v1/jobs (scope admin). It filters by tenant and
// optional status with page-based pagination and decorates each job with its
// segment names.
func (h *Handler) ListJobs(c *gin.Context) {
	tenantID := TenantID(c)
	status := c.Query("status")
	page, limit, err := pagination(c)
	if err != nil {
		writeError(c, err)
		return
	}

	jobs, err := h.Jobs.List(c.Request.Context(), tenantID, status, page, limit)
	if err != nil {
		writeError(c, err)
		return
	}

	total, err := h.Jobs.Count(c.Request.Context(), tenantID, status)
	if err != nil {
		writeError(c, err)
		return
	}

	data := make([]dto.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		segments, err := h.Segments.ListNamesByJobID(c.Request.Context(), job.ID, tenantID)
		if err != nil {
			writeError(c, err)
			return
		}
		data = append(data, toJobResponse(job, segments))
	}

	c.JSON(http.StatusOK, dto.ListJobsResponse{Data: data, Meta: dto.Meta{Page: page, Limit: limit, Total: total}})
}

// GetJob implements GET /api/v1/jobs/:id (scope admin). Only jobs owned by the
// tenant are returned; others produce a 404.
func (h *Handler) GetJob(c *gin.Context) {
	tenantID := TenantID(c)
	jobID := c.Param("id")

	job, err := h.Jobs.GetByID(c.Request.Context(), jobID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}
	segments, err := h.Segments.ListNamesByJobID(c.Request.Context(), job.ID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, toJobResponse(*job, segments))
}

// RetryJob implements POST /api/v1/jobs/:id/retry (scope admin). It only allows
// retrying a job whose status is failed.
func (h *Handler) RetryJob(c *gin.Context) {
	tenantID := TenantID(c)
	jobID := c.Param("id")

	job, err := h.Jobs.GetByID(c.Request.Context(), jobID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}
	if job.Status != model.JobStatusFailed {
		writeError(c, apierrors.ErrJobNotFailed)
		return
	}
	if err := h.Jobs.Retry(c.Request.Context(), jobID, tenantID); err != nil {
		writeError(c, err)
		return
	}

	retried, err := h.Jobs.GetByID(c.Request.Context(), jobID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}
	segments, err := h.Segments.ListNamesByJobID(c.Request.Context(), job.ID, tenantID)
	if err != nil {
		writeError(c, err)
		return
	}

	_ = h.insertAudit(c, "job.retry", &jobID, nil)

	c.JSON(http.StatusOK, toJobResponse(*retried, segments))
}

// validateCreateJob enforces the create-job validation rules from the API
// Contract.
func validateCreateJob(req *dto.CreateJobRequest) error {
	if req.FileKey == "" {
		return apierrors.ErrInvalidFileKey
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || len(title) > maxJobTitleLength {
		return apierrors.ErrInvalidRequest
	}
	if len(req.Segments) > maxSegmentsCount {
		return apierrors.ErrInvalidRequest
	}
	for _, s := range req.Segments {
		if len(s) > maxSegmentNameLength {
			return apierrors.ErrInvalidRequest
		}
	}
	return nil
}

// pagination parses the page and limit query parameters with safe defaults.
func pagination(c *gin.Context) (int, int, error) {
	page := currentPageDefault
	if v := c.Query("page"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 {
			return 0, 0, apierrors.ErrInvalidRequest
		}
		page = p
	}

	limit := listLimitDefault
	if v := c.Query("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil || l < 1 {
			return 0, 0, apierrors.ErrInvalidRequest
		}
		if l > listLimitMax {
			l = listLimitMax
		}
		limit = l
	}
	return page, limit, nil
}

// mapNoRows converts a no-row error into fallbackErr, preserving other errors.
func mapNoRows(err, fallbackErr error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fallbackErr
	}
	return err
}

// toTimestamptz converts time.Time into a valid pgtype.Timestamptz.
func toTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// toUUID parses a string UUID into a valid pgtype.UUID.
func toUUID(s string) pgtype.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// textPtr converts a nullable pgtype.Text to *string.
func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

// timePtr converts a nullable pgtype.Timestamptz to *time.Time.
func timePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
