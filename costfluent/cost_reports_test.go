package costfluent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCreateCostReportReadsItBack(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"rpt_1","title":"Monthly"}`},
		stub{200, `{"id":"rpt_1","title":"Monthly","dateInterval":"thisMonth","chartType":"bar","dateBin":"day",
			"groupBy":[{"dimension":"ServiceName","key":null}],"settings":{"amortize":true}}`},
	)

	report, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{WorkspaceID: "ws_1", Title: "Monthly"})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/cost-reports")
	wantRequest(t, (*got)[1], "GET", "/v1/cost-reports/rpt_1")
	wantQuery(t, (*got)[1], "workspaceId", "ws_1")
	if _, sent := (*got)[0].Body["groupBy"]; sent {
		t.Error("sent groupBy although it was left to the API default")
	}
	if report.ChartType != "bar" || !report.Settings.Amortize ||
		len(report.GroupBy) != 1 || report.GroupBy[0].Dimension != "ServiceName" || report.GroupBy[0].Key != nil {
		t.Fatalf("decoded %+v", report)
	}
}

func TestCostReportChartAxisIsSentAndRead(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"rpt_1"}`},
		stub{200, `{"id":"rpt_1","chartType":"bar","chartAxis":"group","groupBy":[]}`},
		stub{200, `{"id":"rpt_1","chartType":"line","chartAxis":"date","groupBy":[]}`},
	)

	created, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		WorkspaceID: "ws_1", Title: "Side by side", ChartType: ptr("bar"), ChartAxis: ptr("group"),
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "chartAxis", "group")
	if created.ChartAxis != "group" {
		t.Fatalf("decoded %+v", created)
	}

	updated, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{ChartType: ptr("line")})
	if err != nil {
		t.Fatal(err)
	}
	if _, sent := (*got)[2].Body["chartAxis"]; sent {
		t.Error("sent chartAxis although the update left it out")
	}
	if updated.ChartAxis != "date" {
		t.Fatalf("decoded %+v", updated)
	}
}

func TestCreateCostReportFallsBackToTheDefaultWorkspace(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"rpt_1"}`}, stub{200, `{"id":"rpt_1","title":"Monthly"}`})
	input := &CreateCostReportInput{Title: "Monthly"}

	if _, err := c.CreateCostReport(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantQuery(t, (*got)[1], "workspaceId", "ws_default")
	if input.WorkspaceID != "" {
		t.Errorf("the caller's input was changed to workspace %q", input.WorkspaceID)
	}
}

func TestCreateCostReportSendsGroupingSlots(t *testing.T) {
	cases := map[string]struct {
		groupBy []Grouping
		want    string
	}{
		"ungrouped":  {[]Grouping{}, `[]`},
		"two levels": {[]Grouping{{Dimension: "ServiceName"}, {Dimension: "Tag", Key: ptr("team")}}, `[{"dimension":"ServiceName"},{"dimension":"Tag","key":"team"}]`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, got := testClient(t, stub{200, `{"id":"rpt_1"}`}, stub{200, `{"id":"rpt_1","groupBy":[]}`})
			if _, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
				WorkspaceID: "ws_1", Title: "Monthly", GroupBy: tc.groupBy,
			}); err != nil {
				t.Fatal(err)
			}
			wantJSON(t, (*got)[0].Body["groupBy"], tc.want)
		})
	}
}

func TestUpdateCostReportGroupBy(t *testing.T) {
	cases := map[string]struct {
		groupBy []Grouping
		sent    bool
	}{
		"nil leaves it alone": {nil, false},
		"empty clears it":     {[]Grouping{}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, got := testClient(t, stub{200, `{"id":"rpt_1","groupBy":[]}`})
			if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1",
				&UpdateCostReportInput{GroupBy: tc.groupBy}); err != nil {
				t.Fatal(err)
			}
			v, sent := (*got)[0].Body["groupBy"]
			if sent != tc.sent {
				t.Fatalf("groupBy sent %v (%v), want sent %v", sent, v, tc.sent)
			}
			if sent {
				wantJSON(t, v, `[]`)
			}
		})
	}
}

func TestUpdateCostReportClearFolder(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{false: "false omits it", true: "true sends it"}[clear], func(t *testing.T) {
			c, got := testClient(t, stub{200, `{"id":"rpt_1"}`})
			if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1",
				&UpdateCostReportInput{ClearFolder: clear}); err != nil {
				t.Fatal(err)
			}
			v, sent := (*got)[0].Body["clearFolder"]
			if sent != clear {
				t.Fatalf("clearFolder sent %v (%v), want sent %v", sent, v, clear)
			}
			if sent {
				wantJSON(t, v, `true`)
			}
			if _, named := (*got)[0].Body["folderId"]; named {
				t.Fatal("folderId sent, want it omitted")
			}
		})
	}
}

func TestGroupingString(t *testing.T) {
	for g, want := range map[Grouping]string{
		{Dimension: "ServiceName"}:                   "ServiceName",
		{Dimension: "Tag", Key: ptr("team")}:         "Tag:team",
		{Dimension: "VirtualTag", Key: ptr("vtg_1")}: "VirtualTag:vtg_1",
		{Dimension: "Region", Key: ptr("")}:          "Region",
	} {
		if got := g.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", g, got, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func wantJSON(t *testing.T, got any, want string) {
	t.Helper()
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Errorf("json %s, want %s", raw, want)
	}
}

func TestListCostReportsWalksEveryPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"s","next":"https://api.test/v1/cost-reports?page=2&limit=100"},"data":[{"id":"rpt_1","title":"One"}]}`},
		stub{200, `{"links":{"self":"s"},"data":[{"id":"rpt_2","title":"Two"}]}`},
	)

	reports, err := c.ListCostReports(context.Background(), "ws_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "page", "1")
	wantQuery(t, (*got)[0], "limit", "100")
	wantQuery(t, (*got)[1], "page", "2")
	wantQuery(t, (*got)[1], "workspaceId", "ws_1")
	if len(reports) != 2 || reports[1].Title != "Two" {
		t.Fatalf("decoded %+v", reports)
	}
}

func TestListCostReportsSendsTheFolderOnEveryPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"s","next":"https://api.test/v1/cost-reports?page=2&limit=100"},"data":[{"id":"rpt_1","folderId":"fld_1"}]}`},
		stub{200, `{"links":{"self":"s"},"data":[{"id":"rpt_2","folderId":"fld_1"}]}`},
	)

	reports, err := c.ListCostReports(context.Background(), "ws_1", &ListCostReportsOptions{FolderID: "fld_1"})
	if err != nil {
		t.Fatal(err)
	}
	for i, page := range []string{"1", "2"} {
		wantQuery(t, (*got)[i], "folderId", "fld_1")
		wantQuery(t, (*got)[i], "page", page)
		wantQuery(t, (*got)[i], "workspaceId", "ws_1")
	}
	if len(reports) != 2 {
		t.Fatalf("decoded %+v", reports)
	}
}

func TestListCostReportsWithoutAFolderSendsNone(t *testing.T) {
	for name, opts := range map[string]*ListCostReportsOptions{"nil options": nil, "no folder": {}} {
		t.Run(name, func(t *testing.T) {
			c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[]}`})

			if _, err := c.ListCostReports(context.Background(), "ws_1", opts); err != nil {
				t.Fatal(err)
			}
			if v, sent := (*got)[0].Query["folderId"]; sent {
				t.Errorf("sent folderId %v although no folder was asked for", v)
			}
		})
	}
}

func TestListCostReportsPageReadsOnePageAndItsLinks(t *testing.T) {
	for name, opts := range map[string]*ListCostReportsOptions{"every folder": nil, "one folder": {FolderID: "fld_1"}} {
		t.Run(name, func(t *testing.T) {
			c, got := testClient(t,
				stub{200, `{"links":{"self":"s","next":"https://api.test/v1/cost-reports?page=3&limit=100"},"data":[{"id":"rpt_1","title":"One"}]}`},
			)

			resp, err := c.ListCostReportsPage(context.Background(), "ws_1", opts, &PageOptions{Page: 2, Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			if len(*got) != 1 {
				t.Fatalf("sent %d requests, want one page's", len(*got))
			}
			wantQuery(t, (*got)[0], "workspaceId", "ws_1")
			wantQuery(t, (*got)[0], "page", "2")
			wantQuery(t, (*got)[0], "limit", "100")
			if opts != nil {
				wantQuery(t, (*got)[0], "folderId", "fld_1")
			} else if v, sent := (*got)[0].Query["folderId"]; sent {
				t.Errorf("sent folderId %v although no folder was asked for", v)
			}
			if len(resp.Data) != 1 || resp.Data[0].Title != "One" || resp.Links.Next != "https://api.test/v1/cost-reports?page=3&limit=100" {
				t.Errorf("decoded %+v", resp)
			}
		})
	}
}

func TestCostReportReadsWhetherItIsTheDefault(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"id":"rpt_default","isDefault":true}`},
		stub{200, `{"links":{"self":"s"},"data":[{"id":"rpt_default","isDefault":true},{"id":"rpt_2","isDefault":false}]}`},
	)

	report, err := c.GetCostReport(context.Background(), "ws_1", "rpt_default")
	if err != nil {
		t.Fatal(err)
	}
	if !report.IsDefault {
		t.Errorf("GetCostReport decoded %+v, want the default report", report)
	}
	reports, err := c.ListCostReports(context.Background(), "ws_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || !reports[0].IsDefault || reports[1].IsDefault {
		t.Fatalf("ListCostReports decoded %+v, want only the first to be the default", reports)
	}
}

func TestCostGroupDimensionsMatchTheContract(t *testing.T) {
	wantContractValues(t, "CostGroupDimension", CostGroupDimensions())
}

func TestGetCostReportDataReadsTheReportsNumbers(t *testing.T) {
	c, got := testClient(t, stub{200, `{"summary":{"totalCost":"12.50000000","currency":"USD","measure":"cost","comparedThrough":"2026-10-10","comparedCost":"11.00000000","previousComparedThrough":"2026-09-10"},
		"series":[{"date":"2026-10-01","groups":[{"keys":["AmazonEC2"],"cost":"12.50000000","currency":"USD"}]}],
		"breakdown":[{"keys":["AmazonEC2"],"cost":"12.50000000","percent":100}],
		"startDate":"2026-10-01","endDate":"2026-10-31","dataThrough":"2026-10-12"}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/cost-reports/rpt_1/data")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	if data.Summary.TotalCost == nil || *data.Summary.TotalCost != "12.50000000" || len(data.Series) != 1 ||
		len(data.Series[0].Groups) != 1 || data.Breakdown[0].Percent != 100 || data.DataThrough == nil ||
		data.Summary.ComparedThrough == nil || *data.Summary.ComparedThrough != "2026-10-10" ||
		data.Summary.ComparedCost == nil || *data.Summary.ComparedCost != "11.00000000" ||
		data.Summary.PreviousComparedThrough == nil || *data.Summary.PreviousComparedThrough != "2026-09-10" {
		t.Fatalf("decoded %+v", data)
	}
}

func TestGetCostReportDataReadsTheNamesOfItsGroups(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"summary":{},"series":[],"breakdown":[{"keys":["i-0abc","222222222222","c-prod-eu"],"cost":"12.50000000","percent":100}],
			"resourceNames":{"i-0abc":"checkout-api"},"accountNames":{"222222222222":"prod-payments"},"clusterNames":{"c-prod-eu":"Production EU"}}`},
		stub{200, `{"summary":{},"series":[],"breakdown":[],"resourceNames":null,"accountNames":null,"clusterNames":null}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	if data.ResourceNames["i-0abc"] != "checkout-api" || data.AccountNames["222222222222"] != "prod-payments" ||
		data.ClusterNames["c-prod-eu"] != "Production EU" {
		t.Fatalf("decoded resourceNames %v, accountNames %v, clusterNames %v", data.ResourceNames, data.AccountNames, data.ClusterNames)
	}

	unnamed, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_2")
	if err != nil {
		t.Fatal(err)
	}
	if unnamed.ResourceNames != nil || unnamed.AccountNames != nil || unnamed.ClusterNames != nil {
		t.Fatalf("a report grouped by nothing named decoded names: %+v", unnamed)
	}
}

func TestGetCostReportDataReadsEachGroupsComparison(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"summary":{"previousPeriodCost":"9.00000000","comparedCost":"11.00000000"},"series":[],
		"breakdown":[{"keys":["AmazonEC2"],"cost":"12.50000000","percent":100,"previousCost":"8.00000000","comparedCost":"11.00000000","changePercent":37.5},
		{"keys":[],"isOther":true,"cost":"0.00000000","percent":0,"previousCost":"1.00000000","comparedCost":"0.00000000","changePercent":-100},
		{"keys":["AmazonS3"],"cost":"1.00000000","percent":0}]}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	ec2, other, s3 := data.Breakdown[0], data.Breakdown[1], data.Breakdown[2]
	if ec2.PreviousCost == nil || *ec2.PreviousCost != "8.00000000" || ec2.ComparedCost == nil || *ec2.ComparedCost != "11.00000000" ||
		ec2.ChangePercent == nil || *ec2.ChangePercent != 37.5 {
		t.Fatalf("decoded %+v", ec2)
	}
	if !other.IsOther || other.PreviousCost == nil || *other.PreviousCost != "1.00000000" || other.ChangePercent == nil || *other.ChangePercent != -100 {
		t.Fatalf("decoded %+v", other)
	}
	if s3.PreviousCost != nil || s3.ComparedCost != nil || s3.ChangePercent != nil {
		t.Fatalf("a row without a comparison decoded figures: %+v", s3)
	}
}

func TestGetCostReportDataReadsEachUsageGroupsUnitPrice(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"summary":{"measure":"usage","currency":"USD"},"series":[],
		"breakdown":[{"keys":["claude-3-5-sonnet","input"],"cost":"20000000.00000000","percent":95,"unit":"Tokens","unitPrice":0.000003},
		{"keys":[],"isOther":true,"cost":"1000000.00000000","percent":5,"unit":"Tokens","unitPrice":0.000000035},
		{"keys":["mixed"],"cost":"5.00000000","percent":0,"unit":"Hours","unitPrice":null}]}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	sonnet, other, mixed := data.Breakdown[0], data.Breakdown[1], data.Breakdown[2]
	if sonnet.UnitPrice == nil || *sonnet.UnitPrice != 0.000003 {
		t.Fatalf("decoded %+v", sonnet)
	}
	if other.UnitPrice == nil || *other.UnitPrice != 0.000000035 {
		t.Fatalf("the remainder's price decoded as %+v", other)
	}
	if mixed.UnitPrice != nil {
		t.Fatalf("a group without a price decoded one: %+v", mixed)
	}
}

func TestGetCostReportDataReadsTheUnitCost(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"summary":{},"series":[],"breakdown":[],
		"unitCost":{"metricId":"bmt_1","name":"Customers","unit":"customer","currency":"EUR","average":"2.50000000",
		"series":[{"date":"2026-09-01","metricAmount":"40.00000000","unitCost":"2.50000000"},{"date":"2026-09-02"}]}}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	u := data.UnitCost
	if u == nil || u.MetricID != "bmt_1" || u.Average == nil || *u.Average != "2.50000000" || len(u.Series) != 2 ||
		u.Series[0].UnitCost == nil || *u.Series[0].UnitCost != "2.50000000" || u.Series[1].UnitCost != nil {
		t.Fatalf("decoded %+v", u)
	}
}

func TestGetCostReportDataReadsTheCalculationAndValue(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"summary":{},"series":[],"breakdown":[],
		"unitCost":{"metricId":"bmt_1","name":"Revenue","unit":"EUR","calculation":"grossMargin","scale":1,
		"currency":"EUR","average":"0.40000000",
		"series":[{"date":"2026-09-01","metricAmount":"1000.00000000","value":"0.40000000"},{"date":"2026-09-02","metricAmount":"0.00000000"}]}}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	u := data.UnitCost
	if u == nil || u.Calculation != UnitCostGrossMargin || u.Scale != 1 || len(u.Series) != 2 {
		t.Fatalf("decoded %+v", u)
	}
	margin, empty := u.Series[0], u.Series[1]
	if margin.Value == nil || *margin.Value != "0.40000000" || margin.UnitCost != nil {
		t.Fatalf("decoded %+v, want the margin as its value and no unit cost", margin)
	}
	if empty.Value != nil || empty.MetricAmount == nil {
		t.Fatalf("decoded %+v, want no value where the revenue is zero", empty)
	}
}

func TestUnitCostCalculationsMatchTheContract(t *testing.T) {
	wantContractValues(t, "UnitCostCalculation", []string{UnitCostPerUnit, UnitCostGrossMargin, UnitCostMetricValue})
}

// The calculation and scale are sent when set and read back; an update that leaves them out sends
// neither, so the API keeps the report's own.
func TestCostReportUnitCostCalculationIsSentAndRead(t *testing.T) {
	read := `{"id":"rpt_1","title":"Margin","settings":{"unitCostMetricId":"bmt_1",` +
		`"unitCostCalculation":"grossMargin","unitCostScale":1}}`
	c, got := testClient(t,
		stub{201, `{"id":"rpt_1","title":"Margin"}`}, stub{200, read},
		stub{200, `{"id":"rpt_1","title":"Per thousand","settings":{"unitCostMetricId":"bmt_1",` +
			`"unitCostCalculation":"perUnit","unitCostScale":1000}}`},
		stub{200, read})

	metric := "bmt_1"
	report, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		Title:    "Margin",
		Settings: &CostReportSettingsInput{UnitCostMetricID: &metric, UnitCostCalculation: UnitCostGrossMargin},
	})
	if err != nil {
		t.Fatal(err)
	}
	created, _ := (*got)[0].Body["settings"].(map[string]any)
	if len(created) != 2 || created["unitCostCalculation"] != "grossMargin" {
		t.Fatalf("create settings = %v, want the metric and the calculation", created)
	}
	if report.Settings.UnitCostCalculation != UnitCostGrossMargin || report.Settings.UnitCostScale != 1 {
		t.Fatalf("decoded %+v", report.Settings)
	}

	scale := 1000
	updated, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{
		Settings: &CostReportSettingsInput{UnitCostMetricID: &metric, UnitCostCalculation: UnitCostPerUnit, UnitCostScale: &scale},
	})
	if err != nil {
		t.Fatal(err)
	}
	sent, _ := (*got)[2].Body["settings"].(map[string]any)
	if sent["unitCostCalculation"] != "perUnit" || sent["unitCostScale"] != float64(1000) {
		t.Fatalf("update settings = %v, want perUnit per thousand", sent)
	}
	if updated.Settings.UnitCostCalculation != UnitCostPerUnit || updated.Settings.UnitCostScale != 1000 {
		t.Fatalf("decoded %+v", updated.Settings)
	}

	if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{
		Settings: &CostReportSettingsInput{UnitCostMetricID: &metric},
	}); err != nil {
		t.Fatal(err)
	}
	bare, _ := (*got)[3].Body["settings"].(map[string]any)
	if len(bare) != 1 {
		t.Fatalf("update settings = %v, want only the metric", bare)
	}
}

func TestGetCostReportDataReadsTheForecast(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"summary":{"currency":"EUR"},"series":[],"breakdown":[],"forecast":{"series":[
			{"date":"2026-10-23","amount":"10.00000000","lowerBound":"10.00000000","upperBound":"12.50000000"},
			{"date":"2026-10-24","amount":"10.00000000","lowerBound":"7.50000000","upperBound":"12.50000000"}]}}`},
		stub{200, `{"summary":{},"series":[],"breakdown":[]}`})

	data, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	f := data.Forecast
	if f == nil || len(f.Series) != 2 {
		t.Fatalf("decoded %+v", f)
	}
	if p := f.Series[1]; p.Date != "2026-10-24" || p.Amount != "10.00000000" || p.LowerBound != "7.50000000" || p.UpperBound != "12.50000000" {
		t.Fatalf("decoded %+v", p)
	}

	without, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_2")
	if err != nil {
		t.Fatal(err)
	}
	if without.Forecast != nil {
		t.Fatalf("decoded %+v from a report without a forecast", without.Forecast)
	}
}

func TestCreateCostReportSendsTheUnitCostMetric(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"rpt_1","title":"Per customer"}`}, stub{200, `{"id":"rpt_1","title":"Per customer"}`})

	metric := "bmt_1"
	amortize := true
	if _, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		Title: "Per customer", Settings: &CostReportSettingsInput{Amortize: &amortize, UnitCostMetricID: &metric},
	}); err != nil {
		t.Fatal(err)
	}
	settings, ok := (*got)[0].Body["settings"].(map[string]any)
	if !ok || settings["unitCostMetricId"] != "bmt_1" || settings["amortize"] != true {
		t.Fatalf("sent %v", (*got)[0].Body)
	}
}

func TestCostReportLabelFilterIsSentAndRead(t *testing.T) {
	read := `{"id":"rpt_1","title":"Per EU customer","settings":{"unitCostMetricId":"bmt_1",` +
		`"unitCostLabelFilter":[{"key":"region","values":["eu","us"]},{"key":"plan","values":["pro"]}]}}`
	c, got := testClient(t, stub{201, `{"id":"rpt_1","title":"Per EU customer"}`}, stub{200, read})

	metric := "bmt_1"
	report, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		Title: "Per EU customer",
		Settings: &CostReportSettingsInput{
			UnitCostMetricID: &metric,
			UnitCostLabelFilter: []UnitCostLabelCondition{
				{Key: "region", Values: []string{"eu", "us"}},
				{Key: "plan", Values: []string{"pro"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	settings, _ := (*got)[0].Body["settings"].(map[string]any)
	filter, ok := settings["unitCostLabelFilter"].([]any)
	if !ok || len(filter) != 2 {
		t.Fatalf("sent %v", (*got)[0].Body)
	}
	first, _ := filter[0].(map[string]any)
	values, _ := first["values"].([]any)
	if first["key"] != "region" || len(values) != 2 || values[1] != "us" {
		t.Fatalf("sent %v", filter)
	}

	decoded := report.Settings.UnitCostLabelFilter
	if len(decoded) != 2 || decoded[1].Key != "plan" || len(decoded[1].Values) != 1 || decoded[1].Values[0] != "pro" {
		t.Fatalf("decoded %+v", decoded)
	}
}

func TestCostReportWithoutALabelFilterSendsNone(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"rpt_1","title":"Per customer"}`}, stub{200, `{"id":"rpt_1","title":"Per customer"}`})

	metric := "bmt_1"
	if _, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		Title: "Per customer", Settings: &CostReportSettingsInput{UnitCostMetricID: &metric},
	}); err != nil {
		t.Fatal(err)
	}
	settings, _ := (*got)[0].Body["settings"].(map[string]any)
	if _, sent := settings["unitCostLabelFilter"]; sent {
		t.Fatalf("sent %v", settings)
	}
}

// A flag the caller leaves nil is not sent, so the API applies the workspace's default on create
// and keeps the report's own on update; a flag set to false is sent as false.
func TestCostReportSettingsSendOnlyTheFlagsSet(t *testing.T) {
	c, got := testClient(t,
		stub{201, `{"id":"rpt_1","title":"Bare"}`}, stub{200, `{"id":"rpt_1","title":"Bare"}`},
		stub{200, `{"id":"rpt_1","title":"Bare"}`})

	tax, credits := true, false
	if _, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		Title: "Bare", Settings: &CostReportSettingsInput{IncludeTax: &tax, IncludeCredits: &credits},
	}); err != nil {
		t.Fatal(err)
	}
	created, _ := (*got)[0].Body["settings"].(map[string]any)
	if len(created) != 2 || created["includeTax"] != true || created["includeCredits"] != false {
		t.Fatalf("create settings = %v, want only includeTax and includeCredits", created)
	}

	forecast := true
	if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{
		Settings: &CostReportSettingsInput{ShowForecast: &forecast},
	}); err != nil {
		t.Fatal(err)
	}
	updated, _ := (*got)[2].Body["settings"].(map[string]any)
	if len(updated) != 1 || updated["showForecast"] != true {
		t.Fatalf("update settings = %v, want only showForecast", updated)
	}
}

// The comparison window is read from a report and taken as sent on an update, so a caller sends
// back the window it read; settings without one send no date.
func TestCostReportComparisonWindowIsReadAndSentBack(t *testing.T) {
	read := `{"id":"rpt_1","title":"Versus September","settings":{"includeTax":true,` +
		`"comparisonStartDate":"2026-09-01","comparisonEndDate":"2026-09-30"}}`
	c, got := testClient(t, stub{200, read}, stub{200, read}, stub{200, `{"id":"rpt_1","title":"Bare"}`})

	report, err := c.GetCostReport(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	start, end := report.Settings.ComparisonStartDate, report.Settings.ComparisonEndDate
	if start == nil || *start != "2026-09-01" || end == nil || *end != "2026-09-30" {
		t.Fatalf("decoded window %v %v", start, end)
	}

	credits := false
	if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{
		Settings: &CostReportSettingsInput{IncludeCredits: &credits, ComparisonStartDate: start, ComparisonEndDate: end},
	}); err != nil {
		t.Fatal(err)
	}
	sent, _ := (*got)[1].Body["settings"].(map[string]any)
	if len(sent) != 3 || sent["includeCredits"] != false ||
		sent["comparisonStartDate"] != "2026-09-01" || sent["comparisonEndDate"] != "2026-09-30" {
		t.Fatalf("update settings = %v, want includeCredits and the window", sent)
	}

	if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{
		Settings: &CostReportSettingsInput{IncludeCredits: &credits},
	}); err != nil {
		t.Fatal(err)
	}
	bare, _ := (*got)[2].Body["settings"].(map[string]any)
	if len(bare) != 1 {
		t.Fatalf("update settings = %v, want no window keys", bare)
	}
}

func TestComparisonModesMatchTheContract(t *testing.T) {
	wantContractValues(t, "ComparisonMode", []string{ComparisonModePreviousPeriod, ComparisonModeSamePeriodLastYear})
}

// The comparison mode is sent when set and read back; an update that leaves it out sends no mode,
// so the API keeps the report's own.
func TestCostReportComparisonModeIsSentAndRead(t *testing.T) {
	read := `{"id":"rpt_1","title":"Year on year","settings":{"compareToLastPeriod":true,"comparisonMode":"samePeriodLastYear"}}`
	c, got := testClient(t,
		stub{201, `{"id":"rpt_1","title":"Year on year"}`}, stub{200, read},
		stub{200, read})

	compare := true
	report, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{
		Title: "Year on year",
		Settings: &CostReportSettingsInput{
			CompareToLastPeriod: &compare, ComparisonMode: ComparisonModeSamePeriodLastYear,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	created, _ := (*got)[0].Body["settings"].(map[string]any)
	if len(created) != 2 || created["comparisonMode"] != "samePeriodLastYear" {
		t.Fatalf("create settings = %v, want the switch and the mode", created)
	}
	if report.Settings.ComparisonMode != ComparisonModeSamePeriodLastYear {
		t.Fatalf("decoded %+v", report.Settings)
	}

	tax := true
	if _, err := c.UpdateCostReport(context.Background(), "ws_1", "rpt_1", &UpdateCostReportInput{
		Settings: &CostReportSettingsInput{IncludeTax: &tax},
	}); err != nil {
		t.Fatal(err)
	}
	bare, _ := (*got)[2].Body["settings"].(map[string]any)
	if _, sent := bare["comparisonMode"]; sent || len(bare) != 1 {
		t.Fatalf("update settings = %v, want only includeTax", bare)
	}
}

// A year-ago comparison says so, and a comparison the plan's history floor cut is withheld with
// the floor it hit.
func TestGetCostReportDataReadsTheYearAgoComparisonAndItsFloor(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"summary":{},"series":[],"breakdown":[],"comparison":{"startDate":"2025-11-01","endDate":"2025-11-30",
			"samePeriodLastYear":true,"series":[{"date":"2026-11-01","previousDate":"2025-11-01","cost":"4.00000000"}]}}`},
		stub{200, `{"summary":{},"series":[],"breakdown":[],"comparison":null,"comparisonHistoryFloor":"2025-10-04"}`},
		stub{200, `{"summary":{},"series":[],"breakdown":[],"comparison":{"startDate":"2026-10-01","endDate":"2026-10-31",
			"samePeriodLastYear":false,"series":[]}}`})

	yearAgo, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	if yearAgo.Comparison == nil || !yearAgo.Comparison.SamePeriodLastYear || yearAgo.Comparison.StartDate != "2025-11-01" ||
		len(yearAgo.Comparison.Series) != 1 || yearAgo.ComparisonHistoryFloor != nil {
		t.Fatalf("decoded %+v", yearAgo)
	}

	cut, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	if cut.Comparison != nil || cut.ComparisonHistoryFloor == nil || *cut.ComparisonHistoryFloor != "2025-10-04" {
		t.Fatalf("decoded %+v, want no comparison and its floor", cut)
	}

	previous, err := c.GetCostReportData(context.Background(), "ws_1", "rpt_1")
	if err != nil {
		t.Fatal(err)
	}
	if previous.Comparison == nil || previous.Comparison.SamePeriodLastYear || previous.ComparisonHistoryFloor != nil {
		t.Fatalf("decoded %+v, want the previous period", previous)
	}
}

func TestGetCostReportDataFallsBackToTheDefaultWorkspace(t *testing.T) {
	c, got := testClient(t, stub{200, `{"summary":{},"series":[],"breakdown":[]}`})

	if _, err := c.GetCostReportData(context.Background(), "", "rpt_1"); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "workspaceId", "ws_default")
}
