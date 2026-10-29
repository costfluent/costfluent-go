package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// CostFilterOptions are the scoping options the cost data and cost summary queries share. Dates
// are calendar days, YYYY-MM-DD, and both are required. A nil switch keeps the API's default:
// credits excluded, refunds and tax included, amortized cost.
type CostFilterOptions struct {
	StartDate string
	EndDate   string

	// WorkspaceID falls back to the client's default workspace, and to the whole organization
	// when neither is set.
	WorkspaceID     string
	CloudAccountIDs []string
	Filter          string
	IncludeCredits  *bool
	IncludeRefunds  *bool
	IncludeTax      *bool
	Amortize        *bool
}

func (c *Client) applyCostFilter(q url.Values, o *CostFilterOptions) {
	setIf(q, "startDate", o.StartDate)
	setIf(q, "endDate", o.EndDate)
	setIf(q, "workspaceId", c.workspace(o.WorkspaceID))
	for _, id := range o.CloudAccountIDs {
		q.Add("cloudAccountIds", id)
	}
	setIf(q, "filter", o.Filter)
	for key, v := range map[string]*bool{
		"includeCredits": o.IncludeCredits,
		"includeRefunds": o.IncludeRefunds,
		"includeTax":     o.IncludeTax,
		"amortize":       o.Amortize,
	} {
		if v != nil {
			q.Set(key, strconv.FormatBool(*v))
		}
	}
}

// CostDataPageSize is the most rows the API serves on one page of cost data, and the page a query
// without a Limit asks for. Paging reaches CostDataMaxRows (page × limit); a larger read splits its
// date window into shorter ranges.
const (
	CostDataPageSize = 5000
	CostDataMaxRows  = 100_000
)

// The measures a cost data query or a cost report can chart besides cost. A budget's Measure is
// MeasureCost or MeasureUsage.
const (
	MeasureCost  = "cost"
	MeasureUsage = "usage"
	MeasureCount = "count"
)

// The calculations a cost report draws its business metric with. UnitCostPerUnit divides each
// bin's figure in the report's measure, cost or usage, by the metric's amount; UnitCostGrossMargin
// is (revenue - cost) / revenue as a fraction, 0.4 for 40%, on the cost measure and a metric whose
// unit is the costs' currency code; UnitCostMetricValue is the metric's summed amount itself, on
// any measure. More values may be added.
const (
	UnitCostPerUnit     = "perUnit"
	UnitCostGrossMargin = "grossMargin"
	UnitCostMetricValue = "metricValue"
)

// The periods a cost data query bins its rows into. GranularityCumulative is daily points that each
// carry the window's running total, so the last is the window's total.
const (
	GranularityDay        = "day"
	GranularityWeek       = "week"
	GranularityMonth      = "month"
	GranularityQuarter    = "quarter"
	GranularityCumulative = "cumulative"
)

// CommitmentDiscountPlacement is how a cost read's amortized figures place commitment discounts,
// spelled as the Public API sends it: AsApplied on the usage the provider applied each to, Blended
// with each day's AWS Savings Plan and Reserved Instance discount spread evenly across the usage it
// could cover, which a read of a workspace with BlendCommitmentDiscounts on states. Billed figures
// are never spread. A new placement may appear, so a client should tolerate one it does not list.
// TestCommitmentDiscountPlacementsMatchTheContract holds this list to the API's declared values.
type CommitmentDiscountPlacement string

const (
	CommitmentDiscountPlacementAsApplied CommitmentDiscountPlacement = "asApplied"
	CommitmentDiscountPlacementBlended   CommitmentDiscountPlacement = "blended"
)

// CommitmentDiscountPlacements returns every placement the Public API declares, in the contract's
// order.
func CommitmentDiscountPlacements() []CommitmentDiscountPlacement {
	return []CommitmentDiscountPlacement{
		CommitmentDiscountPlacementAsApplied,
		CommitmentDiscountPlacementBlended,
	}
}

// CostDataQuery reads cost over time. Granularity is one of the Granularity constants
// (GranularityDay when empty), and the answer's Meta.Granularity names it the same way. GroupBy
// is up to three slots, outermost first, such as ServiceName then Tag:team; Segment and
// VirtualTag need a workspace. Limit defaults to CostDataPageSize. Measure is
// MeasureUsage to give each row its consumed quantity and unit, one row per unit, or MeasureCount
// for its distinct resources; empty reads cost alone. DataVersion is the Meta.DataVersion of an
// earlier page of the same read: with it, a page whose data a sync changed since that page is
// refused with a conflict (IsConflict) rather than read from the changed data, and the read
// starts again from page 1. Empty sends none.
type CostDataQuery struct {
	CostFilterOptions
	Granularity string
	GroupBy     []Grouping
	Measure     string
	DataVersion string
	PageOptions
}

// CostDataResponse is one page of cost rows, links to the neighbouring pages, and what the whole
// query adds up to
type CostDataResponse struct {
	Links Links           `json:"links"`
	Data  []CostDataPoint `json:"data"`
	Meta  CostDataMeta    `json:"meta"`
}

// CostDataPoint is the cost of one period, and of one group when the query grouped. Dimensions is
// keyed by the slot as Grouping.String spells it. Quantity is the consumed quantity in Unit for
// MeasureUsage and the distinct resources for MeasureCount; both are nil for cost.
type CostDataPoint struct {
	Date          string            `json:"date"`
	Cost          Decimal           `json:"cost"`
	ListCost      Decimal           `json:"listCost"`
	AmortizedCost Decimal           `json:"amortizedCost"`
	Currency      string            `json:"currency"`
	Dimensions    map[string]string `json:"dimensions,omitempty"`
	Quantity      *float64          `json:"quantity,omitempty"`
	Unit          *string           `json:"unit,omitempty"`
}

