package costfluent

import (
	"context"
	"fmt"
	"testing"
)

func TestCreateWorkspaceReadsItBack(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"ws_1","name":"Prod"}`},
		stub{200, `{"id":"ws_1","name":"Prod","currency":"EUR","enableCurrencyConversion":false,
			"conversionMethod":"monthEndRate","enableAutomaticSyncing":true,"timeZone":"Europe/Berlin",
			"providerCount":2,"createdAt":"2026-09-01T00:00:00Z"}`},
	)

	ws, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{
		Name: "Prod", Currency: "EUR", TimeZone: "Europe/Berlin",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/workspaces")
	wantBody(t, (*got)[0], "currency", "EUR")
	wantBody(t, (*got)[0], "timeZone", "Europe/Berlin")
	wantRequest(t, (*got)[1], "GET", "/v1/workspaces/ws_1")
	if ws.ID != "ws_1" || ws.Currency != "EUR" || ws.ProviderCount != 2 || !ws.EnableAutomaticSyncing ||
		ws.TimeZone != "Europe/Berlin" {
		t.Fatalf("decoded %+v", ws)
	}
}

func TestUpdateWorkspaceIsPut(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"ws_1","name":"Renamed","currency":"EUR"}`})

	name := "Renamed"
	if _, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/workspaces/ws_1")
	wantBody(t, (*got)[0], "name", "Renamed")
	if len((*got)[0].Body) != 1 {
		t.Errorf("unset fields were sent: %v", (*got)[0].Body)
	}
}

func TestCreateWorkspaceWithoutAZoneSendsNone(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"ws_1","name":"Prod"}`},
		stub{200, `{"id":"ws_1","name":"Prod","currency":"EUR","timeZone":"UTC"}`},
	)

	if _, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{Name: "Prod", Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if _, sent := (*got)[0].Body["timeZone"]; sent {
		t.Errorf("an unset zone was sent: %v", (*got)[0].Body)
	}
}

func TestUpdateWorkspaceSendsTheZone(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"ws_1","name":"Prod","timeZone":"Europe/London"}`})

	zone := "Europe/London"
	ws, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{TimeZone: &zone})
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "timeZone", "Europe/London")
	if len((*got)[0].Body) != 1 || ws.TimeZone != "Europe/London" {
		t.Errorf("sent %v, decoded %+v", (*got)[0].Body, ws)
	}
}

func TestListAllWorkspacesFollowsPages(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"/v1/workspaces?page=1","next":"/v1/workspaces?page=2"},"data":[{"id":"ws_1"}]}`},
		stub{200, `{"links":{"self":"/v1/workspaces?page=2"},"data":[{"id":"ws_2"}]}`},
	)

	all, err := c.ListAllWorkspaces(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[1], "page", "2")
	wantQuery(t, (*got)[1], "limit", "1")
	if len(all) != 2 || all[1].ID != "ws_2" {
		t.Fatalf("all %+v", all)
	}
}

func TestUpdateWorkspaceSendsOnlyTheReportDefaultsItNames(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"ws_1","name":"Prod","timeZone":"UTC",
		"reportDefaults":{"dateInterval":"lastMonth","dateBin":"week","includeCredits":false,
		"includeRefunds":false,"includeTax":true,"amortize":false,"showForecast":false,"compareToLastPeriod":true}}`})

	interval, amortize := "lastMonth", false
	ws, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{
		ReportDefaults: &ReportDefaultsInput{DateInterval: &interval, Amortize: &amortize},
	})
	if err != nil {
		t.Fatal(err)
	}

	sent, ok := (*got)[0].Body["reportDefaults"].(map[string]any)
	if !ok || len(sent) != 2 || sent["dateInterval"] != "lastMonth" || sent["amortize"] != false {
		t.Fatalf("reportDefaults sent = %v, want only dateInterval and amortize", (*got)[0].Body["reportDefaults"])
	}
	if ws.ReportDefaults.DateBin != "week" || ws.ReportDefaults.Amortize || !ws.ReportDefaults.CompareToLastPeriod {
		t.Fatalf("reportDefaults read = %+v", ws.ReportDefaults)
	}
}

func TestUpdateWorkspaceSendsOnlyTheAnomalySettingsItNames(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"ws_1","name":"Prod","timeZone":"UTC",
		"anomaliesExcludeMarketplace":false,"anomaliesExcludeCommitmentFees":true,
		"anomalyNotificationThreshold":"250.00000000"}`})

	marketplace, threshold := false, Decimal("250")
	ws, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{
		AnomaliesExcludeMarketplace:  &marketplace,
		AnomalyNotificationThreshold: &threshold,
	})
	if err != nil {
		t.Fatal(err)
	}

	body := (*got)[0].Body
	if len(body) != 2 || body["anomaliesExcludeMarketplace"] != false || body["anomalyNotificationThreshold"] != "250" {
		t.Fatalf("sent %v, want only the marketplace flag and the threshold", body)
	}
	if ws.AnomaliesExcludeMarketplace || !ws.AnomaliesExcludeCommitmentFees ||
		!ws.AnomalyNotificationThreshold.Equal(Decimal("250")) {
		t.Fatalf("read %+v", ws)
	}
}

// The blend is sent only when an update names it, false included, and read back from the
// workspace the API answers with.
func TestUpdateWorkspaceSendsTheCommitmentDiscountBlend(t *testing.T) {
	for _, blend := range []bool{true, false} {
		c, got := testClient(t, stub{200, fmt.Sprintf(`{"id":"ws_1","name":"Prod","timeZone":"UTC",
			"enableAutomaticSyncing":true,"blendCommitmentDiscounts":%t}`, blend)})

		ws, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{BlendCommitmentDiscounts: &blend})
		if err != nil {
			t.Fatal(err)
		}

		body := (*got)[0].Body
		if len(body) != 1 || body["blendCommitmentDiscounts"] != blend {
			t.Fatalf("sent %v, want only blendCommitmentDiscounts = %t", body, blend)
		}
		if ws.BlendCommitmentDiscounts != blend || !ws.EnableAutomaticSyncing {
			t.Fatalf("read %+v, want blendCommitmentDiscounts = %t", ws, blend)
		}
	}
}

func TestUpdateWorkspaceLeavesAnUnnamedBlendOut(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"ws_1","name":"Prod","blendCommitmentDiscounts":true}`})

	syncing := false
	if _, err := c.UpdateWorkspace(context.Background(), "ws_1", &UpdateWorkspaceInput{EnableAutomaticSyncing: &syncing}); err != nil {
		t.Fatal(err)
	}
	if _, sent := (*got)[0].Body["blendCommitmentDiscounts"]; sent {
		t.Errorf("an unset blend was sent: %v", (*got)[0].Body)
	}
}
