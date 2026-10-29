package costfluent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateBudget(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"bdg_1","name":"Cloud","amount":"1000.00000000","currency":"EUR",
		"startDate":"2026-10-01","intervalCount":1,"intervalUnit":"month",
		"periods":[{"startDate":"2026-10-01","endDate":"2026-10-31","amount":"1000.00000000"}],
		"status":"active","state":"atRisk","currentPeriod":{"startDate":"2026-10-01","endDate":"2026-10-31","amount":"1000.00000000"},
		"currentSpend":"250.50000000","budgetedToDate":"193.55000000","projectedSpend":"647.13000000","percentUsed":25.05,
		"spendAvailability":"computed","alerts":[{"thresholdPercent":80,"isTriggered":false}],
		"createdAt":"2026-09-01T00:00:00Z"}`})

	segment := "seg_1"
	amount := Decimal("1000")
	budget, err := c.CreateBudget(context.Background(), &CreateBudgetInput{
		Name: "Cloud", Amount: &amount, Currency: "EUR", IntervalCount: 1, IntervalUnit: "month",
		Alerts: []BudgetAlertInput{{ThresholdPercent: 80}}, SegmentID: &segment,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/budgets")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "segmentId", "seg_1")
	wantBody(t, (*got)[0], "intervalUnit", "month")
	wantBody(t, (*got)[0], "amount", "1000")
	if _, sent := (*got)[0].Body["periods"]; sent {
		t.Error("sent periods beside an amount")
	}
	if budget.ID != "bdg_1" || budget.Amount != "1000.00000000" || budget.CurrentSpend != "250.50000000" ||
		budget.PercentUsed == nil || *budget.PercentUsed != 25.05 || budget.SpendAvailability != "computed" || budget.State != "atRisk" ||
		budget.IntervalCount != 1 || budget.IntervalUnit != "month" || len(budget.Periods) != 1 ||
		budget.CurrentPeriod == nil || budget.CurrentPeriod.EndDate != "2026-10-31" ||
		budget.BudgetedToDate != "193.55000000" || budget.ProjectedSpend != "647.13000000" ||
		budget.Alerts[0].ThresholdPercent != 80 {
		t.Fatalf("decoded %+v", budget)
	}
}

func TestUpdateBudgetIsPut(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"bdg_1","amount":"2000.00000000"}`})

	amount := Decimal("2000")
	if _, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Amount: &amount}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/budgets/bdg_1")
	wantBody(t, (*got)[0], "amount", "2000")
}

