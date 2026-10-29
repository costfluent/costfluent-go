package costfluent

import (
	"context"
	"testing"
)

const savingsViewJSON = `{"id":"sav_view_1","title":"Large AWS rightsizing","tab":"toReview","source":"awsCostOptimizationHub",
	"kind":"rightsizing","cloudAccountId":"prv_1","minimumMonthlySaving":"250.50000000","createdAt":"2026-10-25T00:00:00Z"}`

func TestSavingsViewCreateSendsTheViewAndAKey(t *testing.T) {
	c, got := testClient(t, stub{201, savingsViewJSON})

	source, kind, connection, minimum := "awsCostOptimizationHub", "rightsizing", "prv_1", Decimal("250.5")
	view, err := c.CreateSavingsView(context.Background(), &CreateSavingsViewInput{
		Title: "Large AWS rightsizing", Tab: "toReview", Source: &source, Kind: &kind,
		CloudAccountID: &connection, MinimumMonthlySaving: &minimum,
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "POST", "/v1/savings-views")
	wantBody(t, g, "workspaceId", "ws_default")
	wantBody(t, g, "title", "Large AWS rightsizing")
	wantBody(t, g, "tab", "toReview")
	wantBody(t, g, "cloudAccountId", "prv_1")
	wantBody(t, g, "minimumMonthlySaving", "250.5")
	if len(*got) != 1 || view.ID != "sav_view_1" || view.MinimumMonthlySaving == nil || *view.MinimumMonthlySaving != "250.50000000" {
		t.Fatalf("decoded %+v after %d requests", view, len(*got))
	}
}

func TestSavingsViewCreateWithoutAWorkspaceIsRefusedBeforeSending(t *testing.T) {
	c := NewClient(WithBaseURL("https://api.test"), WithAPIKey("key"))

	if _, err := c.CreateSavingsView(context.Background(), &CreateSavingsViewInput{Title: "v", Tab: "toReview"}); err == nil {
		t.Fatal("created a view without a workspace")
	}
}

func TestSavingsViewUpdateReplacesTheViewWhole(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"sav_view_1","title":"Everything in progress","tab":"inProgress","createdAt":"2026-10-25T00:00:00Z"}`})

	view, err := c.UpdateSavingsView(context.Background(), "ws_1", "sav_view_1", &UpdateSavingsViewInput{
		Title: "Everything in progress", Tab: "inProgress",
	}, IfMatch(`"7"`))
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "PUT", "/v1/savings-views/sav_view_1")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantBody(t, g, "title", "Everything in progress")
	wantBody(t, g, "tab", "inProgress")
	for _, cleared := range []string{"source", "kind", "cloudAccountId", "minimumMonthlySaving", "workspaceId"} {
		if _, sent := g.Body[cleared]; sent {
			t.Errorf("sent %s, which an unset filter leaves out so the view clears it", cleared)
		}
	}
	if view.Source != nil || view.CloudAccountID != nil || view.Tab != "inProgress" {
		t.Fatalf("decoded %+v", view)
	}
}

func TestSavingsViewGetListAndDelete(t *testing.T) {
	c, got := testClient(t,
		stub{200, savingsViewJSON},
		stub{200, `{"links":{"self":"https://api.test/v1/savings-views?page=1&limit=50"},"data":[` + savingsViewJSON + `]}`},
		stub{204, ""},
	)
	ctx := context.Background()

	view, err := c.GetSavingsView(ctx, "ws_1", "sav_view_1")
	if err != nil {
		t.Fatal(err)
	}
	list, err := c.ListSavingsViews(ctx, "ws_1", &PageOptions{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSavingsView(ctx, "ws_1", "sav_view_1"); err != nil {
		t.Fatal(err)
	}

	wantRequest(t, (*got)[0], "GET", "/v1/savings-views/sav_view_1")
	wantRequest(t, (*got)[1], "GET", "/v1/savings-views")
	wantQuery(t, (*got)[1], "limit", "50")
	wantRequest(t, (*got)[2], "DELETE", "/v1/savings-views/sav_view_1")
	for _, g := range *got {
		wantQuery(t, g, "workspaceId", "ws_1")
	}
	if view.Kind == nil || *view.Kind != "rightsizing" || len(list.Data) != 1 || list.Data[0].Title != "Large AWS rightsizing" {
		t.Fatalf("decoded %+v and %+v", view, list)
	}
}

func TestSavingsViewListAllWalksEveryPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"https://api.test/v1/savings-views?page=1","next":"https://api.test/v1/savings-views?page=2"},"data":[` + savingsViewJSON + `]}`},
		stub{200, `{"links":{"self":"https://api.test/v1/savings-views?page=2"},"data":[` + savingsViewJSON + `]}`},
	)

	all, err := c.ListAllSavingsViews(context.Background(), "ws_1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || len(*got) != 2 {
		t.Fatalf("got %d views in %d requests", len(all), len(*got))
	}
}

// A stale If-Match is the API's 412, which the SDK reports as a conflict to read again from.
func TestSavingsViewStaleUpdateIsAPreconditionFailure(t *testing.T) {
	c, _ := testClient(t, stub{412, `{"status":412,"title":"Precondition Failed","code":"precondition_failed"}`})

	_, err := c.UpdateSavingsView(context.Background(), "ws_1", "sav_view_1", &UpdateSavingsViewInput{Title: "v", Tab: "toReview"}, IfMatch(`"1"`))
	if !IsPreconditionFailed(err) {
		t.Fatalf("got %v, want a precondition failure", err)
	}
}
