package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// CollectionStatus is one cost source's freshness: how its cost arrives, what it delivered and
// when its next delivery is due.
type CollectionStatus struct {
	ProviderID string         `json:"providerId"`
	Name       string         `json:"name"`
	Key        ProviderKey    `json:"key"`
	Status     ProviderStatus `json:"status"`
	// Cadence is how the cost arrives: hourly, daily, monthToDate, monthlyInvoice or onUpload. A new
	// cadence may appear, so a client should tolerate one it does not list.
	Cadence string `json:"cadence"`
	// Period is what one delivery covers, day or month, and so the unit a missing stretch is counted in.
	Period           string `json:"period"`
	HasProviderTotal bool   `json:"hasProviderTotal"`
	// LastImportedAt is when a collection or upload last committed; nil before the first.
	LastImportedAt *time.Time `json:"lastImportedAt,omitempty"`
	// DataThrough is the latest day the committed data covers, as YYYY-MM-DD; nil before the first commit.
	DataThrough *string `json:"dataThrough,omitempty"`
	// NextDelivery is nil for a source delivered on upload and for a connection that is not collecting.
	NextDelivery *NextDelivery `json:"nextDelivery,omitempty"`
	// DaysOverdue counts days past NextDelivery.DueOn, negative before it; nil when nothing is due.
	DaysOverdue *int `json:"daysOverdue,omitempty"`
	// IsLate is set once the stale-data notice for the source goes out.
	IsLate              bool    `json:"isLate"`
	AwaitingFirstImport bool    `json:"awaitingFirstImport"`
	LastSyncStatus      *string `json:"lastSyncStatus,omitempty"`
}

// NextDelivery is the days the next delivery covers, inclusive, and the day it is due, each as YYYY-MM-DD.
type NextDelivery struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	DueOn     string `json:"dueOn"`
}

// CollectionStatusDetail is one source's status with its months and the stretches no collection covered.
type CollectionStatusDetail struct {
	Summary CollectionStatus `json:"summary"`
	// AmountsWithheldReason says why every amount is nil, for a token that neither manages providers
	// nor reads the whole connection's cost; nil when amounts are shown.
	AmountsWithheldReason *string `json:"amountsWithheldReason,omitempty"`
	// Months runs newest first, from the current month back to the plan's history horizon.
	Months []CollectionMonth `json:"months"`
	// Missing lists, oldest first, the days (or months) no committed collection of a live scope
	// covered. Missing means nothing was collected, not that the provider had no cost.
	Missing []MissingRange `json:"missing"`
}

// CollectionMonth is one month of a source: what arrived, what it cost and the provider's own total.
type CollectionMonth struct {
	// Month is the month's first day, as YYYY-MM-DD.
	Month          string     `json:"month"`
	IsImported     bool       `json:"isImported"`
	RecordCount    int64      `json:"recordCount"`
	LastImportedAt *time.Time `json:"lastImportedAt,omitempty"`
	// OnRequest marks a month older than the first collection reached, collected only by a full sync.
	OnRequest   bool         `json:"onRequest"`
	Restatement *Restatement `json:"restatement,omitempty"`
	// Amounts is the billed cost per currency: empty without rows, nil when withheld.
	Amounts       []MonthAmount `json:"amounts"`
	ProviderTotal ProviderTotal `json:"providerTotal"`
}

// Restatement is the latest re-collection of a month after it closed.
type Restatement struct {
	LastCommittedAt time.Time `json:"lastCommittedAt"`
	Records         int64     `json:"records"`
}

// MonthAmount is a month's billed cost in one currency.
type MonthAmount struct {
	Currency string  `json:"currency"`
	Amount   Decimal `json:"amount"`
}

// ProviderTotal is a month's comparison with the provider's own total. Its amounts are nil where
// there is no comparison, where the sides are not in one currency, and where amounts are withheld.
type ProviderTotal struct {
	// Status is agrees, differs, noData, notCompared or noProviderTotal. A new status may appear.
	Status string `json:"status"`
	// Reason says why there is no comparison; nil when there is one.
	Reason           *string  `json:"reason,omitempty"`
	ProviderAmount   *Decimal `json:"providerAmount,omitempty"`
	CostfluentAmount *Decimal `json:"costfluentAmount,omitempty"`
	// Difference is Costfluent's amount less the provider's.
	Difference *Decimal   `json:"difference,omitempty"`
	Tolerance  *Decimal   `json:"tolerance,omitempty"`
	Currency   *string    `json:"currency,omitempty"`
	ComparedAt *time.Time `json:"comparedAt,omitempty"`
}

// MissingRange is a stretch no committed collection covered, its days inclusive as YYYY-MM-DD.
type MissingRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	// Scopes counts the live collection scopes (accounts, subscriptions, projects) missing every day of it.
	Scopes int `json:"scopes"`
}

// CollectionStatusListResponse is one page of the organization's cost sources.
type CollectionStatusListResponse = ListResponse[CollectionStatus]

// ListCollectionStatus returns one page of every cost source's freshness, by name.
func (c *Client) ListCollectionStatus(ctx context.Context, opts *PageOptions) (*CollectionStatusListResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/providers/collection-status", nil)
	if err != nil {
		return nil, err
	}

	opts.apply(req)

	var resp CollectionStatusListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllCollectionStatus fetches every cost source's freshness across all pages.
func (c *Client) ListAllCollectionStatus(ctx context.Context, pageSize int) ([]CollectionStatus, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[CollectionStatus], error) {
		return c.ListCollectionStatus(ctx, p)
	})
}

// GetProviderCollectionStatus returns one provider's collection status with its months.
func (c *Client) GetProviderCollectionStatus(ctx context.Context, providerID string) (*CollectionStatusDetail, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/providers/"+url.PathEscape(providerID)+"/collection-status", nil)
	if err != nil {
		return nil, err
	}

	var detail CollectionStatusDetail
	if err := c.do(req, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}
