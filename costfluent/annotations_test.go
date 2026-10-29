package costfluent

import (
	"context"
	"testing"
)

const noteJSON = `{"id":"ann_1","date":"2026-10-20","source":"manual","text":"Reserved instances bought",
	"costReportIds":["crp_1","crp_2"],"createdByUserId":"usr_1","createdAt":"2026-10-21T08:00:00Z"}`

const budgetFiringJSON = `{"id":"ann_2","date":"2026-10-22","source":"budgetAlert","costReportIds":["crp_1"],
	"budgetId":"bdg_1","sourceName":"Production","thresholdPercent":80,"thresholdBasis":"forecast",
	"resolvedAt":"2026-10-23T09:30:00Z","resolvedByUserId":"usr_2","createdAt":"2026-10-22T00:05:00Z"}`

func TestListAnnotationsSendsEveryFilter(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/annotations?page=2&limit=10"},"data":[` +
		noteJSON + `,` + budgetFiringJSON + `]}`})

	list, err := c.ListAnnotations(context.Background(), "ws_1", &ListAnnotationsOptions{
		CostReportID: "crp_1", StartDate: "2026-10-01", EndDate: "2026-10-31", IncludeResolved: true,
		PageOptions: PageOptions{Page: 2, Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/annotations")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantQuery(t, g, "costReportId", "crp_1")
	wantQuery(t, g, "startDate", "2026-10-01")
	wantQuery(t, g, "endDate", "2026-10-31")
	wantQuery(t, g, "includeResolved", "true")
	wantQuery(t, g, "page", "2")
	wantQuery(t, g, "limit", "10")

	if len(list.Data) != 2 {
		t.Fatalf("decoded %d annotations, want 2", len(list.Data))
	}
	note, firing := list.Data[0], list.Data[1]
	if note.Source != "manual" || note.Text == nil || *note.Text != "Reserved instances bought" ||
		len(note.CostReportIDs) != 2 || note.CreatedByUserID == nil || note.ResolvedAt != nil || note.ETag != "" {
		t.Fatalf("decoded the note as %+v", note)
	}
	if firing.Source != "budgetAlert" || firing.Text != nil || firing.BudgetID == nil || *firing.BudgetID != "bdg_1" ||
		firing.SourceName == nil || *firing.SourceName != "Production" ||
		firing.ThresholdPercent == nil || *firing.ThresholdPercent != 80 ||
		firing.ThresholdBasis == nil || *firing.ThresholdBasis != "forecast" ||
		firing.ResolvedAt == nil || firing.ResolvedByUserID == nil {
		t.Fatalf("decoded the budget's firing as %+v", firing)
	}
}

// Unset filters are left out, so the API applies its defaults: open annotations only, every report.
func TestListAnnotationsLeavesUnsetFiltersOut(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"https://api.test/v1/annotations"},"data":[]}`},
		stub{200, `{"links":{"self":"https://api.test/v1/annotations"},"data":[]}`},
	)

	if _, err := c.ListAnnotations(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListAnnotations(context.Background(), "", &ListAnnotationsOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, g := range *got {
		wantQuery(t, g, "workspaceId", "ws_default")
		if len(g.Query) != 1 {
			t.Errorf("sent %v; want only the workspace", g.Query)
		}
	}
}

func TestListAnnotationsAcrossEveryPageKeepsTheFilters(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"https://api.test/v1/annotations?page=1","next":"https://api.test/v1/annotations?page=2"},"data":[` + noteJSON + `]}`},
		stub{200, `{"links":{"self":"https://api.test/v1/annotations?page=2"},"data":[` + budgetFiringJSON + `]}`},
	)

	all, err := c.ListAllAnnotations(context.Background(), "ws_1", &ListAnnotationsOptions{CostReportID: "crp_1", IncludeResolved: true}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || len(*got) != 2 || all[1].ID != "ann_2" {
		t.Fatalf("got %d annotations in %d requests", len(all), len(*got))
	}
	for i, g := range *got {
		wantQuery(t, g, "costReportId", "crp_1")
		wantQuery(t, g, "includeResolved", "true")
		wantQuery(t, g, "limit", "1")
		if i == 1 {
			wantQuery(t, g, "page", "2")
		}
	}
}