func TestListBudgetsWithoutWorkspaceListsOrganization(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"/v1/budgets"},"data":[]}`})
	c = c.Workspace("")

	if _, err := c.ListBudgets(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := (*got)[0].Query["workspaceId"]; ok {
		t.Errorf("sent workspaceId %v with no workspace set", (*got)[0].Query["workspaceId"])
	}
}

func TestCreateBudgetSendsAndDecodesChannels(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"bdg_1","alerts":[{"thresholdPercent":80,"appIds":["app_1"]}]}`})

	amount := Decimal("1000")
	budget, err := c.CreateBudget(context.Background(), &CreateBudgetInput{
		Name: "Cloud", Amount: &amount, Currency: "EUR", AppIDs: []string{"app_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ids, ok := (*got)[0].Body["appIds"].([]any); !ok || len(ids) != 1 || ids[0] != "app_1" {
		t.Errorf("body appIds = %v, want [app_1]", (*got)[0].Body["appIds"])
	}
	if len(budget.Alerts[0].AppIDs) != 1 || budget.Alerts[0].AppIDs[0] != "app_1" {
		t.Fatalf("decoded alert channels %v", budget.Alerts[0].AppIDs)
	}
}

// An update leaves channels alone unless it names them; naming none clears them.
func TestUpdateBudgetChannels(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"bdg_1"}`}, stub{200, `{"id":"bdg_1"}`}, stub{200, `{"id":"bdg_1"}`})
	name := "Renamed"
	none := []string{}
	slack := []string{"app_2"}

	for _, input := range []*UpdateBudgetInput{{Name: &name}, {AppIDs: &none}, {AppIDs: &slack}} {
		if _, err := c.UpdateBudget(context.Background(), "bdg_1", input); err != nil {
			t.Fatal(err)
		}
	}

	if _, present := (*got)[0].Body["appIds"]; present {
		t.Errorf("an update without channels sent appIds = %v", (*got)[0].Body["appIds"])
	}
	if ids, ok := (*got)[1].Body["appIds"].([]any); !ok || len(ids) != 0 {
		t.Errorf("clearing sent appIds = %v, want []", (*got)[1].Body["appIds"])
	}
	if ids, ok := (*got)[2].Body["appIds"].([]any); !ok || len(ids) != 1 || ids[0] != "app_2" {
		t.Errorf("replacing sent appIds = %v, want [app_2]", (*got)[2].Body["appIds"])
	}
}

func TestCreateBudgetOnAReportSendsItsScopeCadenceAndPeriods(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"bdg_1","costReportId":"rpt_1","startDate":"2026-03-01",
		"intervalCount":3,"intervalUnit":"month","state":"notStarted",
		"periods":[{"startDate":"2026-03-01","endDate":"2026-05-31","amount":"3000.00000000"},
		{"startDate":"2026-06-01","endDate":"2026-08-31","amount":"3500.00000000"}],
		"spendAvailability":"unavailable"}`})

	report, start := "rpt_1", "2026-03-01"
	budget, err := c.CreateBudget(context.Background(), &CreateBudgetInput{
		Name: "Quarters", Currency: "EUR", CostReportID: &report, StartDate: &start,
		IntervalCount: 3, IntervalUnit: "month",
		Periods: []BudgetPeriod{
			{StartDate: "2026-03-01", EndDate: "2026-05-31", Amount: "3000"},
			{StartDate: "2026-06-01", EndDate: "2026-08-31", Amount: "3500"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "costReportId", "rpt_1")
	wantBody(t, (*got)[0], "startDate", "2026-03-01")
	wantBody(t, (*got)[0], "intervalCount", float64(3))
	if periods, ok := (*got)[0].Body["periods"].([]any); !ok || len(periods) != 2 ||
		periods[1].(map[string]any)["amount"] != "3500" || periods[1].(map[string]any)["endDate"] != "2026-08-31" {
		t.Errorf("body periods = %v", (*got)[0].Body["periods"])
	}
	for _, absent := range []string{"segmentId", "amount"} {
		if _, sent := (*got)[0].Body[absent]; sent {
			t.Errorf("sent %s for a report budget with periods", absent)
		}
	}
	if budget.CostReportID == nil || *budget.CostReportID != "rpt_1" || budget.StartDate != "2026-03-01" ||
		budget.Amount.IsSet() || len(budget.Periods) != 2 || budget.Periods[1].Amount != "3500.00000000" ||
		budget.CurrentPeriod != nil || budget.State != "notStarted" || budget.SpendAvailability != "unavailable" {
		t.Fatalf("decoded %+v", budget)
	}
}

// A usage budget sends its measure and unit beside its report and decodes them back, so a client
// reads the amounts as tokens rather than as the currency every budget still carries. A cost budget
// sends neither, and the API's measure says it counts money.
func TestBudgetMeasureAndUsageUnit(t *testing.T) {
	c, got := testClient(t,
		stub{201, `{"id":"bdg_1","currency":"EUR","measure":"usage","usageUnit":"Tokens","costReportId":"rpt_1",
			"amount":"5000000.00000000","currentSpend":"1250000.00000000","spendAvailability":"computed"}`},
		stub{201, `{"id":"bdg_2","currency":"EUR","measure":"cost","usageUnit":null,"spendAvailability":"computed"}`},
		stub{200, `{"id":"bdg_1","currency":"EUR","measure":"usage","usageUnit":"Tokens","costReportId":"rpt_1",
			"currentSpend":null,"spendAvailability":"noUsage","state":"unknown"}`},
	)
	report, unit := "rpt_1", "Tokens"
	amount := Decimal("5000000")

	usage, err := c.CreateBudget(context.Background(), &CreateBudgetInput{
		Name: "AI tokens", Currency: "EUR", Amount: &amount, Measure: MeasureUsage, UsageUnit: &unit, CostReportID: &report,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "measure", "usage")
	wantBody(t, (*got)[0], "usageUnit", "Tokens")
	wantBody(t, (*got)[0], "costReportId", "rpt_1")
	if usage.Measure != MeasureUsage || usage.UsageUnit == nil || *usage.UsageUnit != "Tokens" ||
		usage.Currency != "EUR" || usage.CurrentSpend != "1250000.00000000" {
		t.Fatalf("decoded %+v", usage)
	}

	cost, err := c.CreateBudget(context.Background(), &CreateBudgetInput{Name: "Cloud", Currency: "EUR", Amount: &amount})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"measure", "usageUnit"} {
		if _, sent := (*got)[1].Body[absent]; sent {
			t.Errorf("a cost budget sent %s: %v", absent, (*got)[1].Body[absent])
		}
	}
	if cost.Measure != MeasureCost || cost.UsageUnit != nil {
		t.Fatalf("decoded %+v", cost)
	}

	// No row in the unit this period is noUsage, never a measured zero, and written again the
	// budget keeps its measure and unit.
	read, err := c.GetBudget(context.Background(), "bdg_1")
	if err != nil {
		t.Fatal(err)
	}
	if read.SpendAvailability != "noUsage" || read.CurrentSpend.IsSet() {
		t.Fatalf("decoded %+v", read)
	}
	out, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"measure":"usage"`, `"usageUnit":"Tokens"`, `"spendAvailability":"noUsage"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("written as %s, want %s", out, want)
		}
	}
	if strings.Contains(string(out), `"currentSpend"`) {
		t.Errorf("written as %s, want no currentSpend", out)
	}
}

