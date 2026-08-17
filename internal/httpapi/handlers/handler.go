// Package handlers implements the HTTP handlers mounted by the router. Handlers
// bind request DTOs, validate input, delegate to repositories/services, and
// translate errors into the shared error response format. They never write raw
// SQL or leak internal error details.
package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/rager"
)

// Handler carries every dependency the HTTP handlers need. All dependencies are
// injected through NewHandler so the package has no global state.
type Handler struct {
	Pool      *pgxpool.Pool
	Cfg       *config.Config
	Uploads   ports.UploadIntentRepository
	Jobs      ports.JobRepository
	Segments  ports.SegmentRepository
	Videos    ports.VideoRepository
	Chunks    ports.ChunkRepository
	Summaries ports.SummaryRepository
	Audit     ports.AuditLogRepository
	Storage   ports.Storage
	Embeder   ports.Embedder
	LLM       ports.LLM
	RAG       *rager.RAGService
}

// NewHandler builds a Handler with all required dependencies.
func NewHandler(
	pool *pgxpool.Pool,
	cfg *config.Config,
	uploads ports.UploadIntentRepository,
	jobs ports.JobRepository,
	segments ports.SegmentRepository,
	videos ports.VideoRepository,
	chunks ports.ChunkRepository,
	summaries ports.SummaryRepository,
	audit ports.AuditLogRepository,
	storage ports.Storage,
	embedder ports.Embedder,
	llm ports.LLM,
	rag *rager.RAGService,
) *Handler {
	return &Handler{
		Pool:      pool,
		Cfg:       cfg,
		Uploads:   uploads,
		Jobs:      jobs,
		Segments:  segments,
		Videos:    videos,
		Chunks:    chunks,
		Summaries: summaries,
		Audit:     audit,
		Storage:   storage,
		Embeder:   embedder,
		LLM:       llm,
		RAG:       rag,
	}
}
