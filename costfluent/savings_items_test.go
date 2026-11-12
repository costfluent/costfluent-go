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
		CostEstimateID: "est_aaaaaaaaaaaaaaaaaaaaaa", MinimumMonthlySaving: "25", Sort: "largestSaving", PageOptions: PageOptions{Page: 2, Limit: 10},
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
	wantQuery(t, g, "costEstimateId", "est_aaaaaaaaaaaaaaaaaaaaaa")
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

// Accept and implement send no body; each answers 204, so the call returns no finding.
func TestAcceptAndImplementSavingsItemSendNoBody(t *testing.T) {
	c, got := testClient(t, stub{204, ""}, stub{204, ""})

	if err := c.AcceptSavingsItem(context.Background(), "ws_1", "sav_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.ImplementSavingsItem(context.Background(), "ws_1", "sav_1"); err != nil {
		t.Fatal(err)
	}
	for i, path := range []string{"/v1/savings-items/sav_1/accept", "/v1/savings-items/sav_1/implement"} {
		g := (*got)[i]
		wantRequest(t, g, "POST", path)
		wantQuery(t, g, "workspaceId", "ws_1")
		if g.Body != nil {
			t.Errorf("%s sent body %v", path, g.Body)
		}
	}
}

func TestRejectSavingsItemSendsTheReason(t *testing.T) {
	c, got := testClient(t, stub{204, ""})

	if err := c.RejectSavingsItem(context.Background(), "", "sav_1", "Reserved capacity covers it"); err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "POST", "/v1/savings-items/sav_1/reject")
	wantQuery(t, g, "workspaceId", "ws_default")
	wantBody(t, g, "reason", "Reserved capacity covers it")
	if len(g.Body) != 1 {
		t.Errorf("body %v, want the reason alone", g.Body)
	}
}

func TestAssignSavingsItemOwnerReturnsTheFindingAsStored(t *testing.T) {
	c, got := testClient(t, stub{200, savingsItemJSON})

	item, err := c.AssignSavingsItemOwner(context.Background(), "ws_1", "sav_1", "Platform")
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want the PUT alone", len(*got))
	}
	g := (*got)[0]
	wantRequest(t, g, "PUT", "/v1/savings-items/sav_1/owner")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantBody(t, g, "owner", "Platform")
	if item.ID != "sav_1" || item.Owner != "Platform" || item.Status != "implemented" {
		t.Fatalf("decoded %+v", item)
	}
}

// A finding already closed is the API's 409, returned as a conflict for each of the four writes.
func TestSavingsItemWritesOnAClosedFindingAreAConflict(t *testing.T) {
	conflict := stub{409, `{"status":409,"title":"Conflict","detail":"The finding is rejected.","code":"conflict"}`}
	c, _ := testClient(t, conflict, conflict, conflict, conflict)
	ctx := context.Background()

	_, ownerErr := c.AssignSavingsItemOwner(ctx, "ws_1", "sav_1", "Platform")
	for name, err := range map[string]error{
		"accept":    c.AcceptSavingsItem(ctx, "ws_1", "sav_1"),
		"implement": c.ImplementSavingsItem(ctx, "ws_1", "sav_1"),
		"reject":    c.RejectSavingsItem(ctx, "ws_1", "sav_1", "No longer relevant"),
		"owner":     ownerErr,
	} {
		if !IsConflict(err) {
			t.Errorf("%s: got %v, want a conflict", name, err)
		}
	}
}
