package costfluent

import (
	"context"
	"net/http"
	"time"
)

// PlanLadderEntry is one rung of the published plan ladder.
//
// A nil limit means unlimited. The whole ladder travels with an entitlement so a caller can see
// what the next plan lifts without a second request — the question "why can I not do this" is never
// asked without "what would fix it".
type PlanLadderEntry struct {
	Code               string   `json:"code"`
	Name               string   `json:"name"`
	PriceAmount        *Decimal `json:"priceAmount"` // nil on Custom, which is quoted rather than priced
	Currency           string   `json:"currency"`
	SpendCeiling       *Decimal `json:"spendCeiling"`
	HistoryDays        int      `json:"historyDays"`
	MaxConnections     *int     `json:"maxConnections"`
	MaxAllocationRules *int     `json:"maxAllocationRules"`
	Features           []string `json:"features"`
}

// Entitlement is what one account is allowed, after its plan and any exceptions are folded together.
//
// A nil limit means unlimited. OverriddenLimits names the limits this account deviates from its
// plan on, so a caller can tell a raised cap from a published one.
type Entitlement struct {
	PlanCode string `json:"planCode"`
	PlanName string `json:"planName"`
	Status   string `json:"status"`
	Currency string `json:"currency"`

	SpendCeiling       *Decimal `json:"spendCeiling"`
	HistoryDays        *int     `json:"historyDays"`
	MaxConnections     *int     `json:"maxConnections"`
	MaxAllocationRules *int     `json:"maxAllocationRules"`
	MaxBackfillMonths  *int     `json:"maxBackfillMonths"`
	MaxRowsPerMonth    *int64   `json:"maxRowsPerMonth"`
	MaxBytesStored     *int64   `json:"maxBytesStored"`

	Features         []string `json:"features"`
	OverriddenLimits []string `json:"overriddenLimits"`

	IngestionHalted       bool   `json:"ingestionHalted"`
	IngestionHaltedReason string `json:"ingestionHaltedReason"`

	// BandPlanCode is the tier three months of measured spend puts this account in. Recorded, never
	// applied: a band is evidence for a conversation, not a charge.
	BandPlanCode *string `json:"bandPlanCode"`

	CurrentPeriodStart string `json:"currentPeriodStart"`
	CurrentPeriodEnd   string `json:"currentPeriodEnd"`

	Ladder []PlanLadderEntry `json:"ladder"`
}

// UsageHistoryEntry is one completed month, as it feeds the band average.
type UsageHistoryEntry struct {
	PeriodStart  string  `json:"periodStart"`
	TrackedSpend Decimal `json:"trackedSpend"`
	RowsIngested int64   `json:"rowsIngested"`
	BytesStored  int64   `json:"bytesStored"`
}

// Usage is what one account has used, measured from the same data the caps are enforced against.
//
// IsSpendComplete is false when some currency had no usable exchange rate; UnconvertedCurrencies
// names them. A short total is never presented as a whole one.
type Usage struct {
	PeriodStart string `json:"periodStart"`
	Currency    string `json:"currency"`

	TrackedSpend          Decimal  `json:"trackedSpend"`
	IsSpendComplete       bool     `json:"isSpendComplete"`
	UnconvertedCurrencies []string `json:"unconvertedCurrencies"`

	RowsIngested    int64 `json:"rowsIngested"`
	BytesStored     int64 `json:"bytesStored"`
	ConnectionCount int   `json:"connectionCount"`
	ActiveUserCount int   `json:"activeUserCount"`

	ComputedAt *time.Time `json:"computedAt"`

	AverageTrackedSpend *Decimal `json:"averageTrackedSpend"`
	BandPlanCode        *string  `json:"bandPlanCode"`

	History []UsageHistoryEntry `json:"history"`
}

// GetEntitlement returns what the token's account is allowed, including the published ladder.
func (c *Client) GetEntitlement(ctx context.Context) (*Entitlement, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/entitlement", nil)
	if err != nil {
		return nil, err
	}

	var entitlement Entitlement
	if err := c.do(req, &entitlement); err != nil {
		return nil, err
	}
	return &entitlement, nil
}

// GetUsage returns the account's metered usage for the current month.
//
// These are the numbers the caps are enforced against, so a script can check its headroom rather
// than discover a limit by being refused halfway through a run.
func (c *Client) GetUsage(ctx context.Context) (*Usage, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/usage", nil)
	if err != nil {
		return nil, err
	}

	var usage Usage
	if err := c.do(req, &usage); err != nil {
		return nil, err
	}
	return &usage, nil
}
