// Package handlers_test contains the HTTP handler security and scope tests
// (SR-01, SR-02, SR-05). They run in gin test mode with httptest and fake
// repositories/providers so no external network or database is required.
//
// The fake types here are also shared by handlers_integration_test.go, which
// additionally exercises the create-job flow against a real Postgres instance.
//
// The test lives in the external package handlers_test so it can assemble the
// full router (internal/httpapi) without creating an import cycle.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	apierrors "github.com/pratamaWahyuadi/learn-rag/internal/core/errors"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/rager"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/dto"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/handlers"
	"github.com/pratamaWahyuadi/learn-rag/internal/httpapi/middleware"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

func testConfig() *config.Config {
	return &config.Config{
		ServerPort:          "8080",
		UploadURLTTLMinutes: 10,
		MaxUploadBytes:      1 << 30,
		EmbeddingBatchSize:  16,
		SummaryMaxTokens:    12000,
		QueryResultK:        5,
	}
}

// ---- fake API key store (satisfies middleware.APIKeyStore) ----

type fakeKeyStore struct {
	keys map[string]*model.APIKey // by SHA-256 hash
}

func (f *fakeKeyStore) GetByHash(_ context.Context, hash string) (*model.APIKey, error) {
	if k, ok := f.keys[hash]; ok {
		return k, nil
	}
	return nil, apierrors.ErrNotFound
}

func (f *fakeKeyStore) TouchLastUsed(_ context.Context, _ string) error { return nil }

func newKeyStore() *fakeKeyStore {
	return &fakeKeyStore{keys: map[string]*model.APIKey{}}
}

// ---- fake repositories (satisfying ports.*) ----

type fakeUploads struct {
	intents map[string]*model.UploadIntent // by file key
}

func newFakeUploads() *fakeUploads {
	return &fakeUploads{intents: map[string]*model.UploadIntent{}}
}
func (f *fakeUploads) Create(_ context.Context, _ *model.UploadIntent) error { return nil }
func (f *fakeUploads) GetByFileKeyForUpdate(_ context.Context, fileKey, _ string) (*model.UploadIntent, error) {
	if i, ok := f.intents[fileKey]; ok {
		return i, nil
	}
	return nil, apierrors.ErrNotFound
}
func (f *fakeUploads) Consume(_ context.Context, _, _ string) error { return nil }

type fakeJobs struct {
	jobs map[string]*model.Job // by id
}

func newFakeJobs() *fakeJobs { return &fakeJobs{jobs: map[string]*model.Job{}} }

func (f *fakeJobs) Create(_ context.Context, j *model.Job, _ []string) error {
	f.jobs[j.ID] = j
	return nil
}
func (f *fakeJobs) List(_ context.Context, _, _ string, _, _ int) ([]model.Job, error) {
	return nil, nil
}
func (f *fakeJobs) Count(_ context.Context, _, _ string) (int, error) { return 0, nil }
func (f *fakeJobs) GetByID(_ context.Context, id, tenantID string) (*model.Job, error) {
	j, ok := f.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, apierrors.ErrNotFound
	}
	return j, nil
}
func (f *fakeJobs) UpdateStatus(_ context.Context, _, _, _, _ string, _ *string) error {
	return nil
}
func (f *fakeJobs) Retry(_ context.Context, id, tenantID string) error {
	j, ok := f.jobs[id]
	if !ok || j.TenantID != tenantID || j.Status != model.JobStatusFailed {
		return apierrors.ErrNotFound
	}
	j.RetryCount++
	j.Status = model.JobStatusPending
	return nil
}
func (f *fakeJobs) ClaimNextPending(_ context.Context) (*model.Job, error) { return nil, nil }
func (f *fakeJobs) GetByIDAllTenants(_ context.Context, id string) (*model.Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return nil, apierrors.ErrNotFound
	}
	return j, nil
}
func (f *fakeJobs) ListForRetention(_ context.Context, _ time.Time) ([]model.RetentionJob, error) {
	return nil, nil
}

type fakeSegments struct {
	byVideo map[string][]string
	byJob   map[string][]string
}

