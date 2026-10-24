package costfluent

import (
	"context"
	"testing"
)

func TestCreateDashboardReadsItBack(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"dsh_1","title":"Overview"}`},
		stub{200, `{"id":"dsh_1","title":"Overview","isDefault":true,"dateInterval":"thisMonth","dateBin":"day",
			"widgets":[{"id":"wdg_1","type":"chart","title":"Spend","position":{"x":0,"y":0,"w":6,"h":4}}],
			"createdAt":"2026-09-01T00:00:00Z"}`},
	)

	dash, err := c.CreateDashboard(context.Background(), &CreateDashboardInput{Title: "Overview", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/dashboards")
	wantBody(t, (*got)[0], "isDefault", true)
	wantRequest(t, (*got)[1], "GET", "/v1/dashboards/dsh_1")
	if !dash.IsDefault || dash.Widgets[0].Position.W != 6 || dash.DateBin != "day" {
		t.Fatalf("decoded %+v", dash)
	}
}

func TestUpdateDashboardIsPut(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"dsh_1","title":"Renamed"}`})

	title := "Renamed"
	if _, err := c.UpdateDashboard(context.Background(), "ws_1", "dsh_1", &UpdateDashboardInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/dashboards/dsh_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
}

func TestSetDashboardSavedFilterAttachesAndDetaches(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"dsh_1","title":"Overview","dateInterval":"thisMonth","dateBin":"day","widgets":[],"savedFilterId":"flt_1"}`},
		stub{200, `{"id":"dsh_1","title":"Overview","dateInterval":"thisMonth","dateBin":"day","widgets":[],"savedFilterId":null}`},
	)

	filter := "flt_1"
	dash, err := c.SetDashboardSavedFilter(context.Background(), "", "dsh_1", &filter)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/dashboards/dsh_1/saved-filter")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "savedFilterId", "flt_1")
	if len((*got)[0].Query) != 0 {
		t.Errorf("query = %v, want the workspace in the body only", (*got)[0].Query)
	}
	if dash.SavedFilterID == nil || *dash.SavedFilterID != "flt_1" {
		t.Fatalf("dashboard %+v", dash)
	}

	detached, err := c.SetDashboardSavedFilter(context.Background(), "ws_1", "dsh_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[1], "workspaceId", "ws_1")
	if value, sent := (*got)[1].Body["savedFilterId"]; !sent || value != nil {
		t.Fatalf("detach sent %v, want an explicit null savedFilterId", (*got)[1].Body)
	}
	if detached.SavedFilterID != nil {
		t.Fatalf("detached dashboard %+v", detached)
	}
}

func TestSetDashboardDefaultSetsAndClears(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"dsh_1","title":"Overview","isDefault":true,"dateInterval":"thisMonth","dateBin":"day","widgets":[]}`},
		stub{200, `{"id":"dsh_1","title":"Overview","isDefault":false,"dateInterval":"thisMonth","dateBin":"day","widgets":[]}`},
	)

	dash, err := c.SetDashboardDefault(context.Background(), "", "dsh_1", true)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/dashboards/dsh_1/default")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "isDefault", true)
	if len((*got)[0].Query) != 0 {
		t.Errorf("query = %v, want the workspace in the body only", (*got)[0].Query)
	}
	if !dash.IsDefault {
		t.Fatalf("dashboard %+v", dash)
	}

	cleared, err := c.SetDashboardDefault(context.Background(), "ws_1", "dsh_1", false)
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[1], "workspaceId", "ws_1")
	if value, sent := (*got)[1].Body["isDefault"]; !sent || value != false {
		t.Fatalf("clear sent %v, want an explicit isDefault false", (*got)[1].Body)
	}
	if cleared.IsDefault {
		t.Fatalf("cleared dashboard %+v", cleared)
	}
}

