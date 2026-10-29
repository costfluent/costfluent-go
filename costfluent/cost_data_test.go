package costfluent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestQueryCostData(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[{"date":"2026-09-01","cost":"12.50000000","listCost":"14.00000000",
		"amortizedCost":"12.50000000","currency":"EUR","dimensions":{"ServiceName":"EC2","Tag:team":"payments"}}],
		"meta":{"startDate":"2026-09-01","endDate":"2026-09-30","granularity":"day","totalRecords":1,
		"totalCost":"12.50000000","currency":"EUR","presentation":{"totalsByCurrency":[{"currency":"EUR","amount":"12.50000000"}]},
		"dataVersion":"3f2a9c0d1e4b5a68"}}`})

	credits, team := true, "team"
	resp, err := c.QueryCostData(context.Background(), &CostDataQuery{
		CostFilterOptions: CostFilterOptions{
			StartDate: "2026-09-01", EndDate: "2026-09-30",
			CloudAccountIDs: []string{"a", "b"}, IncludeCredits: &credits,
		},
		Granularity: GranularityDay,
		GroupBy:     []Grouping{{Dimension: "ServiceName"}, {Dimension: "Tag", Key: &team}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/costs")
	wantQuery(t, g, "startDate", "2026-09-01")
	wantQuery(t, g, "granularity", "day")
	wantQuery(t, g, "workspaceId", "ws_default")
	if gb := g.Query["groupBy"]; len(gb) != 2 || gb[0] != "ServiceName" || gb[1] != "Tag:team" {
		t.Errorf("groupBy %v, want one repeated parameter per slot in order", gb)
	}
	wantQuery(t, g, "includeCredits", "true")
	if len(g.Query["cloudAccountIds"]) != 2 {
		t.Errorf("cloudAccountIds %v", g.Query["cloudAccountIds"])
	}
	if _, sent := g.Query["amortize"]; sent {
		t.Error("sent amortize although it was left to the API default")
	}
	if resp.Data[0].Dimensions["ServiceName"] != "EC2" || resp.Data[0].Dimensions["Tag:team"] != "payments" || !resp.Meta.TotalCost.Equal("12.5") ||
		resp.Meta.Presentation.TotalsByCurrency[0].Amount != "12.50000000" || resp.Data[0].ListCost != "14.00000000" || resp.Meta.Granularity != GranularityDay {
		t.Fatalf("decoded %+v", resp)
	}
	if _, sent := g.Query["measure"]; sent {
		t.Error("sent measure although the query reads cost")
	}
	if _, sent := g.Query["dataVersion"]; sent {
		t.Error("sent dataVersion although the query set none")
	}
	if resp.Meta.DataVersion != "3f2a9c0d1e4b5a68" {
		t.Errorf("meta dataVersion %q", resp.Meta.DataVersion)
	}
}

func TestQueryCostDataSendsTheDataVersion(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[],
		"meta":{"startDate":"2026-09-01","endDate":"2026-09-30","granularity":"day","totalRecords":0,
		"totalCost":"0","currency":"EUR","dataVersion":"3f2a9c0d1e4b5a68"}}`})

	_, err := c.QueryCostData(context.Background(), &CostDataQuery{
		CostFilterOptions: CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"},
		DataVersion:       "3f2a9c0d1e4b5a68",
		PageOptions:       PageOptions{Page: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "dataVersion", "3f2a9c0d1e4b5a68")
}

func TestQueryCostDataWhoseDataChangedIsAConflict(t *testing.T) {
	c, _ := testClient(t, stub{409, `{"status":409,"title":"Conflict","code":"conflict",
		"detail":"The cost data changed since the page whose dataVersion was sent; read the window again from page 1"}`})

	_, err := c.QueryCostData(context.Background(), &CostDataQuery{
		CostFilterOptions: CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"},
		DataVersion:       "3f2a9c0d1e4b5a68",
		PageOptions:       PageOptions{Page: 2},
	})
	if !IsConflict(err) {
		t.Fatalf("err %v, want a conflict", err)
	}
}

func TestQueryCostDataByUsage(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[{"date":"2026-09-01","cost":"12.50000000","listCost":"14.00000000",
		"amortizedCost":"12.50000000","currency":"EUR","dimensions":{"ServiceName":"EC2"},"quantity":730.5,"unit":"Hours"}],
		"meta":{"startDate":"2026-09-01","endDate":"2026-09-30","granularity":"day","measure":"usage","totalRecords":1,
		"totalCost":"12.50000000","currency":"EUR"}}`})

	resp, err := c.QueryCostData(context.Background(), &CostDataQuery{
		CostFilterOptions: CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"},
		GroupBy:           []Grouping{{Dimension: "ServiceName"}},
		Measure:           MeasureUsage,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "measure", "usage")
	row := resp.Data[0]
	if row.Quantity == nil || *row.Quantity != 730.5 || row.Unit == nil || *row.Unit != "Hours" || resp.Meta.Measure != MeasureUsage {
		t.Fatalf("decoded %+v", resp)
	}
}

func TestGetCostSummary(t *testing.T) {
	c, got := testClient(t, stub{200, `{"totalCost":"100.00000000","currency":"EUR","costChangePercent":-5,
		"topServices":[{"serviceName":"EC2","cost":"60.00000000","percentage":60}],
		"byProvider":[{"providerId":"prv_1","providerName":"AWS","providerKey":"aws","cost":"100.00000000"}],
		"dataThrough":"2026-09-20","comparedThrough":"2026-09-19"}`})

	summary, err := c.GetCostSummary(context.Background(), &CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/costs/summary")
	wantQuery(t, (*got)[0], "endDate", "2026-09-30")
	if !summary.TotalCost.Equal("100") || summary.ByProvider[0].ProviderID != "prv_1" || summary.TopServices[0].Percentage != 60 ||
		summary.DataThrough == nil || *summary.DataThrough != "2026-09-20" ||
		summary.ComparedThrough == nil || *summary.ComparedThrough != "2026-09-19" {
		t.Fatalf("decoded %+v", summary)
	}
}

// Cost data is a bulk read: without a limit the query asks for the API's largest page, so a client
// pages through a window in as few rate-limited calls as it can.
func TestQueryCostDataAsksForTheLargestPageUnlessToldOtherwise(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"s"},"data":[],"meta":{}}`},
		stub{200, `{"links":{"self":"s"},"data":[],"meta":{}}`})

	if _, err := c.QueryCostData(context.Background(), &CostDataQuery{CostFilterOptions: CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"}}); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "limit", "5000")

	if _, err := c.QueryCostData(context.Background(), &CostDataQuery{
		CostFilterOptions: CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"},
		PageOptions:       PageOptions{Page: 2, Limit: 50},
	}); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[1], "limit", "50")
	wantQuery(t, (*got)[1], "page", "2")
}

