package handlers

import (
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
)

// toJobResponse converts a domain Job and its segment names into the API DTO.
func toJobResponse(job model.Job, segments []string) dto.JobResponse {
	return dto.JobResponse{
		ID:             job.ID,
		TenantID:       job.TenantID,
		UploadIntentID: job.UploadIntentID,
		FileKey:        job.FileKey,
		Title:          job.Title,
		Kind:           job.Kind,
		Status:         job.Status,
		Stage:          job.Stage,
		ErrorMessage:   job.ErrorMessage,
		RetryCount:     job.RetryCount,
		Segments:       segments,
		StartedAt:      job.StartedAt,
		FinishedAt:     job.FinishedAt,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

// toVideoResponse converts a domain Video into the list/delete API DTO.
func toVideoResponse(v model.Video) dto.VideoResponse {
	return dto.VideoResponse{
		ID:              v.ID,
		TenantID:        v.TenantID,
		JobID:           v.JobID,
		Title:           v.Title,
		Kind:            v.Kind,
		FileKey:         v.FileKey,
		Status:          v.Status,
		DurationSeconds: v.DurationSeconds,
		Segments:        v.Segments,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}

// toSummaryResponse converts a domain Summary into the API DTO.
func toSummaryResponse(s model.Summary) dto.SummaryResponse {
	return dto.SummaryResponse{
		ID:        s.ID,
		Status:    s.Status,
		Content:   s.Content,
		Language:  s.Language,
		Model:     s.Model,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