func newFakeSegments() *fakeSegments {
	return &fakeSegments{byVideo: map[string][]string{}, byJob: map[string][]string{}}
}
func (f *fakeSegments) EnsureByName(_ context.Context, _, name string) (*model.Segment, error) {
	return &model.Segment{ID: "seg-" + name, Name: name}, nil
}
func (f *fakeSegments) AttachJobSegments(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (f *fakeSegments) AttachVideoSegments(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (f *fakeSegments) ListNamesByJobID(_ context.Context, jobID, _ string) ([]string, error) {
	return f.byJob[jobID], nil
}
func (f *fakeSegments) ListNamesByVideoID(_ context.Context, videoID, _ string) ([]string, error) {
	return f.byVideo[videoID], nil
}

type fakeVideos struct {
	videos map[string]*model.Video // by id
}

func newFakeVideos() *fakeVideos { return &fakeVideos{videos: map[string]*model.Video{}} }

func (f *fakeVideos) Create(_ context.Context, v *model.Video) error {
	f.videos[v.ID] = v
	return nil
}
func (f *fakeVideos) GetByID(_ context.Context, id, tenantID string) (*model.Video, error) {
	v, ok := f.videos[id]
	if !ok || v.TenantID != tenantID || v.DeletedAt != nil {
		return nil, apierrors.ErrNotFound
	}
	return v, nil
}
func (f *fakeVideos) List(_ context.Context, _, _, _ string, _, _ int) ([]model.Video, error) {
	return nil, nil
}
func (f *fakeVideos) Count(_ context.Context, _, _, _ string) (int, error) { return 0, nil }
func (f *fakeVideos) SoftDelete(_ context.Context, id, tenantID string) (*time.Time, error) {
	v, ok := f.videos[id]
	if !ok || v.TenantID != tenantID || v.DeletedAt != nil {
		return nil, apierrors.ErrNotFound
	}
	now := time.Now()
	v.DeletedAt = &now
	return &now, nil
}
func (f *fakeVideos) UpdateStatus(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeVideos) DeleteByJobID(_ context.Context, _, _ string) error   { return nil }
func (f *fakeVideos) FailByJobID(_ context.Context, _, _ string) error     { return nil }

type fakeTranscripts struct{}

func (f *fakeTranscripts) Upsert(_ context.Context, _ *model.Transcript) error { return nil }

type fakeSummaries struct {
	summaries map[string]*model.Summary // by video id
}

func newFakeSummaries() *fakeSummaries {
	return &fakeSummaries{summaries: map[string]*model.Summary{}}
}
func (f *fakeSummaries) Upsert(_ context.Context, s *model.Summary) error {
	f.summaries[s.VideoID] = s
	return nil
}
func (f *fakeSummaries) GetByVideoID(_ context.Context, videoID, tenantID string) (*model.Summary, error) {
	s, ok := f.summaries[videoID]
	if !ok || s.TenantID != tenantID {
		return nil, apierrors.ErrNotFound
	}
	return s, nil
}

type fakeChunks struct {
	hits []model.ChunkSearchResult
}

func newFakeChunks() *fakeChunks                                           { return &fakeChunks{} }
func (f *fakeChunks) DeleteByVideoID(_ context.Context, _, _ string) error { return nil }
func (f *fakeChunks) Insert(_ context.Context, _ []model.Chunk) error      { return nil }
func (f *fakeChunks) Search(_ context.Context, _ string, _ pgvector.Vector, _ string, _ int) ([]model.ChunkSearchResult, error) {
	return f.hits, nil
}

type fakeAudit struct{}

func (f *fakeAudit) Insert(_ context.Context, _ *model.AuditLog) error { return nil }

type fakeStorage struct{}

func (f *fakeStorage) GenerateUploadURL(_ context.Context, _, _ string, _ time.Time) (string, error) {
	return "https://presigned.example/r2/upload", nil
}
func (f *fakeStorage) Download(_ context.Context, _, _ string) error { return nil }
func (f *fakeStorage) Delete(_ context.Context, _ string) error      { return nil }

type fakeEmbedder struct {
	embedding []float32
}

func newFakeEmbedder() *fakeEmbedder {
	return &fakeEmbedder{embedding: []float32{0.1, 0.2, 0.3}}
}
func (f *fakeEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = f.embedding
	}
	return out, nil
}

type fakeLLM struct {
	answer string
}

func newFakeLLM(answer string) *fakeLLM                                     { return &fakeLLM{answer: answer} }
func (f *fakeLLM) Summarize(_ context.Context, _, _ string) (string, error) { return "", nil }
func (f *fakeLLM) AnswerQuery(_ context.Context, _, _ string) (string, error) {
	return f.answer, nil
}

// unitTestkit bundles a Handler and all its fakes so tests can both register API
// keys and seed fake data.
type unitTestkit struct {
	handler   *handlers.Handler
	keys      *fakeKeyStore
	jobs      *fakeJobs
	segments  *fakeSegments
	videos    *fakeVideos
	chunks    *fakeChunks
	uploads   *fakeUploads
	summaries *fakeSummaries
}

// newUnitHandler wires a Handler and a RAGService with fakes. The Pool is left
// nil because the create-job flow (the only handler that touches the pool) is
// exercised by the integration test.
func newUnitHandler() *unitTestkit {
	keys := newKeyStore()
	uploads := newFakeUploads()
	jobs := newFakeJobs()
	segments := newFakeSegments()
	videos := newFakeVideos()
	chunks := newFakeChunks()
	summaries := newFakeSummaries()
	embedder := newFakeEmbedder()
	llm := newFakeLLM("")
	rag := rager.NewRAGService(embedder, llm, chunks, segments)

	h := handlers.NewHandler(
		nil, // pool unused by these unit tests
		testConfig(),
		uploads,
		jobs,
		segments,
		videos,
		chunks,
		summaries,
		&fakeAudit{},
		&fakeStorage{},
		embedder,
		llm,
		rag,
	)
	return &unitTestkit{
		handler:   h,
		keys:      keys,
		jobs:      jobs,
		segments:  segments,
		videos:    videos,
		chunks:    chunks,
		uploads:   uploads,
		summaries: summaries,
	}
}

// newUnitRouter assembles the full router with the production middleware stack
// and registers an admin and a query API key on the fake key store for the given
// tenant.
func newUnitRouter(t *testing.T, tenantID string) (*gin.Engine, *unitTestkit) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tk := newUnitHandler()
	tk.keys.keys[middleware.HashKey("admin-key")] = &model.APIKey{ID: "key-admin", TenantID: tenantID, Scope: "admin"}
	tk.keys.keys[middleware.HashKey("query-key")] = &model.APIKey{ID: "key-query", TenantID: tenantID, Scope: "query"}

	auth := middleware.NewAuthenticator(tk.keys)
	r := httpapi.NewRouter(discardLogger(), auth, tk.handler)
	return r, tk
}