// An update changes a usage budget's unit only when it names one.
func TestUpdateBudgetUsageUnit(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"bdg_1"}`}, stub{200, `{"id":"bdg_1","measure":"usage","usageUnit":"GPU-Hours"}`})
	name := "Renamed"
	unit := "GPU-Hours"

	if _, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	budget, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{UsageUnit: &unit})
	if err != nil {
		t.Fatal(err)
	}

	if _, present := (*got)[0].Body["usageUnit"]; present {
		t.Errorf("an update without a unit sent usageUnit = %v", (*got)[0].Body["usageUnit"])
	}
	wantBody(t, (*got)[1], "usageUnit", "GPU-Hours")
	if len((*got)[1].Body) != 1 {
		t.Errorf("a unit change sent %v, want the unit alone", (*got)[1].Body)
	}
	if budget.UsageUnit == nil || *budget.UsageUnit != "GPU-Hours" {
		t.Fatalf("decoded %+v", budget)
	}
}

// An update replaces the periods only when it names them.
func TestUpdateBudgetPeriods(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"bdg_1"}`}, stub{200, `{"id":"bdg_1"}`})
	name := "Renamed"
	periods := []BudgetPeriod{{StartDate: "2026-10-01", EndDate: "2026-10-31", Amount: "900"}}

	for _, input := range []*UpdateBudgetInput{{Name: &name}, {Periods: &periods}} {
		if _, err := c.UpdateBudget(context.Background(), "bdg_1", input); err != nil {
			t.Fatal(err)
		}
	}

	if _, present := (*got)[0].Body["periods"]; present {
		t.Errorf("an update without periods sent periods = %v", (*got)[0].Body["periods"])
	}
	if sent, ok := (*got)[1].Body["periods"].([]any); !ok || len(sent) != 1 || sent[0].(map[string]any)["amount"] != "900" {
		t.Errorf("replacing sent periods = %v", (*got)[1].Body["periods"])
	}
}

