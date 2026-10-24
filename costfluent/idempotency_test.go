package costfluent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// takesIdempotencyKey reports whether the contract documents an Idempotency-Key on op.
func takesIdempotencyKey(op *specOperation) bool {
	for _, p := range op.Parameters {
		if p.In == "header" && p.Name == idempotencyKeyHeader {
			return true
		}
	}
	return false
}

// Every operation the contract documents with an Idempotency-Key sends a new key on each call, and
// no other operation sends one.
func TestCreatesSendANewIdempotencyKeyOnEveryCall(t *testing.T) {
	s := loadSpec(t)
	keyedOps := 0
	for _, name := range sortedOps() {
		want := contractOps[name]
		keyed := takesIdempotencyKey(s.Paths[want.Path][strings.ToLower(want.Method)])
		if keyed {
			keyedOps++
		}

		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			var sent []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				sent = append(sent, r.Header.Get(idempotencyKeyHeader))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"title":"Not Found","status":404}`)
			}))
			t.Cleanup(srv.Close)
			c := NewClient(WithBaseURL(srv.URL), WithAPIKey("key"), WithWorkspace("ws_default"))

			_, _ = callWith(context.Background(), method(t, c, name), "id-1")
			_, _ = callWith(context.Background(), method(t, c, name), "id-1")

			mu.Lock()
			defer mu.Unlock()
			if !keyed {
				for _, key := range sent {
					if key != "" {
						t.Fatalf("sent Idempotency-Key %q on an operation the contract gives none", key)
					}
				}
				return
			}
			if len(sent) != 2 || len(sent[0]) != 32 || sent[0] == sent[1] {
				t.Fatalf("sent keys %q, want a new 32-character key on each of two calls", sent)
			}
		})
	}
	if keyedOps < 18 {
		t.Fatalf("%d operations take an Idempotency-Key, want every create", keyedOps)
	}
}

// A create whose answer is lost to a 5xx is resent under the same key, which the API answers with
// the first response instead of creating again.
func TestCreateRetriesUnderTheSameIdempotencyKey(t *testing.T) {
	var posts atomic.Int32
	var mu sync.Mutex
	var keys []string
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"id":"fld_1","title":"Platform"}`)
			return
		}
		mu.Lock()
		keys = append(keys, r.Header.Get(idempotencyKeyHeader))
		mu.Unlock()
		if posts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"fld_1","title":"Platform"}`)
	})

	folder, err := c.CreateFolder(context.Background(), &CreateFolderInput{WorkspaceID: "ws_1", Title: "Platform"})
	if err != nil || folder.ID != "fld_1" {
		t.Fatalf("folder %+v, err %v", folder, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keys) != 3 || keys[0] == "" || keys[1] != keys[0] || keys[2] != keys[0] {
		t.Fatalf("sent keys %q, want one key on all three attempts", keys)
	}
}

func TestIsIdempotencyKeyReused(t *testing.T) {
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"status":422,"code":"idempotency_key_reused","detail":"used with another request"}`)
	})

	_, err := c.CreateTeam(context.Background(), &CreateTeamInput{Name: "Platform"})
	if !IsIdempotencyKeyReused(err) {
		t.Fatalf("err = %v, want idempotency_key_reused", err)
	}
	if IsIdempotencyKeyReused(nil) {
		t.Fatal("a nil error is not a reused key")
	}
}
