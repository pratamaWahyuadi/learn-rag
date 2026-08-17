-- name: CreateJob :one
INSERT INTO jobs (
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
) RETURNING
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at;

-- name: ListJobs :many
SELECT
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at
FROM jobs
WHERE tenant_id = sqlc.arg('tenant_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: GetJobByID :one
SELECT
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at
FROM jobs
WHERE id = $1
  AND tenant_id = $2;

-- name: GetJobByIDAllTenants :one
SELECT
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at
FROM jobs
WHERE id = $1;

-- name: UpdateJobStatus :one
UPDATE jobs
SET status = $3,
    stage = $4,
    error_message = $5,
    started_at = COALESCE(started_at, CASE WHEN $3 = 'processing' THEN now() END),
    finished_at = CASE
        WHEN $3 IN ('completed', 'failed') THEN now()
        ELSE finished_at
    END
WHERE id = $1
  AND tenant_id = $2
RETURNING
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at;

-- name: RetryJob :one
UPDATE jobs
SET status = 'pending',
    stage = 'queued',
    error_message = NULL,
    retry_count = retry_count + 1,
    started_at = NULL,
    finished_at = NULL
WHERE id = $1
  AND tenant_id = $2
RETURNING
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at;

-- name: ClaimNextPendingJob :one
UPDATE jobs
SET status = 'processing',
    stage = 'downloading',
    started_at = COALESCE(started_at, now())
WHERE id = (
    SELECT id
    FROM jobs
    WHERE status = 'pending'
    ORDER BY created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING
    id,
    tenant_id,
    upload_intent_id,
    file_key,
    title,
    kind,
    status,
    stage,
    error_message,
    retry_count,
    started_at,
    finished_at,
    created_at,
    updated_at;
