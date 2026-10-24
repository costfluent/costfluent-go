package costfluent

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func retryingClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL))
	// Keep the backoff short so the tests exercise it without waiting.
	c.httpClient.Transport.(*retryTransport).baseDelay = time.Millisecond
	return c
}

// sleepRecorder stands in for the transport's backoff: it records each wait and returns at once.
type sleepRecorder struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (s *sleepRecorder) sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	s.waits = append(s.waits, d)
	s.mu.Unlock()
	return ctx.Err()
}

func (s *sleepRecorder) all() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.waits...)
}

func recordSleeps(c *Client) *sleepRecorder {
	s := &sleepRecorder{}
	c.httpClient.Transport.(*retryTransport).sleep = s.sleep
	return s
}

const quotaExceeded = `{"status":429,"title":"Too many requests","detail":"API call quota exceeded: at most 1000 per 1h. Retry after 3400 seconds.","retryable":true}`

func TestRetryResendsAPostThatWasRateLimited(t *testing.T) {
	var posts atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"id":"ws_1","name":"n","currency":"EUR"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"name":"n"`) {
			t.Errorf("POST %d body %q", posts.Load()+1, body)
		}
		if posts.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, quotaExceeded)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"ws_1"}`)
	})
	slept := recordSleeps(c)

	ws, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{Name: "n", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	if ws.ID != "ws_1" || posts.Load() != 2 {
		t.Fatalf("got %+v after %d POSTs, want ws_1 after 2", ws, posts.Load())
	}
	if got := slept.all(); len(got) != 1 || got[0] != 2*time.Second {
		t.Errorf("waited %v, want the Retry-After of 2s", got)
	}
}

// A proxy may drop Retry-After; the API's RateLimit-Reset carries the same seconds.
func TestRetryWaitsOutRateLimitResetWithoutRetryAfter(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			w.Header().Set("RateLimit-Limit", "100")
			w.Header().Set("RateLimit-Remaining", "0")
			w.Header().Set("RateLimit-Reset", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, quotaExceeded)
			return
		}
		_, _ = io.WriteString(w, `{"id":"ws_1","name":"n","currency":"EUR"}`)
	})
	slept := recordSleeps(c)

	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}
	if got := slept.all(); len(got) != 1 || got[0] != 3*time.Second {
		t.Errorf("waited %v, want the RateLimit-Reset of 3s", got)
	}
}

func TestRetryDoesNotResendARejectedPost(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"status":400,"title":"Validation failed","detail":"name is required"}`)
	})

	_, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{Currency: "EUR"})
	if !IsBadRequest(err) {
		t.Fatalf("err = %v, want the 400", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("POST sent %d times, want 1", calls.Load())
	}
}

// A Retry-After past the cap means the token's hourly quota is spent: waiting would hold the
// caller for most of an hour, so the 429 is returned at once and says when the quota resets.
func TestRateLimitBeyondTheRetryCapFailsFast(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			var calls atomic.Int32
			c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", "3400")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, quotaExceeded)
			})
			slept := recordSleeps(c)

			var err error
			if method == http.MethodGet {
				_, err = c.GetWorkspace(context.Background(), "ws_1")
			} else {
				_, err = c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{Name: "n", Currency: "EUR"})
			}
			if !IsRateLimited(err) {
				t.Fatalf("err = %v, want the 429", err)
			}
			if calls.Load() != 1 || len(slept.all()) != 0 {
				t.Fatalf("sent %d times and waited %v, want one attempt and no wait", calls.Load(), slept.all())
			}
			for _, want := range []string{"at most 1000 per 1h", "56m40s"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say %q", err, want)
				}
			}
		})
	}
}

// The timeout bounds each attempt, so a retried request is not cut off by the time its earlier
// attempts and backoff took.
func TestTimeoutAppliesToEachAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ws_1"}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL), WithTimeout(300*time.Millisecond))
	recordSleeps(c)

	ws, err := c.GetWorkspace(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("err = %v; two 200ms attempts under a 300ms timeout", err)
	}
	if ws.ID != "ws_1" || calls.Load() != 2 {
		t.Fatalf("got %+v after %d calls", ws, calls.Load())
	}
}

// An attempt that runs out its own timeout is final: the server is slow, not refusing, and
// resending would multiply the caller's wait by the retry count.
func TestAnAttemptThatTimesOutIsNotResent(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	// Long enough for the attempt to reach the server on a loaded machine, or nothing was sent to resend.
	c := NewClient(WithBaseURL(srv.URL), WithTimeout(time.Second))
	recordSleeps(c)

	_, err := c.GetWorkspace(context.Background(), "ws_1")
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("sent %d times, want 1", calls.Load())
	}
}

func TestRetryResendsIdempotentRequests(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"name":"n"}` {
			t.Errorf("attempt %d body %q", calls.Load()+1, body)
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"id":"ws_1"}`)
	})

	name := "n"
	ws, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if ws.ID != "ws_1" || calls.Load() != 3 {
		t.Fatalf("got %+v after %d calls", ws, calls.Load())
	}
}

func TestRetryDoesNotResendPost(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	})

	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	if !IsServerError(err) {
		t.Fatalf("err = %v, want a server error", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("POST sent %d times, want 1", calls.Load())
	}
}

func TestRetryReturnsLastResponseReadable(t *testing.T) {
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"status":500,"title":"Server error","detail":"database unavailable"}`)
	})

	_, err := c.GetWorkspace(context.Background(), "ws_1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an APIError", err)
	}
	if apiErr.Detail != "database unavailable" {
		t.Fatalf("detail = %q; the last attempt's body was lost", apiErr.Detail)
	}
}

func TestRetryStopsWhenContextIsCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.GetWorkspace(ctx, "ws_1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the context's deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("returned after %s; the backoff ignored the context", elapsed)
	}
}

func TestWorkspaceIsAQueryParameter(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[]}`}, stub{200, `{"links":{"self":"s"},"data":[]}`})

	if _, err := c.ListFolders(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "workspaceId", "ws_default")

	if _, err := c.ListFolders(context.Background(), "ws_explicit"); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[1], "workspaceId", "ws_explicit")
}

func TestWorkspaceScopedCallNeedsAWorkspace(t *testing.T) {
	c := NewClient(WithBaseURL("http://127.0.0.1:0"))
	if _, err := c.ListFolders(context.Background(), ""); !errors.Is(err, errNoWorkspace) {
		t.Fatalf("err = %v, want errNoWorkspace", err)
	}
}

// The API reads a JSON Content-Type as the promise of a body and answers an empty one with 400,
// so a request without a body must not claim one.
func TestContentTypeOnlyWithABody(t *testing.T) {
	var contentTypes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentTypes = append(contentTypes, r.Method+" "+r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			_, _ = io.WriteString(w, `{"id":"ws_1"}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = io.WriteString(w, `{"id":"ws_1","name":"n"}`)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL))

	if _, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{Name: "n", Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}

	want := []string{"POST application/json", "GET ", "DELETE "}
	if len(contentTypes) != len(want) {
		t.Fatalf("requests = %q, want %q", contentTypes, want)
	}
	for i := range want {
		if contentTypes[i] != want[i] {
			t.Errorf("request %d = %q, want %q", i+1, contentTypes[i], want[i])
		}
	}
}

func okResponse(r *http.Request) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}
}

func TestRetryResendsAfterATransportError(t *testing.T) {
	var calls int
	rt := &retryTransport{baseDelay: time.Microsecond, base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("connection reset by peer")
		}
		return okResponse(r), nil
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.test/v1/workspaces", nil)

	resp, err := rt.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK || calls != 3 {
		t.Fatalf("resp %v, err %v after %d calls", resp, err, calls)
	}
	_ = resp.Body.Close()
}

func TestRetryGivesUpAfterTheLastAttempt(t *testing.T) {
	var calls int
	boom := errors.New("connection refused")
	rt := &retryTransport{baseDelay: time.Microsecond, maxRetries: 2,
		base: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, boom })}
	req, _ := http.NewRequest(http.MethodDelete, "https://api.example.test/v1/workspaces/ws_1", nil)

	resp, err := rt.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the last transport error", err)
	}
	if calls != 3 {
		t.Fatalf("sent %d times, want 1 + 2 retries", calls)
	}
}

func TestRetryDoesNotResendAClientError(t *testing.T) {
	var calls int
	rt := &retryTransport{baseDelay: time.Microsecond, base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusConflict, Body: http.NoBody, Request: r}, nil
	})}
	req, _ := http.NewRequest(http.MethodPut, "https://api.example.test/v1/workspaces/ws_1", strings.NewReader(`{}`))

	resp, err := rt.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusConflict || calls != 1 {
		t.Fatalf("resp %v, err %v after %d calls", resp, err, calls)
	}
	_ = resp.Body.Close()
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("disk read failed") }

