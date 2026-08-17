-- name: CreateVideo :one
INSERT INTO videos (
    id,
    tenant_id,
    job_id,
    title,
    kind,
    file_key,
    status,
    duration_seconds,
    deleted_at,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING
    id,
    tenant_id,
    job_id,
    title,
    kind,
    file_key,
    status,
    duration_seconds,
    deleted_at,
    created_at,
    updated_at;

-- name: GetVideoByID :one
SELECT
    id,
    tenant_id,
    job_id,
    title,
    kind,
    file_key,
    status,
    duration_seconds,
    deleted_at,
    created_at,
    updated_at
FROM videos
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;

-- name: ListVideos :many
SELECT
    v.id,
    v.tenant_id,
    v.job_id,
    v.title,
    v.kind,
    v.file_key,
    v.status,
    v.duration_seconds,
    v.deleted_at,
    v.created_at,
    v.updated_at
FROM videos v
WHERE v.tenant_id = sqlc.arg('tenant_id')
  AND v.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR v.status = sqlc.narg('status'))
  AND (sqlc.narg('segment_name')::text IS NULL OR EXISTS (
      SELECT 1
      FROM video_segments vs
      JOIN segments s ON s.id = vs.segment_id
      WHERE vs.video_id = v.id
        AND s.tenant_id = sqlc.arg('tenant_id')
        AND lower(s.name) = lower(sqlc.narg('segment_name'))
  ))
ORDER BY v.created_at DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: CountVideos :one
SELECT count(*)
FROM videos v
WHERE v.tenant_id = sqlc.arg('tenant_id')
  AND v.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR v.status = sqlc.narg('status'))
  AND (sqlc.narg('segment_name')::text IS NULL OR EXISTS (
      SELECT 1
      FROM video_segments vs
      JOIN segments s ON s.id = vs.segment_id
      WHERE vs.video_id = v.id
        AND s.tenant_id = sqlc.arg('tenant_id')
        AND lower(s.name) = lower(sqlc.narg('segment_name'))
  ));

-- name: SoftDeleteVideo :one
UPDATE videos
SET deleted_at = now()
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING
    id,
    tenant_id,
    job_id,
    title,
    kind,
    file_key,
    status,
    duration_seconds,
    deleted_at,
    created_at,
    updated_at;

-- name: DeleteVideoByJobID :exec
DELETE FROM videos
WHERE job_id = $1
  AND tenant_id = $2;

-- name: FailVideoByJobID :exec
UPDATE videos
SET status = 'failed'
WHERE job_id = $1
  AND tenant_id = $2;

-- name: UpdateVideoStatus :one
UPDATE videos
SET status = $3
WHERE id = $1
  AND tenant_id = $2
RETURNING
    id,
    tenant_id,
    job_id,
    title,
    kind,
    file_key,
    status,
    duration_seconds,
    deleted_at,
    created_at,
    updated_at;
