package groq

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pratamaWahyuadi/learn-rag/internal/circuitbreaker"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
)

func TestTranscribe(t *testing.T) {
	// Create a temp audio file to upload.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "audio.mp4")
	if err := os.WriteFile(filePath, []byte("fake media bytes"), 0o600); err != nil {
		t.Fatalf("write media: %v", err)
	}

	var gotAuth, gotModel string
	var gotFileName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		gotModel = r.Form.Get("model")
		files := r.MultipartForm.File["file"]
		if len(files) > 0 {
			gotFileName = files[0].Filename
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"halo dunia","language":"id"}`))
	}))
	defer ts.Close()

	tr, err := New(Config{APIKey: "k", Endpoint: ts.URL}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := tr.Transcribe(context.Background(), ports.TranscribeInput{FilePath: filePath})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if res.Content != "halo dunia" {
		t.Fatalf("unexpected content: %q", res.Content)
	}
	if res.Language == nil || *res.Language != "id" {
		t.Fatalf("unexpected language: %v", res.Language)
	}
	if res.Model == nil || *res.Model != "whisper-large-v3" {
		t.Fatalf("unexpected model: %v", res.Model)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	// Default model is sent even when the caller does not request one.
	if gotModel != "whisper-large-v3" {
		t.Fatalf("unexpected model field: %q", gotModel)
	}
	if gotFileName != "audio.mp4" {
		t.Fatalf("unexpected uploaded filename: %q", gotFileName)
	}
}

// TestTranscribeCustomModel verifies the model requested by the caller is
// actually sent to Groq (regression for the input.Model passthrough bug).
func TestTranscribeCustomModel(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "audio.mp4")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write media: %v", err)
	}

	var gotModel string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		gotModel = r.Form.Get("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer ts.Close()

	tr, err := New(Config{APIKey: "k", Endpoint: ts.URL}, circuitbreaker.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := "whisper-large-v3-turbo"
	if _, err := tr.Transcribe(context.Background(), ports.TranscribeInput{FilePath: filePath, Model: want}); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if gotModel != want {
		t.Fatalf("model sent to Groq = %q, want %q", gotModel, want)
	}
}

func TestBuildMultipart(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.wav")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	boundary := "test-boundary-123"
	newBody := buildMultipart(filePath, "en", "whisper-large-v3-turbo", boundary)
	body, err := newBody()
	if err != nil {
		t.Fatalf("buildMultipart: %v", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read streaming body: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty body")
	}

	contentType := "multipart/form-data; boundary=" + boundary
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		t.Fatalf("invalid multipart content-type %q: %v", contentType, err)
	}
	reader := multipart.NewReader(bytes.NewReader(data), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		_ = part
	}
}
