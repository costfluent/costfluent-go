package costfluent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// These sweeps drive every operation in contractOps through the failure paths a caller relies on,
// so a new method that forgets to return an error, or decodes past one, fails here.

func sortedOps() []string {
	names := make([]string, 0, len(contractOps))
	for name := range contractOps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// callWith invokes a client method passing str for every string argument, a zero value for every
// input struct and no request options.
func callWith(ctx context.Context, m reflect.Value, str string) (out []reflect.Value, err error) {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	n := m.Type().NumIn()
	if m.Type().IsVariadic() {
		n--
	}
	args := make([]reflect.Value, n)
	for i := range args {
		in := m.Type().In(i)
		switch {
		case in == ctxType:
			args[i] = reflect.ValueOf(ctx)
		case in.Kind() == reflect.String:
			args[i] = reflect.ValueOf(str).Convert(in)
		case in.Kind() == reflect.Pointer && in.Elem().Kind() == reflect.Struct:
			args[i] = reflect.New(in.Elem())
		default:
			args[i] = reflect.Zero(in)
		}
	}
	out = m.Call(args)
	err, _ = out[len(out)-1].Interface().(error)
	return out, err
}

// countingServer answers every request with status and body, and counts what it received.
func countingServer(t *testing.T, status int, body string) (string, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if status >= 400 {
			w.Header().Set("Content-Type", "application/problem+json")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &calls
}

func method(t *testing.T, c *Client, name string) reflect.Value {
	t.Helper()
	m := reflect.ValueOf(c).MethodByName(name)
	if !m.IsValid() {
		t.Fatalf("client has no method %s", name)
	}
	return m
}

func TestEveryOperationReturnsTheAPIError(t *testing.T) {
	const problem = `{"title":"Not Found","status":404,"code":"not_found","retryable":false,
		"detail":"The resource does not exist.","traceId":"00-trace-01"}`

	for _, name := range sortedOps() {
		t.Run(name, func(t *testing.T) {
			url, calls := countingServer(t, http.StatusNotFound, problem)
			c := NewClient(WithBaseURL(url), WithAPIKey("key"), WithWorkspace("ws_default"))

			out, err := callWith(context.Background(), method(t, c, name), "id-1")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want an APIError", err)
			}
			if !IsNotFound(err) || apiErr.Code != "not_found" || apiErr.TraceID != "00-trace-01" {
				t.Errorf("decoded %+v", apiErr)
			}
			if len(out) == 2 && !out[0].IsZero() {
				t.Errorf("returned %v alongside the error", out[0])
			}
			if calls.Load() != 1 {
				t.Errorf("sent %d requests, want 1: a 404 is final", calls.Load())
			}
		})
	}
}

func TestEveryOperationRejectsAMalformedResponse(t *testing.T) {
	for _, name := range sortedOps() {
		t.Run(name, func(t *testing.T) {
			url, _ := countingServer(t, http.StatusOK, `{"id":`)
			c := NewClient(WithBaseURL(url), WithAPIKey("key"), WithWorkspace("ws_default"))
			m := method(t, c, name)
			if m.Type().NumOut() != 2 {
				return // returns only an error and decodes no body
			}

			out, err := callWith(context.Background(), m, "id-1")
			if err == nil || !strings.Contains(err.Error(), "decoding response") {
				t.Fatalf("err = %v, want a decoding error", err)
			}
			if !out[0].IsZero() {
				t.Errorf("returned %v alongside the error", out[0])
			}
		})
	}
}

// workspaceScoped reports whether the spec operation requires a workspaceId, in its query or its
// body.
func workspaceScoped(s *spec, op *specOperation) bool {
	for _, p := range op.Parameters {
		if p.In == "query" && p.Name == "workspaceId" && p.Required {
			return true
		}
	}
	if op.RequestBody == nil {
		return false
	}
	body := s.resolve(jsonSchemaOf(op.RequestBody.Content))
	if body == nil {
		return false
	}
	for _, key := range body.Required {
		if key == "workspaceId" {
			return true
		}
	}
	return false
}

func TestWorkspaceScopedOperationsNeedAWorkspace(t *testing.T) {
	s := loadSpec(t)
	for _, name := range sortedOps() {
		want := contractOps[name]
		op := s.Paths[want.Path][strings.ToLower(want.Method)]
		if op == nil {
			t.Fatalf("%s: %s %s is not in the spec", name, want.Method, want.Path)
		}
		scoped := workspaceScoped(s, op)

		t.Run(name, func(t *testing.T) {
			url, calls := countingServer(t, http.StatusNotFound, `{"title":"Not Found","status":404}`)
			c := NewClient(WithBaseURL(url), WithAPIKey("key"))

			_, err := callWith(context.Background(), method(t, c, name), "")
			if scoped {
				if !errors.Is(err, errNoWorkspace) {
					t.Fatalf("err = %v, want errNoWorkspace", err)
				}
				if calls.Load() != 0 {
					t.Errorf("sent %d requests before finding no workspace", calls.Load())
				}
				return
			}
			if errors.Is(err, errNoWorkspace) {
				t.Fatalf("%s %s takes no workspace, but the client asked for one", want.Method, want.Path)
			}
		})
	}
}

// workspaceSent returns the workspaceId a request carried, in its query or its JSON body.
func workspaceSent(r *http.Request) string {
	if id := r.URL.Query().Get("workspaceId"); id != "" {
		return id
	}
	var body struct {
		WorkspaceID string `json:"workspaceId"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	return body.WorkspaceID
}

func TestWorkspaceScopedOperationsFallBackToTheDefaultWorkspace(t *testing.T) {
	s := loadSpec(t)
	for _, name := range sortedOps() {
		want := contractOps[name]
		if !workspaceScoped(s, s.Paths[want.Path][strings.ToLower(want.Method)]) {
			continue
		}

		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			var sent []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				sent = append(sent, workspaceSent(r))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"title":"Not Found","status":404}`))
			}))
			t.Cleanup(srv.Close)
			c := NewClient(WithBaseURL(srv.URL), WithAPIKey("key"), WithWorkspace("ws_default"))

			_, _ = callWith(context.Background(), method(t, c, name), "")
			mu.Lock()
			defer mu.Unlock()
			if len(sent) != 1 || sent[0] != "ws_default" {
				t.Fatalf("sent workspaces %q, want the default ws_default once", sent)
			}
		})
	}
}

func TestEveryOperationHonoursACancelledContext(t *testing.T) {
	for _, name := range sortedOps() {
		t.Run(name, func(t *testing.T) {
			url, calls := countingServer(t, http.StatusOK, `{"id":"id-1"}`)
			c := NewClient(WithBaseURL(url), WithAPIKey("key"), WithWorkspace("ws_default"))
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := callWith(ctx, method(t, c, name), "id-1")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if calls.Load() != 0 {
				t.Errorf("sent %d requests on a cancelled context", calls.Load())
			}
		})
	}
}

func TestEveryOperationFailsBeforeSendingWhenTheRequestCannotBeBuilt(t *testing.T) {
	for _, name := range sortedOps() {
		t.Run(name, func(t *testing.T) {
			c := NewClient(WithBaseURL("http://bad host"), WithAPIKey("key"), WithWorkspace("ws_default"))

			out, err := callWith(context.Background(), method(t, c, name), "id-1")
			if err == nil || !strings.Contains(err.Error(), "base URL") {
				t.Fatalf("err = %v, want the request construction failure", err)
			}
			if len(out) == 2 && !out[0].IsZero() {
				t.Errorf("returned %v alongside the error", out[0])
			}
		})
	}
}
