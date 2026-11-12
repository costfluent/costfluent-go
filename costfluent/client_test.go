package costfluent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestResolveWorkspacePrefersTheExplicitID(t *testing.T) {
	cases := []struct {
		name     string
		client   *Client
		explicit string
		want     string
	}{
		{"explicit over default", NewClient(WithWorkspace("ws_default")), "ws_explicit", "ws_explicit"},
		{"default when empty", NewClient(WithWorkspace("ws_default")), "", "ws_default"},
		{"explicit without default", NewClient(), "ws_explicit", "ws_explicit"},
		{"neither", NewClient(), "", ""},
		{"scoped clone's default", NewClient(WithWorkspace("ws_a")).Workspace("ws_b"), "", "ws_b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.client.ResolveWorkspace(tc.explicit); got != tc.want {
				t.Errorf("ResolveWorkspace(%q) = %q, want %q", tc.explicit, got, tc.want)
			}
		})
	}
}

// agentServer answers every request with an empty object, or 204 for one sending no body the
// answer needs, and records each request's method, path and AgentHeader.
func agentServer(t *testing.T) (string, func() []agentRequest) {
	t.Helper()
	var mu sync.Mutex
	var got []agentRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, agentRequest{r.Method, r.URL.Path, r.Header.Get(AgentHeader), r.Header.Values(AgentHeader) != nil})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"obj_1"}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []agentRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]agentRequest(nil), got...)
	}
}

type agentRequest struct {
	method, path, agent string
	marked              bool
}

// agentCalls sends a read, a create, an update, an action and a delete, each built a different way.
func agentCalls(t *testing.T, c *Client) {
	t.Helper()
	ctx := context.Background()
	if _, err := c.GetMe(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListBudgets(ctx, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateAnnotation(ctx, &CreateAnnotationInput{Date: "2026-11-01", Text: "deploy", CostReportIDs: []string{"rpt_1"}}); err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	if _, err := c.UpdateBudget(ctx, "bud_1", &UpdateBudgetInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if err := c.AcknowledgeAnomaly(ctx, "anm_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.PauseCostAlert(ctx, "", "cal_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteBudget(ctx, "bud_1"); err != nil {
		t.Fatal(err)
	}
}

func TestAgentMarksEveryRequestOfItsCopyAndNoneOfTheParents(t *testing.T) {
	url, requests := agentServer(t)
	parent := NewClient(WithBaseURL(url), WithAPIKey("key"), WithWorkspace("ws_default"), WithUserAgent("costfluent-mcp/1.4.0"))
	agent := parent.Agent("costfluent-mcp")

	agentCalls(t, agent)
	marked := requests()
	if len(marked) != 7 {
		t.Fatalf("%d requests, want 7", len(marked))
	}
	for _, r := range marked {
		if r.agent != "costfluent-mcp/1.4.0" {
			t.Errorf("%s %s carries %s %q, want costfluent-mcp/1.4.0", r.method, r.path, AgentHeader, r.agent)
		}
	}

	agentCalls(t, parent)
	for _, r := range requests()[len(marked):] {
		if r.marked {
			t.Errorf("the parent's %s %s carries %s %q", r.method, r.path, AgentHeader, r.agent)
		}
	}
	if got := agent.ResolveWorkspace(""); got != "ws_default" {
		t.Errorf("the agent copy's workspace = %q, want the parent's ws_default", got)
	}
}

func TestAgentKeepsItsMarkerThroughAWorkspaceCopyAndAWorkspaceCopyKeepsItsWorkspace(t *testing.T) {
	url, requests := agentServer(t)
	base := NewClient(WithBaseURL(url), WithAPIKey("key"), WithUserAgent("costfluent/2.0.1"))

	if _, err := base.Agent("costfluent-mcp").Workspace("ws_a").ListCostAlerts(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Workspace("ws_b").Agent("costfluent-mcp").ListCostAlerts(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range requests() {
		if r.agent != "costfluent-mcp/2.0.1" {
			t.Errorf("%s %s carries %q", r.method, r.path, r.agent)
		}
	}
}

func TestAgentNamesTheVersionTheUserAgentReports(t *testing.T) {
	for ua, want := range map[string]string{
		"costfluent-mcp/1.4.0":              "assistant/1.4.0",
		"costfluent-go/0.9.0 (linux/amd64)": "assistant/0.9.0",
		"costfluent-go":                     "assistant",
		"costfluent-go/":                    "assistant",
		"":                                  "assistant",
	} {
		url, requests := agentServer(t)
		c := NewClient(WithBaseURL(url), WithUserAgent(ua)).Agent("assistant")
		if _, err := c.GetMe(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := requests()[0].agent; got != want {
			t.Errorf("User-Agent %q: %s %q, want %q", ua, AgentHeader, got, want)
		}
	}
}
