package costfluent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	if c.agent != "" {
		req.Header.Set(AgentHeader, c.agent)
	}
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

// statusRecorder is a result whose meaning depends on the success status that carried it, such as
// a post answering 201 when it created and 200 when it replaced.
type statusRecorder interface {
	setStatus(code int)
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
		if answer, ok := v.(statusRecorder); ok {
			answer.setStatus(resp.StatusCode)
		}
	}
	return nil
}

// retryTransport resends a request the API turned away for now: any method on a 429, because the
// API's rate limiter answers before a request reaches an endpoint, for as long as the waits stay
// within maxRetryAfter in all; and the unconditional idempotent requests, creates under an
// Idempotency-Key among them, on a 5xx, a 404 from the proxy rather than the API, or a transport
// error, up to maxRetries times. Each attempt runs under its own timeout.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	// maxRetryAfter is the longest a call waits out rate limits in all, and the longest wait a
	// server error asks for that is waited out: zero is defaultMaxRetryAfter, and waitOutNone none.
	maxRetryAfter  time.Duration
	attemptTimeout time.Duration
	now            func() time.Time
	sleep          func(context.Context, time.Duration) error
}

// defaultMaxRetryAfter is the longest a call waits out rate limits in all: one per-minute window, the
// longest a per-minute refusal takes to clear. A single longer Retry-After means the token's hourly
// quota is spent, and the 429 is returned at once rather than held for most of an hour.
const defaultMaxRetryAfter = 60 * time.Second

// waitOutNone is the maxRetryAfter that returns every rate limit and every wait the API asks for at
// once: a Retry-After of 0 is still above it.
const waitOutNone = -time.Nanosecond

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

	// A 429 is resent while the waits it asks for stay within maxRetryAfter in all, whatever their
	// count: under contention the API's limiter frees one slot at a time and answers each refusal with
	// about a second, so a count would give up while the budget refills. Each wait counts at least a
	// second, which ends a 429 that asks for none, as an intermediary's Retry-After: 0 does, after 61
	// attempts. Server and connection errors keep their count of maxRetries.
	var retries int
	// backoffs counts the waits the API did not ask for: the backoff's exponent, so a server error
	// after rate limits backs off from its first step.
	var backoffs int
	var rateLimited time.Duration
	for {
		resp, err := t.attempt(req, bodyBytes)
		limited := err == nil && resp.StatusCode == http.StatusTooManyRequests
		if err != nil {
			var timedOut *attemptTimeoutError
			if retries >= maxRetries || !retryable(req) || errors.As(err, &timedOut) {
				return nil, err
			}
		} else if !shouldRetry(req, resp) || (!limited && retries >= maxRetries) {
			return resp, nil
		}

		delay, asked := t.calculateDelay(backoffs, baseDelay, maxDelay, resp)
		if !asked {
			backoffs++
		}
		if limited {
			counted := max(delay, time.Second)
			// Subtracted, not added: rateLimited never exceeds maxRetryAfter, but a sum with a huge
			// Retry-After wraps negative and would pass.
			if counted > maxRetryAfter-rateLimited {
				return resp, nil
			}
			rateLimited += counted
		} else {
			retries++
			if asked && delay > maxRetryAfter {
				return resp, nil
			}
		}
		if resp != nil {
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
// stale precondition. An unconditional DELETE is resent, and when the first attempt removed the
// object before its answer was lost, the resend's not-found is reported as the API answered it: a
// success made up here would also be reported for an id that never existed or that the token
// cannot see.
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

// calculateDelay returns the wait before the next attempt and whether the API asked for it. A wait
// the API did not ask for is the backoff's step-th, doubled from baseDelay up to maxDelay, with jitter.
func (t *retryTransport) calculateDelay(step int, baseDelay, maxDelay time.Duration, resp *http.Response) (time.Duration, bool) {
	if resp != nil {
		if d, ok := retryAfter(resp.Header, t.clock()); ok {
			return d, true
		}
	}

	// Doubled step by step rather than shifted, which overflows time.Duration from about step 34.
	delay := baseDelay
	for i := 0; i < step && delay < maxDelay; i++ {
		delay *= 2
	}
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add 0-25% jitter
	if delay > 0 {
		jitter := time.Duration(rand.Int63n(int64(delay / 4))) //nolint:gosec // backoff jitter needs no unpredictability
		delay += jitter
	}

	return delay, false
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
	if secs, err := strconv.ParseInt(ra, 10, 64); err == nil {
		// Clamped before the multiplication, which wraps past about 292 years into any value, a
		// short wait included.
		return time.Duration(min(max(secs, 0), math.MaxInt64/int64(time.Second))) * time.Second, true
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
