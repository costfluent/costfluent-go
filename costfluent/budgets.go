package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Budget represents a budget: a start date, a cadence of IntervalCount days, weeks, months or
// years, and an amount for each period. Measure says what the amounts count: money in Currency for
// a cost budget, a quantity in UsageUnit for a usage budget.
type Budget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Amount is the amount every stored period carries, and unset when they differ.
	Amount Decimal `json:"amount"`
	// Currency is the budget's currency, which a usage budget keeps although nothing it counts is
	// in it.
	Currency string `json:"currency"`
	// Measure is MeasureCost or MeasureUsage, fixed once created. More values may be added.
	Measure string `json:"measure"`
	// UsageUnit is the one usage unit a usage budget counts, such as Tokens, compared exactly with
	// the unit its report's rows carry; nil for a cost budget.
	UsageUnit     *string `json:"usageUnit,omitempty"`
	StartDate     string  `json:"startDate"`
	IntervalCount int     `json:"intervalCount"`
	// IntervalUnit is day, week, month or year; a quarter is three months.
	IntervalUnit string `json:"intervalUnit"`
	// Periods are the stored periods in order from the first. A day after the last one falls in
	// a period carrying the last amount.
	Periods []BudgetPeriod `json:"periods,omitempty"`
	Status  string         `json:"status"`
	// State is onTrack, atRisk, over, notStarted before the start date, or unknown when the
	// spend cannot be measured. More values may be added.
	State string `json:"state"`
	// CurrentPeriod is the period holding the workspace's today, nil before the start date.
	CurrentPeriod *BudgetPeriod `json:"currentPeriod,omitempty"`
	// SegmentID is the allocation segment the budget measures, if any.
	SegmentID *string `json:"segmentId,omitempty"`
	// CostReportID is the saved cost report the budget measures, if any. Exclusive with SegmentID.
	CostReportID *string `json:"costReportId,omitempty"`
	// CurrentSpend, BudgetedToDate, ProjectedSpend and PercentUsed are unset when the API sends
	// them as null: the spend is not computed, or, for all but CurrentSpend, there is no current
	// period. They are left out when written rather than written as an empty amount or 0, which
	// would read as a measured spend.
	CurrentSpend Decimal `json:"currentSpend,omitempty"`
	// BudgetedToDate is the current period's amount prorated over its days so far.
	BudgetedToDate Decimal `json:"budgetedToDate,omitempty"`
	// ProjectedSpend is the current period's spend by its end at the pace kept so far.
	ProjectedSpend Decimal `json:"projectedSpend,omitempty"`
	// PercentUsed is the spend as a percentage of the current period's whole amount.
	PercentUsed *float64 `json:"percentUsed"`
	// SpendAvailability is computed when CurrentSpend is the answer in the budget's currency, zero
	// included (Unconverted names what it leaves out); unavailable when the budget's scope cannot
	// be measured (its report was deleted, or its segment is gone); outsidePlan when the current
	// period lies wholly before the oldest day the plan keeps cost history for; or noUsage when no
	// row of a usage budget's report carries its unit this period. CurrentSpend means nothing
	// unless it is computed. More values may be added.
	SpendAvailability string `json:"spendAvailability"`
	// Unconverted is the spend in other billing currencies the budget does not count: no rate for
	// its month, or the workspace's conversion is off. CurrentSpend, PercentUsed, ProjectedSpend
	// and State exclude it, so a budget can read onTrack while this spend puts its scope over.
	// Empty unless SpendAvailability is computed.
	Unconverted []CurrencyAmount `json:"unconverted,omitempty"`
	// HistoryFloor is the plan's history floor (YYYY-MM-DD) when the current period started
	// before it: CurrentSpend counts from it, so it is partial, or, with outsidePlan, the whole
	// period lies before it. Nil when the whole period is measured.
	HistoryFloor *string       `json:"historyFloor,omitempty"`
	Alerts       []BudgetAlert `json:"alerts,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    *time.Time    `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (b *Budget) setETag(etag string) { b.ETag = etag }

// BudgetPeriod is one period of a budget: its first and last day (YYYY-MM-DD), both counted, and
// its amount. As input, the periods are the cadence's own in order from the start date.
type BudgetPeriod struct {
	StartDate string  `json:"startDate"`
	EndDate   string  `json:"endDate"`
	Amount    Decimal `json:"amount"`
}

// BudgetAlert is one threshold on a budget and whether spend has crossed it
type BudgetAlert struct {
	ThresholdPercent int `json:"thresholdPercent"`
	// AppIDs are the notification channels this threshold delivers to.
	AppIDs      []string   `json:"appIds,omitempty"`
	IsTriggered bool       `json:"isTriggered"`
	TriggeredAt *time.Time `json:"triggeredAt,omitempty"`
}

// BudgetAlertInput sets a threshold, as a percentage of the current period's amount. When several
// are crossed at once only the highest notifies.
type BudgetAlertInput struct {
	ThresholdPercent int `json:"thresholdPercent"`
}

