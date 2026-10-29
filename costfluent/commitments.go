package costfluent

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Commitment is a reservation, savings plan or committed use discount held through one of a
// workspace's connections, with its term, utilisation and how soon it ends
type Commitment struct {
	// ID is the provider's own id, as cost rows carry it
	ID string `json:"id"`
	// ProviderID is the connection the commitment was read through
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	// Provider is the connection's provider key, such as aws
	Provider string `json:"provider"`
	// Type is ReservedInstance, SavingsPlan or CommittedUseDiscount
	Type        string `json:"type"`
	Description string `json:"description"`
	Account     string `json:"account"`
	Region      string `json:"region"`
	Service     string `json:"service"`
	// Status is active, expiring, expired or unknown (no end stated)
	Status string `json:"status"`
	// ProviderStatus is the provider's own word, such as Active or Retired
	ProviderStatus string     `json:"providerStatus"`
	StartsAt       *time.Time `json:"startsAt,omitempty"`
	EndsAt         *time.Time `json:"endsAt,omitempty"`
	// DaysUntilExpiry is whole days to the end; negative once ended
	DaysUntilExpiry *int `json:"daysUntilExpiry,omitempty"`
	// TermYears is 1, 3 or 5
	TermYears *int `json:"termYears,omitempty"`
	// PaymentOption is All Upfront, Partial Upfront or No Upfront
	PaymentOption    *string  `json:"paymentOption,omitempty"`
	HourlyCommitment *Decimal `json:"hourlyCommitment,omitempty"`
	FullTermCost     *Decimal `json:"fullTermCost,omitempty"`
	// UtilizationPercentage is the used share over the utilisation window, 0 to 100
	UtilizationPercentage *float64  `json:"utilizationPercentage,omitempty"`
	UnusedCost            *Decimal  `json:"unusedCost,omitempty"`
	Currency              string    `json:"currency"`
	UtilizationFrom       string    `json:"utilizationFrom"`
	UtilizationTo         string    `json:"utilizationTo"`
	LastReadAt            time.Time `json:"lastReadAt"`
}

// CommitmentList is one page of a workspace's commitments, by end date
type CommitmentList = ListResponse[Commitment]

// ListCommitmentsOptions narrows the commitments as the console's filters do; each matches ignoring
// case, and a Status, TermYears or PaymentOption the list never shows is refused
type ListCommitmentsOptions struct {
	Provider      string
	Status        string
	Type          string
	Service       string
	Account       string
	Region        string
	TermYears     int
	PaymentOption string
	PageOptions
}

// ListCommitments returns one page of the commitments held through the workspace's connections
func (c *Client) ListCommitments(ctx context.Context, workspaceID string, opts *ListCommitmentsOptions) (*CommitmentList, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/commitments", workspaceID, nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		setIf(q, "provider", opts.Provider)
		setIf(q, "status", opts.Status)
		setIf(q, "type", opts.Type)
		setIf(q, "service", opts.Service)
		setIf(q, "account", opts.Account)
		setIf(q, "region", opts.Region)
		if opts.TermYears > 0 {
			q.Set("termYears", strconv.Itoa(opts.TermYears))
		}
		setIf(q, "paymentOption", opts.PaymentOption)
		req.URL.RawQuery = q.Encode()
		opts.apply(req)
	}

	var resp CommitmentList
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllCommitments fetches every commitment the options select, across all pages
func (c *Client) ListAllCommitments(ctx context.Context, workspaceID string, opts *ListCommitmentsOptions, pageSize int) ([]Commitment, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[Commitment], error) {
		page := ListCommitmentsOptions{}
		if opts != nil {
			page = *opts
		}
		page.PageOptions = *p
		return c.ListCommitments(ctx, workspaceID, &page)
	})
}

// CommitmentCoverage is a workspace's commitment coverage, effective savings rate and unused
// commitment over a window, per bin and provider and in total, at amortized cost
type CommitmentCoverage struct {
	// Currency is the presentation currency; nil when the window mixes currencies unconverted
	Currency    *string             `json:"currency,omitempty"`
	Granularity string              `json:"granularity"`
	Data        []CommitmentFigures `json:"data"`
	Totals      []CommitmentFigures `json:"totals"`
	// HistoryFloor is the earliest day the plan let the read reach, when it withheld older data
	HistoryFloor *string `json:"historyFloor,omitempty"`
	// Unconverted is the spend in figures' rows that had no rate into Currency
	Unconverted []CurrencyAmount `json:"unconverted"`
}

// CommitmentFigures is one bin's or the window's commitment figures for one provider
type CommitmentFigures struct {
	// Date is the bin's first day; nil on a window total
	Date                 *string  `json:"date,omitempty"`
	Provider             string   `json:"provider"`
	CoveredOnDemandCost  Decimal  `json:"coveredOnDemandCost"`
	EligibleOnDemandCost Decimal  `json:"eligibleOnDemandCost"`
	ActualCost           Decimal  `json:"actualCost"`
	UsedCommitment       Decimal  `json:"usedCommitment"`
	UnusedCommitment     *Decimal `json:"unusedCommitment,omitempty"`
	CoveragePercent      *float64 `json:"coveragePercent,omitempty"`
	// EffectiveSavingsRatePercent is what the eligible usage saved against on demand
	EffectiveSavingsRatePercent *float64 `json:"effectiveSavingsRatePercent,omitempty"`
	UtilizationPercent          *float64 `json:"utilizationPercent,omitempty"`
}

// CommitmentCoverageOptions is the window coverage reads. Dates are calendar days, YYYY-MM-DD, both
// included. Granularity is day (the default), week, month or quarter. Filter is a cost filter as
// its JSON document or as CQL.
type CommitmentCoverageOptions struct {
	StartDate   string
	EndDate     string
	Granularity string
	Filter      string
}

// GetCommitmentCoverage reads the workspace's commitment coverage over a window
func (c *Client) GetCommitmentCoverage(ctx context.Context, workspaceID string, opts *CommitmentCoverageOptions) (*CommitmentCoverage, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/commitments/coverage", workspaceID, nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		setIf(q, "startDate", opts.StartDate)
		setIf(q, "endDate", opts.EndDate)
		setIf(q, "granularity", opts.Granularity)
		setIf(q, "filter", opts.Filter)
		req.URL.RawQuery = q.Encode()
	}

	var coverage CommitmentCoverage
	if err := c.do(req, &coverage); err != nil {
		return nil, err
	}
	return &coverage, nil
}
