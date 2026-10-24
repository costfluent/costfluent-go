package costfluent

import (
	"context"
	"testing"
)

func TestListAnomalies(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/anomalies?page=2&limit=20",
		"next":"https://api.test/v1/anomalies?page=3&limit=20"},"data":[{"id":"anm_1","workspaceId":"ws_1","providerId":"prv_1",
		"anomalyDate":"2026-09-20","serviceName":"EC2","anomalyType":"spike","severity":"high","expectedCost":"10.00000000",
		"actualCost":"40.00000000","deviationPercent":300,"currency":"EUR","status":"ignored",
		"ignoreReason":"Planned load test","isAcknowledged":false,
		"detectedAt":"2026-09-21T02:00:00Z"}]}`})

	list, err := c.ListAnomalies(context.Background(), &ListAnomaliesOptions{
		Severity: "high", UnacknowledgedOnly: true, PageOptions: PageOptions{Page: 2, Limit: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/anomalies")
	wantQuery(t, g, "severity", "high")
	wantQuery(t, g, "unacknowledgedOnly", "true")
	wantQuery(t, g, "page", "2")
	wantQuery(t, g, "limit", "20")
	a := list.Data[0]
	if !list.Links.HasNextPage() || a.DeviationPercent != 300 || a.ProviderID != "prv_1" || a.WorkspaceID != "ws_1" ||
		a.Status != "ignored" || a.IgnoreReason == nil || *a.IgnoreReason != "Planned load test" || a.StatusReason != nil {
		t.Fatalf("decoded %+v", list)
	}
}

func TestAcknowledgeAnomalySendsNoBody(t *testing.T) {
	c, got := testClient(t, stub{204, ""})

	if err := c.AcknowledgeAnomaly(context.Background(), "anm_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/anomalies/anm_1/acknowledge")
	if (*got)[0].Body != nil {
		t.Errorf("sent body %v", (*got)[0].Body)
	}
}
