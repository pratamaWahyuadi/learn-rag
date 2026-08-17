-- name: GetSegmentByName :one
SELECT
    id,
    tenant_id,
    name,
    created_at
FROM segments
WHERE tenant_id = $1
  AND lower(name) = lower($2);

-- name: CreateSegment :one
INSERT INTO segments (
    id,
    tenant_id,
    name,
    created_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING
    id,
    tenant_id,
    name,
    created_at;

-- name: EnsureSegmentByName :one
WITH ins AS (
    INSERT INTO segments (id, tenant_id, name, created_at)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT DO NOTHING
    RETURNING id, tenant_id, name, created_at
)
SELECT id, tenant_id, name, created_at FROM ins
UNION ALL
SELECT id, tenant_id, name, created_at FROM segments
WHERE tenant_id = $2 AND lower(name) = lower($3)
LIMIT 1;

-- name: AttachJobSegment :exec
INSERT INTO job_segments (
    job_id,
    segment_id,
    tenant_id
) VALUES (
    $1, $2, $3
)
ON CONFLICT DO NOTHING;

-- name: AttachVideoSegment :exec
INSERT INTO video_segments (
    video_id,
    segment_id,
    tenant_id
) VALUES (
    $1, $2, $3
)
ON CONFLICT DO NOTHING;

-- name: ListSegmentNamesByJobID :many
SELECT s.name
FROM segments s
JOIN job_segments js ON js.segment_id = s.id
WHERE js.job_id = $1
  AND js.tenant_id = $2
ORDER BY s.name;

-- name: ListSegmentNamesByVideoID :many
SELECT s.name
FROM segments s
JOIN video_segments vs ON vs.segment_id = s.id
WHERE vs.video_id = $1
  AND vs.tenant_id = $2
ORDER BY s.name;
