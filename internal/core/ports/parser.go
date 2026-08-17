package ports

import "context"

// DocumentParser extracts text from documents such as PDFs (implemented by
// LlamaParse). It has no OCR fallback; extraction failure is returned as an
// error.
type DocumentParser interface {
	Parse(ctx context.Context, filePath, contentType string) (string, error)
}