// doRequest performs an HTTP request with an optional API key header.
func doRequest(t *testing.T, r http.Handler, method, path, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set(middleware.APIKeyHeader, key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doJSONRequest performs an HTTP request with a JSON body and API key header.
func doJSONRequest(t *testing.T, r http.Handler, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set(middleware.APIKeyHeader, key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v; body=%s", err, w.Body.String())
	}
	if resp.Error.Code != wantCode {
		t.Fatalf("expected error code %q, got %q (body=%s)", wantCode, resp.Error.Code, w.Body.String())
	}
}

// ---- Scope isolation (SR-02) ----

func TestScopeQueryKeyBlockedFromAdmin(t *testing.T) {
	// A query-scoped key calling an admin endpoint → 403 forbidden.
	r, _ := newUnitRouter(t, "tenant-1")

	w := doRequest(t, r, http.MethodGet, "/api/v1/jobs", "query-key")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for query key on admin route, got %d", w.Code)
	}
	assertErrorCode(t, w, "forbidden")
}

func TestScopeAdminKeyBlockedFromQuery(t *testing.T) {
	// An admin-scoped key calling the query endpoint → 403 forbidden.
	r, _ := newUnitRouter(t, "tenant-1")

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/query", "admin-key",
		dto.QueryRequest{Question: "hello"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin key on query route, got %d", w.Code)
	}
	assertErrorCode(t, w, "forbidden")
}

// ---- Cross-tenant isolation / IDOR (SR-01, SR-05) ----

func TestGetJobTenantIsolation(t *testing.T) {
	// A job owned by tenant-other must be invisible (404) from tenant-1.
	r, tk := newUnitRouter(t, "tenant-1")
	tk.jobs.jobs["job-b"] = &model.Job{
		ID: "job-b", TenantID: "tenant-other", Title: "job B", Kind: model.KindVideo,
		Status: model.JobStatusCompleted, Stage: "completed",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	w := doRequest(t, r, http.MethodGet, "/api/v1/jobs/job-b", "admin-key")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign job, got %d", w.Code)
	}
	assertErrorCode(t, w, "not_found")
}

func TestGetVideoTenantIsolation(t *testing.T) {
	r, tk := newUnitRouter(t, "tenant-1")
	tk.videos.videos["video-b"] = &model.Video{
		ID: "video-b", TenantID: "tenant-other", JobID: "job-1", Title: "video B",
		Kind: model.KindVideo, FileKey: "video-b.mp4", Status: model.VideoStatusCompleted,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	w := doRequest(t, r, http.MethodGet, "/api/v1/videos/video-b", "admin-key")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign video, got %d", w.Code)
	}
	assertErrorCode(t, w, "not_found")
}

func TestDeleteVideoTenantIsolation(t *testing.T) {
	r, tk := newUnitRouter(t, "tenant-1")
	tk.videos.videos["video-b"] = &model.Video{
		ID: "video-b", TenantID: "tenant-other", JobID: "job-1", Title: "video B",
		Kind: model.KindVideo, FileKey: "video-b.mp4", Status: model.VideoStatusCompleted,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	w := doRequest(t, r, http.MethodDelete, "/api/v1/videos/video-b", "admin-key")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign video delete, got %d", w.Code)
	}
	assertErrorCode(t, w, "not_found")
}

// ---- Retry guard (API Contract #7) ----

func TestRetryJobNotFailed(t *testing.T) {
	r, tk := newUnitRouter(t, "tenant-1")
	tk.jobs.jobs["job-1"] = &model.Job{
		ID: "job-1", TenantID: "tenant-1", Title: "job 1", Kind: model.KindVideo,
		Status: model.JobStatusCompleted, Stage: "completed",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	w := doRequest(t, r, http.MethodPost, "/api/v1/jobs/job-1/retry", "admin-key")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for retry on non-failed job, got %d", w.Code)
	}
	assertErrorCode(t, w, "job_not_failed")
}

// ---- Query with fake providers (FR-009, FR-010) ----

func TestQueryWithFakeProviders(t *testing.T) {
	r, tk := newUnitRouter(t, "tenant-1")
	tk.chunks.hits = []model.ChunkSearchResult{
		{
			Chunk: model.Chunk{
				VideoID: "v1", ChunkIndex: 3, Content: "Tag HTML adalah elemen dasar.",
			},
			VideoTitle: "Pengenalan HTML",
		},
	}
	tk.segments.byVideo["v1"] = []string{"web desain", "html dasar"}
	// Point the RAG LLM at a fake with a canned answer.
	tk.handler.RAG = rager.NewRAGService(newFakeEmbedder(), newFakeLLM("HTML adalah singkatan dari HyperText Markup Language."), tk.chunks, tk.segments)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/query", "query-key",
		dto.QueryRequest{Question: "Apa itu HTML?"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	var resp dto.QueryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal query response: %v", err)
	}
	if resp.Answer == nil || *resp.Answer == "" {
		t.Fatalf("expected a non-empty answer, got %v", resp.Answer)
	}
	if len(resp.References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(resp.References))
	}
	ref := resp.References[0]
	if ref.VideoID != "v1" || ref.VideoTitle != "Pengenalan HTML" || ref.ChunkIndex != 3 {
		t.Fatalf("unexpected reference: %+v", ref)
	}
	if len(ref.Segments) != 2 {
		t.Fatalf("expected reference segments, got %v", ref.Segments)
	}
}

func TestQueryRequiresQuestion(t *testing.T) {
	r, _ := newUnitRouter(t, "tenant-1")

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/query", "query-key",
		dto.QueryRequest{Question: "   "})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing question, got %d", w.Code)
	}
	assertErrorCode(t, w, "question_required")
}
