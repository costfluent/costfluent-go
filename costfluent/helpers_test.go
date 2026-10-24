package costfluent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// exchange is one request a test server received and the answer it gave.
type exchange struct {
	Method, Path string
	Query        map[string][]string
	Body         map[string]any
}

type stub struct {
	Status int
	Body   string
}

// testClient serves each request with the next stub in order and records what it received.
func testClient(t *testing.T, stubs ...stub) (*Client, *[]exchange) {
	t.Helper()
	var mu sync.Mutex
	var got []exchange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		ex := exchange{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &ex.Body); err != nil {
				t.Errorf("request body is not a JSON object: %s", raw)
			}
		}
		mu.Lock()
		n := len(got)
		got = append(got, ex)
		mu.Unlock()
		if n >= len(stubs) {
			t.Errorf("unexpected request %d: %s %s", n+1, r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s := stubs[n]
		if s.Status >= 400 {
			w.Header().Set("Content-Type", "application/problem+json")
		} else if s.Body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(s.Status)
		_, _ = io.WriteString(w, s.Body)
	}))
	t.Cleanup(srv.Close)
	return NewClient(WithBaseURL(srv.URL), WithAPIKey("key"), WithWorkspace("ws_default")), &got
}

func wantRequest(t *testing.T, got exchange, method, path string) {
	t.Helper()
	if got.Method != method || got.Path != path {
		t.Fatalf("request %s %s, want %s %s", got.Method, got.Path, method, path)
	}
}

func wantQuery(t *testing.T, got exchange, key, value string) {
	t.Helper()
	if v := got.Query[key]; len(v) != 1 || v[0] != value {
		t.Errorf("query %s = %v, want %q", key, v, value)
	}
}

func wantBody(t *testing.T, got exchange, key string, value any) {
	t.Helper()
	if v, ok := got.Body[key]; !ok || v != value {
		t.Errorf("body %s = %v (present %v), want %v", key, v, ok, value)
	}
}