func TestRetryFailsWhenTheBodyCannotBeBuffered(t *testing.T) {
	called := false
	rt := &retryTransport{base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return okResponse(r), nil
	})}
	req, _ := http.NewRequest(http.MethodPut, "https://api.example.test/v1/workspaces/ws_1", failingReader{})

	if _, err := rt.RoundTrip(req); err == nil || !strings.Contains(err.Error(), "reading request body") {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Error("sent a request whose body could not be read")
	}
}

func TestRetryDelay(t *testing.T) {
	now := time.Date(2026, 10, 21, 7, 28, 0, 0, time.UTC)
	rt := &retryTransport{now: func() time.Time { return now }}
	withRetryAfter := func(v string) *http.Response {
		return &http.Response{Header: http.Header{"Retry-After": {v}}}
	}
	within := func(t *testing.T, got, lo time.Duration) {
		t.Helper()
		if hi := lo + lo/4; got < lo || got >= hi {
			t.Errorf("delay %s, want [%s, %s)", got, lo, hi)
		}
	}

	if got := rt.calculateDelay(0, time.Second, 30*time.Second, withRetryAfter("7")); got != 7*time.Second {
		t.Errorf("Retry-After 7 gave %s", got)
	}
	if got := rt.calculateDelay(3, time.Second, 30*time.Second, withRetryAfter("0")); got != 0 {
		t.Errorf("Retry-After 0 gave %s", got)
	}
	if got := rt.calculateDelay(1, time.Second, 30*time.Second, withRetryAfter("Wed, 21 Oct 2026 07:28:09 GMT")); got != 9*time.Second {
		t.Errorf("Retry-After as a date 9s ahead gave %s", got)
	}
	if got := rt.calculateDelay(1, time.Second, 30*time.Second, withRetryAfter("Wed, 21 Oct 2026 07:27:00 GMT")); got != 0 {
		t.Errorf("Retry-After as a past date gave %s", got)
	}
	// A value that is neither seconds nor a date is ignored, so the backoff applies.
	within(t, rt.calculateDelay(1, time.Second, 30*time.Second, withRetryAfter("soon")), 2*time.Second)
	within(t, rt.calculateDelay(0, time.Second, 30*time.Second, nil), time.Second)
	within(t, rt.calculateDelay(2, time.Second, 30*time.Second, nil), 4*time.Second)
	within(t, rt.calculateDelay(10, time.Second, 30*time.Second, nil), 30*time.Second)
}

func TestUnreachableServerIsNotAnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	c := NewClient(WithBaseURL(url))

	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	if err == nil || !strings.Contains(err.Error(), "executing request") {
		t.Fatalf("err = %v", err)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) || StatusCode(err) != 0 {
		t.Errorf("a connection failure reads as an API error: %v", err)
	}
}

func TestInvalidBaseURLFailsBeforeSending(t *testing.T) {
	c := NewClient(WithBaseURL("http://bad host"))
	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err == nil || !strings.Contains(err.Error(), "is not a URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnencodableBodyFailsBeforeSending(t *testing.T) {
	c := NewClient()
	if _, err := c.newRequest(context.Background(), http.MethodPost, "/v1/workspaces", make(chan int)); err == nil ||
		!strings.Contains(err.Error(), "marshaling request body") {
		t.Fatalf("err = %v", err)
	}
}

// The API applied the first conditional PUT and its response was lost: a resend carries the same
// If-Match, now stale, and its 412 would report the caller's own update as a change made by
// someone else.
func TestRetryDoesNotResendAConditionalUpdateAfterAServerError(t *testing.T) {
	for _, status := range []int{
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var puts atomic.Int32
			c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
				if puts.Add(1) == 1 {
					w.WriteHeader(status)
					return
				}
				if r.Header.Get("If-Match") == `"v1"` {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusPreconditionFailed)
					_, _ = io.WriteString(w, `{"status":412,"title":"Precondition failed"}`)
					return
				}
				t.Errorf("PUT %d carried If-Match %q", puts.Load(), r.Header.Get("If-Match"))
			})

			name := "n"
			_, err := c.UpdateBudget(context.Background(), "bud_1", &UpdateBudgetInput{Name: &name}, IfMatch(`"v1"`))
			if IsPreconditionFailed(err) {
				t.Fatalf("err = %v: the resent PUT's stale If-Match was refused", err)
			}
			if !IsServerError(err) {
				t.Fatalf("err = %v, want the %d", err, status)
			}
			if puts.Load() != 1 {
				t.Fatalf("PUT sent %d times, want 1", puts.Load())
			}
		})
	}
}

