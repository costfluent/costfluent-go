package costfluent

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

const plannedJSON = `"planned":{"monthlyCostBefore":"10.00000000","monthlyCostAfter":"35.12345678",
	"monthlyChange":"25.12345678","unpricedResources":1}`

// An amount goes out as the decimal string it was written as, an unpriced cost and every unset
// optional field are not sent at all, and the workspace falls back to the client's default.
func TestSubmitCostEstimateSendsTheEstimateAndReadsTheCreatedReceipt(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"est_aaaaaaaaaaaaaaaaaaaaaa","state":"proposed",` + plannedJSON + `}`})

	receipt, err := c.SubmitCostEstimate(context.Background(), &SubmitCostEstimateInput{
		Repository: "acme/infra", PullRequest: 42, Source: "infracost", SourceVersion: "0.10.41",
		Commit: "0123abc", URL: "https://github.com/acme/infra/pull/42", Currency: "EUR",
		Tag: &CostEstimateTag{Key: "costfluent-estimate", Value: "acme-infra-42"},
		Resources: []CostEstimateResource{
			{Address: "aws_instance.web", Type: "aws_instance", Change: CostEstimateChangeReplace,
				MonthlyCostBefore: ptr[Decimal]("10.00000000"), MonthlyCostAfter: ptr[Decimal]("35.12345678"),
				ResourceIDs: []string{"i-0abc"}},
			{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Change: CostEstimateChangeCreate},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "POST", "/v1/cost-estimates")
	if len(g.Query) != 0 {
		t.Errorf("query %v, want none: the workspace goes in the body", g.Query)
	}
	for key, want := range map[string]any{
		"workspaceId": "ws_default", "repository": "acme/infra", "pullRequest": float64(42),
		"source": "infracost", "sourceVersion": "0.10.41", "commit": "0123abc",
		"url": "https://github.com/acme/infra/pull/42", "currency": "EUR",
	} {
		wantBody(t, g, key, want)
	}
	if _, sent := g.Body["plan"]; sent {
		t.Errorf("sent plan on a pull request's estimate: %v", g.Body)
	}
	if tag, _ := g.Body["tag"].(map[string]any); tag["key"] != "costfluent-estimate" || tag["value"] != "acme-infra-42" {
		t.Errorf("tag = %v", g.Body["tag"])
	}
	resources, _ := g.Body["resources"].([]any)
	if len(resources) != 2 {
		t.Fatalf("resources = %v, want two", g.Body["resources"])
	}
	priced, _ := resources[0].(map[string]any)
	for key, want := range map[string]any{
		"address": "aws_instance.web", "type": "aws_instance", "change": "replace",
		"monthlyCostBefore": "10.00000000", "monthlyCostAfter": "35.12345678",
	} {
		if priced[key] != want {
			t.Errorf("first resource %s = %#v, want %#v", key, priced[key], want)
		}
	}
	if ids, _ := priced["resourceIds"].([]any); len(ids) != 1 || ids[0] != "i-0abc" {
		t.Errorf("first resource ids = %v", priced["resourceIds"])
	}
	if unpriced, _ := resources[1].(map[string]any); len(unpriced) != 3 {
		t.Errorf("second resource %v, want only address, type and change", unpriced)
	}

	if receipt.ID != "est_aaaaaaaaaaaaaaaaaaaaaa" || receipt.State != CostEstimateStateProposed ||
		!receipt.Created || receipt.Replayed || receipt.AppliedOn != nil || receipt.Planned == nil {
		t.Fatalf("decoded %+v", receipt)
	}
	if p := *receipt.Planned; p.MonthlyCostBefore != "10.00000000" || p.MonthlyCostAfter != "35.12345678" ||
		p.MonthlyChange != "25.12345678" || p.UnpricedResources != 1 {
		t.Fatalf("planned %+v", p)
	}
}

// A later post for the same reference replaces the estimate and answers 200; an API older than
// the receipt's totals answers without planned.
func TestSubmitCostEstimateReadsAReplacedEstimateWithoutTotals(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"est_aaaaaaaaaaaaaaaaaaaaaa","state":"proposed"}`})

	receipt, err := c.SubmitCostEstimate(context.Background(), &SubmitCostEstimateInput{
		WorkspaceID: "ws_1", Plan: "nightly", Source: "infracost", Currency: "USD",
		Resources: []CostEstimateResource{{Address: "aws_instance.web", Type: "aws_instance", Change: CostEstimateChangeDelete}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantBody(t, g, "workspaceId", "ws_1")
	wantBody(t, g, "plan", "nightly")
	for _, key := range []string{"repository", "pullRequest", "tag", "url"} {
		if _, sent := g.Body[key]; sent {
			t.Errorf("sent %s on a named plan's estimate: %v", key, g.Body)
		}
	}
	if receipt.Created || receipt.Planned != nil || receipt.State != CostEstimateStateProposed {
		t.Fatalf("decoded %+v, want a replaced estimate without totals", receipt)
	}
}

// A post whose first answer is lost is resent under the same key, and the API's replay of the
// stored estimate is marked, so the caller knows it was stored once.
func TestSubmitCostEstimateMarksAReplayedAnswer(t *testing.T) {
	var posts atomic.Int32
	var mu sync.Mutex
	var keys []string
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get(idempotencyKeyHeader))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if posts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set(replayedHeader, "true")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"est_aaaaaaaaaaaaaaaaaaaaaa","state":"proposed"}`)
	})

	receipt, err := c.SubmitCostEstimate(context.Background(), &SubmitCostEstimateInput{
		WorkspaceID: "ws_1", Plan: "nightly", Source: "infracost", Currency: "USD",
		Resources: []CostEstimateResource{{Address: "a", Type: "t", Change: CostEstimateChangeCreate}},
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !receipt.Replayed || !receipt.Created || len(keys) != 2 || keys[0] == "" || keys[1] != keys[0] {
		t.Fatalf("receipt %+v after keys %q, want the replay marked and one key on both attempts", receipt, keys)
	}
}

// An estimate whose change was applied takes no new post: the conflict is returned, not retried.
func TestSubmitCostEstimateReturnsAnAppliedEstimateAsAConflict(t *testing.T) {
	var posts atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, _ *http.Request) {
		posts.Add(1)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"title":"Conflict","status":409,"code":"conflict","detail":"The estimate was already applied."}`)
	})

	_, err := c.SubmitCostEstimate(context.Background(), &SubmitCostEstimateInput{
		WorkspaceID: "ws_1", Plan: "nightly", Source: "infracost", Currency: "USD",
		Resources: []CostEstimateResource{{Address: "a", Type: "t", Change: CostEstimateChangeCreate}},
	})
	if !IsConflict(err) || posts.Load() != 1 {
		t.Fatalf("err %v after %d posts, want one conflict", err, posts.Load())
	}
}

