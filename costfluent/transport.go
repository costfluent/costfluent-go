package costfluent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// newRequest creates a new HTTP request with common headers
func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	if c.baseURLErr != nil {
		return nil, c.baseURLErr
	}
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// The API reads a JSON Content-Type as the promise of a body and answers an empty one with 400.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	// On the request rather than in a transport, so the client drops it on a redirect to another host.
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return req, nil
}

// newWorkspaceRequest is newRequest for an operation the API scopes by the workspaceId query
// parameter.
func (c *Client) newWorkspaceRequest(
	ctx context.Context, method, path, workspaceID string, body any,
) (*http.Request, error) {
	id, err := c.requireWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("workspaceId", id)
	req.URL.RawQuery = q.Encode()
	return req, nil
}

// do executes the request and decodes the response, keeping a versioned resource's ETag.
func (c *Client) do(req *http.Request, v any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return parseError(resp, time.Now())
	}

	if v != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		if resource, ok := v.(versioned); ok {
			resource.setETag(resp.Header.Get("ETag"))
		}
		if created, ok := v.(replayable); ok {
			created.setReplayed(resp.Header.Get(replayedHeader) == "true")
		}
	}
	return nil
}

// retryTransport resends a request the API turned away for now: any method on a 429, because the
// API's rate limiter answers before a request reaches an endpoint, and the unconditional idempotent
// requests, creates under an Idempotency-Key among them, on a 5xx, a 404 from the proxy rather than
// the API, or a transport error. Each attempt runs under its own timeout.
type retryTransport struct {
	base           http.RoundTripper
	maxRetries     int
	baseDelay      time.Duration
	maxDelay       time.Duration
	maxRetryAfter  time.Duration
	attemptTimeout time.Duration
	now            func() time.Time
	sleep          func(context.Context, time.Duration) error
}

// defaultMaxRetryAfter is the longest Retry-After the transport waits out. A longer one means the
// token's hourly quota is spent, and the 429 is returned rather than held for most of an hour.
const defaultMaxRetryAfter = 60 * time.Second

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	maxRetries := t.maxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	baseDelay := t.baseDelay
	if baseDelay == 0 {
		baseDelay = time.Second
	}
	maxDelay := t.maxDelay
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}
	maxRetryAfter := t.maxRetryAfter
	if maxRetryAfter == 0 {
		maxRetryAfter = defaultMaxRetryAfter
	}
	sleep := t.sleep
	if sleep == nil {
		sleep = sleepContext
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		resp, err := t.attempt(req, bodyBytes)
		last := attempt >= maxRetries
		if err != nil {
			var timedOut *attemptTimeoutError
			if last || !retryable(req) || errors.As(err, &timedOut) {
				return nil, err
			}
		} else if last || !shouldRetry(req, resp) {
			return resp, nil
		}

		delay := t.calculateDelay(attempt, baseDelay, maxDelay, resp)
		if resp != nil {
			if delay > maxRetryAfter {
				return resp, nil
			}
			_ = resp.Body.Close()
		}
		if err := sleep(req.Context(), delay); err != nil {
			return nil, err
		}
	}
}

// attempt sends one copy of req under the per-attempt timeout, which keeps running while the
// caller reads the response body and ends when the body is closed.
func (t *retryTransport) attempt(req *http.Request, body []byte) (*http.Response, error) {
	ctx, cancel := req.Context(), context.CancelFunc(func() {})
	if t.attemptTimeout > 0 {
		ctx, cancel = context.WithTimeout(req.Context(), t.attemptTimeout)
	}
	r := req.Clone(ctx)
	if body != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	resp, err := t.base.RoundTrip(r)
	if err != nil {
		timedOut := ctx.Err() != nil && req.Context().Err() == nil
		cancel()
		if timedOut {
			return nil, &attemptTimeoutError{timeout: t.attemptTimeout, err: err}
		}
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// attemptTimeoutError is a net.Error, so callers can test it with Timeout() as they would the
// http.Client's own timeout.
type attemptTimeoutError struct {
	timeout time.Duration
	err     error
}

func (e *attemptTimeoutError) Error() string {
	return fmt.Sprintf("request timed out after %s: %v", e.timeout, e.err)
}

func (e *attemptTimeoutError) Unwrap() error   { return e.err }
func (e *attemptTimeoutError) Timeout() bool   { return true }
func (e *attemptTimeoutError) Temporary() bool { return true }

// retryable reports whether a request is safe to resend after a lost response or a 5xx: a POST
// triggers, so resending one the server may have run could run it twice unless it carries an
// Idempotency-Key, and a request with If-Match the server applied would be refused by its own now
// stale precondition.
func retryable(req *http.Request) bool {
	if req.Header.Get("If-Match") != "" {
		return false
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	case http.MethodPost:
		// A create under an Idempotency-Key runs once however often it is sent: a retry is
		// answered with the first response.
		return req.Header.Get(idempotencyKeyHeader) != ""
	}
	return false
}

// shouldRetry reports whether a response is worth another attempt. A 429 is, for every method,
// because the rate limiter refuses a request before any endpoint runs it. A 404 that is not the
// API's problem details is retried like a 5xx: the proxy had no route to the API yet, but an
// endpoint answering a bare 404 looks the same, so it does not prove a trigger never ran.
func shouldRetry(req *http.Request, resp *http.Response) bool {
	return resp.StatusCode == http.StatusTooManyRequests ||
		((resp.StatusCode >= 500 || edgeNotFound(resp)) && retryable(req))
}

func (t *retryTransport) calculateDelay(attempt int, baseDelay, maxDelay time.Duration, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := retryAfter(resp.Header, t.clock()); ok {
			return d
		}
	}

	// Exponential backoff with jitter
	delay := baseDelay * (1 << attempt)
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add 0-25% jitter
	if delay > 0 {
		jitter := time.Duration(rand.Int63n(int64(delay / 4))) //nolint:gosec // backoff jitter needs no unpredictability
		delay += jitter
	}

	return delay
}

func (t *retryTransport) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

// retryAfter reads a Retry-After header, given in seconds or as an HTTP date, or else the
// RateLimit-Reset seconds the API sends beside it on a 429.
func retryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	ra := h.Get("Retry-After")
	if ra == "" {
		ra = h.Get("RateLimit-Reset")
	}
	if ra == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(ra); err == nil {
		return max(time.Duration(secs)*time.Second, 0), true
	}
	if at, err := http.ParseTime(ra); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}

// debugTransport logs requests and responses
type debugTransport struct {
	base   http.RoundTripper
	logger func(format string, args ...any)
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.logger("[costfluent] -> %s %s", req.Method, req.URL)
	for k, v := range req.Header {
		if k == "Authorization" {
			t.logger("[costfluent]    %s: [REDACTED]", k)
		} else {
			t.logger("[costfluent]    %s: %v", k, v)
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		t.logger("[costfluent] <- ERROR: %v", err)
		return nil, err
	}

	t.logger("[costfluent] <- %d %s", resp.StatusCode, resp.Status)
	return resp, nil
}
