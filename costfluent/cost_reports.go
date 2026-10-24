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
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	FolderID     *string `json:"folderId,omitempty"`
	ChartType    string  `json:"chartType"`
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
		"CommitmentType", "PricingCategory", "Sku",
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
// workspace to chart cost per unit of, with the cost measure in a bin other than Cumulative; it is
// taken as sent, so settings without it chart cost alone. UnitCostLabelFilter chooses which of the
// metric's values the cost is divided by, and is taken as sent the same way: empty divides by every
// value. ComparisonStartDate and ComparisonEndDate are the window the report is compared against,
// when one is set.
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
// read as on CostReportSettings. ComparisonStartDate and ComparisonEndDate, both YYYY-MM-DD, are the
// window the report is compared against and are taken as sent like the unit cost metric: an update
// whose settings leave them out clears the window, so send back the window the report holds.
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
	ComparisonStartDate *string                  `json:"comparisonStartDate,omitempty"`
	ComparisonEndDate   *string                  `json:"comparisonEndDate,omitempty"`
}

// CreateCostReportInput creates a saved cost report.
type CreateCostReportInput struct {
	WorkspaceID  string                   `json:"workspaceId"`
	Title        string                   `json:"title"`
	FolderID     *string                  `json:"folderId,omitempty"`
	ChartType    *string                  `json:"chartType,omitempty"`
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
	Title          *string                  `json:"title,omitempty"`
	FolderID       *string                  `json:"folderId,omitempty"`
	ClearFolder    bool                     `json:"clearFolder,omitempty"`
	ChartType      *string                  `json:"chartType,omitempty"`
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

	// UnitCost is the cost per unit of the report's business metric, when it names one.
	UnitCost *CostReportUnitCost `json:"unitCost,omitempty"`
}

// CostReportUnitCost is a report's cost per unit of a business metric, period by period. Currency
// is empty when the report's costs mix currencies, and then no period has a unit cost; Average is
// the window's figure over the periods that have both a cost and a metric amount.
type CostReportUnitCost struct {
	MetricID string                    `json:"metricId"`
	Name     string                    `json:"name"`
	Unit     string                    `json:"unit"`
	Currency *string                   `json:"currency,omitempty"`
	Average  *Decimal                  `json:"average,omitempty"`
	Series   []CostReportUnitCostPoint `json:"series"`
}

// CostReportUnitCostPoint is one period's metric amount and cost per unit; either is nil where the
// metric has no amount, and the unit cost also where the amount is zero.
type CostReportUnitCostPoint struct {
	Date         string   `json:"date,omitempty"`
	MetricAmount *Decimal `json:"metricAmount,omitempty"`
	UnitCost     *Decimal `json:"unitCost,omitempty"`
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
