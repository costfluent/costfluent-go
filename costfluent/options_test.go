package costfluent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func headerServer(t *testing.T) (string, *[]http.Header) {
	t.Helper()
	var got []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Clone())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ws_1","name":"Production","currency":"EUR"}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &got
}

func TestRequestCarriesAuthAndUserAgent(t *testing.T) {
	url, got := headerServer(t)
	c := NewClient(WithBaseURL(url), WithAPIKey("cf_test_key"))

	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}
	h := (*got)[0]
	if h.Get("Authorization") != "Bearer cf_test_key" {
		t.Errorf("Authorization = %q", h.Get("Authorization"))
	}
	if h.Get("User-Agent") != "costfluent-go" {
		t.Errorf("User-Agent = %q", h.Get("User-Agent"))
	}
	if h.Get("Accept") != "application/json" {
		t.Errorf("Accept = %q", h.Get("Accept"))
	}
}

func TestWithoutAnAPIKeyNoAuthorizationIsSent(t *testing.T) {
	url, got := headerServer(t)
	c := NewClient(WithBaseURL(url))

	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}
	if v, ok := (*got)[0]["Authorization"]; ok {
		t.Errorf("sent Authorization %q without a key", v)
	}
}

func TestUserAgentNamesTheReleaseABuildDependsOn(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"a release as a dependency", &debug.BuildInfo{
			Main: debug.Module{Path: "example.com/app", Version: "(devel)"},
			Deps: []*debug.Module{{Path: "example.com/other", Version: "v2.0.0"}, {Path: modulePath, Version: "v1.4.2"}},
		}, true, "costfluent-go/1.4.2"},
		{"the module itself", &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: "(devel)"}}, true, "costfluent-go"},
		{"a copy under another module", &debug.BuildInfo{Main: debug.Module{Path: "example.com/provider", Version: "v0.3.0"}}, true, "costfluent-go"},
		{"no build information", nil, false, "costfluent-go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := userAgent(tc.info, tc.ok); got != tc.want {
				t.Errorf("userAgent = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWithUserAgentReplacesTheDefault(t *testing.T) {
	url, got := headerServer(t)
	c := NewClient(WithBaseURL(url), WithUserAgent("terraform-provider-costfluent/0.1.0"))

	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}
	if ua := (*got)[0].Get("User-Agent"); ua != "terraform-provider-costfluent/0.1.0" {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestWithHTTPClientSendsThroughItsTransport(t *testing.T) {
	var seen []*http.Request
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       http.NoBody,
			Request:    r,
		}, nil
	})}
	c := NewClient(WithHTTPClient(hc), WithAPIKey("key"))

	if err := c.DeleteWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 {
		t.Fatalf("custom transport saw %d requests, want 1", len(seen))
	}
	r := seen[0]
	if r.URL.String() != DefaultBaseURL+"/v1/workspaces/ws_1" || r.Method != http.MethodDelete {
		t.Errorf("request %s %s", r.Method, r.URL)
	}
	if r.Header.Get("Authorization") != "Bearer key" {
		t.Errorf("auth was not layered over the custom transport: %q", r.Header.Get("Authorization"))
	}
}

