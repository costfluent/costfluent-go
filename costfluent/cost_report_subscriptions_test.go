package costfluent

import (
	"context"
	"testing"
)

const subscriptionJSON = `{"id":"crs_1","costReportId":"rpt_1","name":"Weekly","scheduleType":"weekly",
	"deliveryHour":9,"timeZone":"Europe/Berlin","appIds":["app_1"],"isActive":true}`

func TestCreateCostReportSubscriptionReadsItBackInItsWorkspace(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"crs_1","name":"Weekly"}`}, stub{200, subscriptionJSON})

	sub, err := c.CreateCostReportSubscription(context.Background(), &CreateCostReportSubscriptionInput{
		WorkspaceID: "wsp_1", CostReportID: "rpt_1", Name: "Weekly", ScheduleType: "weekly", AppIDs: []string{"app_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/cost-report-subscriptions")
	wantBody(t, (*got)[0], "workspaceId", "wsp_1")
	if _, sent := (*got)[0].Body["deliveryHour"]; sent {
		t.Errorf("an unset hour was sent: %v", (*got)[0].Body)
	}
	wantRequest(t, (*got)[1], "GET", "/v1/cost-report-subscriptions/crs_1")
	wantQuery(t, (*got)[1], "workspaceId", "wsp_1")
	if sub.TimeZone != "Europe/Berlin" || sub.AppIDs[0] != "app_1" {
		t.Fatalf("decoded %+v", sub)
	}
}

func TestUpdateCostReportSubscriptionReturnsWhatItsPatchAnswered(t *testing.T) {
	c, got := testClient(t, stub{200, subscriptionJSON})

	hour := 7
	sub, err := c.UpdateCostReportSubscription(context.Background(), "crs_1", &UpdateCostReportSubscriptionInput{
		WorkspaceID: "wsp_1", DeliveryHour: &hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want the PATCH alone", len(*got))
	}
	wantRequest(t, (*got)[0], "PATCH", "/v1/cost-report-subscriptions/crs_1")
	wantBody(t, (*got)[0], "deliveryHour", float64(7))
	if len((*got)[0].Body) != 2 {
		t.Errorf("unset fields were sent: %v", (*got)[0].Body)
	}
	if sub.ID != "crs_1" || sub.TimeZone != "Europe/Berlin" || sub.AppIDs[0] != "app_1" {
		t.Fatalf("decoded %+v", sub)
	}
}

func TestPausingAndResumingNameTheWorkspace(t *testing.T) {
	c, got := testClient(t, stub{204, ``}, stub{204, ``})

	if err := c.DeactivateCostReportSubscription(context.Background(), "wsp_1", "crs_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.ActivateCostReportSubscription(context.Background(), "wsp_1", "crs_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/cost-report-subscriptions/crs_1/deactivate")
	wantQuery(t, (*got)[0], "workspaceId", "wsp_1")
	wantRequest(t, (*got)[1], "POST", "/v1/cost-report-subscriptions/crs_1/activate")
}
