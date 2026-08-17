-- name: UpsertSummary :one
INSERT INTO summaries (
    id,
    tenant_id,
    video_id,
    status,
    content,
    language,
    model,
    error_message,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (video_id) DO UPDATE SET
    status = EXCLUDED.status,
    content = EXCLUDED.content,
    language = EXCLUDED.language,
    model = EXCLUDED.model,
    error_message = EXCLUDED.error_message
RETURNING
    id,
    tenant_id,
    video_id,
    status,
    content,
    language,
    model,
    error_message,
    created_at,
    updated_at;

-- name: GetSummaryByVideoID :one
SELECT
    id,
    tenant_id,
    video_id,
    status,
    content,
    language,
    model,
    error_message,
    created_at,
    updated_at
FROM summaries
WHERE video_id = $1
  AND tenant_id = $2;