func TestNewClientLeavesTheCallersHTTPClientAlone(t *testing.T) {
	otherURL, other := headerServer(t)
	apiURL, _ := headerServer(t)
	hc := &http.Client{Timeout: 7 * time.Second}

	_ = NewClient(WithHTTPClient(hc), WithBaseURL(apiURL), WithAPIKey("cf_secret_key"))

	if hc.Transport != nil || hc.Timeout != 7*time.Second {
		t.Errorf("caller's client changed: transport %T, timeout %s", hc.Transport, hc.Timeout)
	}
	resp, err := hc.Get(otherURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if v, ok := (*other)[0]["Authorization"]; ok {
		t.Errorf("the caller's own request to another host carried Authorization %q", v)
	}
}

func TestClientsSharingAnHTTPClientSendTheirOwnKeys(t *testing.T) {
	url, got := headerServer(t)
	hc := &http.Client{}
	first := NewClient(WithHTTPClient(hc), WithBaseURL(url), WithAPIKey("key_first"))
	second := NewClient(WithHTTPClient(hc), WithBaseURL(url), WithAPIKey("key_second"))

	for _, c := range []*Client{first, second, first} {
		if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range []string{"Bearer key_first", "Bearer key_second", "Bearer key_first"} {
		if h := (*got)[i].Get("Authorization"); h != want {
			t.Errorf("request %d Authorization = %q, want %q", i+1, h, want)
		}
	}
}

func TestClientsSharingAnHTTPClientRetryOnce(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	hc := &http.Client{}
	_ = NewClient(WithHTTPClient(hc), WithBaseURL(srv.URL))
	second := NewClient(WithHTTPClient(hc), WithBaseURL(srv.URL))

	if _, err := second.GetWorkspace(context.Background(), "ws_1"); StatusCode(err) != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want the 503", err)
	}
	if n := calls.Load(); n != 4 {
		t.Fatalf("sent %d requests, want 1 + 3 retries", n)
	}
}

func TestWithTimeoutAppliesInEitherOrderWithoutChangingTheCallersClient(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	orders := map[string]func(*http.Client) []Option{
		"timeout first": func(hc *http.Client) []Option {
			return []Option{WithTimeout(50 * time.Millisecond), WithHTTPClient(hc)}
		},
		"timeout last": func(hc *http.Client) []Option {
			return []Option{WithHTTPClient(hc), WithTimeout(50 * time.Millisecond)}
		},
	}
	for name, options := range orders {
		t.Run(name, func(t *testing.T) {
			hc := &http.Client{Timeout: time.Minute}
			c := NewClient(append(options(hc), WithBaseURL(srv.URL))...)
			if got := attemptTimeout(t, c); got != 50*time.Millisecond {
				t.Fatalf("attempt timeout = %s, want 50ms", got)
			}

			_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				t.Fatalf("err = %v, want a timeout", err)
			}
			if hc.Timeout != time.Minute {
				t.Errorf("caller's client timeout changed to %s", hc.Timeout)
			}
		})
	}
}

// attemptTimeout is the timeout each attempt runs under. The http.Client carries none, because it
// would bound the whole retry loop.
func attemptTimeout(t *testing.T, c *Client) time.Duration {
	t.Helper()
	if c.httpClient.Timeout != 0 {
		t.Fatalf("http.Client timeout = %s, want none", c.httpClient.Timeout)
	}
	return c.httpClient.Transport.(*retryTransport).attemptTimeout
}

func TestWithHTTPClientKeepsItsOwnTimeout(t *testing.T) {
	hc := &http.Client{Timeout: 7 * time.Second}
	if got := attemptTimeout(t, NewClient(WithHTTPClient(hc))); got != 7*time.Second {
		t.Fatalf("timeout = %s, want the caller's 7s", got)
	}
}

// hostRoutedClient dials each host name to the test server serving it, so a redirect can cross to
// another host while every server listens on 127.0.0.1.
func hostRoutedClient(hosts map[string]*httptest.Server) *http.Client {
	var d net.Dialer
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			srv, ok := hosts[host]
			if !ok {
				return nil, fmt.Errorf("no test server for %s", host)
			}
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		},
	}}
}

