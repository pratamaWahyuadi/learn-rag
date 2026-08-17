package repo

import (
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
)

// Compile-time assertions that repository adapters satisfy their ports.
var (
	_ ports.APIKeyRepository       = (*APIKeyRepository)(nil)
	_ ports.UploadIntentRepository = (*UploadIntentRepository)(nil)
	_ ports.JobRepository          = (*JobRepository)(nil)
	_ ports.SegmentRepository      = (*SegmentRepository)(nil)
	_ ports.VideoRepository        = (*VideoRepository)(nil)
	_ ports.TranscriptRepository   = (*TranscriptRepository)(nil)
	_ ports.ChunkRepository        = (*ChunkRepository)(nil)
	_ ports.SummaryRepository      = (*SummaryRepository)(nil)
	_ ports.AuditLogRepository     = (*AuditLogRepository)(nil)
)
