package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// TranscriptRepository implements ports.TranscriptRepository.
type TranscriptRepository struct {
	pool *pgxpool.Pool
}

// NewTranscriptRepository builds a TranscriptRepository backed by pool.
func NewTranscriptRepository(pool *pgxpool.Pool) *TranscriptRepository {
	return &TranscriptRepository{pool: pool}
}

// Upsert inserts or updates the transcript for a video based on video_id.
func (r *TranscriptRepository) Upsert(ctx context.Context, transcript *model.Transcript) error {
	return database.WithTenantTx(ctx, r.pool, transcript.TenantID, func(q *queries.Queries) error {
		_, err := q.UpsertTranscript(ctx, queries.UpsertTranscriptParams{
			ID:        transcript.ID,
			TenantID:  transcript.TenantID,
			VideoID:   transcript.VideoID,
			Content:   transcript.Content,
			Language:  fromStrPtr(transcript.Language),
			Model:     fromStrPtr(transcript.Model),
			CreatedAt: fromTime(transcript.CreatedAt),
		})
		return err
	})
}