// An update replaces the thresholds only when it names them, sending each with its basis, and
// decodes them back in the API's order: spend thresholds lowest first, then forecast ones.
func TestUpdateBudgetAlerts(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"bdg_1","alerts":[{"thresholdPercent":80,"basis":"actual","isTriggered":true}]}`},
		stub{200, `{"id":"bdg_1","alerts":[{"thresholdPercent":80,"basis":"actual","appIds":["app_1"],"isTriggered":true,"triggeredAt":"2026-10-20T08:00:00Z"},
			{"thresholdPercent":90,"basis":"forecast","appIds":["app_1"],"isTriggered":false}]}`},
	)
	name := "Renamed"
	alerts := []BudgetAlertInput{{ThresholdPercent: 80}, {ThresholdPercent: 90, Basis: BudgetAlertBasisForecast}}

	if _, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	budget, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Alerts: &alerts})
	if err != nil {
		t.Fatal(err)
	}

	if _, present := (*got)[0].Body["alerts"]; present {
		t.Errorf("an update without thresholds sent alerts = %v", (*got)[0].Body["alerts"])
	}
	sent, ok := (*got)[1].Body["alerts"].([]any)
	if !ok || len(sent) != 2 || len((*got)[1].Body) != 1 {
		t.Fatalf("replacing sent %v, want the two thresholds alone", (*got)[1].Body)
	}
	if first := sent[0].(map[string]any); first["thresholdPercent"] != float64(80) || len(first) != 1 {
		t.Errorf("a threshold without a basis sent %v, want its percent alone", first)
	}
	if second := sent[1].(map[string]any); second["thresholdPercent"] != float64(90) || second["basis"] != "forecast" {
		t.Errorf("a forecast threshold sent %v", second)
	}
	if len(budget.Alerts) != 2 ||
		budget.Alerts[0].ThresholdPercent != 80 || budget.Alerts[0].Basis != BudgetAlertBasisActual ||
		!budget.Alerts[0].IsTriggered || budget.Alerts[0].TriggeredAt == nil ||
		budget.Alerts[1].ThresholdPercent != 90 || budget.Alerts[1].Basis != BudgetAlertBasisForecast ||
		budget.Alerts[1].IsTriggered || len(budget.Alerts[1].AppIDs) != 1 {
		t.Fatalf("decoded alerts %+v", budget.Alerts)
	}
}

// A threshold's basis is sent only when set, so the API's default of spend applies, and decodes on
// every alert; written again, as the MCP server does, each alert keeps its basis.
func TestBudgetAlertBasis(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"bdg_1","alerts":[{"thresholdPercent":50,"basis":"actual","isTriggered":false},
		{"thresholdPercent":100,"basis":"actual","isTriggered":false},{"thresholdPercent":100,"basis":"forecast","isTriggered":false}]}`})
	amount := Decimal("1000")

	budget, err := c.CreateBudget(context.Background(), &CreateBudgetInput{
		Name: "Cloud", Amount: &amount, Currency: "EUR",
		Alerts: []BudgetAlertInput{
			{ThresholdPercent: 50},
			{ThresholdPercent: 100, Basis: BudgetAlertBasisActual},
			{ThresholdPercent: 100, Basis: BudgetAlertBasisForecast},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sent, ok := (*got)[0].Body["alerts"].([]any)
	if !ok || len(sent) != 3 {
		t.Fatalf("body alerts = %v", (*got)[0].Body["alerts"])
	}
	for i, want := range []any{nil, "actual", "forecast"} {
		if basis := sent[i].(map[string]any)["basis"]; basis != want {
			t.Errorf("threshold %d sent basis %v, want %v", i, basis, want)
		}
	}
	if len(budget.Alerts) != 3 || budget.Alerts[0].Basis != BudgetAlertBasisActual ||
		budget.Alerts[1].Basis != BudgetAlertBasisActual || budget.Alerts[2].Basis != BudgetAlertBasisForecast ||
		budget.Alerts[2].ThresholdPercent != 100 {
		t.Fatalf("decoded alerts %+v", budget.Alerts)
	}
	out, err := json.Marshal(budget.Alerts[2])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"basis":"forecast"`) {
		t.Errorf("written as %s, want its basis", out)
	}
}

// A read hands back the budget's ETag, and an update sending it applies only while the budget is
// unchanged: the second writer working from the same read gets IsPreconditionFailed. Without
// IfMatch the last write wins.
func TestUpdateBudgetSendsTheETagItReadAndReportsAStaleOne(t *testing.T) {
	version := `"7"`
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			sent = append(sent, r.Header.Get("If-Match"))
			if m := r.Header.Get("If-Match"); m != "" && m != version {
				w.WriteHeader(http.StatusPreconditionFailed)
				_, _ = io.WriteString(w, `{"status":412,"code":"precondition_failed","title":"Resource Conflict"}`)
				return
			}
			version = `"8"`
		}
		w.Header().Set("ETag", version)
		_, _ = io.WriteString(w, `{"id":"bdg_1","amount":"10.00000000"}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("key"), WithWorkspace("ws_default"))

	read, err := c.GetBudget(context.Background(), "bdg_1")
	if err != nil || read.ETag != `"7"` {
		t.Fatalf("read %+v, %v; want ETag \"7\"", read, err)
	}
	name := "First"
	updated, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Name: &name}, IfMatch(read.ETag))
	if err != nil || updated.ETag != `"8"` {
		t.Fatalf("update %+v, %v; want the new ETag \"8\"", updated, err)
	}
	if _, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Name: &name}, IfMatch(read.ETag)); !IsPreconditionFailed(err) {
		t.Fatalf("a stale If-Match answered %v, want a precondition failure", err)
	}
	if len(sent) != 2 || sent[0] != `"7"` || sent[1] != `"7"` {
		t.Errorf("sent If-Match %q", sent)
	}
	if _, err := c.UpdateBudget(context.Background(), "bdg_1", &UpdateBudgetInput{Name: &name}); err != nil || len(sent) != 3 || sent[2] != "" {
		t.Errorf("without IfMatch the last write wins and no header is sent; sent %q, err %v", sent, err)
	}
}

