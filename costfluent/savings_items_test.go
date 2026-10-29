package costfluent

import (
	"context"
	"testing"
)

const savingsItemJSON = `{"id":"sav_1","title":"Rightsize vm-prod-01","evidence":"p95 CPU 4% over 30 days",
	"estimatedMonthlySaving":"40.00000000","currency":"EUR","costBasis":"ActualCost","owner":"Platform",
	"confidence":"medium","risk":"low","effort":"medium","status":"implemented","source":"awsCostOptimizationHub",
	"kind":"rightsizing","resource":"i-0abc","scope":"costs.resource_id = 'i-0abc'","lastSeenAt":"2026-10-20T00:00:00Z",
	"lastMeasurementOutcome":"inconclusive","lastMeasurementScope":"costs.service = 'Amazon EC2'",
	"lastMeasurementRealizedMonthly":"12.50000000","lastMeasuredAt":"2026-10-21T00:00:00Z",
	"acceptedAt":"2026-10-01T00:00:00Z","implementedAt":"2026-10-02T00:00:00Z","createdAt":"2026-09-30T00:00:00Z"}`

func TestListSavingsItems(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/savings-items?page=2&limit=10",
		"next":"https://api.test/v1/savings-items?page=3&limit=10"},"data":[` + savingsItemJSON + `]}`})

	list, err := c.ListSavingsItems(context.Background(), "ws_1", &ListSavingsItemsOptions{
		Tab: "inProgress", Source: "awsCostOptimizationHub", Kind: "rightsizing", CloudAccountID: "prv_1",
		MinimumMonthlySaving: "25", Sort: "largestSaving", PageOptions: PageOptions{Page: 2, Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/savings-items")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantQuery(t, g, "tab", "inProgress")
	wantQuery(t, g, "source", "awsCostOptimizationHub")
	wantQuery(t, g, "kind", "rightsizing")
	wantQuery(t, g, "cloudAccountId", "prv_1")
	wantQuery(t, g, "minimumMonthlySaving", "25")
	wantQuery(t, g, "sort", "largestSaving")
	wantQuery(t, g, "page", "2")
	wantQuery(t, g, "limit", "10")
	item := list.Data[0]
	if !list.Links.HasNextPage() || item.ID != "sav_1" || item.EstimatedMonthlySaving != "40.00000000" ||
		item.Status != "implemented" || item.Kind == nil || *item.Kind != "rightsizing" ||
		item.LastMeasurementOutcome == nil || *item.LastMeasurementOutcome != "inconclusive" ||
		item.LastMeasurementRealizedMonthly == nil || *item.LastMeasurementRealizedMonthly != "12.50000000" ||
		item.ImplementedAt == nil || item.VerifiedAt != nil || item.RejectionReason != nil {
		t.Fatalf("decoded %+v", list)
	}
}

// A listing without options sends only the workspace, the client's default when none is named.
func TestListSavingsItemsWithoutOptionsNamesOnlyTheWorkspace(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/savings-items"},"data":[]}`})

	if _, err := c.ListSavingsItems(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantQuery(t, g, "workspaceId", "ws_default")
	if len(g.Query) != 1 {
		t.Fatalf("query %v, want only workspaceId", g.Query)
	}
}

func TestListAllSavingsItemsWalksEveryPageWithTheFilters(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"https://api.test/v1/savings-items?page=1","next":"https://api.test/v1/savings-items?page=2"},"data":[` + savingsItemJSON + `]}`},
		stub{200, `{"links":{"self":"https://api.test/v1/savings-items?page=2"},"data":[` + savingsItemJSON + `]}`},
	)

	all, err := c.ListAllSavingsItems(context.Background(), "ws_1", &ListSavingsItemsOptions{Tab: "toReview"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d findings, want 2", len(all))
	}
	for i, g := range *got {
		wantQuery(t, g, "tab", "toReview")
		wantQuery(t, g, "limit", "1")
		wantQuery(t, g, "page", []string{"1", "2"}[i])
	}
}

func TestGetSavingsItem(t *testing.T) {
	c, got := testClient(t, stub{200, savingsItemJSON})

	item, err := c.GetSavingsItem(context.Background(), "ws_1", "sav_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/savings-items/sav_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	if item.Evidence != "p95 CPU 4% over 30 days" || item.Owner != "Platform" || item.Scope == nil {
		t.Fatalf("decoded %+v", item)
	}
}

// A value the queue does not take is the API's 400, returned as an error, never an empty page.
func TestListSavingsItemsRefusedValueIsAnError(t *testing.T) {
	c, _ := testClient(t, stub{400, `{"status":400,"title":"Bad Request","detail":"tab must be toReview, inProgress, verified or closed.","code":"validation_error"}`})

	if _, err := c.ListSavingsItems(context.Background(), "ws_1", &ListSavingsItemsOptions{Tab: "open"}); !IsBadRequest(err) {
		t.Fatalf("got %v, want a validation error", err)
	}
}
