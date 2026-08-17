package repo

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// AuditLogRepository implements ports.AuditLogRepository.
type AuditLogRepository struct {
	pool *pgxpool.Pool
}

// NewAuditLogRepository builds an AuditLogRepository backed by pool.
func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

// Insert writes an internal audit record. audit_logs does not use RLS, but the
// value is wrapped in a tenant-scoped transaction for consistency.
func (r *AuditLogRepository) Insert(ctx context.Context, log *model.AuditLog) error {
	metadata, err := json.Marshal(log.Metadata)
	if err != nil {
		return err
	}
	return database.WithTenantTx(ctx, r.pool, log.TenantID, func(q *queries.Queries) error {
		_, err := q.InsertAuditLog(ctx, queries.InsertAuditLogParams{
			ID:         log.ID,
			TenantID:   log.TenantID,
			ActorKeyID: fromUUIDPtr(log.ActorKeyID),
			Action:     log.Action,
			ObjectID:   fromUUIDPtr(log.ObjectID),
			Metadata:   metadata,
			CreatedAt:  fromTime(log.CreatedAt),
		})
		return err
	})
}
