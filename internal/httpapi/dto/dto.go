// Package dto defines the request and response structs used by the HTTP API.
// Every field matches the JSON shape in the API Contract (docs/04_api_contracts.md)
// using snake_case field names.
package dto

import "time"

// ContentTypeConfig maps an allowed upload MIME type to its object-key
// extension and pipeline kind.
type ContentTypeConfig struct {
	Ext  string
	Kind string
}

// AllowedContentTypes is the whitelist of MIME types accepted by upload
// intents (FR-001). Keyed by MIME type.
var AllowedContentTypes = map[string]ContentTypeConfig{
	"video/mp4":       {Ext: ".mp4", Kind: "video"},
	"video/quicktime": {Ext: ".mov", Kind: "video"},
	"audio/mpeg":      {Ext: ".mp3", Kind: "audio"},
	"audio/wav":       {Ext: ".wav", Kind: "audio"},
	"application/pdf": {Ext: ".pdf", Kind: "pdf"},
}

// ---------------------------------------------------------------------------
// Upload intents
// ---------------------------------------------------------------------------

// CreateUploadIntentRequest is the request body for POST /api/v1/upload-intents.
type CreateUploadIntentRequest struct {
	ContentType string `json:"content_type"`
}

// CreateUploadIntentResponse is the response body for a successful upload
// intent creation (201 Created).
type CreateUploadIntentResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	FileKey      string    `json:"file_key"`
	ContentType  string    `json:"content_type"`
	Status       string    `json:"status"`
	PresignedURL string    `json:"presigned_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

// CreateJobRequest is the request body for POST /api/v1/jobs.
type CreateJobRequest struct {
	FileKey  string   `json:"file_key"`
	Title    string   `json:"title"`
	Segments []string `json:"segments"`
}

// JobResponse is the shared job shape returned by create, list, detail, and
// retry endpoints.
type JobResponse struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	UploadIntentID *string    `json:"upload_intent_id"`
	FileKey        string     `json:"file_key"`
	Title          string     `json:"title"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Stage          string     `json:"stage"`
	ErrorMessage   *string    `json:"error_message"`
	RetryCount     int        `json:"retry_count"`
	Segments       []string   `json:"segments"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ListJobsResponse is the paginated response for GET /api/v1/jobs.
type ListJobsResponse struct {
	Data []JobResponse `json:"data"`
	Meta Meta          `json:"meta"`
}

// Meta holds pagination metadata shared by list endpoints.
type Meta struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// ---------------------------------------------------------------------------
// Videos
// ---------------------------------------------------------------------------

// VideoResponse is the shape of a video in list responses.
type VideoResponse struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	JobID           string    `json:"job_id"`
	Title           string    `json:"title"`
	Kind            string    `json:"kind"`
	FileKey         string    `json:"file_key"`
	Status          string    `json:"status"`
	DurationSeconds *int      `json:"duration_seconds"`
	Segments        []string  `json:"segments"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListVideosResponse is the paginated response for GET /api/v1/videos.
type ListVideosResponse struct {
	Data []VideoResponse `json:"data"`
	Meta Meta            `json:"meta"`
}

// VideoDetailResponse extends VideoResponse with an optional summary for
// GET /api/v1/videos/{id}.
type VideoDetailResponse struct {
	VideoResponse
	Summary *SummaryResponse `json:"summary"`
}

// SummaryResponse is the per-video summary embedded in a video detail.
type SummaryResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Content   *string   `json:"content,omitempty"`
	Language  *string   `json:"language,omitempty"`
	Model     *string   `json:"model,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeleteVideoResponse is the response body for DELETE /api/v1/videos/{id}.
type DeleteVideoResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

// QueryRequest is the request body for POST /api/v1/query.
type QueryRequest struct {
	Question string `json:"question"`
	Segment  string `json:"segment"`
}

// QueryResponse is the response body for POST /api/v1/query.
type QueryResponse struct {
	Answer     *string     `json:"answer"`
	References []Reference `json:"references"`
}

// Reference is a single retrieved source shown alongside the RAG answer.
type Reference struct {
	VideoID    string   `json:"video_id"`
	VideoTitle string   `json:"video_title"`
	ChunkIndex int      `json:"chunk_index"`
	Snippet    string   `json:"snippet"`
	Segments   []string `json:"segments"`
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// HealthResponse is the response body for GET /healthz.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
	DB      string `json:"db"`
}
