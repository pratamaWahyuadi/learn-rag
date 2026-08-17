-- name: UpsertTranscript :one
INSERT INTO transcripts (
    id,
    tenant_id,
    video_id,
    content,
    language,
    model,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (video_id) DO UPDATE SET
    content = EXCLUDED.content,
    language = EXCLUDED.language,
    model = EXCLUDED.model
RETURNING
    id,
    tenant_id,
    video_id,
    content,
    language,
    model,
    created_at;
