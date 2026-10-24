package costfluent

import (
	"context"
	"testing"
)

func TestCreateBusinessMetricReadsItBack(t *testing.T) {
	c, got := testClient(t,
		stub{201, `{"id":"bmt_1","name":"Customers"}`},
		stub{200, `{"id":"bmt_1","name":"Customers","unit":"customer","labelColumns":["region"],"createdAt":"2026-09-01T00:00:00Z"}`},
	)

	m, err := c.CreateBusinessMetric(context.Background(), &CreateBusinessMetricInput{
		Name: "Customers", Unit: "customer", LabelColumns: []string{"region"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/business-metrics")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "unit", "customer")
	wantRequest(t, (*got)[1], "GET", "/v1/business-metrics/bmt_1")
	if m.Unit != "customer" || len(m.LabelColumns) != 1 || m.LabelColumns[0] != "region" {
		t.Fatalf("decoded %+v", m)
	}
}

func TestUpdateBusinessMetricIsPutWithOnlyWhatChanges(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"bmt_1","name":"Orders","unit":"order","labelColumns":[]}`})

	name := "Orders"
	if _, err := c.UpdateBusinessMetric(context.Background(), "ws_1", "bmt_1", &UpdateBusinessMetricInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/business-metrics/bmt_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	wantBody(t, (*got)[0], "name", "Orders")
	if _, sent := (*got)[0].Body["unit"]; sent {
		t.Fatalf("an unset unit was sent: %v", (*got)[0].Body)
	}
}

func TestImportBusinessMetricValuesSendsExactAmounts(t *testing.T) {
	c, got := testClient(t, stub{200, `{"created":1,"updated":0,"unchanged":0}`})

	result, err := c.ImportBusinessMetricValues(context.Background(), "ws_1", "bmt_1", []BusinessMetricValue{
		{Date: "2026-09-01", Amount: "120.5", Labels: map[string]string{"region": "eu"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/business-metrics/bmt_1/values")
	values, ok := (*got)[0].Body["values"].([]any)
	if !ok || len(values) != 1 || values[0].(map[string]any)["amount"] != "120.5" {
		t.Fatalf("sent %v", (*got)[0].Body)
	}
	if result.Created != 1 {
		t.Fatalf("decoded %+v", result)
	}
}

func TestListBusinessMetricValuesNarrowsByDayAndPage(t *testing.T) {
	c, got := testClient(t, stub{200, `{"data":[{"date":"2026-09-01","amount":"3.00000000","labels":{"region":"eu"}}],"links":{}}`})

	page, err := c.ListBusinessMetricValues(context.Background(), "ws_1", "bmt_1", &BusinessMetricValuesOptions{
		PageOptions: PageOptions{Limit: 500}, StartDate: "2026-09-01", EndDate: "2026-09-30",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/business-metrics/bmt_1/values")
	wantQuery(t, (*got)[0], "startDate", "2026-09-01")
	wantQuery(t, (*got)[0], "endDate", "2026-09-30")
	for _, gone := range []string{"from", "to"} {
		if _, sent := (*got)[0].Query[gone]; sent {
			t.Errorf("sent %s, which the API no longer reads", gone)
		}
	}
	wantQuery(t, (*got)[0], "limit", "500")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	if len(page.Data) != 1 || page.Data[0].Amount != "3.00000000" || page.Data[0].Labels["region"] != "eu" {
		t.Fatalf("decoded %+v", page)
	}
}

func TestListAllBusinessMetricsAndDelete(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"data":[{"id":"bmt_1","name":"Customers","unit":"customer","labelColumns":[]}],"links":{}}`},
		stub{204, ``},
	)

	all, err := c.ListAllBusinessMetrics(context.Background(), "ws_1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != "bmt_1" {
		t.Fatalf("decoded %+v", all)
	}
	if err := c.DeleteBusinessMetric(context.Background(), "ws_1", "bmt_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[1], "DELETE", "/v1/business-metrics/bmt_1")
}
