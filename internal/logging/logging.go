// Package logging provides a safe, structured JSON logger based on slog.
//
// Security note (Threat #7): values whose keys match the exported field
// constants below must never appear in log output. Care is taken so that no
// default middleware logs request headers, bodies, query parameters, or API
// keys.
package logging

import (
	"log/slog"
	"os"
)

// Sensitive field names that must never be logged in full (Threat #7).
const (
	FieldXAPIKey       = "x-api-key"
	FieldPresignedURL  = "presigned_url"
	FieldFileKey       = "file_key"
	FieldTranscript    = "transcript"
	FieldPrompt        = "prompt"
	FieldAuthorization = "authorization"
)

// SensitiveFields is the canonical set of field names that must be redacted
// from any structured log output.
var SensitiveFields = []string{
	FieldXAPIKey,
	FieldPresignedURL,
	FieldFileKey,
	FieldTranscript,
	FieldPrompt,
	FieldAuthorization,
}

// NewLogger returns a JSON-structured logger writing to stdout. It is
// intentionally safe by default: it does not register any handler that logs
// request bodies, headers, or query parameters.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