// The API sends a budget's figures as null when its spend is unknown, unavailable or outside the
// plan, or before it starts. Decoded and written again, as the MCP server does, they must stay
// absent rather than read as a measured 0 or an empty amount, and a measured zero must stay 0.
func TestBudgetFiguresTheAPISendsAsNullStayUnset(t *testing.T) {
	for _, tc := range []struct {
		name, body       string
		want, wantAbsent []string
	}{
		{"unavailable", `"spendAvailability":"unavailable","state":"unknown","currentSpend":null,"budgetedToDate":null,"projectedSpend":null,"percentUsed":null`,
			[]string{`"percentUsed":null`}, []string{`"currentSpend"`, `"budgetedToDate"`, `"projectedSpend"`}},
		{"outside the plan", `"spendAvailability":"outsidePlan","state":"unknown","currentSpend":null,"budgetedToDate":null,"projectedSpend":null,"percentUsed":null`,
			[]string{`"percentUsed":null`}, []string{`"currentSpend"`, `"budgetedToDate"`, `"projectedSpend"`}},
		{"not started", `"spendAvailability":"computed","state":"notStarted","currentSpend":"0.00000000","budgetedToDate":null,"projectedSpend":null,"percentUsed":null`,
			[]string{`"percentUsed":null`, `"currentSpend":"0.00000000"`}, []string{`"budgetedToDate"`, `"projectedSpend"`}},
		{"measured zero", `"spendAvailability":"computed","state":"onTrack","currentSpend":"0.00000000","budgetedToDate":"0.00000000","projectedSpend":"0.00000000","percentUsed":0`,
			[]string{`"percentUsed":0`, `"currentSpend":"0.00000000"`, `"budgetedToDate":"0.00000000"`, `"projectedSpend":"0.00000000"`}, nil},
		{"measured", `"spendAvailability":"computed","state":"atRisk","currentSpend":"250.50000000","budgetedToDate":"193.55000000","projectedSpend":"647.13000000","percentUsed":25.05`,
			[]string{`"percentUsed":25.05`, `"currentSpend":"250.50000000"`}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := testClient(t, stub{200, `{"id":"bdg_1","name":"Cloud","currency":"EUR","startDate":"2026-10-01",` + tc.body + `}`})
			budget, err := c.GetBudget(context.Background(), "bdg_1")
			if err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(budget)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(string(out), w) {
					t.Errorf("written as %s, want %s", out, w)
				}
			}
			for _, a := range tc.wantAbsent {
				if strings.Contains(string(out), a) {
					t.Errorf("written as %s, want no %s", out, a)
				}
			}
		})
	}
}

// A budget names the spend in other currencies it does not count, and the floor its spend counts
// from when the plan's history starts inside the period. Both decode exactly and are written again
// as read, as the MCP server does; a body without them decodes to neither.
func TestGetBudgetDecodesUnconvertedAndHistoryFloor(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"id":"bdg_1","name":"Cloud","currency":"EUR","startDate":"2026-01-01",
		"spendAvailability":"computed","state":"onTrack","currentSpend":"4000.00000000","percentUsed":40,
		"unconverted":[{"currency":"USD","amount":"7000.12345678"},{"currency":"TWD","amount":"5000.00000000"}],
		"historyFloor":"2026-04-06"}`})
	budget, err := c.GetBudget(context.Background(), "bdg_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(budget.Unconverted) != 2 ||
		budget.Unconverted[0] != (CurrencyAmount{Currency: "USD", Amount: "7000.12345678"}) ||
		budget.Unconverted[1] != (CurrencyAmount{Currency: "TWD", Amount: "5000.00000000"}) {
		t.Errorf("unconverted decoded as %+v", budget.Unconverted)
	}
	if budget.HistoryFloor == nil || *budget.HistoryFloor != "2026-04-06" {
		t.Errorf("history floor decoded as %v", budget.HistoryFloor)
	}
	out, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`"unconverted":[{"currency":"USD","amount":"7000.12345678"},{"currency":"TWD","amount":"5000.00000000"}]`, `"historyFloor":"2026-04-06"`} {
		if !strings.Contains(string(out), w) {
			t.Errorf("written as %s, want %s", out, w)
		}
	}

	c, _ = testClient(t, stub{200, `{"id":"bdg_1","name":"Cloud","currency":"EUR","startDate":"2026-10-01",
		"spendAvailability":"computed","state":"onTrack","currentSpend":"0.00000000","percentUsed":0}`})
	budget, err = c.GetBudget(context.Background(), "bdg_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(budget.Unconverted) != 0 || budget.HistoryFloor != nil {
		t.Errorf("a body without them decoded as unconverted %+v, history floor %v", budget.Unconverted, budget.HistoryFloor)
	}
}

func TestBudgetAlertBasesMatchTheContract(t *testing.T) {
	wantContractValues(t, "BudgetAlertBasis", []string{BudgetAlertBasisActual, BudgetAlertBasisForecast})
}
