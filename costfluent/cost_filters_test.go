package costfluent

import (
	"context"
	"testing"
)

func TestTranslateCostFilterSendsTheTextAndReadsBothSpellings(t *testing.T) {
	c, got := testClient(t, stub{200, `{"filter":"[{\"field\":\"ServiceName\",\"operator\":\"In\",\"values\":[\"Amazon EC2\"]}]","query":"costs.service = 'Amazon EC2'"}`})

	tr, err := c.TranslateCostFilter(context.Background(), "costs.service='Amazon EC2'")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/cost-filters/translate")
	wantQuery(t, (*got)[0], "filter", "costs.service='Amazon EC2'")
	if tr.Query != "costs.service = 'Amazon EC2'" || tr.Filter == "" {
		t.Fatalf("decoded %+v", tr)
	}
}
