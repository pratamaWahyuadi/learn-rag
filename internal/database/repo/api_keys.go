package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database/queries"
)

// APIKeyRepository implements ports.APIKeyRepository using the generated sqlc
// queries. GetByHash deliberately bypasses tenant RLS because the tenant is not
// known until the key is resolved.
type APIKeyRepository struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepository builds an APIKeyRepository backed by the given pool.
func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

// GetByHash returns the active API key for the given SHA-256 hash. It returns
// ErrNotFound when no key matches or the key has been revoked.
func (r *APIKeyRepository) GetByHash(ctx context.Context, hash string) (*model.APIKey, error) {
	q := queries.New(r.pool)
	row, err := q.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierrors.ErrNotFound
		}
		return nil, err
	}
	return &model.APIKey{
		ID:         row.ID,
		TenantID:   row.TenantID,
		Name:       row.Name,
		KeyHash:    row.KeyHash,
		Scope:      row.Scope,
		RevokedAt:  toTimePtr(row.RevokedAt),
		LastUsedAt: toTimePtr(row.LastUsedAt),
		CreatedAt:  toTime(row.CreatedAt),
		UpdatedAt:  toTime(row.UpdatedAt),
	}, nil
}

// TouchLastUsed updates last_used_at for the given API key in the background.
// api_keys has no RLS and this is a best-effort write, so it runs directly on
// the pool. Any error is intentionally ignored by callers.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id string) error {
	q := queries.New(r.pool)
	return q.TouchAPIKeyLastUsed(ctx, id)
}
