//go:build integration

// The create-job flow (POST /api/v1/jobs) consumes an upload intent inside a
// real tenant-scoped Postgres transaction (database.WithTenantTx), so verifying
// that a file_key not owned by the tenant returns 404 requires a real database.
// These tests are gated behind the `integration` build tag so `make test`
// (go test ./...) stays green without a database. Run them with:
//
//	DATABASE_URL=postgres://rag:rag@localhost:5432/rag go test -tags integration ./internal/httpapi/handlers/...
package handlers_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/database"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/middleware"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("integration test requires DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedTenant(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewString()
	slug = slug + "-" + id[:8]
	if _, err := pool.Exec(ctx,
		"INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)", id, slug, slug); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	return id
}

func seedUploadIntent(t *testing.T, pool *pgxpool.Pool, tenantID, fileKey string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO upload_intents
		(id, tenant_id, file_key, content_type, status, expires_at, created_at)
		VALUES ($1, $2, $3, 'video/mp4', 'issued', now() + interval '1 hour', now())`,
		uuid.NewString(), tenantID, fileKey); err != nil {
		t.Fatalf("seed upload intent: %v", err)
	}
}

// TestCreateJobForeignUploadIntent404 proves a file_key owned by another tenant
// is invisible during job creation and yields 404 not_found (SR-01, SR-05).
func TestCreateJobForeignUploadIntent404(t *testing.T) {
	pool := integrationPool(t)

	tenantA := seedTenant(t, pool, "createjob-a")
	tenantB := seedTenant(t, pool, "createjob-b")
	foreignKey := "foreign-" + uuid.NewString()[:8]
	seedUploadIntent(t, pool, tenantB, foreignKey)

	tk := newUnitHandler()
	tk.handler.Pool = pool
	tk.keys.keys[middleware.HashKey("admin-key")] = &model.APIKey{
		ID: "key-admin", TenantID: tenantA, Scope: "admin",
	}
	auth := middleware.NewAuthenticator(tk.keys)
	r := httpapi.NewRouter(discardLogger(), auth, tk.handler)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/jobs", "admin-key",
		dto.CreateJobRequest{FileKey: foreignKey, Title: "Some title", Segments: nil})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign upload intent, got %d (body=%s)", w.Code, w.Body.String())
	}
	assertErrorCode(t, w, apierrors.ErrNotFound.Code)
	if len(tk.jobs.jobs) != 0 {
		t.Fatal("no job should have been created for a foreign upload intent")
	}
}

// TestCreateJobOwnedUploadIntent exercises the happy path through a real
// transaction: a job is created and the upload intent is marked consumed.
func TestCreateJobOwnedUploadIntent(t *testing.T) {
	pool := integrationPool(t)

	tenantA := seedTenant(t, pool, "createjob-owned")
	ownedKey := "owned-" + uuid.NewString()[:8]
	seedUploadIntent(t, pool, tenantA, ownedKey)

	tk := newUnitHandler()
	tk.handler.Pool = pool
	tk.keys.keys[middleware.HashKey("admin-key")] = &model.APIKey{
		ID: "key-admin", TenantID: tenantA, Scope: "admin",
	}
	auth := middleware.NewAuthenticator(tk.keys)
	r := httpapi.NewRouter(discardLogger(), auth, tk.handler)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/jobs", "admin-key",
		dto.CreateJobRequest{FileKey: ownedKey, Title: "Belajar Web", Segments: []string{"web design"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body=%s)", w.Code, w.Body.String())
	}

	// The upload intent must have been marked consumed inside the transaction.
	var status string
	if err := pool.QueryRow(context.Background(),
		"SELECT status FROM upload_intents WHERE file_key = $1", ownedKey).Scan(&status); err != nil {
		t.Fatalf("read intent status: %v", err)
	}
	if status != "consumed" {
		t.Fatalf("expected intent status consumed, got %q", status)
	}
}
