// Package errors defines the shared sentinel errors used across the
// application. Each error carries an API error code and its HTTP status, as
// specified in the API Contract ("Format Error Konsisten").
package errors

import (
	"net/http"
)

// APIError is a domain error that carries a stable API error code and the
// corresponding HTTP status. The message is always safe to return to clients;
// internal implementation details never leak through this type.
type APIError struct {
	Code       string
	HTTPStatus int
	message    string
}

// Error implements the error interface.
func (e *APIError) Error() string { return e.message }

// Message returns the safe, client-facing message.
func (e *APIError) Message() string { return e.message }

// newAPIError constructs an APIError instance.
func newAPIError(code string, status int, msg string) *APIError {
	return &APIError{Code: code, HTTPStatus: status, message: msg}
}

// Sentinel errors mapped to the API Contract error codes and HTTP statuses.
var (
	ErrInvalidRequest         = newAPIError("invalid_request", http.StatusBadRequest, "Request tidak valid.")
	ErrUnsupportedContentType = newAPIError("unsupported_content_type", http.StatusBadRequest, "Tipe konten tidak diizinkan.")
	ErrInvalidFileKey         = newAPIError("invalid_file_key", http.StatusBadRequest, "File key tidak sesuai.")
	ErrExpiredUploadIntent    = newAPIError("expired_upload_intent", http.StatusBadRequest, "Upload intent sudah kedaluwarsa.")
	ErrUploadIntentConsumed   = newAPIError("upload_intent_consumed", http.StatusBadRequest, "Upload intent sudah dipakai.")
	ErrJobNotFailed           = newAPIError("job_not_failed", http.StatusBadRequest, "Retry hanya bisa dilakukan untuk job yang gagal.")
	ErrQuestionRequired       = newAPIError("question_required", http.StatusBadRequest, "Field question wajib diisi.")
	ErrNotFound               = newAPIError("not_found", http.StatusNotFound, "Resource tidak ditemukan.")
	ErrUnauthorized           = newAPIError("unauthorized", http.StatusUnauthorized, "API key tidak valid.")
	ErrForbidden              = newAPIError("forbidden", http.StatusForbidden, "API key tidak memiliki akses ke resource ini.")
	ErrRateLimited            = newAPIError("rate_limited", http.StatusTooManyRequests, "Terlalu banyak permintaan. Coba lagi nanti.")
	ErrInternal               = newAPIError("internal_error", http.StatusInternalServerError, "Terjadi kesalahan internal.")
)

// Code returns the API error code for err. Unknown errors (anything that is
// not an *APIError) map to the generic internal_error code so that internal
// details never leak to the client.
func Code(err error) string {
	if e, ok := err.(*APIError); ok {
		return e.Code
	}
	return ErrInternal.Code
}

// HTTPStatus returns the HTTP status for err, defaulting to 500 for any
// non-APIError so a failed handler never exposes internal information.
func HTTPStatus(err error) int {
	if e, ok := err.(*APIError); ok {
		return e.HTTPStatus
	}
	return ErrInternal.HTTPStatus
}

// Message returns the safe client-facing message for err. Non-APIError values
// fall back to the generic internal error message.
func Message(err error) string {
	if e, ok := err.(*APIError); ok {
		return e.message
	}
	return ErrInternal.message
}