// BudgetsListResponse is the response for listing budgets
type BudgetsListResponse = ListResponse[Budget]

// ListBudgets returns paginated budgets, of one workspace when workspaceID or the client's
// default workspace is set, and of the whole organization otherwise
func (c *Client) ListBudgets(ctx context.Context, workspaceID string, opts *PageOptions) (*BudgetsListResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/budgets", nil)
	if err != nil {
		return nil, err
	}
	if id := c.workspace(workspaceID); id != "" {
		q := req.URL.Query()
		q.Set("workspaceId", id)
		req.URL.RawQuery = q.Encode()
	}
	opts.apply(req)

	var resp BudgetsListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllBudgets fetches all budgets across all pages
func (c *Client) ListAllBudgets(ctx context.Context, workspaceID string, pageSize int) ([]Budget, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[Budget], error) {
		return c.ListBudgets(ctx, workspaceID, p)
	})
}

// GetBudget returns a single budget by ID, with the ETag an update can send back
func (c *Client) GetBudget(ctx context.Context, id string) (*Budget, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/budgets/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	return c.doBudget(req)
}

// doBudget sends a request answered with one budget.
func (c *Client) doBudget(req *http.Request) (*Budget, error) {
	var budget Budget
	if err := c.do(req, &budget); err != nil {
		return nil, err
	}
	return &budget, nil
}

// CreateBudgetInput for creating a new budget. WorkspaceID falls back to the client's default
// workspace; StartDate (YYYY-MM-DD) is today when unset; IntervalCount and IntervalUnit are one
// month when unset. Amount gives every period the same amount and Periods one amount each, exactly
// one of the two. SegmentID scopes the budget to one allocation segment's cost and CostReportID to
// what one saved cost report counts, at most one of the two; AppIDs are the notification channels
// every threshold delivers to. Measure is MeasureCost when unset; a MeasureUsage budget counts
// UsageUnit on CostReportID's rows, needs both and takes no SegmentID, and a cost budget takes no
// UsageUnit. A report holds at most one budget of each measure.
type CreateBudgetInput struct {
	WorkspaceID   string             `json:"workspaceId"`
	Name          string             `json:"name"`
	Currency      string             `json:"currency"`
	Measure       string             `json:"measure,omitempty"`
	UsageUnit     *string            `json:"usageUnit,omitempty"`
	StartDate     *string            `json:"startDate,omitempty"`
	IntervalCount int                `json:"intervalCount,omitempty"`
	IntervalUnit  string             `json:"intervalUnit,omitempty"`
	Amount        *Decimal           `json:"amount,omitempty"`
	Periods       []BudgetPeriod     `json:"periods,omitempty"`
	Alerts        []BudgetAlertInput `json:"alerts,omitempty"`
	SegmentID     *string            `json:"segmentId,omitempty"`
	CostReportID  *string            `json:"costReportId,omitempty"`
	AppIDs        []string           `json:"appIds,omitempty"`
}

// CreateBudget creates a new budget
func (c *Client) CreateBudget(ctx context.Context, input *CreateBudgetInput) (*Budget, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/budgets", &body)
	if err != nil {
		return nil, err
	}
	return c.doBudget(req)
}

// UpdateBudgetInput for updating a budget. The start date, cadence, currency, measure and alerts are
// fixed at creation. Amount gives every period that amount and Periods replaces them, at most one of
// the two, nil leaving the amounts. AppIDs replaces the notification channels: nil leaves them, an
// empty slice removes them all. UsageUnit changes a usage budget's unit and keeps its periods'
// amounts; a cost budget refuses it, and a usage budget refuses a segment or ClearCostReport.
type UpdateBudgetInput struct {
	Name    *string         `json:"name,omitempty"`
	Amount  *Decimal        `json:"amount,omitempty"`
	Periods *[]BudgetPeriod `json:"periods,omitempty"`
	// SegmentID or CostReportID moves the budget onto that scope; ClearSegment or ClearCostReport
	// returns it to the whole workspace. Naming none keeps the scope.
	SegmentID       *string   `json:"segmentId,omitempty"`
	ClearSegment    bool      `json:"clearSegment,omitempty"`
	CostReportID    *string   `json:"costReportId,omitempty"`
	ClearCostReport bool      `json:"clearCostReport,omitempty"`
	AppIDs          *[]string `json:"appIds,omitempty"`
	UsageUnit       *string   `json:"usageUnit,omitempty"`
}

// UpdateBudget updates an existing budget
func (c *Client) UpdateBudget(ctx context.Context, id string, input *UpdateBudgetInput, opts ...RequestOption) (*Budget, error) {
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/budgets/"+url.PathEscape(id), input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.doBudget(req)
}

// DeleteBudget removes a budget
func (c *Client) DeleteBudget(ctx context.Context, id string, opts ...RequestOption) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/v1/budgets/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
