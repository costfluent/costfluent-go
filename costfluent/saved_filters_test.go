package costfluent

import (
	"context"
	"testing"
)

func TestCreateSavedFilterReadsItBack(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"flt_1","title":"EC2"}`},
		stub{200, `{"id":"flt_1","title":"EC2","filter":"service = 'EC2'","isDefault":false,"createdAt":"2026-09-01T00:00:00Z"}`},
	)

	f, err := c.CreateSavedFilter(context.Background(), &CreateSavedFilterInput{Title: "EC2", Filter: "service = 'EC2'"})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/saved-filters")
	wantBody(t, (*got)[0], "filter", "service = 'EC2'")
	wantRequest(t, (*got)[1], "GET", "/v1/saved-filters/flt_1")
	if f.Filter != "service = 'EC2'" || f.Title != "EC2" {
		t.Fatalf("decoded %+v", f)
	}
}

func TestUpdateSavedFilterIsPut(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"flt_1","title":"EC2","filter":"x"}`})

	isDefault := true
	if _, err := c.UpdateSavedFilter(context.Background(), "ws_1", "flt_1", &UpdateSavedFilterInput{IsDefault: &isDefault}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/saved-filters/flt_1")
	wantBody(t, (*got)[0], "isDefault", true)
}
