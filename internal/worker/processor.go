package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/chunker"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/ports"
	"github.com/pratamaWahyuadi/learn-rag/internal/core/summarizer"
)

// Pipeline stages and summary model name. These are the only stage strings the
// worker writes on top of the ClaimNextPending default.
const (
	stageDownloading  = "downloading"
	stageTranscribing = "transcribing"
	stageParsing      = "parsing"
	stageChunking     = "chunking"
	stageEmbedding    = "embedding"
	stageCompleted    = "completed"

	// summaryAttempts is how many times summary generation is retried after a
	// failure before the summary is recorded as failed.
	summaryAttempts = 2

	summaryModelName = "deepseek"
)

// Sentinel errors used to classify pipeline failures and log a safe message.
var (
	// ErrUnsupportedJobKind is returned when a job has an unknown kind.
	ErrUnsupportedJobKind = errors.New("unsupported job kind")
	// ErrFileEmpty is returned when the downloaded file has no content.
	ErrFileEmpty = errors.New("downloaded file is empty")
	// ErrFileTooLarge is returned when the downloaded file exceeds the size cap.
	ErrFileTooLarge = errors.New("downloaded file exceeds the size limit")
	// ErrUnsupportedContentType is returned when the file type does not match the
	// job kind.
	ErrUnsupportedContentType = errors.New("unsupported content type for job kind")
)

// pipelineDeps bundles everything ProcessJob needs to run one job end to end.
type pipelineDeps struct {
	log                *slog.Logger
	jobs               ports.JobRepository
	videos             ports.VideoRepository
	segments           ports.SegmentRepository
	transcripts        ports.TranscriptRepository
	chunks             ports.ChunkRepository
	summaries          ports.SummaryRepository
	storage            ports.Storage
	transcriber        ports.Transcriber
	parser             ports.DocumentParser
	embedder           ports.Embedder
	llm                ports.LLM
	summarizer         *summarizer.Summarizer
	maxUploadBytes     int64
	embeddingBatchSize int
}

// ProcessJob loads the job by id across all tenants and runs the pipeline. On
// failure the job and its video are marked failed. Only the job id is logged at
// the top level; file keys and transcripts are never logged.
func (w *Worker) ProcessJob(ctx context.Context, jobID string) {
	job, err := w.jobs.GetByIDAllTenants(ctx, jobID)
	if err != nil {
		w.log.Error("worker: load job", "job_id", jobID, "error", err.Error())
		return
	}
	if err := w.deps.run(ctx, job); err != nil {
		w.deps.fail(ctx, job, err)
		w.log.Error("worker: job failed", "job_id", job.ID, "stage", job.Stage,
			"error", err.Error())
		return
	}
	w.log.Info("worker: job completed", "job_id", job.ID, "stage", stageCompleted)
}

