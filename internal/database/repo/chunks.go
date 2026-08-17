package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// ChunkRepository implements ports.ChunkRepository.
type ChunkRepository struct {
	pool *pgxpool.Pool
}

// NewChunkRepository builds a ChunkRepository backed by pool.
func NewChunkRepository(pool *pgxpool.Pool) *ChunkRepository {
	return &ChunkRepository{pool: pool}
}

// DeleteByVideoID removes all chunks belonging to a video for the tenant.
func (r *ChunkRepository) DeleteByVideoID(ctx context.Context, videoID, tenantID string) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		return q.DeleteChunksByVideoID(ctx, queries.DeleteChunksByVideoIDParams{
			VideoID:  videoID,
			TenantID: tenantID,
		})
	})
}

// Insert persists multiple chunks in a single tenant-scoped transaction.
func (r *ChunkRepository) Insert(ctx context.Context, chunks []model.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tenantID := chunks[0].TenantID
	return database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		for _, chunk := range chunks {
			if _, err := q.InsertChunk(ctx, queries.InsertChunkParams{
				ID:         chunk.ID,
				TenantID:   chunk.TenantID,
				VideoID:    chunk.VideoID,
				ChunkIndex: int32(chunk.ChunkIndex),
				Content:    chunk.Content,
				Embedding:  chunk.Embedding,
				CreatedAt:  fromTime(chunk.CreatedAt),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Search retrieves the nearest chunks for completed, non-deleted videos owned
// by the tenant, optionally restricted to a segment.
func (r *ChunkRepository) Search(ctx context.Context, tenantID string, embedding pgvector.Vector, segmentName string, limit int) ([]model.ChunkSearchResult, error) {
	var results []model.ChunkSearchResult
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(q *queries.Queries) error {
		rows, err := q.SearchChunks(ctx, queries.SearchChunksParams{
			TenantID:    tenantID,
			SegmentName: optionalText(segmentName),
			Embedding:   embedding,
			Limit:       int32(limit),
		})
		if err != nil {
			return err
		}
		results = make([]model.ChunkSearchResult, 0, len(rows))
		for _, row := range rows {
			results = append(results, model.ChunkSearchResult{
				Chunk: model.Chunk{
					ID:         row.ID,
					TenantID:   row.TenantID,
					VideoID:    row.VideoID,
					ChunkIndex: int(row.ChunkIndex),
					Content:    row.Content,
					CreatedAt:  toTime(row.CreatedAt),
				},
				VideoTitle: row.VideoTitle,
			})
		}
		return nil
	})
	return results, err
}