func TestCreateAnnotationSendsTheNoteToTheDefaultWorkspace(t *testing.T) {
	c, got := testClient(t, stub{201, noteJSON})

	annotation, err := c.CreateAnnotation(context.Background(), &CreateAnnotationInput{
		Date: "2026-10-20", Text: "Reserved instances bought", CostReportIDs: []string{"crp_1", "crp_2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "POST", "/v1/annotations")
	wantBody(t, g, "workspaceId", "ws_default")
	wantBody(t, g, "date", "2026-10-20")
	wantBody(t, g, "text", "Reserved instances bought")
	if reports, _ := g.Body["costReportIds"].([]any); len(reports) != 2 || reports[0] != "crp_1" || reports[1] != "crp_2" {
		t.Errorf("sent costReportIds %v", g.Body["costReportIds"])
	}
	if len(g.Query) != 0 {
		t.Errorf("sent query %v; a create names its workspace in the body", g.Query)
	}
	if len(*got) != 1 || annotation.ID != "ann_1" || annotation.Date != "2026-10-20" {
		t.Fatalf("decoded %+v after %d requests", annotation, len(*got))
	}
}

func TestCreateAnnotationWithoutAWorkspaceIsRefusedBeforeSending(t *testing.T) {
	c := NewClient(WithBaseURL("https://api.test"), WithAPIKey("key"))

	if _, err := c.CreateAnnotation(context.Background(), &CreateAnnotationInput{
		Date: "2026-10-20", Text: "n", CostReportIDs: []string{"crp_1"},
	}); err == nil {
		t.Fatal("created an annotation without a workspace")
	}
}

func TestCreateAnnotationKeepsTheVersionItWasStoredAt(t *testing.T) {
	c, _ := etagServer(t)

	annotation, err := c.CreateAnnotation(context.Background(), &CreateAnnotationInput{Date: "2026-10-20", Text: "n"})
	if err != nil {
		t.Fatal(err)
	}
	if annotation.ETag != `"9"` {
		t.Fatalf("ETag %q; want \"9\"", annotation.ETag)
	}
}

func TestResolveAnnotationPostsTheActionAndReturnsTheAnnotation(t *testing.T) {
	resolved := `{"id":"ann_1","date":"2026-10-20","source":"manual","text":"n","costReportIds":["crp_1"],
		"resolvedAt":"2026-10-24T12:00:00Z","resolvedByUserId":"usr_1","createdAt":"2026-10-21T08:00:00Z"}`
	c, got := testClient(t, stub{200, resolved}, stub{200, noteJSON})
	ctx := context.Background()

	annotation, err := c.ResolveAnnotation(ctx, "ws_1", "ann_1")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := c.ReopenAnnotation(ctx, "ws_1", "ann_1")
	if err != nil {
		t.Fatal(err)
	}

	wantRequest(t, (*got)[0], "POST", "/v1/annotations/ann_1/resolve")
	wantRequest(t, (*got)[1], "POST", "/v1/annotations/ann_1/reopen")
	for _, g := range *got {
		wantQuery(t, g, "workspaceId", "ws_1")
		if g.Body != nil {
			t.Errorf("%s %s sent body %v; the action takes none", g.Method, g.Path, g.Body)
		}
	}
	if annotation.ResolvedAt == nil || annotation.ResolvedByUserID == nil || *annotation.ResolvedByUserID != "usr_1" {
		t.Fatalf("decoded the resolved annotation as %+v", annotation)
	}
	if reopened.ResolvedAt != nil {
		t.Fatalf("decoded the reopened annotation as %+v", reopened)
	}
}

// Resolving and reopening are conditional writes like an update: each sends IfMatch and hands back
// the version it produced, for the next write to send.
func TestResolveAnnotationAndReopenSendIfMatchAndKeepTheNewVersion(t *testing.T) {
	actions := map[string]func(c *Client, opts ...RequestOption) (*Annotation, error){
		"resolve": func(c *Client, o ...RequestOption) (*Annotation, error) {
			return c.ResolveAnnotation(context.Background(), "", "ann_1", o...)
		},
		"reopen": func(c *Client, o ...RequestOption) (*Annotation, error) {
			return c.ReopenAnnotation(context.Background(), "", "ann_1", o...)
		},
		"update": func(c *Client, o ...RequestOption) (*Annotation, error) {
			return c.UpdateAnnotation(context.Background(), "", "ann_1", &UpdateAnnotationInput{}, o...)
		},
	}
	for name, action := range actions {
		t.Run(name, func(t *testing.T) {
			c, sent := etagServer(t)

			annotation, err := action(c, IfMatch(`"5"`))
			if err != nil {
				t.Fatal(err)
			}
			if annotation.ETag != `"9"` {
				t.Fatalf("ETag %q; want \"9\"", annotation.ETag)
			}
			if _, err := action(c); err != nil {
				t.Fatal(err)
			}
			if len(*sent) != 2 || (*sent)[0] != `"5"` || (*sent)[1] != "" {
				t.Fatalf("sent If-Match %q; want \"5\", then none without IfMatch", *sent)
			}
		})
	}
}

func TestResolveAnnotationWithAStaleVersionIsAPreconditionFailure(t *testing.T) {
	c, _ := testClient(t, stub{412, `{"status":412,"title":"Precondition Failed","code":"precondition_failed"}`})

	_, err := c.ResolveAnnotation(context.Background(), "ws_1", "ann_1", IfMatch(`"1"`))
	if !IsPreconditionFailed(err) {
		t.Fatalf("got %v, want a precondition failure", err)
	}
}

func TestAnnotationGetUpdateAndDelete(t *testing.T) {
	c, got := testClient(t,
		stub{200, noteJSON},
		stub{200, `{"id":"ann_1","date":"2026-10-21","source":"manual","text":"Moved a day","costReportIds":["crp_2"],
			"createdAt":"2026-10-21T08:00:00Z","updatedAt":"2026-10-24T10:00:00Z"}`},
		stub{204, ""},
	)
	ctx := context.Background()

	annotation, err := c.GetAnnotation(ctx, "ws_1", "ann_1")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := c.UpdateAnnotation(ctx, "ws_1", "ann_1", &UpdateAnnotationInput{
		Date: "2026-10-21", Text: "Moved a day", CostReportIDs: []string{"crp_2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteAnnotation(ctx, "ws_1", "ann_1"); err != nil {
		t.Fatal(err)
	}

	wantRequest(t, (*got)[0], "GET", "/v1/annotations/ann_1")
	wantRequest(t, (*got)[1], "PUT", "/v1/annotations/ann_1")
	wantBody(t, (*got)[1], "date", "2026-10-21")
	wantBody(t, (*got)[1], "text", "Moved a day")
	if _, sent := (*got)[1].Body["workspaceId"]; sent {
		t.Error("an update sent workspaceId in the body; it names the workspace in the query")
	}
	wantRequest(t, (*got)[2], "DELETE", "/v1/annotations/ann_1")
	for _, g := range *got {
		wantQuery(t, g, "workspaceId", "ws_1")
	}
	if annotation.Text == nil || *annotation.Text != "Reserved instances bought" {
		t.Fatalf("decoded %+v", annotation)
	}
	if updated.UpdatedAt == nil || len(updated.CostReportIDs) != 1 || updated.CostReportIDs[0] != "crp_2" {
		t.Fatalf("decoded the update as %+v", updated)
	}
}

// A budget's or cost alert's annotation records what fired and is not edited; the API answers 409.
func TestAnnotationUpdateOfAnAlertsFiringIsAConflict(t *testing.T) {
	c, _ := testClient(t, stub{409, `{"status":409,"title":"Conflict","code":"conflict"}`})

	_, err := c.UpdateAnnotation(context.Background(), "ws_1", "ann_2", &UpdateAnnotationInput{
		Date: "2026-10-22", Text: "n", CostReportIDs: []string{"crp_1"},
	})
	if !IsConflict(err) {
		t.Fatalf("got %v, want a conflict", err)
	}
}
