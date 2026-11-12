package costfluent

import (
	"context"
	"net/http"
)

// CostEstimateChange is what a planned change does to one resource, spelled as the Public API
// sends it. A new change may appear, so a client should tolerate one it does not list.
// TestCostEstimateChangesMatchTheContract holds this list to the API's declared values.
type CostEstimateChange string

const (
	CostEstimateChangeCreate  CostEstimateChange = "create"
	CostEstimateChangeUpdate  CostEstimateChange = "update"
	CostEstimateChangeReplace CostEstimateChange = "replace"
	CostEstimateChangeDelete  CostEstimateChange = "delete"
)

// CostEstimateChanges returns every change the Public API declares, in the contract's order.
func CostEstimateChanges() []CostEstimateChange {
	return []CostEstimateChange{
		CostEstimateChangeCreate,
		CostEstimateChangeUpdate,
		CostEstimateChangeReplace,
		CostEstimateChangeDelete,
	}
}

// CostEstimateState is where an estimate stands, spelled as the Public API sends it. A new state
// may appear, so a client should tolerate one it does not list.
// TestCostEstimateStatesMatchTheContract holds this list to the API's declared values.
type CostEstimateState string

const (
	CostEstimateStateProposed     CostEstimateState = "proposed"
	CostEstimateStateApplied      CostEstimateState = "applied"
	CostEstimateStateReconciled   CostEstimateState = "reconciled"
	CostEstimateStateInconclusive CostEstimateState = "inconclusive"
	CostEstimateStateExpired      CostEstimateState = "expired"
)

// CostEstimateStates returns every state the Public API declares, in the contract's order.
func CostEstimateStates() []CostEstimateState {
	return []CostEstimateState{
		CostEstimateStateProposed,
		CostEstimateStateApplied,
		CostEstimateStateReconciled,
		CostEstimateStateInconclusive,
		CostEstimateStateExpired,
	}
}

// CostEstimateTag is the tag key and value an estimate's resources will carry, which the
// reconciliation reads their cost by.
type CostEstimateTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CostEstimateResource is one resource of a planned change, by its address in the plan. A nil
// monthly cost is one the estimator could not price, never zero. ResourceIDs are the one to three
// ids an existing resource already has; a created resource takes none.
type CostEstimateResource struct {
	Address           string             `json:"address"`
	Type              string             `json:"type"`
	Change            CostEstimateChange `json:"change"`
	MonthlyCostBefore *Decimal           `json:"monthlyCostBefore,omitempty"`
	MonthlyCostAfter  *Decimal           `json:"monthlyCostAfter,omitempty"`
	ResourceIDs       []string           `json:"resourceIds,omitempty"`
}

// SubmitCostEstimateInput is a planned change's monthly cost estimate. It names its reference,
// Repository with PullRequest or Plan, exactly one of the two. WorkspaceID falls back to the
// client's default workspace. Currency is an ISO 4217 code; Resources holds 1 to 2,000.
type SubmitCostEstimateInput struct {
	WorkspaceID   string `json:"workspaceId"`
	Repository    string `json:"repository,omitempty"`
	PullRequest   int    `json:"pullRequest,omitempty"`
	Plan          string `json:"plan,omitempty"`
	Source        string `json:"source"`
	SourceVersion string `json:"sourceVersion,omitempty"`
	Commit        string `json:"commit,omitempty"`
	// URL is an https link to the change, such as the pull request
	URL       string                 `json:"url,omitempty"`
	Currency  string                 `json:"currency"`
	Tag       *CostEstimateTag       `json:"tag,omitempty"`
	Resources []CostEstimateResource `json:"resources"`
}

// AppliedResource is a resource the apply created or replaced, by its address in the plan, and
// the one to three ids it got.
type AppliedResource struct {
	Address     string   `json:"address"`
	ResourceIDs []string `json:"resourceIds"`
}

// ApplyCostEstimateInput names the estimate an apply was for by the reference it was posted under,
// Repository with PullRequest or Plan, exactly one of the two. WorkspaceID falls back to the
// client's default workspace.
type ApplyCostEstimateInput struct {
	WorkspaceID string            `json:"workspaceId"`
	Repository  string            `json:"repository,omitempty"`
	PullRequest int               `json:"pullRequest,omitempty"`
	Plan        string            `json:"plan,omitempty"`
	Resources   []AppliedResource `json:"resources,omitempty"`
}

// CostEstimatePlanned is an estimate's planned monthly totals as the API summed them, in the
// estimate's currency. UnpricedResources counts the resources the estimator could not price,
// which the totals leave out.
type CostEstimatePlanned struct {
	MonthlyCostBefore Decimal `json:"monthlyCostBefore"`
	MonthlyCostAfter  Decimal `json:"monthlyCostAfter"`
	MonthlyChange     Decimal `json:"monthlyChange"`
	UnpricedResources int     `json:"unpricedResources"`
}

// CostEstimateReceipt is what posting or applying an estimate answers: the estimate, its state
// and its planned totals, never a measured cost, so a token holding submitCostEstimates alone can
// post.
type CostEstimateReceipt struct {
	ID    string            `json:"id"`
	State CostEstimateState `json:"state"`
	// AppliedOn is the apply date, YYYY-MM-DD, once the change was applied
	AppliedOn *string `json:"appliedOn,omitempty"`
	// Planned is nil from an API older than the field
	Planned *CostEstimatePlanned `json:"planned,omitempty"`
	// Created is true when SubmitCostEstimate created the estimate, false when it replaced the one
	// posted earlier for the same reference; always false from ApplyCostEstimate.
	Created bool `json:"-"`
	// Replayed is true when SubmitCostEstimate's answer is the API's replay of an earlier attempt
	// of the same post, which a retry receives when the first answer was lost.
	Replayed bool `json:"-"`
}

func (r *CostEstimateReceipt) setReplayed(yes bool) { r.Replayed = yes }

func (r *CostEstimateReceipt) setStatus(code int) { r.Created = code == http.StatusCreated }

// SubmitCostEstimate posts a planned change's estimate, as CI does on every push: the first post
// for a reference creates the estimate, a later one replaces it whole until the change is applied,
// after which the API answers a conflict (IsConflict). A retry of the same call stores nothing
// more: it carries an Idempotency-Key. It needs a token with submitCostEstimates.
func (c *Client) SubmitCostEstimate(ctx context.Context, input *SubmitCostEstimateInput) (*CostEstimateReceipt, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/cost-estimates", &body)
	if err != nil {
		return nil, err
	}
	return c.doCostEstimateReceipt(req)
}

// ApplyCostEstimate records that the change an estimate was posted for was applied, today in the
// workspace's time zone, with the ids of the resources it created or replaced. The same apply
// again answers the estimate unchanged; one with other ids, or of an expired estimate, is a
// conflict (IsConflict). It needs a token with submitCostEstimates.
func (c *Client) ApplyCostEstimate(ctx context.Context, input *ApplyCostEstimateInput) (*CostEstimateReceipt, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/cost-estimates/apply", &body)
	if err != nil {
		return nil, err
	}
	return c.doCostEstimateReceipt(req)
}

func (c *Client) doCostEstimateReceipt(req *http.Request) (*CostEstimateReceipt, error) {
	var receipt CostEstimateReceipt
	if err := c.do(req, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}