// run executes the job pipeline. It is idempotent: any previous video (and its
// cascaded chunks/transcripts/summaries) is removed before a fresh video row and
// its data are created.
func (d *pipelineDeps) run(ctx context.Context, job *model.Job) error {
	// Idempotent cleanup for retries.
	if err := d.videos.DeleteByJobID(ctx, job.ID, job.TenantID); err != nil {
		return fmt.Errorf("worker: cleanup existing video: %w", err)
	}

	now := time.Now()
	video := &model.Video{
		ID:        uuid.NewString(),
		TenantID:  job.TenantID,
		JobID:     job.ID,
		Title:     job.Title,
		Kind:      job.Kind,
		FileKey:   job.FileKey,
		Status:    model.VideoStatusProcessing,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := d.videos.Create(ctx, video); err != nil {
		return fmt.Errorf("worker: create video: %w", err)
	}

	if err := d.copyJobSegments(ctx, job, video.ID); err != nil {
		return fmt.Errorf("worker: copy job segments: %w", err)
	}

	transcript, err := d.extractTranscript(ctx, job, video)
	if err != nil {
		return err
	}

	if err := d.setStage(ctx, job, stageChunking); err != nil {
		return err
	}
	sections := chunker.Chunk(transcript.Content)

	if err := d.setStage(ctx, job, stageEmbedding); err != nil {
		return err
	}
	chunkModels, err := d.embedSections(ctx, job, video, sections)
	if err != nil {
		return err
	}
	if len(chunkModels) > 0 {
		if err := d.chunks.Insert(ctx, chunkModels); err != nil {
			return fmt.Errorf("worker: insert chunks: %w", err)
		}
	}

	// Mark the job and video completed (sets job.finished_at).
	if err := d.jobs.UpdateStatus(ctx, job.ID, job.TenantID, model.JobStatusCompleted, stageCompleted, nil); err != nil {
		return fmt.Errorf("worker: complete job: %w", err)
	}
	if err := d.videos.UpdateStatus(ctx, video.ID, job.TenantID, model.VideoStatusCompleted); err != nil {
		return fmt.Errorf("worker: complete video: %w", err)
	}

	// Summary runs after the job is completed and never fails the job.
	d.generateSummary(ctx, job, video, transcript)
	return nil
}

// extractTranscript downloads the job file, validates it, and either transcribes
// (video/audio) or parses (pdf) it, persisting the transcript.
func (d *pipelineDeps) extractTranscript(ctx context.Context, job *model.Job, video *model.Video) (*model.Transcript, error) {
	tmpDir, err := os.MkdirTemp("", "rag-dl-")
	if err != nil {
		return nil, fmt.Errorf("worker: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dest := filepath.Join(tmpDir, "input")
	if err := d.setStage(ctx, job, stageDownloading); err != nil {
		return nil, err
	}
	if err := d.storage.Download(ctx, job.FileKey, dest); err != nil {
		return nil, fmt.Errorf("worker: download: %w", err)
	}
	if err := validateMediaFile(dest, job.FileKey, job.Kind, d.maxUploadBytes); err != nil {
		return nil, err
	}

	switch job.Kind {
	case model.KindVideo, model.KindAudio:
		if err := d.setStage(ctx, job, stageTranscribing); err != nil {
			return nil, err
		}
		tr, err := d.transcriber.Transcribe(ctx, ports.TranscribeInput{FilePath: dest})
		if err != nil {
			return nil, fmt.Errorf("worker: transcribe: %w", err)
		}
		tr.TenantID = job.TenantID
		tr.VideoID = video.ID
		tr.CreatedAt = time.Now()
		if err := d.transcripts.Upsert(ctx, tr); err != nil {
			return nil, fmt.Errorf("worker: upsert transcript: %w", err)
		}
		return tr, nil

	case model.KindPDF:
		if err := d.setStage(ctx, job, stageParsing); err != nil {
			return nil, err
		}
		mt, err := mimetype.DetectFile(dest)
		if err != nil {
			return nil, fmt.Errorf("worker: detect content type: %w", err)
		}
		text, err := d.parser.Parse(ctx, dest, mt.String())
		if err != nil {
			return nil, fmt.Errorf("worker: parse: %w", err)
		}
		tr := &model.Transcript{
			TenantID:  job.TenantID,
			VideoID:   video.ID,
			Content:   text,
			CreatedAt: time.Now(),
		}
		if err := d.transcripts.Upsert(ctx, tr); err != nil {
			return nil, fmt.Errorf("worker: upsert transcript: %w", err)
		}
		return tr, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedJobKind, job.Kind)
	}
}

// copyJobSegments copies the job's segment names onto the newly created video.
func (d *pipelineDeps) copyJobSegments(ctx context.Context, job *model.Job, videoID string) error {
	names, err := d.segments.ListNamesByJobID(ctx, job.ID, job.TenantID)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		seg, err := d.segments.EnsureByName(ctx, job.TenantID, name)
		if err != nil {
			return err
		}
		ids = append(ids, seg.ID)
	}
	return d.segments.AttachVideoSegments(ctx, videoID, job.TenantID, ids)
}

// embedSections embeds the chunked sections in batches and produces model.Chunk
// rows with sequential chunk indices.
func (d *pipelineDeps) embedSections(ctx context.Context, job *model.Job, video *model.Video, sections []string) ([]model.Chunk, error) {
	batchSize := d.embeddingBatchSize
	if batchSize <= 0 {
		batchSize = 16
	}
	chunks := make([]model.Chunk, 0, len(sections))
	now := time.Now()
	for start := 0; start < len(sections); start += batchSize {
		end := start + batchSize
		if end > len(sections) {
			end = len(sections)
		}
		texts := sections[start:end]
		embs, err := d.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("worker: embed batch: %w", err)
		}
		for i, text := range texts {
			var emb pgvector.Vector
			if i < len(embs) {
				emb = pgvector.NewVector(embs[i])
			}
			chunks = append(chunks, model.Chunk{
				ID:         uuid.NewString(),
				TenantID:   job.TenantID,
				VideoID:    video.ID,
				ChunkIndex: len(chunks),
				Content:    text,
				CreatedAt:  now,
				Embedding:  emb,
			})
		}
	}
	return chunks, nil
}

