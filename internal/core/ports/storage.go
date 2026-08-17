package ports

import (
	"context"
	"time"
)

// Storage is the object-storage abstraction (implemented by Cloudflare R2).
type Storage interface {
	// GenerateUploadURL returns a time-limited presigned URL for uploading a
	// file directly to object storage.
	GenerateUploadURL(ctx context.Context, fileKey, contentType string, expiresAt time.Time) (string, error)
	// Download writes the object identified by fileKey to the local destPath.
	Download(ctx context.Context, fileKey, destPath string) error
	// Delete removes the object identified by fileKey. A missing object is not
	// treated as an error.
	Delete(ctx context.Context, fileKey string) error
}
