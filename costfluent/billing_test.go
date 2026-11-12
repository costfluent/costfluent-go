package costfluent

import (
	"context"
	"testing"
)

func TestGetEntitlementDecodesTheHaltCause(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"planCode":"Free","ingestionHalted":true,"ingestionHaltedReason":"Unpaid invoice past the grace window","ingestionHaltCause":"unpaid","ladder":[]}`},
		stub{200, `{"planCode":"Free","ingestionHalted":false,"ingestionHaltedReason":"","ladder":[]}`})

	halted, err := c.GetEntitlement(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cause := halted.IngestionHaltCause; cause == nil || *cause != "unpaid" {
		t.Errorf("halted cause = %v, want unpaid", cause)
	}

	running, err := c.GetEntitlement(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cause := running.IngestionHaltCause; cause != nil {
		t.Errorf("running cause = %q, want nil", *cause)
	}
}

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

// The API withholds the organization's tracked spend from a token that does not read costs in every
// workspace by sending null, which must decode as unset rather than as an amount of zero.
func TestGetUsageDecodesAWithheldSpendAsUnset(t *testing.T) {
	c, got := testClient(t, stub{200, `{"periodStart":"2026-07-01","currency":"EUR",
		"trackedSpend":null,"isSpendComplete":false,"unconvertedCurrencies":["GBP"],
		"rowsIngested":120000,"bytesStored":0,"connectionCount":4,"activeUserCount":1,
		"averageTrackedSpend":null,"bandPlanCode":"Business",
		"history":[{"periodStart":"2026-06-01","trackedSpend":null,"rowsIngested":90000,"bytesStored":0}]}`})

	usage, err := c.GetUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/usage")
	if usage.TrackedSpend.IsSet() {
		t.Errorf("trackedSpend = %q, want unset", usage.TrackedSpend)
	}
	if usage.AverageTrackedSpend != nil {
		t.Errorf("averageTrackedSpend = %q, want nil", *usage.AverageTrackedSpend)
	}
	if len(usage.History) != 1 || usage.History[0].TrackedSpend.IsSet() {
		t.Errorf("history = %+v, want one month with its spend unset", usage.History)
	}
	if usage.RowsIngested != 120000 || usage.ConnectionCount != 4 || usage.History[0].RowsIngested != 90000 {
		t.Errorf("counters = %+v, want them read", usage)
	}
}
