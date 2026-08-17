-- name: CreateUploadIntent :one
INSERT INTO upload_intents (
    id,
    tenant_id,
    file_key,
    content_type,
    status,
    expires_at,
    consumed_at,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING
    id,
    tenant_id,
    file_key,
    content_type,
    status,
    expires_at,
    consumed_at,
    created_at;

-- name: GetUploadIntentByFileKeyForUpdate :one
SELECT
    id,
    tenant_id,
    file_key,
    content_type,
    status,
    expires_at,
    consumed_at,
    created_at
FROM upload_intents
WHERE file_key = $1
  AND tenant_id = $2
FOR UPDATE;

-- name: MarkUploadIntentConsumed :one
UPDATE upload_intents
SET status = 'consumed',
    consumed_at = now()
WHERE file_key = $1
  AND tenant_id = $2
  AND status = 'issued'
RETURNING
    id,
    tenant_id,
    file_key,
    content_type,
    status,
    expires_at,
    consumed_at,
    created_at;
