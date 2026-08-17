package ports

import (
	"context"

	"github.com/pratamaWahyuadi/learn-rag/internal/core/model"
)

// TranscribeInput describes a media file to be transcribed.
type TranscribeInput struct {
	// FilePath is the local path of the downloaded media file.
	FilePath string
	// Language optionally hints the dominant language of the content.
	Language string
	// Model optionally overrides the transcription model.
	Model string
}

// Transcriber transcribes video/audio content (implemented by Groq Whisper).
type Transcriber interface {
	Transcribe(ctx context.Context, input TranscribeInput) (*model.Transcript, error)
}