func TestSubmitCostEstimateReturnsARefusedEstimateAsABadRequest(t *testing.T) {
	c, _ := testClient(t, stub{400, `{"status":400,"title":"Bad Request","code":"validation_error",
		"detail":"currency must be a currency Costfluent supports."}`})

	_, err := c.SubmitCostEstimate(context.Background(), &SubmitCostEstimateInput{
		Plan: "nightly", Source: "infracost", Currency: "XXX",
		Resources: []CostEstimateResource{{Address: "a", Type: "t", Change: CostEstimateChangeCreate}},
	})
	if !IsBadRequest(err) {
		t.Fatalf("err = %v, want a bad request", err)
	}
}

func TestApplyCostEstimateSendsTheAppliedIdsAndReadsTheReceipt(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"est_aaaaaaaaaaaaaaaaaaaaaa","state":"applied","appliedOn":"2026-11-09",` + plannedJSON + `}`})

	receipt, err := c.ApplyCostEstimate(context.Background(), &ApplyCostEstimateInput{
		Repository: "acme/infra", PullRequest: 42,
		Resources: []AppliedResource{{Address: "aws_instance.web", ResourceIDs: []string{"i-0def", "arn:aws:ec2:eu-central-1:1:instance/i-0def"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "POST", "/v1/cost-estimates/apply")
	wantBody(t, g, "workspaceId", "ws_default")
	wantBody(t, g, "repository", "acme/infra")
	wantBody(t, g, "pullRequest", float64(42))
	resources, _ := g.Body["resources"].([]any)
	applied, _ := resources[0].(map[string]any)
	if ids, _ := applied["resourceIds"].([]any); len(resources) != 1 || applied["address"] != "aws_instance.web" || len(ids) != 2 {
		t.Fatalf("resources = %v", g.Body["resources"])
	}
	if receipt.State != CostEstimateStateApplied || receipt.AppliedOn == nil || *receipt.AppliedOn != "2026-11-09" ||
		receipt.Created || receipt.Planned == nil || receipt.Planned.MonthlyChange != "25.12345678" {
		t.Fatalf("decoded %+v", receipt)
	}
}

// The apply operation declares no Idempotency-Key, so it carries none, and a 5xx is returned
// rather than resent: the client cannot tell whether the apply was recorded.
func TestApplyCostEstimateCarriesNoKeyAndIsNotResentAfterAServerError(t *testing.T) {
	var posts atomic.Int32
	var key atomic.Value
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		key.Store(r.Header.Get(idempotencyKeyHeader))
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"title":"Bad Gateway","status":502}`)
	})

	_, err := c.ApplyCostEstimate(context.Background(), &ApplyCostEstimateInput{WorkspaceID: "ws_1", Plan: "nightly"})
	if StatusCode(err) != http.StatusBadGateway || posts.Load() != 1 || key.Load() != "" {
		t.Fatalf("err %v after %d posts with key %q, want one 502 and no key", err, posts.Load(), key.Load())
	}
}

func TestApplyCostEstimateReturnsAnApplyWithOtherIdsAsAConflict(t *testing.T) {
	c, _ := testClient(t, stub{409, `{"title":"Conflict","status":409,"code":"conflict"}`})

	_, err := c.ApplyCostEstimate(context.Background(), &ApplyCostEstimateInput{Plan: "nightly"})
	if !IsConflict(err) {
		t.Fatalf("err = %v, want a conflict", err)
	}
}

func TestCostEstimateChangesMatchTheContract(t *testing.T) {
	wantContractValues(t, "CostEstimateChange", CostEstimateChanges())
}

func TestCostEstimateStatesMatchTheContract(t *testing.T) {
	wantContractValues(t, "CostEstimateState", CostEstimateStates())
}