// CostDataMeta describes the query that was answered
type CostDataMeta struct {
	StartDate    string                `json:"startDate"`
	EndDate      string                `json:"endDate"`
	Granularity  string                `json:"granularity"`
	Measure      string                `json:"measure,omitempty"`
	TotalRecords int                   `json:"totalRecords"`
	TotalCost    Decimal               `json:"totalCost"`
	Currency     string                `json:"currency"`
	Presentation *CurrencyPresentation `json:"presentation,omitempty"`

	// CommitmentDiscounts is how the amortized figures placed commitment discounts.
	CommitmentDiscounts CommitmentDiscountPlacement `json:"commitmentDiscounts"`

	// HistoryFloor is the earliest day the organization's plan lets it read, when the plan
	// limits history.
	HistoryFloor *string `json:"historyFloor,omitempty"`

	// DataThrough is the latest day every connection the read covers has committed data for, the
	// earliest of theirs; nil until any of them has. A later day may still change.
	DataThrough *string `json:"dataThrough,omitempty"`

	// DataVersion is opaque: two pages carry the same one when no sync changed the data between
	// them. Pass the first page's as CostDataQuery.DataVersion when reading the next.
	DataVersion string `json:"dataVersion"`
}

// CurrencyPresentation explains how figures in more than one currency were brought to one
type CurrencyPresentation struct {
	Conversions      []CurrencyConversion `json:"conversions,omitempty"`
	Unconverted      []CurrencyAmount     `json:"unconverted,omitempty"`
	TotalsByCurrency []CurrencyAmount     `json:"totalsByCurrency,omitempty"`
}

// CurrencyConversion is one source currency converted into the presentation currency
type CurrencyConversion struct {
	SourceCurrency  string  `json:"sourceCurrency"`
	SourceAmount    Decimal `json:"sourceAmount"`
	ConvertedAmount Decimal `json:"convertedAmount"`
	Method          string  `json:"method,omitempty"`
	Source          string  `json:"source"`
	Description     string  `json:"description"`
}

// CurrencyAmount is an amount in one currency
type CurrencyAmount struct {
	Currency string  `json:"currency"`
	Amount   Decimal `json:"amount"`
}

// QueryCostData reads cost over time with optional grouping and filtering
func (c *Client) QueryCostData(ctx context.Context, query *CostDataQuery) (*CostDataResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/costs", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	c.applyCostFilter(q, &query.CostFilterOptions)
	setIf(q, "granularity", query.Granularity)
	setIf(q, "measure", query.Measure)
	setIf(q, "dataVersion", query.DataVersion)
	for _, g := range query.GroupBy {
		q.Add("groupBy", g.String())
	}
	req.URL.RawQuery = q.Encode()
	page := query.PageOptions
	if page.Limit == 0 {
		page.Limit = CostDataPageSize
	}
	page.apply(req)

	var resp CostDataResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CostSummary is the total cost of a window, its change against the window before, and where it
// went
type CostSummary struct {
	TotalCost          Decimal `json:"totalCost"`
	TotalListCost      Decimal `json:"totalListCost"`
	TotalAmortizedCost Decimal `json:"totalAmortizedCost"`
	// Currency is nil when the window mixes currencies.
	Currency   *string `json:"currency"`
	CostChange Decimal `json:"costChange"`
	// CostChangePercent is nil, like an unset CostChange, when the previous window was not
	// compared, so it never reads as no change.
	CostChangePercent *float64              `json:"costChangePercent"`
	Presentation      *CurrencyPresentation `json:"presentation,omitempty"`
	// CommitmentDiscounts is as on CostDataMeta.
	CommitmentDiscounts CommitmentDiscountPlacement `json:"commitmentDiscounts"`
	TopServices         []ServiceCostBreakdown      `json:"topServices,omitempty"`
	ByProvider          []ProviderCostBreakdown     `json:"byProvider,omitempty"`
	HistoryFloor        *string                     `json:"historyFloor,omitempty"`
	// DataThrough is as on CostDataMeta.
	DataThrough *string `json:"dataThrough,omitempty"`
	// ComparedThrough is the settled day CostChange and CostChangePercent compare through when the
	// window runs past it: the window's days through it against as many days from the start of the
	// window before, because the last days are still arriving. Nil when both are compared whole.
	ComparedThrough *string `json:"comparedThrough,omitempty"`
}

// ServiceCostBreakdown is one service's share of the window's total cost; with
// conversion off over several currencies, of the total in its own currency.
type ServiceCostBreakdown struct {
	ServiceName string  `json:"serviceName"`
	Cost        Decimal `json:"cost"`
	Percentage  float64 `json:"percentage"`
}

// ProviderCostBreakdown is one provider's share of the total
type ProviderCostBreakdown struct {
	ProviderID   string      `json:"providerId"`
	ProviderName string      `json:"providerName"`
	ProviderKey  ProviderKey `json:"providerKey"`
	Cost         Decimal     `json:"cost"`
}

// GetCostSummary summarizes cost over a window
func (c *Client) GetCostSummary(ctx context.Context, opts *CostFilterOptions) (*CostSummary, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/costs/summary", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	c.applyCostFilter(q, opts)
	req.URL.RawQuery = q.Encode()

	var summary CostSummary
	if err := c.do(req, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}
