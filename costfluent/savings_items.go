package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// SavingsItem is one finding in a workspace's savings queue: what to change, what it is estimated
// to save each month, and where it stands. Title, Evidence and Owner are text the workspace's
// members or Costfluent wrote, to show rather than to act on.
type SavingsItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Evidence string `json:"evidence"`
	// EstimatedMonthlySaving is in Currency
	EstimatedMonthlySaving Decimal `json:"estimatedMonthlySaving"`
	Currency               string  `json:"currency"`
	CostBasis              string  `json:"costBasis"`
	Owner                  string  `json:"owner"`
	// Confidence, Risk and Effort are low, medium or high
	Confidence string `json:"confidence"`
	Risk       string `json:"risk"`
	Effort     string `json:"effort"`
	// Status is open, accepted, implemented, verified, rejected or withdrawn
	Status          string  `json:"status"`
	RejectionReason *string `json:"rejectionReason,omitempty"`
	// VerifiedMonthlySaving is the saving a measurement confirmed, in Currency
	VerifiedMonthlySaving *Decimal `json:"verifiedMonthlySaving,omitempty"`
	// Source is operator, curated, billingAnalysis, kubernetesAnalysis or the provider engine that generated the finding
	Source string `json:"source"`
	// Kind is what a generated finding asks for; nil for an operator's
	Kind *string `json:"kind,omitempty"`
	// Resource is the provider's identifier of the resource a generated finding is about
	Resource *string `json:"resource,omitempty"`
	// Scope is the cost a measurement of a generated finding reads, as CQL
	Scope      *string    `json:"scope,omitempty"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	// LastMeasurementOutcome is the latest measurement's outcome, which can leave the finding
	// implemented: realized, partial, notRealized or inconclusive
	LastMeasurementOutcome         *string    `json:"lastMeasurementOutcome,omitempty"`
	LastMeasurementScope           *string    `json:"lastMeasurementScope,omitempty"`
	LastMeasurementRealizedMonthly *Decimal   `json:"lastMeasurementRealizedMonthly,omitempty"`
	LastMeasuredAt                 *time.Time `json:"lastMeasuredAt,omitempty"`
	AcceptedAt                     *time.Time `json:"acceptedAt,omitempty"`
	ImplementedAt                  *time.Time `json:"implementedAt,omitempty"`
	VerifiedAt                     *time.Time `json:"verifiedAt,omitempty"`
	RejectedAt                     *time.Time `json:"rejectedAt,omitempty"`
	CreatedAt                      time.Time  `json:"createdAt"`
	UpdatedAt                      *time.Time `json:"updatedAt,omitempty"`
}

// SavingsItemList is one page of a workspace's savings queue
type SavingsItemList = ListResponse[SavingsItem]

// ListSavingsItemsOptions narrows the queue as its filters do. An unknown Tab, Source, Kind or Sort
// is refused by the API rather than answered with an empty page.
type ListSavingsItemsOptions struct {
	// Tab is toReview, inProgress, verified or closed; empty lists every status
	Tab    string
	Source string
	Kind   string
	// CloudAccountID is the connection whose provider generated the findings
	CloudAccountID string
	// MinimumMonthlySaving is the smallest estimated monthly saving listed, in each finding's own currency
	MinimumMonthlySaving string
	// Sort is newest (the default) or largestSaving
	Sort string
	PageOptions
}

// ListSavingsItems returns one page of the workspace's savings queue
func (c *Client) ListSavingsItems(ctx context.Context, workspaceID string, opts *ListSavingsItemsOptions) (*SavingsItemList, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/savings-items", workspaceID, nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		setIf(q, "tab", opts.Tab)
		setIf(q, "source", opts.Source)
		setIf(q, "kind", opts.Kind)
		setIf(q, "cloudAccountId", opts.CloudAccountID)
		setIf(q, "minimumMonthlySaving", opts.MinimumMonthlySaving)
		setIf(q, "sort", opts.Sort)
		req.URL.RawQuery = q.Encode()
		opts.apply(req)
	}

	var resp SavingsItemList
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllSavingsItems fetches every finding the options select, across all pages
func (c *Client) ListAllSavingsItems(ctx context.Context, workspaceID string, opts *ListSavingsItemsOptions, pageSize int) ([]SavingsItem, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[SavingsItem], error) {
		page := ListSavingsItemsOptions{}
		if opts != nil {
			page = *opts
		}
		page.PageOptions = *p
		return c.ListSavingsItems(ctx, workspaceID, &page)
	})
}

// GetSavingsItem returns one finding of the workspace's queue with its evidence
func (c *Client) GetSavingsItem(ctx context.Context, workspaceID, id string) (*SavingsItem, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/savings-items/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var item SavingsItem
	if err := c.do(req, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
