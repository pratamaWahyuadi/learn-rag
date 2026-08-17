// Package httpclient provides a small HTTP client helper with timeout, retry,
// and exponential backoff for calls to external providers. It deliberately does
// not log request or response bodies, so secrets such as API keys, presigned
// URLs, transcripts, or prompts never leak through logging.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// retryableStatuses are HTTP statuses eligible for a retry: rate limiting and
// transient server errors.
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusInternalServerError ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

// HTTPClient wraps the default HTTP transport with retry/backoff behavior.
type HTTPClient struct {
	client      *http.Client
	maxRetries  int
	initialWait time.Duration
	maxWait     time.Duration
}

// Option configures an HTTPClient.
type Option func(*HTTPClient)

// WithMaxRetries overrides the number of retries on 429/5xx (default 2).
func WithMaxRetries(n int) Option {
	return func(c *HTTPClient) { c.maxRetries = n }
}

// WithTimeout overrides the per-request timeout (default 30 seconds).
func WithTimeout(d time.Duration) Option {
	return func(c *HTTPClient) { c.client.Timeout = d }
}

// New returns an HTTPClient with a 30 second timeout and 2 retries.
func New(opts ...Option) *HTTPClient {
	c := &HTTPClient{
		client:      &http.Client{Timeout: 30 * time.Second},
		maxRetries:  2,
		initialWait: 200 * time.Millisecond,
		maxWait:     4 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// DoJSON sends a JSON request and decodes a 2xx response body into out. It
// retries 429/5xx responses up to maxRetries with exponential backoff. The
// request body and response bodies are never logged.
func (c *HTTPClient) DoJSON(ctx context.Context, method, url string, headers http.Header, body, out any) error {
	var reqBody []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("httpclient: marshal request: %w", err)
		}
		reqBody = b
	}

	resp, err := c.doWithRetry(ctx, method, url, headers, reqBody, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ResponseError{StatusCode: resp.StatusCode}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("httpclient: decode response: %w", err)
		}
	}
	return nil
}

// Do sends an HTTP request with the given raw body and content type, retrying
// on 429/5xx responses and transient network errors. The content-type header is
// set exactly as provided so callers can send non-JSON (e.g. multipart) bodies.
func (c *HTTPClient) Do(ctx context.Context, method, url string, headers http.Header, body []byte, contentType string) (*http.Response, error) {
	return c.doWithRetry(ctx, method, url, headers, body, contentType)
}

// doWithRetry performs the request, retrying on 429/5xx responses and on
// transient network errors, with exponential backoff between attempts.
func (c *HTTPClient) doWithRetry(ctx context.Context, method, url string, headers http.Header, body []byte, contentType string) (*http.Response, error) {
	attempt := 0
	wait := c.initialWait

	for {
		attempt++

		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("httpclient: build request: %w", err)
		}
		for k, vs := range headers {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		if body != nil && contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			// Transient network error; retry unless context is done.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt <= c.maxRetries {
				if !c.sleep(ctx, wait) {
					return nil, ctx.Err()
				}
				wait = c.nextWait(wait)
				continue
			}
			return nil, err
		}

		if retryableStatus(resp.StatusCode) && attempt <= c.maxRetries {
			// Drain the body so the connection can be reused, then retry.
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if !c.sleep(ctx, wait) {
				return nil, ctx.Err()
			}
			wait = c.nextWait(wait)
			continue
		}

		return resp, nil
	}
}

// sleep waits for the backoff delay, aborting early if the context is done.
func (c *HTTPClient) sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// nextWait doubles the backoff delay, capped at maxWait.
func (c *HTTPClient) nextWait(wait time.Duration) time.Duration {
	wait *= 2
	if wait > c.maxWait {
		return c.maxWait
	}
	return wait
}

// ResponseError reports the HTTP status of a non-2xx response.
type ResponseError struct {
	StatusCode int
}

// Error implements the error interface.
func (e *ResponseError) Error() string {
	return fmt.Sprintf("httpclient: unexpected status %d", e.StatusCode)
}
