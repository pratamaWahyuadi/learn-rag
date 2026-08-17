package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pratamaWahyuadi/learn-rag/internal/config"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
)

func TestRetentionIntervalFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"unset uses default", "", defaultRetentionInterval},
		{"valid seconds", "300", 300 * time.Second},
		{"zero falls back", "0", defaultRetentionInterval},
		{"negative falls back", "-5", defaultRetentionInterval},
		{"non-numeric falls back", "abc", defaultRetentionInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old, had := os.LookupEnv("RETENTION_INTERVAL")
			t.Cleanup(func() {
				if had {
					_ = os.Setenv("RETENTION_INTERVAL", old)
				} else {
					_ = os.Unsetenv("RETENTION_INTERVAL")
				}
			})
			if tt.env == "" {
				_ = os.Unsetenv("RETENTION_INTERVAL")
			} else {
				_ = os.Setenv("RETENTION_INTERVAL", tt.env)
			}
			if got := retentionInterval(); got != tt.want {
				t.Fatalf("retentionInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrencyClamp(t *testing.T) {
	cfg := &config.Config{}
	w := &Worker{cfg: cfg}
	for _, tt := range []struct {
		in, want int
	}{
		{0, 3},
		{1, 1},
		{3, 3},
		{5, 5},
		{9, 5},
	} {
		cfg.WorkerConcurrency = tt.in
		if got := w.concurrency(); got != tt.want {
			t.Fatalf("concurrency(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSanitizeError(t *testing.T) {
	if got := sanitizeError(ErrFileEmpty); got != ErrFileEmpty.Error() {
		t.Fatalf("sanitizeError(ErrFileEmpty) = %q", got)
	}
	if got := sanitizeError(nil); got != "worker: job failed" {
		t.Fatalf("sanitizeError(nil) = %q, want fallback", got)
	}
}

func TestMatchesKind(t *testing.T) {
	cases := []struct {
		mime, kind string
		want       bool
	}{
		{"video/mp4", model.KindVideo, true},
		{"audio/mpeg", model.KindAudio, true},
		{"application/pdf", model.KindPDF, true},
		{"audio/mp4", model.KindVideo, false},
		{"image/png", model.KindPDF, false},
	}
	for _, c := range cases {
		if got := matchesKind(c.mime, c.kind); got != c.want {
			t.Fatalf("matchesKind(%q, %q) = %v, want %v", c.mime, c.kind, got, c.want)
		}
	}
}

func TestValidateMediaFileSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.mp3")
	if err := os.WriteFile(path, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}

	// Over the cap -> ErrFileTooLarge.
	if err := validateMediaFile(path, "sample.mp3", model.KindAudio, 5); err != ErrFileTooLarge {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
	// Within cap and valid extension -> allowed without requiring real MIME.
	if err := validateMediaFile(path, "sample.mp3", model.KindAudio, 20); err != nil {
		t.Fatalf("expected allowed, got %v", err)
	}
	// Extension mismatch and small non-matching binary content -> unsupported.
	if err := validateMediaFile(path, "notes.txt", model.KindPDF, 20); err != ErrUnsupportedContentType {
		t.Fatalf("expected ErrUnsupportedContentType, got %v", err)
	}
}
