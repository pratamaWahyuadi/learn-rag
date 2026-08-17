// Package model defines the pure domain models of the RAG pipeline. These
// structs are free of any dependency on Gin, sqlc, or external providers so
// they can be used safely across handlers, workers, and repositories.
package model

import "time"

// Shared status enums as string constants.
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"

	VideoStatusProcessing = "processing"
	VideoStatusCompleted  = "completed"
	VideoStatusFailed     = "failed"

	ScopeAdmin = "admin"
	ScopeQuery = "query"
)

// Mixed content kind used by both jobs and videos.
const (
	KindVideo = "video"
	KindAudio = "audio"
	KindPDF   = "pdf"
)

// Tenant is a B2B client that owns all its data in isolation.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey authenticates server-to-server requests. Only the SHA-256 hash of the
// key is stored; plaintext keys never persist.
type APIKey struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"key_hash"`
	Scope      string     `json:"scope"`
	RevokedAt  *time.Time `json:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// UploadIntent records an intent to upload a file directly to R2 via a
// time-limited presigned URL.
type UploadIntent struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	FileKey     string     `json:"file_key"`
	ContentType string     `json:"content_type"`
	Status      string     `json:"status"`
	ExpiresAt   time.Time  `json:"expires_at"`
	ConsumedAt  *time.Time `json:"consumed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Job is a pipeline job that processes one uploaded material.
type Job struct {
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
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Video is a processed course material produced from one job.
type Video struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	JobID           string     `json:"job_id"`
	Title           string     `json:"title"`
	Kind            string     `json:"kind"`
	FileKey         string     `json:"file_key"`
	Status          string     `json:"status"`
	DurationSeconds *int       `json:"duration_seconds"`
	DeletedAt       *time.Time `json:"deleted_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Segments        []string   `json:"segments,omitempty"`
}

// Segment is a flat tag attached to materials.
type Segment struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Transcript is the full transcription/parsing result of a material.
type Transcript struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	VideoID   string    `json:"video_id"`
	Content   string    `json:"content"`
	Language  *string   `json:"language"`
	Model     *string   `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

// Chunk is a 3–4 sentence text segment with its dense BGE-M3 embedding.
type Chunk struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	VideoID    string    `json:"video_id"`
	ChunkIndex int       `json:"chunk_index"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// Summary is a per-video summary produced by the LLM.
type Summary struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	VideoID      string    `json:"video_id"`
	Status       string    `json:"status"`
	Content      *string   `json:"content"`
	Language     *string   `json:"language"`
	Model        *string   `json:"model"`
	ErrorMessage *string   `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// QueryReference is a single retrieved reference returned with a RAG answer.
type QueryReference struct {
	VideoID    string   `json:"video_id"`
	VideoTitle string   `json:"video_title"`
	ChunkIndex int      `json:"chunk_index"`
	Snippet    string   `json:"snippet"`
	Segments   []string `json:"segments"`
}

// QueryResult is the response body for POST /api/v1/query.
type QueryResult struct {
	Answer     *string          `json:"answer"`
	References []QueryReference `json:"references"`
}

// ChunkSearchResult couples a retrieved chunk with its owning video title,
// used for RAG vector-search results.
type ChunkSearchResult struct {
	Chunk
	VideoTitle string `json:"video_title"`
}

// AuditLog records a security-relevant mutation action.
type AuditLog struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	ActorKeyID *string        `json:"actor_key_id"`
	Action     string         `json:"action"`
	ObjectID   *string        `json:"object_id"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}