// The API sends costChangePercent as null when the previous window was not compared, and currency
// as null when the window mixes currencies. Decoded and written again they must stay null rather
// than read as no change or an empty currency, and a zero change must stay 0.
func TestCostSummaryFiguresTheAPISendsAsNullStayNull(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{"not compared, mixed currencies", `"currency":null,"costChange":null,"costChangePercent":null`,
			[]string{`"currency":null`, `"costChangePercent":null`}},
		{"no change", `"currency":"EUR","costChange":"0.00000000","costChangePercent":0`,
			[]string{`"currency":"EUR"`, `"costChangePercent":0`}},
		{"a change", `"currency":"USD","costChange":"-5.00000000","costChangePercent":-5`,
			[]string{`"currency":"USD"`, `"costChangePercent":-5`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := testClient(t, stub{200, `{"totalCost":"100.00000000",` + tc.body + `}`})
			summary, err := c.GetCostSummary(context.Background(), &CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"})
			if err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(summary)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(string(out), w) {
					t.Errorf("written as %s, want %s", out, w)
				}
			}
		})
	}
}

// A read states how its amortized figures placed commitment discounts, on a page of cost data and
// on a summary alike.
func TestCostReadsStateHowCommitmentDiscountsArePlaced(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"links":{"self":"s"},"data":[],"meta":{"startDate":"2026-09-01","endDate":"2026-09-30",
			"granularity":"day","totalRecords":0,"totalCost":"0","currency":"USD","commitmentDiscounts":"blended",
			"dataVersion":"3f2a9c0d1e4b5a68"}}`},
		stub{200, `{"totalCost":"100.00000000","currency":"USD","commitmentDiscounts":"asApplied"}`},
	)
	window := CostFilterOptions{StartDate: "2026-09-01", EndDate: "2026-09-30"}

	data, err := c.QueryCostData(context.Background(), &CostDataQuery{CostFilterOptions: window})
	if err != nil {
		t.Fatal(err)
	}
	if data.Meta.CommitmentDiscounts != CommitmentDiscountPlacementBlended {
		t.Errorf("cost data placement %q, want blended", data.Meta.CommitmentDiscounts)
	}

	summary, err := c.GetCostSummary(context.Background(), &window)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CommitmentDiscounts != CommitmentDiscountPlacementAsApplied {
		t.Errorf("summary placement %q, want asApplied", summary.CommitmentDiscounts)
	}
}

func TestCommitmentDiscountPlacementsMatchTheContract(t *testing.T) {
	wantContractValues(t, "CommitmentDiscountPlacement", CommitmentDiscountPlacements())
}

func TestMeasuresMatchTheContract(t *testing.T) {
	wantContractValues(t, "CostMeasure", []string{MeasureCost, MeasureUsage, MeasureCount})
}

func TestGranularitiesMatchTheContract(t *testing.T) {
	wantContractValues(t, "DateBin", []string{
		GranularityDay, GranularityWeek, GranularityMonth, GranularityQuarter, GranularityCumulative,
	})
}
