package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestCodeAndHTTPStatus(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{"invalid_request", ErrInvalidRequest, "invalid_request", http.StatusBadRequest},
		{"unsupported_content_type", ErrUnsupportedContentType, "unsupported_content_type", http.StatusBadRequest},
		{"invalid_file_key", ErrInvalidFileKey, "invalid_file_key", http.StatusBadRequest},
		{"expired_upload_intent", ErrExpiredUploadIntent, "expired_upload_intent", http.StatusBadRequest},
		{"upload_intent_consumed", ErrUploadIntentConsumed, "upload_intent_consumed", http.StatusBadRequest},
		{"job_not_failed", ErrJobNotFailed, "job_not_failed", http.StatusBadRequest},
		{"question_required", ErrQuestionRequired, "question_required", http.StatusBadRequest},
		{"not_found", ErrNotFound, "not_found", http.StatusNotFound},
		{"unauthorized", ErrUnauthorized, "unauthorized", http.StatusUnauthorized},
		{"forbidden", ErrForbidden, "forbidden", http.StatusForbidden},
		{"rate_limited", ErrRateLimited, "rate_limited", http.StatusTooManyRequests},
		{"internal", ErrInternal, "internal_error", http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Code(tc.err); got != tc.wantCode {
				t.Errorf("Code(%v) = %q, want %q", tc.err, got, tc.wantCode)
			}
			if got := HTTPStatus(tc.err); got != tc.wantStatus {
				t.Errorf("HTTPStatus(%v) = %d, want %d", tc.err, got, tc.wantStatus)
			}
		})
	}
}

func TestUnknownErrorMapsToInternal(t *testing.T) {
	err := errors.New("boom: secret db details")

	if got := Code(err); got != "internal_error" {
		t.Errorf("Code(%v) = %q, want internal_error", err, got)
	}
	if got := HTTPStatus(err); got != http.StatusInternalServerError {
		t.Errorf("HTTPStatus(%v) = %d, want 500", err, got)
	}
	if got := Message(err); got == "" || got == err.Error() {
		t.Errorf("Message(%v) must not leak the wrapped error, got %q", err, got)
	}
}

func TestErrorsIsWorks(t *testing.T) {
	var sentinel = ErrNotFound
	if !errors.Is(sentinel, ErrNotFound) {
		t.Error("expected errors.Is(sentinel, ErrNotFound) to be true")
	}
}
