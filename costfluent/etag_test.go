package costfluent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// etagServer answers every request with an empty object and the ETag "9", and records the
// If-Match each request carried.
func etagServer(t *testing.T) (*Client, *[]string) {
	t.Helper()
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = append(sent, r.Header.Get("If-Match"))
		w.Header().Set("ETag", `"9"`)
		if r.Method == http.MethodDelete || bodiless(r.URL.Path) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"x_1"}`)
	}))
	t.Cleanup(srv.Close)
	return NewClient(WithBaseURL(srv.URL), WithAPIKey("key"), WithWorkspace("ws_1")), &sent
}

// bodiless is an action the API answers with 204 and the new version's ETag.
func bodiless(path string) bool {
	for _, action := range []string{"/pause", "/resume", "/activate", "/deactivate"} {
		if strings.HasSuffix(path, action) {
			return true
		}
	}
	return false
}

// Every read of a row-versioned resource hands back the ETag its response carried.
func TestReadsOfVersionedResourcesKeepTheirETag(t *testing.T) {
	ctx := context.Background()
	reads := map[string]func(c *Client) (string, error){
		"app": func(c *Client) (string, error) { r, err := c.GetApp(ctx, "a"); return etagOf(r, err) },
		"budget": func(c *Client) (string, error) {
			r, err := c.GetBudget(ctx, "b")
			return etagOf(r, err)
		},
		"business metric": func(c *Client) (string, error) {
			r, err := c.GetBusinessMetric(ctx, "", "m")
			return etagOf(r, err)
		},
		"cost alert": func(c *Client) (string, error) {
			r, err := c.GetCostAlert(ctx, "", "a")
			return etagOf(r, err)
		},
		"cost report": func(c *Client) (string, error) {
			r, err := c.GetCostReport(ctx, "", "r")
			return etagOf(r, err)
		},
		"dashboard": func(c *Client) (string, error) {
			r, err := c.GetDashboard(ctx, "", "d")
			return etagOf(r, err)
		},
		"dashboard widget": func(c *Client) (string, error) {
			r, err := c.GetDashboardWidget(ctx, "", "d", "w")
			return etagOf(r, err)
		},
		"folder": func(c *Client) (string, error) { r, err := c.GetFolder(ctx, "", "f"); return etagOf(r, err) },
		"saved filter": func(c *Client) (string, error) {
			r, err := c.GetSavedFilter(ctx, "", "f")
			return etagOf(r, err)
		},
		"team": func(c *Client) (string, error) { r, err := c.GetTeam(ctx, "t"); return etagOf(r, err) },
		"virtual tag": func(c *Client) (string, error) {
			r, err := c.GetVirtualTag(ctx, "", "v")
			return etagOf(r, err)
		},
		"workspace": func(c *Client) (string, error) { r, err := c.GetWorkspace(ctx, "w"); return etagOf(r, err) },
		"updated folder": func(c *Client) (string, error) {
			r, err := c.UpdateFolder(ctx, "f", &UpdateFolderInput{})
			return etagOf(r, err)
		},
		"provider": func(c *Client) (string, error) { r, err := c.GetProvider(ctx, "p"); return etagOf(r, err) },
		"created provider": func(c *Client) (string, error) {
			r, err := c.CreateProvider(ctx, &CreateProviderInput{})
			return etagOf(r, err)
		},
		"updated provider": func(c *Client) (string, error) {
			r, err := c.UpdateProvider(ctx, "p", &UpdateProviderInput{})
			return etagOf(r, err)
		},
		"created segment": func(c *Client) (string, error) {
			r, err := c.CreateSegment(ctx, &CreateSegmentInput{})
			return etagOf(r, err)
		},
		"updated segment": func(c *Client) (string, error) {
			r, err := c.UpdateSegment(ctx, "", "s", &UpdateSegmentInput{})
			return etagOf(r, err)
		},
		"created allocation rule": func(c *Client) (string, error) {
			r, err := c.CreateAllocationRule(ctx, &CreateAllocationRuleInput{})
			return etagOf(r, err)
		},
		"updated allocation rule": func(c *Client) (string, error) {
			r, err := c.UpdateAllocationRule(ctx, "", "r", &UpdateAllocationRuleInput{})
			return etagOf(r, err)
		},
		"cost report subscription": func(c *Client) (string, error) {
			r, err := c.GetCostReportSubscription(ctx, "", "s")
			return etagOf(r, err)
		},
		"updated cost alert": func(c *Client) (string, error) {
			r, err := c.UpdateCostAlert(ctx, "", "a", &UpdateCostAlertInput{})
			return etagOf(r, err)
		},
		"updated virtual tag": func(c *Client) (string, error) {
			r, err := c.UpdateVirtualTag(ctx, "v", &UpdateVirtualTagInput{})
			return etagOf(r, err)
		},
		"updated cost report subscription": func(c *Client) (string, error) {
			r, err := c.UpdateCostReportSubscription(ctx, "s", &UpdateCostReportSubscriptionInput{})
			return etagOf(r, err)
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			c, _ := etagServer(t)
			etag, err := read(c)
			if err != nil || etag != `"9"` {
				t.Fatalf("ETag %q, err %v; want \"9\"", etag, err)
			}
		})
	}
}

// Every write and deletion of a row-versioned resource sends IfMatch as the If-Match header, and
// sends none without it.
func TestWritesOfVersionedResourcesSendIfMatch(t *testing.T) {
	ctx := context.Background()
	writes := map[string]func(c *Client, opts ...RequestOption) error{
		"update app": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateApp(ctx, "a", &UpdateAppInput{}, o...)
			return err
		},
		"delete app": func(c *Client, o ...RequestOption) error { return c.DeleteApp(ctx, "a", o...) },
		"update budget": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateBudget(ctx, "b", &UpdateBudgetInput{}, o...)
			return err
		},
		"delete budget": func(c *Client, o ...RequestOption) error { return c.DeleteBudget(ctx, "b", o...) },
		"update business metric": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateBusinessMetric(ctx, "", "m", &UpdateBusinessMetricInput{}, o...)
			return err
		},
		"delete business metric": func(c *Client, o ...RequestOption) error { return c.DeleteBusinessMetric(ctx, "", "m", o...) },
		"delete cost alert":      func(c *Client, o ...RequestOption) error { return c.DeleteCostAlert(ctx, "", "a", o...) },
		"pause cost alert":       func(c *Client, o ...RequestOption) error { return c.PauseCostAlert(ctx, "", "a", o...) },
		"resume cost alert":      func(c *Client, o ...RequestOption) error { return c.ResumeCostAlert(ctx, "", "a", o...) },
		"update cost report": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateCostReport(ctx, "", "r", &UpdateCostReportInput{}, o...)
			return err
		},
		"delete cost report": func(c *Client, o ...RequestOption) error { return c.DeleteCostReport(ctx, "", "r", o...) },
		"update dashboard": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateDashboard(ctx, "", "d", &UpdateDashboardInput{}, o...)
			return err
		},
		"delete dashboard": func(c *Client, o ...RequestOption) error { return c.DeleteDashboard(ctx, "", "d", o...) },
		"set dashboard saved filter": func(c *Client, o ...RequestOption) error {
			_, err := c.SetDashboardSavedFilter(ctx, "", "d", nil, o...)
			return err
		},
		"update dashboard widget": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateDashboardWidget(ctx, "", "d", "w", &UpdateDashboardWidgetInput{}, o...)
			return err
		},
		"delete dashboard widget": func(c *Client, o ...RequestOption) error {
			return c.DeleteDashboardWidget(ctx, "", "d", "w", o...)
		},
		"update folder": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateFolder(ctx, "f", &UpdateFolderInput{}, o...)
			return err
		},
		"delete folder": func(c *Client, o ...RequestOption) error { return c.DeleteFolder(ctx, "", "f", o...) },
		"set folder saved filter": func(c *Client, o ...RequestOption) error {
			_, err := c.SetFolderSavedFilter(ctx, "", "f", nil, o...)
			return err
		},
		"update saved filter": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateSavedFilter(ctx, "", "f", &UpdateSavedFilterInput{}, o...)
			return err
		},
		"delete saved filter": func(c *Client, o ...RequestOption) error { return c.DeleteSavedFilter(ctx, "", "f", o...) },
		"update team": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateTeam(ctx, "t", &UpdateTeamInput{}, o...)
			return err
		},
		"delete team": func(c *Client, o ...RequestOption) error { return c.DeleteTeam(ctx, "t", o...) },
		"update virtual tag": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateVirtualTag(ctx, "v", &UpdateVirtualTagInput{}, o...)
			return err
		},
		"delete virtual tag": func(c *Client, o ...RequestOption) error { return c.DeleteVirtualTag(ctx, "", "v", o...) },
		"update workspace": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateWorkspace(ctx, "w", &UpdateWorkspaceInput{}, o...)
			return err
		},
		"delete workspace": func(c *Client, o ...RequestOption) error { return c.DeleteWorkspace(ctx, "w", o...) },
		"update provider": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateProvider(ctx, "p", &UpdateProviderInput{}, o...)
			return err
		},
		"delete provider": func(c *Client, o ...RequestOption) error { return c.DeleteProvider(ctx, "p", o...) },
		"update segment": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateSegment(ctx, "", "s", &UpdateSegmentInput{}, o...)
			return err
		},
		"delete segment": func(c *Client, o ...RequestOption) error { return c.DeleteSegment(ctx, "", "s", o...) },
		"update allocation rule": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateAllocationRule(ctx, "", "r", &UpdateAllocationRuleInput{}, o...)
			return err
		},
		"delete allocation rule": func(c *Client, o ...RequestOption) error {
			return c.DeleteAllocationRule(ctx, "", "r", o...)
		},
		"update cost report subscription": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateCostReportSubscription(ctx, "s", &UpdateCostReportSubscriptionInput{}, o...)
			return err
		},
		"delete cost report subscription": func(c *Client, o ...RequestOption) error {
			return c.DeleteCostReportSubscription(ctx, "", "s", o...)
		},
		"activate cost report subscription": func(c *Client, o ...RequestOption) error {
			return c.ActivateCostReportSubscription(ctx, "", "s", o...)
		},
		"deactivate cost report subscription": func(c *Client, o ...RequestOption) error {
			return c.DeactivateCostReportSubscription(ctx, "", "s", o...)
		},
		"update cost alert": func(c *Client, o ...RequestOption) error {
			_, err := c.UpdateCostAlert(ctx, "", "a", &UpdateCostAlertInput{}, o...)
			return err
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			c, sent := etagServer(t)
			if err := write(c, IfMatch(`"5"`)); err != nil {
				t.Fatal(err)
			}
			if err := write(c, IfMatch("")); err != nil {
				t.Fatal(err)
			}
			if err := write(c); err != nil {
				t.Fatal(err)
			}
			if len(*sent) == 0 || (*sent)[0] != `"5"` {
				t.Fatalf("sent If-Match %q; want \"5\" first", *sent)
			}
			for _, unconditional := range (*sent)[1:] {
				if unconditional == `"5"` {
					t.Fatalf("sent If-Match %q; want none without IfMatch", *sent)
				}
			}
		})
	}
}

// An update the API answers with the resource keeps that answer and its ETag: it sends one
// request, so no read-back can return a later version than the one the write produced.
func TestUpdatesReturnTheVersionTheirWriteProduced(t *testing.T) {
	ctx := context.Background()
	updates := map[string]func(c *Client) (string, error){
		"cost alert": func(c *Client) (string, error) {
			r, err := c.UpdateCostAlert(ctx, "", "a", &UpdateCostAlertInput{}, IfMatch(`"5"`))
			return etagOf(r, err)
		},
		"virtual tag": func(c *Client) (string, error) {
			r, err := c.UpdateVirtualTag(ctx, "v", &UpdateVirtualTagInput{}, IfMatch(`"5"`))
			return etagOf(r, err)
		},
		"cost report subscription": func(c *Client) (string, error) {
			r, err := c.UpdateCostReportSubscription(ctx, "s", &UpdateCostReportSubscriptionInput{}, IfMatch(`"5"`))
			return etagOf(r, err)
		},
	}
	for name, update := range updates {
		t.Run(name, func(t *testing.T) {
			c, sent := etagServer(t)
			etag, err := update(c)
			if err != nil || etag != `"9"` {
				t.Fatalf("ETag %q, err %v; want the update's \"9\"", etag, err)
			}
			if len(*sent) != 1 || (*sent)[0] != `"5"` {
				t.Fatalf("sent If-Match %q; want one conditional write and no read-back", *sent)
			}
		})
	}
}

func etagOf[T any](resource *T, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return reflect.ValueOf(resource).Elem().FieldByName("ETag").String(), nil
}
