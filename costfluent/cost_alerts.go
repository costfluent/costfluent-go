package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Cost alert threshold types
const (
	CostAlertThresholdAbsolute           = "absolute"
	CostAlertThresholdPercentageIncrease = "percentageIncrease"
	// CostAlertThresholdBudgetPercentage measures the month-to-date spend, from the 1st of the
	// month through the last settled day, in the alert's currency: the alert fires once it
	// reaches ThresholdValue as an amount. It references no budget; the name is the API's wire
	// value, kept so the configurations that send it keep working.
	CostAlertThresholdBudgetPercentage = "budgetPercentage"
	CostAlertThresholdTagCoverageBelow = "tagCoverageBelow"
)

// Cost alert comparison periods, for a percentageIncrease threshold
const (
	CostAlertComparePreviousDay      = "previousDay"
	CostAlertComparePreviousWeek     = "previousWeek"
	CostAlertComparePreviousMonth    = "previousMonth"
	CostAlertCompareSameDayLastMonth = "sameDayLastMonth"
)

// CostAlert watches a workspace's cost, narrowed by providers and a filter, against a threshold
// and notifies the linked apps when it is crossed. MinimumAmount, set only on a percentageIncrease
// alert, is the increase in currency the cost must also reach before the alert fires.
type CostAlert struct {
	ID                         string     `json:"id"`
	Name                       string     `json:"name"`
	ProviderIDs                []string   `json:"providerIds,omitempty"`
	Filter                     *string    `json:"filter,omitempty"`
	ThresholdType              string     `json:"thresholdType"`
	ThresholdValue             Decimal    `json:"thresholdValue"`
	ComparisonPeriod           *string    `json:"comparisonPeriod,omitempty"`
	MinimumAmount              Decimal    `json:"minimumAmount,omitempty"`
	AppIDs                     []string   `json:"appIds"`
	EvaluationFrequencyMinutes int        `json:"evaluationFrequencyMinutes"`
	LastEvaluatedAt            *time.Time `json:"lastEvaluatedAt,omitempty"`
	Status                     string     `json:"status"`
	CreatedAt                  time.Time  `json:"createdAt"`
	UpdatedAt                  *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (a *CostAlert) setETag(etag string) { a.ETag = etag }

// CostAlertsListResponse is the response for listing cost alerts
type CostAlertsListResponse = ListResponse[CostAlert]

// ListCostAlerts returns paginated cost alerts
func (c *Client) ListCostAlerts(ctx context.Context, workspaceID string, opts *PageOptions) (*CostAlertsListResponse, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/cost-alerts", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp CostAlertsListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllCostAlerts fetches all cost alerts across all pages
func (c *Client) ListAllCostAlerts(ctx context.Context, workspaceID string, pageSize int) ([]CostAlert, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[CostAlert], error) {
		return c.ListCostAlerts(ctx, workspaceID, p)
	})
}

// GetCostAlert returns a single cost alert by ID
func (c *Client) GetCostAlert(ctx context.Context, workspaceID, id string) (*CostAlert, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/cost-alerts/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var alert CostAlert
	if err := c.do(req, &alert); err != nil {
		return nil, err
	}
	return &alert, nil
}

// CreateCostAlertInput for creating a new cost alert. WorkspaceID falls back to the client's
// default workspace.
type CreateCostAlertInput struct {
	WorkspaceID                string   `json:"workspaceId"`
	Name                       string   `json:"name"`
	ThresholdType              string   `json:"thresholdType"`
	ThresholdValue             Decimal  `json:"thresholdValue"`
	ComparisonPeriod           *string  `json:"comparisonPeriod,omitempty"`
	MinimumAmount              *Decimal `json:"minimumAmount,omitempty"`
	ProviderIDs                []string `json:"providerIds,omitempty"`
	Filter                     *string  `json:"filter,omitempty"`
	AppIDs                     []string `json:"appIds,omitempty"`
	EvaluationFrequencyMinutes *int     `json:"evaluationFrequencyMinutes,omitempty"`
}

// CreateCostAlert creates a new cost alert and returns it as stored
func (c *Client) CreateCostAlert(ctx context.Context, input *CreateCostAlertInput) (*CostAlert, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/cost-alerts", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*CostAlert, error) {
		return c.GetCostAlert(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateCostAlertInput for updating a cost alert; unset fields are left as they are. A
// MinimumAmount of zero removes the floor. ProviderIDs and AppIDs replace their lists: nil leaves
// a list, an empty slice empties it, which watches every provider again or removes every channel.
type UpdateCostAlertInput struct {
	Name                       *string   `json:"name,omitempty"`
	ThresholdType              *string   `json:"thresholdType,omitempty"`
	ThresholdValue             *Decimal  `json:"thresholdValue,omitempty"`
	ComparisonPeriod           *string   `json:"comparisonPeriod,omitempty"`
	ClearComparisonPeriod      bool      `json:"clearComparisonPeriod,omitempty"`
	MinimumAmount              *Decimal  `json:"minimumAmount,omitempty"`
	ClearMinimumAmount         bool      `json:"clearMinimumAmount,omitempty"`
	ProviderIDs                *[]string `json:"providerIds,omitempty"`
	Filter                     *string   `json:"filter,omitempty"`
	ClearFilter                bool      `json:"clearFilter,omitempty"`
	AppIDs                     *[]string `json:"appIds,omitempty"`
	EvaluationFrequencyMinutes *int      `json:"evaluationFrequencyMinutes,omitempty"`
}

// UpdateCostAlert updates an existing cost alert and returns it as stored
func (c *Client) UpdateCostAlert(ctx context.Context, workspaceID, id string, input *UpdateCostAlertInput, opts ...RequestOption) (*CostAlert, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/cost-alerts/"+url.PathEscape(id), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var alert CostAlert
	if err := c.do(req, &alert); err != nil {
		return nil, err
	}
	return &alert, nil
}

// DeleteCostAlert removes a cost alert
func (c *Client) DeleteCostAlert(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	return c.costAlertAction(ctx, http.MethodDelete, workspaceID, id, "", opts...)
}

// PauseCostAlert stops evaluating a cost alert until it is resumed
func (c *Client) PauseCostAlert(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	return c.costAlertAction(ctx, http.MethodPost, workspaceID, id, "/pause", opts...)
}

// ResumeCostAlert resumes evaluating a paused cost alert
func (c *Client) ResumeCostAlert(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	return c.costAlertAction(ctx, http.MethodPost, workspaceID, id, "/resume", opts...)
}

func (c *Client) costAlertAction(ctx context.Context, method, workspaceID, id, suffix string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, method, "/v1/cost-alerts/"+url.PathEscape(id)+suffix, workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
