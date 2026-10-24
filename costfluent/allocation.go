package costfluent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Segment CRUD lives in segments.go: allocation rules claim cost for segments,
// and there is one allocation vocabulary rather than two names for the same row.

// AllocationRule maps a slice of provider cost to a typed segment.
//
// Filter is the cost filter JSON document, the same form a cost report's filter comes back in. The
// API refuses a rule filter that constrains nothing or reads a virtual tag.
type AllocationRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Priority    int        `json:"priority"`
	Filter      string     `json:"filter"`
	CostKind    string     `json:"costKind"`
	SegmentID   string     `json:"segmentId"`
	SegmentName *string    `json:"segmentName,omitempty"`
	CostCentre  *string    `json:"costCentre,omitempty"`
	IsEnabled   bool       `json:"isEnabled"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version a create or an update answered, to send back with IfMatch. Empty
	// from a list, which carries no versions, and there is no single read.
	ETag string `json:"-"`
}

func (r *AllocationRule) setETag(etag string) { r.ETag = etag }

// CreateAllocationRuleInput creates one ordered rule.
type CreateAllocationRuleInput struct {
	WorkspaceID string `json:"workspaceId"`
	Name        string `json:"name"`

	// Priority orders the set; lower wins. Ties break by creation time, so the order is total and
	// stable. Gaps are deliberate — inserting a rule ahead of another needs no renumbering.
	Priority int `json:"priority"`

	// Filter is CQL text, such as costs.service = 'AmazonEC2' AND costs.region = 'eu-central-1',
	// or the cost filter JSON document. The rule always reads back as the JSON document.
	Filter    string `json:"filter"`
	SegmentID string `json:"segmentId"`
	IsEnabled bool   `json:"isEnabled"`
}

// UpdateAllocationRuleInput replaces a rule's definition in one edit.
type UpdateAllocationRuleInput struct {
	Name      string `json:"name"`
	Priority  int    `json:"priority"`
	Filter    string `json:"filter"`
	SegmentID string `json:"segmentId"`
	IsEnabled bool   `json:"isEnabled"`
}

// ReorderAllocationRulesInput applies a whole ordering at once.
//
// Every rule in the workspace, exactly once. A partial ordering would leave the omitted rules at
// whatever priority they had, which is an order nobody declared.
type ReorderAllocationRulesInput struct {
	WorkspaceID string   `json:"workspaceId"`
	RuleIDs     []string `json:"ruleIds"`
}

// ListAllocationRules returns the rule set in compiled precedence order.
//
// The order is the meaning of the set, so this is also the export: a customer keeping their mapping
// in version control gets the same sequence the query engine evaluates.
func (c *Client) ListAllocationRules(ctx context.Context, workspaceID string) ([]AllocationRule, error) {
	return listWorkspace[AllocationRule](c, ctx, "/v1/allocation/rules", workspaceID)
}

// CreateAllocationRule creates one rule.
func (c *Client) CreateAllocationRule(
	ctx context.Context, input *CreateAllocationRuleInput,
) (*AllocationRule, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/allocation/rules", &body)
	if err != nil {
		return nil, err
	}

	var rule AllocationRule
	if err := c.do(req, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateAllocationRule replaces a rule's definition.
func (c *Client) UpdateAllocationRule(
	ctx context.Context, workspaceID, ruleID string, input *UpdateAllocationRuleInput, opts ...RequestOption,
) (*AllocationRule, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/allocation/rules/"+url.PathEscape(ruleID), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var rule AllocationRule
	if err := c.do(req, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// ReorderAllocationRules applies a whole ordering in one call.
func (c *Client) ReorderAllocationRules(
	ctx context.Context, input *ReorderAllocationRulesInput,
) ([]AllocationRule, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/allocation/rules/order", &body)
	if err != nil {
		return nil, err
	}

	var rules []AllocationRule
	if err := c.do(req, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// DeleteAllocationRule removes a rule.
//
// Allocation is evaluated at query time, so this restates every past month immediately — which is
// the point, not a side effect.
func (c *Client) DeleteAllocationRule(ctx context.Context, workspaceID, ruleID string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/allocation/rules/"+url.PathEscape(ruleID), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

// AllocationCoverage is how much of a window the rule set claims, per billing currency. StartDate
// and EndDate are the window read, both days included.
type AllocationCoverage struct {
	StartDate  string                       `json:"startDate"`
	EndDate    string                       `json:"endDate"`
	CostBasis  string                       `json:"costBasis"`
	Currencies []AllocationCoverageCurrency `json:"currencies"`
}

// AllocationCoverageCurrency is one currency's allocation. Segments plus unallocated always sum to
// the total: a breakdown that does not conserve its own total is not a breakdown.
type AllocationCoverageCurrency struct {
	BillingCurrency         string  `json:"billingCurrency"`
	TotalCost               Decimal `json:"totalCost"`
	DirectCost              Decimal `json:"directCost"`
	SharedCost              Decimal `json:"sharedCost"`
	UndistributedSharedCost Decimal `json:"undistributedSharedCost"`
	UnallocatedCost         Decimal `json:"unallocatedCost"`
	UnallocatedRecordCount  int64   `json:"unallocatedRecordCount"`
	RecordCount             int64   `json:"recordCount"`

	// Coverage is nil when the window's total is zero. Coverage of nothing is undefined, not 100%.
	Coverage *float64                    `json:"coverage"`
	Segments []AllocationCoverageSegment `json:"segments"`

	// Pools are each shared segment's pool and how it was split, so every share and the rounding
	// residual read exactly.
	Pools []AllocationCoveragePool `json:"pools"`
}

// AllocationCoverageSegment is one segment's share of the window.
type AllocationCoverageSegment struct {
	SegmentID     string  `json:"segmentId"`
	Name          string  `json:"name"`
	CostCentre    string  `json:"costCentre"`
	DirectCost    Decimal `json:"directCost"`
	SharedCost    Decimal `json:"sharedCost"`
	AllocatedCost Decimal `json:"allocatedCost"`
	RecordCount   int64   `json:"recordCount"`
}

// AllocationCoveragePool is one shared segment's pool in one currency, and how it was split.
type AllocationCoveragePool struct {
	SegmentID string `json:"segmentId"`
	// Label is the shared segment's label; empty for the unlabelled one.
	Label string `json:"label"`
	// Method is one of the SegmentSplit constants.
	Method string `json:"method"`
	// Cost is what shared rules put in the pool.
	Cost Decimal `json:"cost"`
	// DistributedCost is what reached segments: Cost, or zero for a proportional pool with no
	// positive direct cost to weigh by, which stays unallocated.
	DistributedCost Decimal `json:"distributedCost"`
	// Residual is what the last share differs from its own exact share rounded to eight places: the
	// rounding the last share absorbs so the pool is conserved.
	Residual    Decimal `json:"residual"`
	RecordCount int64   `json:"recordCount"`
	// Shares are each segment's share, in split order; the last took the residual.
	Shares []AllocationCoverageShare `json:"shares"`
}

// AllocationCoverageShare is one segment's share of a pool.
type AllocationCoverageShare struct {
	SegmentID string  `json:"segmentId"`
	Share     Decimal `json:"share"`
}

// GetAllocationCoverage reads how much of a window the rule set claims.
//
// Dates are UTC calendar days in YYYY-MM-DD form, and both are included: endDate is the window's
// last day. A window covers at most 366 days. costBasis defaults to ActualCost.
func (c *Client) GetAllocationCoverage(
	ctx context.Context, workspaceID, startDate, endDate, costBasis string,
) (*AllocationCoverage, error) {
	if costBasis == "" {
		costBasis = "ActualCost"
	}

	path := fmt.Sprintf(
		"/v1/allocation/coverage?startDate=%s&endDate=%s&costBasis=%s",
		url.QueryEscape(startDate),
		url.QueryEscape(endDate),
		url.QueryEscape(costBasis))

	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, path, workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var coverage AllocationCoverage
	if err := c.do(req, &coverage); err != nil {
		return nil, err
	}
	return &coverage, nil
}
