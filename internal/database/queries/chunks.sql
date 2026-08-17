-- name: DeleteChunksByVideoID :exec
DELETE FROM chunks
WHERE video_id = $1
  AND tenant_id = $2;

-- name: InsertChunk :one
INSERT INTO chunks (
    id,
    tenant_id,
    video_id,
    chunk_index,
    content,
    embedding,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING
    id,
    tenant_id,
    video_id,
    chunk_index,
    content,
    embedding,
    created_at;

-- name: SearchChunks :many
SELECT
    c.id,
    c.tenant_id,
    c.video_id,
    c.chunk_index,
    c.content,
    c.created_at,
    v.title AS video_title
FROM chunks c
JOIN videos v ON v.id = c.video_id
WHERE c.tenant_id = sqlc.arg('tenant_id')
  AND v.status = 'completed'
  AND v.deleted_at IS NULL
  AND (sqlc.narg('segment_name')::text IS NULL OR EXISTS (
      SELECT 1
      FROM video_segments vs
      JOIN segments s ON s.id = vs.segment_id
      WHERE vs.video_id = v.id
        AND s.tenant_id = sqlc.arg('tenant_id')
        AND lower(s.name) = lower(sqlc.narg('segment_name'))
  ))
ORDER BY c.embedding <=> sqlc.arg('embedding')::vector
LIMIT sqlc.arg('limit');
