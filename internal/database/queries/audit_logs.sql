-- name: InsertAuditLog :one
INSERT INTO audit_logs (
    id,
    tenant_id,
    actor_key_id,
    action,
    object_id,
    metadata,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING
    id,
    tenant_id,
    actor_key_id,
    action,
    object_id,
    metadata,
    created_at;