type retryFailure struct {
	name string
	fail func(*http.Request) (*http.Response, error)
}

func retryFailures() []retryFailure {
	status := func(code int) func(*http.Request) (*http.Response, error) {
		return func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: http.NoBody, Request: r}, nil
		}
	}
	return []retryFailure{
		{"502", status(http.StatusBadGateway)},
		{"503", status(http.StatusServiceUnavailable)},
		{"connection reset", func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection reset by peer")
		}},
	}
}

// sendFailingOnce sends req through a transport whose first attempt fails and whose next succeeds,
// and reports how many attempts reached it.
func sendFailingOnce(t *testing.T, req *http.Request, fail func(*http.Request) (*http.Response, error)) int {
	t.Helper()
	var calls int
	rt := &retryTransport{baseDelay: time.Microsecond, base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return fail(r)
		}
		return okResponse(r), nil
	})}
	resp, _ := rt.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	return calls
}

func TestRetryDoesNotResendAPreconditionRequest(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, f := range retryFailures() {
			t.Run(method+" "+f.name, func(t *testing.T) {
				req, _ := http.NewRequest(method, "https://api.example.test/v1/budgets/bud_1", strings.NewReader(`{}`))
				req.Header.Set("If-Match", `"v1"`)

				if calls := sendFailingOnce(t, req, f.fail); calls != 1 {
					t.Fatalf("sent %d times, want 1", calls)
				}
			})
		}
	}
}

func TestRetryStillResendsUnconditionalIdempotentRequests(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		for _, f := range retryFailures() {
			t.Run(method+" "+f.name, func(t *testing.T) {
				req, _ := http.NewRequest(method, "https://api.example.test/v1/budgets/bud_1", strings.NewReader(`{}`))

				if calls := sendFailingOnce(t, req, f.fail); calls != 2 {
					t.Fatalf("sent %d times, want a resend", calls)
				}
			})
		}
	}
}

// The rate limiter refuses a request before any endpoint runs it, so a conditional one is resent.
func TestRetryResendsARateLimitedPreconditionRequest(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req, _ := http.NewRequest(method, "https://api.example.test/v1/budgets/bud_1", strings.NewReader(`{}`))
			req.Header.Set("If-Match", `"v1"`)
			limited := func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusTooManyRequests, Body: http.NoBody, Request: r}, nil
			}

			if calls := sendFailingOnce(t, req, limited); calls != 2 {
				t.Fatalf("sent %d times, want a resend", calls)
			}
		})
	}
}

func TestARequestThatCannotBeBuiltSendsNothing(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) })

	_, err := c.newRequest(context.Background(), "NOT A METHOD", "/v1/me", nil)
	if err == nil || !strings.Contains(err.Error(), "creating request") {
		t.Fatalf("err = %v, want a refusal to build the request", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("sent %d times, want 0", calls.Load())
	}
}

// A container being recreated leaves the edge without a route, which it answers with a plain-text
// 404 until the new one is up: the read that follows succeeds and nothing is reported gone.
func TestRetryOutlastsTheEdgeAnsweringWithoutABackend(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, "404 page not found\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ws_1","name":"n","currency":"EUR"}`)
	})

	ws, err := c.GetWorkspace(context.Background(), "ws_1")
	if err != nil || ws.ID != "ws_1" || calls.Load() != 3 {
		t.Fatalf("got %+v, %v after %d requests, want the workspace on the third", ws, err, calls.Load())
	}
}

// A trigger the edge turned away may still be resent by nobody but its caller, as after a 5xx: the
// 404 does not prove the request never reached an endpoint.
func TestEdgeNotFoundDoesNotResendATrigger(t *testing.T) {
	var calls atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "404 page not found\n")
	})

	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	if IsNotFound(err) || StatusCode(err) != http.StatusNotFound || !IsRetryable(err) {
		t.Fatalf("err = %v, want a retryable 404 that is not NotFound", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("trigger sent %d times, want 1", calls.Load())
	}
}
