package costfluent

import (
	"context"
	"testing"
)

// Custom is quoted, not priced: the API omits its price, which decodes as nil rather than zero, so
// a caller never reads a published fee of nothing.
func TestGetEntitlementDecodesAQuotedRungWithoutAPrice(t *testing.T) {
	c, got := testClient(t, stub{200, `{"planCode":"Pro","ladder":[
		{"code":"Pro","name":"Pro","priceAmount":"25","currency":"EUR","spendCeiling":"7500","historyDays":183,"features":[]},
		{"code":"Custom","name":"Custom","currency":"EUR","historyDays":1095,"features":[]}]}`})

	entitlement, err := c.GetEntitlement(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/entitlement")
	if len(entitlement.Ladder) != 2 {
		t.Fatalf("ladder = %+v", entitlement.Ladder)
	}
	if price := entitlement.Ladder[0].PriceAmount; price == nil || price.String() != "25" {
		t.Errorf("Pro price = %v, want 25", price)
	}
	if price := entitlement.Ladder[1].PriceAmount; price != nil {
		t.Errorf("Custom price = %v, want nil", price)
	}
}
