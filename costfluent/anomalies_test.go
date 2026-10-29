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

func TestListAnomaliesDecodesTheAttribution(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/anomalies"},"data":[{"id":"anm_1",
		"workspaceId":"ws_1","providerId":"prv_1","anomalyDate":"2026-09-20","serviceName":"Amazon EC2","region":"eu-central-1",
		"anomalyType":"spike","severity":"high","expectedCost":"10.00000000","actualCost":"440.00000000",
		"deviationPercent":4300,"currency":"EUR","status":"open","isAcknowledged":false,"detectedAt":"2026-09-21T02:00:00Z",
		"attributedResourceId":"i-0abc","contributors":[
			{"accountId":"111111111111","resourceId":"i-0abc","increase":"412.00000000"},
			{"accountId":null,"resourceId":"bucket-logs","increase":"3.25000000"}],
		"attributedSku":"EUC1-BoxUsage:m7i.4xlarge","skuContributors":[
			{"sku":"EUC1-BoxUsage:m7i.4xlarge","increase":"412.00000000"}]}]}`})

	list, err := c.ListAnomalies(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a := list.Data[0]
	if a.Region == nil || *a.Region != "eu-central-1" || a.AttributedResourceID == nil || *a.AttributedResourceID != "i-0abc" ||
		a.AttributedSku == nil || *a.AttributedSku != "EUC1-BoxUsage:m7i.4xlarge" {
		t.Fatalf("decoded attribution %+v", a)
	}
	if len(a.Contributors) != 2 || a.Contributors[0].AccountID == nil || *a.Contributors[0].AccountID != "111111111111" ||
		a.Contributors[0].ResourceID != "i-0abc" || a.Contributors[0].Increase != "412.00000000" ||
		a.Contributors[1].AccountID != nil || a.Contributors[1].ResourceID != "bucket-logs" {
		t.Fatalf("decoded contributors %+v", a.Contributors)
	}
	if len(a.SkuContributors) != 1 || a.SkuContributors[0].Sku != "EUC1-BoxUsage:m7i.4xlarge" || a.SkuContributors[0].Increase != "412.00000000" {
		t.Fatalf("decoded SKU contributors %+v", a.SkuContributors)
	}
}

// An anomaly read from a stack that predates the attribution decodes with none.
func TestListAnomaliesWithoutTheAttributionDecodesNone(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/anomalies"},"data":[{"id":"anm_1",
		"workspaceId":"ws_1","providerId":"prv_1","anomalyDate":"2026-09-20","serviceName":"EC2","anomalyType":"spike",
		"severity":"high","expectedCost":"10.00000000","actualCost":"40.00000000","deviationPercent":300,"currency":"EUR",
		"status":"open","isAcknowledged":false,"detectedAt":"2026-09-21T02:00:00Z"}]}`})

	list, err := c.ListAnomalies(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	a := list.Data[0]
	if a.AttributedResourceID != nil || a.AttributedSku != nil || len(a.Contributors) != 0 || len(a.SkuContributors) != 0 {
		t.Fatalf("decoded %+v", a)
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
