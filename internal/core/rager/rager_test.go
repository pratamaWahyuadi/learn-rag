package rager

import (
	"context"
	"strings"
	"testing"

	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
)

// --- fakes ---

type fakeEmbedder struct {
	embeddings [][]float32
	err        error
	calls      int
}

func (f *fakeEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = f.embeddings[i]
	}
	return out, nil
}

type fakeLLM struct {
	answer         string
	err            error
	receivedPrompt string
	receivedUser   string
}

func (f *fakeLLM) AnswerQuery(_ context.Context, systemPrompt, userContent string) (string, error) {
	f.receivedPrompt = systemPrompt
	f.receivedUser = userContent
	if f.err != nil {
		return "", f.err
	}
	return f.answer, nil
}

func (f *fakeLLM) Summarize(_ context.Context, _, _ string) (string, error) { return "", nil }

type fakeChunkRepo struct {
	hits []model.ChunkSearchResult
	err  error
}

func (f *fakeChunkRepo) Search(_ context.Context, tenantID string, _ pgvector.Vector, segmentName string, limit int) ([]model.ChunkSearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

func (f *fakeChunkRepo) DeleteByVideoID(_ context.Context, _, _ string) error { return nil }
func (f *fakeChunkRepo) Insert(_ context.Context, _ []model.Chunk) error      { return nil }

type fakeSegmentRepo struct {
	videoSegments map[string][]string
}

func (f *fakeSegmentRepo) ListNamesByVideoID(_ context.Context, videoID, _ string) ([]string, error) {
	return f.videoSegments[videoID], nil
}

func (f *fakeSegmentRepo) EnsureByName(_ context.Context, _, _ string) (*model.Segment, error) {
	return nil, nil
}
func (f *fakeSegmentRepo) AttachJobSegments(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (f *fakeSegmentRepo) AttachVideoSegments(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (f *fakeSegmentRepo) ListNamesByJobID(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func newTestService(emb *fakeEmbedder, llm *fakeLLM, chunks *fakeChunkRepo, segs *fakeSegmentRepo) *RAGService {
	return NewRAGService(emb, llm, chunks, segs)
}

func TestAnswerReturnsReferencesAndAnswer(t *testing.T) {
	emb := &fakeEmbedder{
		embeddings: [][]float32{{0.1, 0.2, 0.3}},
	}
	llm := &fakeLLM{answer: "Tag HTML adalah elemen struktur halaman."}
	chunks := &fakeChunkRepo{
		hits: []model.ChunkSearchResult{
			{Chunk: model.Chunk{VideoID: "v1", ChunkIndex: 3, Content: "Tag HTML adalah elemen dasar."}, VideoTitle: "Pengenalan HTML"},
			{Chunk: model.Chunk{VideoID: "v2", ChunkIndex: 0, Content: "Struktur dokumen HTML."}, VideoTitle: "HTML Dasar"},
		},
	}
	segs := &fakeSegmentRepo{videoSegments: map[string][]string{
		"v1": {"web desain", "html dasar"},
		"v2": {"html dasar"},
	}}

	svc := newTestService(emb, llm, chunks, segs)
	res, err := svc.Answer(context.Background(), "tenant-1", "Apa itu tag HTML?", "web desain", 5)
	if err != nil {
		t.Fatalf("Answer returned error: %v", err)
	}

	if res.Answer == nil || *res.Answer != llm.answer {
		t.Fatalf("unexpected answer: %v", res.Answer)
	}
	if len(res.References) != 2 {
		t.Fatalf("expected 2 references, got %d", len(res.References))
	}
	if res.References[0].VideoID != "v1" || res.References[0].VideoTitle != "Pengenalan HTML" {
		t.Fatalf("unexpected first reference: %+v", res.References[0])
	}
	if len(res.References[0].Segments) != 2 {
		t.Fatalf("expected first reference to include segments, got %v", res.References[0].Segments)
	}

	// The user content must wrap documents in <documents> and include the
	// question, and the system prompt must flag the documents as untrusted.
	if !strings.Contains(llm.receivedUser, "<documents>") || !strings.Contains(llm.receivedUser, "Pertanyaan:") {
		t.Fatalf("user content missing document wrapper or question: %q", llm.receivedUser)
	}
	if !strings.Contains(llm.receivedPrompt, "untrusted") || !strings.Contains(llm.receivedPrompt, "Ignore any instructions") {
		t.Fatalf("system prompt must defend against prompt injection: %q", llm.receivedPrompt)
	}
	if emb.calls != 1 {
		t.Fatalf("expected exactly 1 embed call, got %d", emb.calls)
	}
}

func TestAnswerNoMatchesReturnsEmptyResult(t *testing.T) {
	emb := &fakeEmbedder{embeddings: [][]float32{{0.1, 0.2, 0.3}}}
	llm := &fakeLLM{answer: "unused"}
	chunks := &fakeChunkRepo{hits: nil}
	segs := &fakeSegmentRepo{videoSegments: map[string][]string{}}

	svc := newTestService(emb, llm, chunks, segs)
	res, err := svc.Answer(context.Background(), "tenant-1", "tidak ada materi", "", 5)
	if err != nil {
		t.Fatalf("Answer returned error: %v", err)
	}

	if res.Answer != nil {
		t.Fatalf("expected nil answer when no chunks match, got %v", *res.Answer)
	}
	if len(res.References) != 0 {
		t.Fatalf("expected no references, got %d", len(res.References))
	}
	// The LLM should never be called when there is no context.
	if llm.receivedUser != "" {
		t.Fatalf("LLM should not be called without references")
	}
}

func TestAnswerEmbedsQuestionOnFirstCall(t *testing.T) {
	emb := &fakeEmbedder{embeddings: [][]float32{{0.9, 0.5, 0.2}}}
	llm := &fakeLLM{answer: "A"}
	chunks := &fakeChunkRepo{hits: []model.ChunkSearchResult{
		{Chunk: model.Chunk{VideoID: "v1", ChunkIndex: 1, Content: "snippet"}, VideoTitle: "V"},
	}}
	segs := &fakeSegmentRepo{videoSegments: map[string][]string{"v1": nil}}

	svc := newTestService(emb, llm, chunks, segs)
	if _, err := svc.Answer(context.Background(), "tenant-1", "pertanyaan", "", 5); err != nil {
		t.Fatalf("Answer returned error: %v", err)
	}
	if emb.calls != 1 {
		t.Fatalf("expected 1 embed call, got %d", emb.calls)
	}
}