func TestRedirectKeepsTheKeyOnTheAPIHostOnly(t *testing.T) {
	var mu sync.Mutex
	auth := map[string][]string{}
	record := func(server string, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		auth[server] = append(auth[server], r.Header.Get("Authorization"))
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("other", r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ws_1"}`))
	}))
	t.Cleanup(other.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("api", r)
		switch r.URL.Path {
		case "/v1/workspaces/ws_cross":
			http.Redirect(w, r, "http://files.example.test/export", http.StatusFound)
		case "/v1/workspaces/ws_same":
			http.Redirect(w, r, "/v1/workspaces/ws_1", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"ws_1"}`))
		}
	}))
	t.Cleanup(api.Close)
	// The API host is loopback because the client accepts plain http nowhere else.
	hc := hostRoutedClient(map[string]*httptest.Server{"localhost": api, "files.example.test": other})
	c := NewClient(WithHTTPClient(hc), WithBaseURL("http://localhost"), WithAPIKey("cf_secret_key"))

	for _, id := range []string{"ws_cross", "ws_same"} {
		if _, err := c.GetWorkspace(context.Background(), id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if got := auth["other"]; len(got) != 1 || got[0] != "" {
		t.Errorf("the other host received Authorization %q, want none", got)
	}
	want := []string{"Bearer cf_secret_key", "Bearer cf_secret_key", "Bearer cf_secret_key"}
	if got := auth["api"]; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the API host received Authorization %q, want %q", got, want)
	}
}

func TestEveryRetryCarriesTheKey(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Authorization"))
		n := len(seen)
		mu.Unlock()
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ws_1"}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("cf_secret_key"))

	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := "[Bearer cf_secret_key Bearer cf_secret_key Bearer cf_secret_key]"; fmt.Sprint(seen) != want {
		t.Errorf("attempts carried %q, want %s", seen, want)
	}
}

func TestWithTimeoutBoundsTheRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	c := NewClient(WithBaseURL(srv.URL), WithTimeout(50*time.Millisecond))

	// A POST is not retried, so the one timeout is what the caller sees.
	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if StatusCode(err) != 0 {
		t.Errorf("a timeout carries status %d", StatusCode(err))
	}
}

func TestDefaultTimeoutIsBounded(t *testing.T) {
	if got := attemptTimeout(t, NewClient()); got != DefaultTimeout {
		t.Fatalf("timeout = %s, want %s", got, DefaultTimeout)
	}
}

func TestBaseURL(t *testing.T) {
	if got := NewClient().BaseURL(); got != DefaultBaseURL {
		t.Errorf("default base URL %q", got)
	}
	if got := NewClient(WithBaseURL("https://api.example.test")).BaseURL(); got != "https://api.example.test" {
		t.Errorf("base URL %q", got)
	}
}

func TestWorkspaceReturnsAnIndependentClient(t *testing.T) {
	base := NewClient(WithWorkspace("ws_a"))
	scoped := base.Workspace("ws_b")
	if base.workspaceID != "ws_a" || scoped.workspaceID != "ws_b" {
		t.Fatalf("base %q, scoped %q", base.workspaceID, scoped.workspaceID)
	}
}

func TestWithDebugLogsTheExchangeWithoutTheKey(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })

	url, _ := headerServer(t)
	c := NewClient(WithBaseURL(url), WithAPIKey("cf_secret_key"), WithDebug(true))
	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{"[costfluent] -> GET " + url + "/v1/workspaces/ws_1", "[costfluent] <- 200"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cf_secret_key") {
		t.Errorf("log leaked the API key:\n%s", out)
	}
}

func TestDebugTransportRedactsAuthorizationAndLogsErrors(t *testing.T) {
	var lines []string
	logf := func(format string, args ...any) {
		var b strings.Builder
		log.New(&b, "", 0).Printf(format, args...)
		lines = append(lines, strings.TrimSpace(b.String()))
	}
	boom := errors.New("connection reset")
	dt := &debugTransport{
		base:   roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, boom }),
		logger: logf,
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.test/v1/workspaces", nil)
	req.Header.Set("Authorization", "Bearer cf_secret_key")

	resp, err := dt.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if resp != nil || !errors.Is(err, boom) {
		t.Fatalf("resp %v, err %v", resp, err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Authorization: [REDACTED]") || strings.Contains(joined, "cf_secret_key") {
		t.Errorf("authorization not redacted:\n%s", joined)
	}
	if !strings.Contains(joined, "<- ERROR: connection reset") {
		t.Errorf("error not logged:\n%s", joined)
	}
}

func TestWithBaseURLRefusesPlainHTTPOutsideLoopbackBeforeSendingTheKey(t *testing.T) {
	var sent atomic.Int32
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		sent.Add(1)
		return nil, errors.New("must not be reached")
	})}

	for _, base := range []string{"http://api.costfluent.com", "http://10.0.0.5:8080", "ftp://api.costfluent.com", "api.costfluent.com", "://broken"} {
		c := NewClient(WithBaseURL(base), WithAPIKey("cf_org_secret"), WithHTTPClient(hc))
		_, err := c.GetWorkspace(context.Background(), "ws_1")
		if err == nil || !strings.Contains(err.Error(), "base URL") {
			t.Errorf("%s: err = %v, want a base URL refusal", base, err)
		}
		if strings.Contains(fmt.Sprint(err), "cf_org_secret") {
			t.Errorf("%s: the refusal names the key: %v", base, err)
		}
	}
	if n := sent.Load(); n != 0 {
		t.Fatalf("%d requests left the client; a refused base URL must send nothing", n)
	}
}

func TestWithBaseURLAcceptsHTTPSAndLoopbackHTTP(t *testing.T) {
	for _, base := range []string{"https://api.costfluent.com", "http://localhost:5200", "http://127.0.0.1:5200", "http://[::1]:5200"} {
		if err := checkBaseURL(base); err != nil {
			t.Errorf("%s: %v", base, err)
		}
	}
}

// rateLimitedOnce answers the first request 429 with retryAfter and every later one 200.
func rateLimitedOnce(t *testing.T, retryAfter string, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", retryAfter)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"status":429,"title":"Too many requests","retryable":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"ws_1","name":"Production"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWithMaxRetryAfterZeroReturnsARateLimitAtOnceWithItsWait(t *testing.T) {
	var calls atomic.Int32
	srv := rateLimitedOnce(t, "2", &calls)
	c := NewClient(WithBaseURL(srv.URL), WithMaxRetryAfter(0))
	slept := recordSleeps(c)

	_, err := c.GetWorkspace(context.Background(), "ws_1")
	var ae *APIError
	if !errors.As(err, &ae) || !IsRateLimited(err) {
		t.Fatalf("err = %v, want the 429", err)
	}
	if calls.Load() != 1 || len(slept.all()) != 0 {
		t.Errorf("sent %d times and waited %v, want one attempt and no wait", calls.Load(), slept.all())
	}
	if ae.RetryAfter() != 2*time.Second {
		t.Errorf("RetryAfter() = %s, want the 2s the API asked for", ae.RetryAfter())
	}
}

func TestWithMaxRetryAfterZeroStillRetriesAServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"status":503,"title":"Unavailable","retryable":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"ws_1","name":"Production"}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL), WithMaxRetryAfter(0))
	recordSleeps(c)

	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil || calls.Load() != 2 {
		t.Fatalf("err %v after %d attempts, want the backoff retry to succeed on the second", err, calls.Load())
	}
}

func TestWithMaxRetryAfterWaitsOutOnlyWhatItAllows(t *testing.T) {
	var calls atomic.Int32
	srv := rateLimitedOnce(t, "2", &calls)
	c := NewClient(WithBaseURL(srv.URL), WithMaxRetryAfter(5*time.Second))
	slept := recordSleeps(c)
	if _, err := c.GetWorkspace(context.Background(), "ws_1"); err != nil {
		t.Fatalf("a 2s wait under a 5s bound: %v", err)
	}
	if waits := slept.all(); len(waits) != 1 || waits[0] != 2*time.Second {
		t.Errorf("waited %v, want the 2s asked for", waits)
	}

	var longer atomic.Int32
	srv = rateLimitedOnce(t, "10", &longer)
	c = NewClient(WithBaseURL(srv.URL), WithMaxRetryAfter(5*time.Second))
	slept = recordSleeps(c)
	if _, err := c.GetWorkspace(context.Background(), "ws_1"); !IsRateLimited(err) || longer.Load() != 1 || len(slept.all()) != 0 {
		t.Errorf("a 10s wait over a 5s bound: err %v after %d attempts, waited %v; want the 429 at once", err, longer.Load(), slept.all())
	}
}
