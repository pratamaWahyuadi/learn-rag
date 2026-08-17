package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// WithTenantTx runs fn inside a single Postgres transaction with Row Level
// Security scoped to tenantID. It executes `SELECT set_config('app.tenant_id',
// $1, true)` (SET LOCAL) so every RLS policy in the transaction only sees rows
// owned by that tenant. The transaction is committed when fn returns nil and
// rolled back otherwise.
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(q *queries.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("database: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("database: set tenant: %w", err)
	}

	q := queries.New(tx)
	if err := fn(q); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("database: commit tx: %w", err)
	}
	return nil
}