func TestCreateDashboardWidgetSendsTheReportAndReadsItBack(t *testing.T) {
	c, got := testClient(t,
		stub{201, `{"id":"wdg_1","title":"Spend"}`},
		stub{200, `{"id":"wdg_1","type":"costReport","costReportId":"crp_1","display":"chart","content":null,
			"title":"Spend","position":{"x":0,"y":2,"w":6,"h":4}}`},
	)

	report := "crp_1"
	widget, err := c.CreateDashboardWidget(context.Background(), "", "dsh_1", &CreateDashboardWidgetInput{
		Type:         WidgetTypeCostReport,
		CostReportID: &report,
		Title:        "Spend",
		Position:     WidgetPosition{X: 0, Y: 2, W: 6, H: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/dashboards/dsh_1/widgets")
	wantQuery(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "type", "costReport")
	wantBody(t, (*got)[0], "costReportId", "crp_1")
	for _, unset := range []string{"display", "content", "workspaceId"} {
		if _, sent := (*got)[0].Body[unset]; sent {
			t.Errorf("create sent unset %s: %v", unset, (*got)[0].Body)
		}
	}
	if pos, _ := (*got)[0].Body["position"].(map[string]any); pos["y"] != 2.0 || pos["w"] != 6.0 {
		t.Errorf("position sent %v", (*got)[0].Body["position"])
	}
	wantRequest(t, (*got)[1], "GET", "/v1/dashboards/dsh_1/widgets/wdg_1")
	wantQuery(t, (*got)[1], "workspaceId", "ws_default")
	if widget.CostReportID == nil || *widget.CostReportID != "crp_1" || widget.Display == nil ||
		*widget.Display != WidgetDisplayChart || widget.Content != nil || widget.Position.H != 4 {
		t.Fatalf("decoded %+v", widget)
	}
}

func TestCreateDashboardWidgetWhoseReadFailsKeepsTheID(t *testing.T) {
	c, _ := testClient(t,
		stub{201, `{"id":"wdg_1","title":"Notes"}`},
		stub{403, `{"title":"Forbidden","status":403,"code":"Forbidden"}`},
	)

	content := "Spend is *up*"
	_, err := c.CreateDashboardWidget(context.Background(), "ws_1", "dsh_1", &CreateDashboardWidgetInput{
		Type: WidgetTypeText, Content: &content, Title: "Notes", Position: WidgetPosition{W: 3, H: 2},
	})
	if id, ok := CreatedID(err); !ok || id != "wdg_1" {
		t.Fatalf("err = %v, want one carrying the created ID", err)
	}
}

func TestUpdateDashboardWidgetSendsOnlyWhatChanged(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"wdg_1","type":"text","content":"New words","title":"Notes",
		"position":{"x":1,"y":1,"w":3,"h":2}}`})

	content := "New words"
	widget, err := c.UpdateDashboardWidget(context.Background(), "ws_1", "dsh_1", "wdg_1", &UpdateDashboardWidgetInput{
		Content:  &content,
		Position: &WidgetPosition{X: 1, Y: 1, W: 3, H: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/dashboards/dsh_1/widgets/wdg_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	wantBody(t, (*got)[0], "content", "New words")
	for _, unset := range []string{"title", "display"} {
		if _, sent := (*got)[0].Body[unset]; sent {
			t.Errorf("update sent unset %s: %v", unset, (*got)[0].Body)
		}
	}
	if widget.Content == nil || *widget.Content != "New words" || widget.Display != nil {
		t.Fatalf("decoded %+v", widget)
	}
}

func TestDeleteDashboardWidgetAndAMissingOneIsNotFound(t *testing.T) {
	c, got := testClient(t,
		stub{204, ""},
		stub{404, `{"title":"Not Found","status":404,"code":"NotFound"}`},
	)

	if err := c.DeleteDashboardWidget(context.Background(), "ws_1", "dsh_1", "wdg_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "DELETE", "/v1/dashboards/dsh_1/widgets/wdg_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")

	if _, err := c.GetDashboardWidget(context.Background(), "ws_1", "dsh_1", "wdg_9"); !IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
}
