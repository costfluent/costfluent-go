package costfluent

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestLinksNavigation(t *testing.T) {
	cases := []struct {
		links      Links
		next, prev bool
	}{
		{Links{Self: "/v1/x?page=1"}, false, false},
		{Links{Self: "/v1/x?page=1", Next: "/v1/x?page=2"}, true, false},
		{Links{Self: "/v1/x?page=2", Prev: "/v1/x?page=1", Next: "/v1/x?page=3"}, true, true},
		{Links{Self: "/v1/x?page=3", Prev: "/v1/x?page=2"}, false, true},
	}
	for _, tc := range cases {
		if tc.links.HasNextPage() != tc.next || tc.links.HasPrevPage() != tc.prev {
			t.Errorf("%+v: next %v prev %v", tc.links, tc.links.HasNextPage(), tc.links.HasPrevPage())
		}
	}
}

func TestPageOptionsSetOnlyWhatIsGiven(t *testing.T) {
	cases := []struct {
		opts  *PageOptions
		query string
	}{
		{nil, ""},
		{&PageOptions{}, ""},
		{&PageOptions{Page: 2}, "page=2"},
		{&PageOptions{Limit: 25}, "limit=25"},
		{&PageOptions{Page: 3, Limit: 50}, "limit=50&page=3"},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.test/v1/budgets", nil)
		tc.opts.apply(req)
		if req.URL.RawQuery != tc.query {
			t.Errorf("%+v: query %q, want %q", tc.opts, req.URL.RawQuery, tc.query)
		}
	}
}

func TestListAllDefaultsThePageSize(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"/v1/providers?page=1"},"data":[{"id":"prv_1"}]}`})

	all, err := c.ListAllProviders(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "page", "1")
	wantQuery(t, (*got)[0], "limit", "100")
	if len(all) != 1 || all[0].ID != "prv_1" {
		t.Fatalf("all %+v", all)
	}
}

// A server that keeps advertising a next page while serving nothing must not loop forever.
func TestListAllStopsOnAnEmptyPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"/v1/budgets?page=1","next":"/v1/budgets?page=2"},"data":[{"id":"bdg_1"}]}`},
		stub{200, `{"links":{"self":"/v1/budgets?page=2","next":"/v1/budgets?page=3"},"data":[]}`},
	)

	all, err := c.ListAllBudgets(context.Background(), "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 || len(all) != 1 || all[0].ID != "bdg_1" {
		t.Fatalf("%d requests, all %+v", len(*got), all)
	}
	wantQuery(t, (*got)[1], "workspaceId", "ws_default")
}

func TestListAllReturnsNothingWhenAPageFails(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"links":{"self":"/v1/cost-alerts?page=1","next":"/v1/cost-alerts?page=2"},"data":[{"id":"cal_1"}]}`},
		stub{403, `{"title":"Forbidden","status":403,"code":"forbidden","retryable":false}`},
	)

	all, err := c.ListAllCostAlerts(context.Background(), "ws_1", 1)
	if !IsForbidden(err) {
		t.Fatalf("err = %v, want forbidden", err)
	}
	if all != nil {
		t.Fatalf("returned a partial listing %+v", all)
	}
}

func TestWorkspaceListingWalksTheLargestPages(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"p1","next":"p2"},"data":[{"id":"alr_1","priority":1},{"id":"alr_2","priority":2}]}`},
		stub{200, `{"links":{"self":"p2"},"data":[{"id":"alr_3","priority":3}]}`},
	)

	rules, err := c.ListAllocationRules(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	for i, ex := range *got {
		wantRequest(t, ex, http.MethodGet, "/v1/allocation/rules")
		wantQuery(t, ex, "workspaceId", "ws_1")
		wantQuery(t, ex, "limit", "100")
		wantQuery(t, ex, "page", []string{"1", "2"}[i])
	}
	if len(rules) != 3 || rules[0].ID != "alr_1" || rules[2].ID != "alr_3" {
		t.Fatalf("rules %+v; the API's order is the rule set's meaning", rules)
	}
}

func TestListExchangeRatesQuery(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"/v1/exchange-rates"},"data":[{"baseCurrency":"EUR","targetCurrency":"GBP",
			"rate":0.8412,"effectiveDate":"2026-10-09","source":"ECB"}]}`},
		stub{200, `{"links":{"self":"/v1/exchange-rates"},"data":[]}`},
		stub{200, `{"links":{"self":"/v1/exchange-rates"},"data":[]}`},
	)
	start := time.Date(2026, 10, 1, 23, 30, 0, 0, time.UTC)
	end := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)

	resp, err := c.ListExchangeRates(context.Background(), &ExchangeRateOptions{
		StartDate: &start, EndDate: &end, BaseCurrency: "EUR", TargetCurrency: "GBP",
		PageOptions: PageOptions{Page: 2, Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	ex := (*got)[0]
	wantRequest(t, ex, http.MethodGet, "/v1/exchange-rates")
	for k, v := range map[string]string{
		"startDate": "2026-10-01", "endDate": "2026-10-09", "baseCurrency": "EUR", "targetCurrency": "GBP",
		"page": "2", "limit": "10",
	} {
		wantQuery(t, ex, k, v)
	}
	if _, ok := ex.Query["workspaceId"]; ok {
		t.Error("exchange rates are not workspace-scoped")
	}
	if len(resp.Data) != 1 || resp.Data[0].Rate != 0.8412 || resp.Data[0].EffectiveDate != "2026-10-09" {
		t.Fatalf("decoded %+v", resp.Data)
	}

	if _, err := c.ListExchangeRates(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len((*got)[1].Query) != 0 {
		t.Errorf("nil options sent query %v", (*got)[1].Query)
	}

	if _, err := c.ListExchangeRates(context.Background(), &ExchangeRateOptions{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if q := (*got)[2].Query; len(q) != 1 || len(q["baseCurrency"]) != 1 || q["baseCurrency"][0] != "USD" {
		t.Errorf("unset options leaked into query %v", q)
	}
}
