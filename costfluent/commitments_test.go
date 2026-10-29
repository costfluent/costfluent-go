package costfluent

import (
	"context"
	"testing"
)

const commitmentJSON = `{"id":"ri-soon","providerId":"prv_1","providerName":"AWS Production","provider":"aws",
	"type":"ReservedInstance","description":"2 x m5.large","account":"111122223333","region":"eu-central-1",
	"service":"Amazon EC2","status":"expiring","providerStatus":"Active","startsAt":"2025-10-23T12:00:00Z",
	"endsAt":"2026-10-23T12:00:00Z","daysUntilExpiry":10,"termYears":1,"paymentOption":"Partial Upfront",
	"hourlyCommitment":"0.07000000","fullTermCost":"613.20000000","utilizationPercentage":87.5,
	"unusedCost":"12.25000000","currency":"USD","utilizationFrom":"2026-09-13","utilizationTo":"2026-10-13",
	"lastReadAt":"2026-10-13T12:00:00Z"}`

func TestListCommitments(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/commitments?page=1&limit=20"},"data":[` + commitmentJSON + `]}`})

	list, err := c.ListCommitments(context.Background(), "ws_1", &ListCommitmentsOptions{
		Provider: "aws", Status: "expiring", Type: "ReservedInstance", Service: "Amazon EC2", Account: "111122223333",
		Region: "eu-central-1", TermYears: 1, PaymentOption: "Partial Upfront", PageOptions: PageOptions{Page: 1, Limit: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/commitments")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantQuery(t, g, "provider", "aws")
	wantQuery(t, g, "status", "expiring")
	wantQuery(t, g, "type", "ReservedInstance")
	wantQuery(t, g, "service", "Amazon EC2")
	wantQuery(t, g, "account", "111122223333")
	wantQuery(t, g, "region", "eu-central-1")
	wantQuery(t, g, "termYears", "1")
	wantQuery(t, g, "paymentOption", "Partial Upfront")
	cm := list.Data[0]
	if cm.ProviderID != "prv_1" || cm.Status != "expiring" || cm.DaysUntilExpiry == nil || *cm.DaysUntilExpiry != 10 ||
		cm.UtilizationPercentage == nil || *cm.UtilizationPercentage != 87.5 || cm.FullTermCost == nil ||
		*cm.FullTermCost != "613.20000000" || cm.EndsAt == nil || cm.UtilizationTo != "2026-10-13" {
		t.Fatalf("decoded %+v", cm)
	}
}

// A commitment without a stated end has no term, cost split or expiry, and a zero term sends no filter.
func TestListCommitmentsWithoutAnEndDecodesNoneOfItsTerm(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/commitments"},"data":[{"id":"sp-open",
		"providerId":"prv_1","providerName":"AWS","provider":"aws","type":"SavingsPlan","description":"Compute Savings Plan",
		"account":"1","region":"","service":"","status":"unknown","providerStatus":"","currency":"USD",
		"utilizationFrom":"2026-09-13","utilizationTo":"2026-10-13","lastReadAt":"2026-10-13T12:00:00Z"}]}`})

	list, err := c.ListCommitments(context.Background(), "ws_1", &ListCommitmentsOptions{Status: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if _, sent := (*got)[0].Query["termYears"]; sent {
		t.Error("sent termYears for a zero term")
	}
	cm := list.Data[0]
	if cm.EndsAt != nil || cm.TermYears != nil || cm.DaysUntilExpiry != nil || cm.PaymentOption != nil || cm.FullTermCost != nil {
		t.Fatalf("decoded %+v", cm)
	}
}

func TestListAllCommitmentsWalksEveryPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"https://api.test/v1/commitments?page=1","next":"https://api.test/v1/commitments?page=2"},"data":[` + commitmentJSON + `]}`},
		stub{200, `{"links":{"self":"https://api.test/v1/commitments?page=2"},"data":[` + commitmentJSON + `]}`},
	)

	all, err := c.ListAllCommitments(context.Background(), "ws_1", &ListCommitmentsOptions{Provider: "aws"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d commitments, want 2", len(all))
	}
	for _, g := range *got {
		wantQuery(t, g, "provider", "aws")
	}
}

func TestGetCommitmentCoverage(t *testing.T) {
	c, got := testClient(t, stub{200, `{"currency":"USD","granularity":"month","data":[{"date":"2026-03-01","provider":"aws",
		"coveredOnDemandCost":"10.00000000","eligibleOnDemandCost":"14.00000000","actualCost":"11.00000000",
		"usedCommitment":"6.00000000","unusedCommitment":"1.00000000","coveragePercent":71.43,"effectiveSavingsRatePercent":21.43,
		"utilizationPercent":85.71}],"totals":[{"date":null,"provider":"aws","coveredOnDemandCost":"10.00000000",
		"eligibleOnDemandCost":"14.00000000","actualCost":"11.00000000","usedCommitment":"6.00000000","unusedCommitment":null,
		"coveragePercent":71.43,"effectiveSavingsRatePercent":21.43,"utilizationPercent":null}],
		"historyFloor":"2026-04-14","unconverted":[{"currency":"TWD","amount":"200.00000000"}]}`})

	coverage, err := c.GetCommitmentCoverage(context.Background(), "ws_1", &CommitmentCoverageOptions{
		StartDate: "2026-03-01", EndDate: "2026-09-30", Granularity: "month", Filter: "costs.provider = 'aws'",
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/commitments/coverage")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantQuery(t, g, "startDate", "2026-03-01")
	wantQuery(t, g, "endDate", "2026-09-30")
	wantQuery(t, g, "granularity", "month")
	wantQuery(t, g, "filter", "costs.provider = 'aws'")
	if coverage.Granularity != "month" || coverage.HistoryFloor == nil || *coverage.HistoryFloor != "2026-04-14" ||
		len(coverage.Unconverted) != 1 || coverage.Data[0].Date == nil || *coverage.Data[0].CoveragePercent != 71.43 ||
		coverage.Totals[0].Date != nil || coverage.Totals[0].UnusedCommitment != nil || coverage.Totals[0].UtilizationPercent != nil {
		t.Fatalf("decoded %+v", coverage)
	}
}