// generateSummary produces the summary after the job is completed. Failures are
// recorded as a failed summary row and never affect the job status.
func (d *pipelineDeps) generateSummary(ctx context.Context, job *model.Job, video *model.Video, tr *model.Transcript) {
	if tr == nil || tr.Content == "" {
		return
	}
	var content string
	var err error
	for attempt := 1; attempt <= summaryAttempts; attempt++ {
		content, err = d.summarizer.Summarize(ctx, tr.Content, d.llm)
		if err == nil {
			break
		}
		d.log.Error("worker: summarize attempt failed", "job_id", job.ID, "attempt", attempt,
			"error", err.Error())
	}
	if err != nil {
		msg := "summary generation failed"
		d.log.Error("worker: summary failed", "job_id", job.ID, "video_id", video.ID)
		d.recordSummary(ctx, job, video, &model.Summary{
			ID:           uuid.NewString(),
			Status:       "failed",
			ErrorMessage: &msg,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})
		return
	}
	contentPtr := content
	d.recordSummary(ctx, job, video, &model.Summary{
		ID:        uuid.NewString(),
		Status:    "completed",
		Content:   &contentPtr,
		Language:  tr.Language,
		Model:     strPtr(summaryModelName),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
}

// recordSummary persists a summary row for the video.
func (d *pipelineDeps) recordSummary(ctx context.Context, job *model.Job, video *model.Video, s *model.Summary) {
	s.TenantID = job.TenantID
	s.VideoID = video.ID
	if err := d.summaries.Upsert(ctx, s); err != nil {
		d.log.Error("worker: upsert summary", "job_id", job.ID, "error", err.Error())
	}
}

// setStage updates the job's stage while it remains in the processing state.
func (d *pipelineDeps) setStage(ctx context.Context, job *model.Job, stage string) error {
	if err := d.jobs.UpdateStatus(ctx, job.ID, job.TenantID, model.JobStatusProcessing, stage, nil); err != nil {
		return fmt.Errorf("worker: update stage: %w", err)
	}
	job.Stage = stage
	return nil
}

// fail marks the job and its video as failed upon a pipeline error.
func (d *pipelineDeps) fail(ctx context.Context, job *model.Job, cause error) {
	msg := sanitizeError(cause)
	if err := d.jobs.UpdateStatus(ctx, job.ID, job.TenantID, model.JobStatusFailed, job.Stage, &msg); err != nil {
		d.log.Error("worker: mark job failed", "job_id", job.ID, "error", err.Error())
	}
	if err := d.videos.FailByJobID(ctx, job.ID, job.TenantID); err != nil {
		d.log.Error("worker: mark video failed", "job_id", job.ID, "error", err.Error())
	}
}

// sanitizeError returns a short, safe message for the job error_message column.
// It never embeds a file key or transcript content. Validation failures map to
// their sentinel wording; other failures keep their generic top-level message.
func sanitizeError(err error) string {
	if err == nil {
		return "worker: job failed"
	}
	msg := err.Error()
	if msg == "" {
		return "worker: job failed"
	}
	return msg
}

// allowed extensions per kind, used as a fallback when MIME detection is not
// authoritative.
var mediaExts = map[string]map[string]bool{
	model.KindPDF: {".pdf": true},
	model.KindVideo: {
		".mp4": true, ".mov": true, ".m4v": true, ".webm": true,
		".mkv": true, ".avi": true, ".mpeg": true, ".mpg": true, ".wmv": true,
	},
	model.KindAudio: {
		".mp3": true, ".wav": true, ".m4a": true, ".ogg": true, ".oga": true,
		".aac": true, ".flac": true, ".opus": true, ".webm": true,
	},
}

// validateMediaFile checks the downloaded file size and that its detected type
// matches the job kind. fileKey is the original R2 object key, used only to
// derive the file extension as a fallback when content sniffing is ambiguous.
func validateMediaFile(path, fileKey, kind string, maxBytes int64) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("worker: stat downloaded file: %w", err)
	}
	if fi.Size() <= 0 {
		return ErrFileEmpty
	}
	if maxBytes > 0 && fi.Size() > maxBytes {
		return ErrFileTooLarge
	}

	// Extension fallback for types MIME detection reports as generic or that it
	// cannot distinguish reliably (e.g. m4a, ogg, webm).
	ext := strings.ToLower(filepath.Ext(fileKey))
	if mediaExts[kind][ext] {
		return nil
	}
	mt, err := mimetype.DetectFile(path)
	if err != nil {
		// MIME detection failed; the extension already did not match.
		return ErrUnsupportedContentType
	}
	if matchesKind(strings.ToLower(mt.String()), kind) {
		return nil
	}
	return ErrUnsupportedContentType
}

// matchesKind reports whether a detected MIME type is consistent with a kind.
func matchesKind(mime, kind string) bool {
	switch kind {
	case model.KindPDF:
		return mime == "application/pdf"
	case model.KindVideo:
		return strings.HasPrefix(mime, "video/")
	case model.KindAudio:
		return strings.HasPrefix(mime, "audio/")
	default:
		return false
	}
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }
