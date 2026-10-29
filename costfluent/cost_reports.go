package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// CostReport is a saved cost analysis: one window, up to three grouping slots, one filter and the
// money settings the numbers are computed with.
//
// A CostReportSubscription delivers one of these on a schedule; the report itself is the saved
// definition, not a frozen copy of what it showed.
type CostReport struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	FolderID  *string `json:"folderId,omitempty"`
	ChartType string  `json:"chartType"`
	// ChartAxis is what the chart sets along its horizontal axis: "date", or "group", the report's
	// grouping, which a bar or stacked bar chart of a grouped, non-cumulative report draws.
	ChartAxis    string  `json:"chartAxis"`
	DateBin      string  `json:"dateBin"`
	DateInterval string  `json:"dateInterval"`
	StartDate    *string `json:"startDate,omitempty"`
	EndDate      *string `json:"endDate,omitempty"`

	// Filter is the cost filter as its JSON document, the same one the app puts in its URL.
	// Nil means the report asks for no filter.
	Filter *string `json:"filter,omitempty"`

	// GroupBy is the slots the report breaks its window down by, outermost first; empty for one
	// total.
	GroupBy []Grouping `json:"groupBy"`

	Settings  CostReportSettings `json:"settings"`
	CreatedAt *time.Time         `json:"createdAt,omitempty"`
	UpdatedAt *time.Time         `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (r *CostReport) setETag(etag string) { r.ETag = etag }

// Grouping is one slot cost is broken down by: one of CostGroupDimensions and, for the keyed ones,
// its key: the tag key for Tag, the virtual tag ID for VirtualTag.
type Grouping struct {
	Dimension string  `json:"dimension"`
	Key       *string `json:"key,omitempty"`
}

// CostGroupDimensions returns every dimension cost can be grouped by, as the Public API names
// them, in the contract's order. The API reads them case-insensitively.
func CostGroupDimensions() []string {
	return []string{
		"Provider", "ProviderName", "SubAccount", "ServiceName", "Region", "ResourceName", "ChargeCategory",
		"Tag", "VirtualTag", "Segment", "Cluster", "Namespace", "BillingAccount", "ServiceCategory",
		"ServiceSubcategory", "UsageUnit", "ChargeClass", "Tagged", "ResourceId", "CommitmentStatus",
		"CommitmentType", "PricingCategory", "Sku", "CommitmentId", "Operation", "ChargeDescription",
		"ResourceType", "ProductFamily", "SkuId", "InvoiceId", "InvoiceIssuer", "BillingPeriod", "ResourceGroup",
	}
}

// String is the slot as the cost data query and a row's dimensions spell it: ServiceName, or
// Tag:team when the slot has a key.
func (g Grouping) String() string {
	if g.Key == nil || *g.Key == "" {
		return g.Dimension
	}
	return g.Dimension + ":" + *g.Key
}

// CostReportSettings decide what the report counts. Measure chooses what it charts (MeasureCost,
// MeasureUsage or MeasureCount; empty keeps the report's own on update and cost on create); every
// other setting changes the money: Amortize selects effective cost over billed cost, and the
// inclusions are charge-category predicates. UnitCostMetricID names a business metric of the
// workspace to chart beside the report in a bin other than Cumulative, drawn as UnitCostCalculation
// says: UnitCostPerUnit (the default) divides the cost or usage measure by the metric's amount,
// UnitCostGrossMargin charts the margin on the cost measure, and UnitCostMetricValue the metric's own
// amount on any measure. UnitCostScale charts a per unit figure per that many units: 1 (the default),
// 100, 1000, 1000000 or 1000000000, other than 1 only beside UnitCostPerUnit. The metric is taken as
// sent, so settings without it chart the measure alone and return the calculation and scale to their
// defaults. UnitCostLabelFilter chooses which of the metric's values count, and is taken as sent the
// same way: empty counts every value. ComparisonStartDate and ComparisonEndDate are the window the
// report is compared against, when one is set.
type CostReportSettings struct {
	Amortize            bool                     `json:"amortize"`
	IncludeCredits      bool                     `json:"includeCredits"`
	IncludeRefunds      bool                     `json:"includeRefunds"`
	IncludeTax          bool                     `json:"includeTax"`
	ShowForecast        bool                     `json:"showForecast"`
	CompareToLastPeriod bool                     `json:"compareToLastPeriod"`
	Measure             string                   `json:"measure,omitempty"`
	UnitCostMetricID    *string                  `json:"unitCostMetricId,omitempty"`
	UnitCostLabelFilter []UnitCostLabelCondition `json:"unitCostLabelFilter,omitempty"`
	UnitCostCalculation string                   `json:"unitCostCalculation,omitempty"`
	UnitCostScale       int                      `json:"unitCostScale,omitempty"`
	ComparisonStartDate *string                  `json:"comparisonStartDate,omitempty"`
	ComparisonEndDate   *string                  `json:"comparisonEndDate,omitempty"`
}

// UnitCostLabelCondition is one label column of the unit cost metric and the values of it the unit
// cost counts. A metric value counts when its label in every condition's column is one of that
// condition's values.
type UnitCostLabelCondition struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// CostReportSettingsInput is the settings a create or update sends. A nil flag is left out: a new
// report takes the workspace's default for it and an update keeps the report's own, so only the
// flags a caller sets are decided by the caller. Measure, UnitCostMetricID and UnitCostLabelFilter
// read as on CostReportSettings. UnitCostCalculation and UnitCostScale do too, and an empty
// calculation or a nil scale is left out like a flag: a new report charts per unit, per 1, and an
// update naming a metric keeps the report's own. ComparisonStartDate and ComparisonEndDate, both
// YYYY-MM-DD, are the window the report is compared against and are taken as sent like the unit
// cost metric: an update whose settings leave them out clears the window, so send back the window
// the report holds.
type CostReportSettingsInput struct {
	Amortize            *bool                    `json:"amortize,omitempty"`
	IncludeCredits      *bool                    `json:"includeCredits,omitempty"`
	IncludeRefunds      *bool                    `json:"includeRefunds,omitempty"`
	IncludeTax          *bool                    `json:"includeTax,omitempty"`
	ShowForecast        *bool                    `json:"showForecast,omitempty"`
	CompareToLastPeriod *bool                    `json:"compareToLastPeriod,omitempty"`
	Measure             string                   `json:"measure,omitempty"`
	UnitCostMetricID    *string                  `json:"unitCostMetricId,omitempty"`
	UnitCostLabelFilter []UnitCostLabelCondition `json:"unitCostLabelFilter,omitempty"`
	UnitCostCalculation string                   `json:"unitCostCalculation,omitempty"`
	UnitCostScale       *int                     `json:"unitCostScale,omitempty"`
	ComparisonStartDate *string                  `json:"comparisonStartDate,omitempty"`
	ComparisonEndDate   *string                  `json:"comparisonEndDate,omitempty"`
}

// CreateCostReportInput creates a saved cost report.
type CreateCostReportInput struct {
	WorkspaceID string  `json:"workspaceId"`
	Title       string  `json:"title"`
	FolderID    *string `json:"folderId,omitempty"`
	ChartType   *string `json:"chartType,omitempty"`
	// ChartAxis nil draws by date.
	ChartAxis    *string                  `json:"chartAxis,omitempty"`
	DateBin      *string                  `json:"dateBin,omitempty"`
	DateInterval *string                  `json:"dateInterval,omitempty"`
	StartDate    *string                  `json:"startDate,omitempty"`
	EndDate      *string                  `json:"endDate,omitempty"`
	Filter       *string                  `json:"filter,omitempty"`
	Settings     *CostReportSettingsInput `json:"settings,omitempty"`

	// GroupBy nil leaves the API's default grouping, ServiceName; an empty, non-nil slice asks for
	// one total.
	GroupBy []Grouping `json:"groupBy,omitzero"`
}

// UpdateCostReportInput changes a saved report's definition.
//
// ClearFilter exists because nil is a value here: omitting Filter leaves the saved filter alone,
// while ClearFilter removes it. Without the distinction a filter could be added but never taken
// away. ClearFolder does the same for the folder: a nil FolderID leaves the report where it is,
// ClearFolder takes it out of its folder, and the API refuses both together. GroupBy draws the
// same line with the slice itself: nil leaves the saved grouping alone, and an empty, non-nil
// slice clears it.
type UpdateCostReportInput struct {
	Title       *string `json:"title,omitempty"`
	FolderID    *string `json:"folderId,omitempty"`
	ClearFolder bool    `json:"clearFolder,omitempty"`
	ChartType   *string `json:"chartType,omitempty"`
	// ChartAxis nil keeps the axis while the updated report can draw it; the API draws a report by
	// date once a chart type, grouping or bin it cannot draw replaces it.
	ChartAxis      *string                  `json:"chartAxis,omitempty"`
	DateBin        *string                  `json:"dateBin,omitempty"`
	DateInterval   *string                  `json:"dateInterval,omitempty"`
	StartDate      *string                  `json:"startDate,omitempty"`
	ClearStartDate bool                     `json:"clearStartDate,omitempty"`
	EndDate        *string                  `json:"endDate,omitempty"`
	ClearEndDate   bool                     `json:"clearEndDate,omitempty"`
	Filter         *string                  `json:"filter,omitempty"`
	ClearFilter    bool                     `json:"clearFilter,omitempty"`
	GroupBy        []Grouping               `json:"groupBy,omitzero"`
	Settings       *CostReportSettingsInput `json:"settings,omitempty"`
}

// ListCostReports returns a workspace's saved cost reports.
func (c *Client) ListCostReports(ctx context.Context, workspaceID string) ([]CostReport, error) {
	return listWorkspace[CostReport](c, ctx, "/v1/cost-reports", workspaceID)
}

// GetCostReport returns a single saved cost report by ID.
func (c *Client) GetCostReport(ctx context.Context, workspaceID, reportID string) (*CostReport, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/cost-reports/"+url.PathEscape(reportID), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var report CostReport
	if err := c.do(req, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// CostReportData is what a saved report shows for its own window, filter, grouping and settings:
// the total, the series per period and group, and the breakdown of the window by group.
type CostReportData struct {
	Summary      CostReportSummary     `json:"summary"`
	Series       []CostReportPeriod    `json:"series"`
	Breakdown    []CostReportBreakdown `json:"breakdown"`
	StartDate    string                `json:"startDate,omitempty"`
	EndDate      string                `json:"endDate,omitempty"`
	Comparison   *CostReportComparison `json:"comparison,omitempty"`
	HistoryFloor *string               `json:"historyFloor,omitempty"`

	// DataThrough is the last day the numbers are complete for, when the API knows it.
	DataThrough *string `json:"dataThrough,omitempty"`

	// UnitCost is the report's business metric in its unit cost calculation, when it names one.
	UnitCost *CostReportUnitCost `json:"unitCost,omitempty"`

	// Forecast is the report's forecast over the days still arriving or to come, when it shows one.
	Forecast *CostReportForecast `json:"forecast,omitempty"`
}

// CostReportForecast is a report's forecast, period by period, from the first day whose cost is
// still arriving to the end of the window, in the summary's currency. It is nil when the report
// does not show a forecast, measures usage or count, its window ends today or earlier, or no
// forecast can be fitted.
type CostReportForecast struct {
	Series []CostReportForecastPoint `json:"series"`
}

// CostReportForecastPoint is one period's forecast: its cost so far, with each day still arriving
// or to come counted at the larger of its cost so far and the projection, and that amount's range.
type CostReportForecastPoint struct {
	Date       string  `json:"date,omitempty"`
	Amount     Decimal `json:"amount"`
	LowerBound Decimal `json:"lowerBound"`
	UpperBound Decimal `json:"upperBound"`
}

// CostReportUnitCost is a report's business metric period by period, in the report's Calculation
// (one of the UnitCost constants) and Scale, so it can be read without the report's settings.
// Currency is the costs' currency for UnitCostPerUnit on the cost measure and for
// UnitCostGrossMargin, and nil for every other calculation and measure; it is also nil when the
// costs mix currencies, and then neither of those two has a value in any period. Average is the
// window's figure over the periods that have one: the ratio of the totals per unit, the margin of
// the totals for a gross margin, and nil for UnitCostMetricValue.
type CostReportUnitCost struct {
	MetricID    string                    `json:"metricId"`
	Name        string                    `json:"name"`
	Unit        string                    `json:"unit"`
	Calculation string                    `json:"calculation"`
	Scale       int                       `json:"scale"`
	Currency    *string                   `json:"currency,omitempty"`
	Average     *Decimal                  `json:"average,omitempty"`
	Series      []CostReportUnitCostPoint `json:"series"`
}

// CostReportUnitCostPoint is one period's metric amount and calculated Value: the cost or usage per
// unit, the margin as a fraction (0.4 for 40%) or the metric's amount. Either is nil where the metric
// has no amount, and the value also where a ratio has no divisor. UnitCost is the same figure for
// UnitCostPerUnit on the cost measure, and nil for every other calculation and measure.
type CostReportUnitCostPoint struct {
	Date         string   `json:"date,omitempty"`
	MetricAmount *Decimal `json:"metricAmount,omitempty"`
	UnitCost     *Decimal `json:"unitCost,omitempty"`
	Value        *Decimal `json:"value,omitempty"`
}

// CostReportSummary is the report's total over its window and its change against the window
// before, in the report's measure.
type CostReportSummary struct {
	TotalCost          *Decimal              `json:"totalCost,omitempty"`
	PreviousPeriodCost *Decimal              `json:"previousPeriodCost,omitempty"`
	ChangePercent      *float64              `json:"changePercent,omitempty"`
	Currency           *string               `json:"currency,omitempty"`
	Presentation       *CurrencyPresentation `json:"presentation,omitempty"`
	Measure            string                `json:"measure,omitempty"`
	TotalsByUnit       []UnitQuantity        `json:"totalsByUnit,omitempty"`
	// ComparedThrough is the settled day PreviousPeriodCost and ChangePercent compare through when
	// the window runs past it, as on CostSummary; nil when both are compared whole.
	ComparedThrough *string `json:"comparedThrough,omitempty"`
	// ComparedCost is the window's total through ComparedThrough, the side ChangePercent compares
	// with PreviousPeriodCost; nil when both are compared whole, where TotalCost is that side.
	ComparedCost *Decimal `json:"comparedCost,omitempty"`
	// PreviousComparedThrough is the last day of the comparison window PreviousPeriodCost covers,
	// as many days from its start as the window has through ComparedThrough; nil when both are
	// compared whole or the window has no settled day yet.
	PreviousComparedThrough *string `json:"previousComparedThrough,omitempty"`
}

// UnitQuantity is a usage total in one unit.
type UnitQuantity struct {
	Unit     string  `json:"unit"`
	Quantity float64 `json:"quantity"`
}

// CostReportPeriod is one period of the report's series, split by its groups.
type CostReportPeriod struct {
	Date   string            `json:"date,omitempty"`
	Groups []CostReportGroup `json:"groups"`
}

// CostReportGroup is one group's amount in one period. Keys hold one value per grouping slot,
// outermost first; IsOther marks the group that folds the rest together.
type CostReportGroup struct {
	Keys     []string `json:"keys"`
	Currency *string  `json:"currency,omitempty"`
	IsOther  bool     `json:"isOther,omitempty"`
	Cost     Decimal  `json:"cost"`
	Unit     *string  `json:"unit,omitempty"`
}

// CostReportBreakdown is one group's amount over the whole window and its share of the total.
type CostReportBreakdown struct {
	Keys     []string `json:"keys"`
	Currency *string  `json:"currency,omitempty"`
	IsOther  bool     `json:"isOther,omitempty"`
	Cost     Decimal  `json:"cost"`
	Percent  float64  `json:"percent"`
	Unit     *string  `json:"unit,omitempty"`
	// PreviousCost is this group's part of the summary's PreviousPeriodCost, over the same days, 0
	// where it had none; the Other row's is every other group's, and an Other row of no cost carries
	// what only the comparison window had. Nil where the summary has no previous cost.
	PreviousCost *Decimal `json:"previousCost,omitempty"`
	// ComparedCost is this group's part of the summary's ComparedCost; nil when the windows are
	// compared whole, where Cost is that side.
	ComparedCost *Decimal `json:"comparedCost,omitempty"`
	// ChangePercent is ComparedCost, or Cost when compared whole, against PreviousCost; nil where
	// PreviousCost is zero or not compared.
	ChangePercent *float64 `json:"changePercent,omitempty"`
}

// CostReportComparison is the window before the report's own, day by day, when the report
// compares to the last period.
type CostReportComparison struct {
	StartDate string                      `json:"startDate,omitempty"`
	EndDate   string                      `json:"endDate,omitempty"`
	Series    []CostReportComparisonPoint `json:"series"`
}

// CostReportComparisonPoint pairs a day of the window with the day it is compared to.
type CostReportComparisonPoint struct {
	Date         string  `json:"date,omitempty"`
	PreviousDate string  `json:"previousDate,omitempty"`
	Cost         Decimal `json:"cost"`
}

// GetCostReportData returns the numbers a saved report shows, computed now.
func (c *Client) GetCostReportData(ctx context.Context, workspaceID, reportID string) (*CostReportData, error) {
	path := "/v1/cost-reports/" + url.PathEscape(reportID) + "/data"
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, path, workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var data CostReportData
	if err := c.do(req, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// CreateCostReport creates a saved cost report and returns it as stored.
func (c *Client) CreateCostReport(ctx context.Context, input *CreateCostReportInput) (*CostReport, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/cost-reports", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*CostReport, error) {
		return c.GetCostReport(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateCostReport changes a saved cost report's definition.
func (c *Client) UpdateCostReport(
	ctx context.Context, workspaceID, reportID string, input *UpdateCostReportInput, opts ...RequestOption,
) (*CostReport, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/cost-reports/"+url.PathEscape(reportID), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var report CostReport
	if err := c.do(req, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// DeleteCostReport removes a saved cost report.
func (c *Client) DeleteCostReport(ctx context.Context, workspaceID, reportID string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/cost-reports/"+url.PathEscape(reportID), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
