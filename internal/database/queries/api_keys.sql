-- name: GetAPIKeyByHash :one
SELECT
    id,
    tenant_id,
    name,
    key_hash,
    scope,
    revoked_at,
    last_used_at,
    created_at,
    updated_at
FROM api_keys
WHERE key_hash = $1
  AND revoked_at IS NULL
LIMIT 1;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = now()
WHERE id = $1;
