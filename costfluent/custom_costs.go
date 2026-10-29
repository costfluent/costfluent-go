package costfluent

import (
	"context"
	"net/url"
)

// CostEntry is one cost a custom connection reports, in FOCUS terms. Dates are YYYY-MM-DD; a cost
// whose ChargePeriodEnd is after its ChargePeriodStart is spread over its days, one daily record
// each. Amounts are decimal strings with at most eight decimals.
type CostEntry struct {
	ChargePeriodStart string  `json:"chargePeriodStart"`
	ChargeCategory    string  `json:"chargeCategory"`
	ServiceName       string  `json:"serviceName"`
	BilledCost        Decimal `json:"billedCost"`

	ChargePeriodEnd        *string           `json:"chargePeriodEnd,omitempty"`
	LineItemID             *string           `json:"lineItemId,omitempty"`
	BillingCurrency        *string           `json:"billingCurrency,omitempty"`
	EffectiveCost          *Decimal          `json:"effectiveCost,omitempty"`
	ListCost               *Decimal          `json:"listCost,omitempty"`
	ConsumedQuantity       *float64          `json:"consumedQuantity,omitempty"`
	ConsumedUnit           *string           `json:"consumedUnit,omitempty"`
	ServiceCategory        *string           `json:"serviceCategory,omitempty"`
	ServiceSubcategory     *string           `json:"serviceSubcategory,omitempty"`
	ChargeDescription      *string           `json:"chargeDescription,omitempty"`
	ResourceID             *string           `json:"resourceId,omitempty"`
	ResourceName           *string           `json:"resourceName,omitempty"`
	ResourceType           *string           `json:"resourceType,omitempty"`
	RegionID               *string           `json:"regionId,omitempty"`
	RegionName             *string           `json:"regionName,omitempty"`
	SubAccountID           *string           `json:"subAccountId,omitempty"`
	SubAccountName         *string           `json:"subAccountName,omitempty"`
	BillingAccountID       *string           `json:"billingAccountId,omitempty"`
	SkuID                  *string           `json:"skuId,omitempty"`
	CommitmentDiscountID   *string           `json:"commitmentDiscountId,omitempty"`
	CommitmentDiscountType *string           `json:"commitmentDiscountType,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty"`
}

// SubmitCostsInput is a batch of costs for one custom connection. ReplaceLines replaces the earlier
// days of each line the batch names, which needs manageProviders as well as syncProviders.
type SubmitCostsInput struct {
	Costs        []CostEntry `json:"costs"`
	ReplaceLines bool        `json:"replaceLines,omitempty"`
}

// CostValidationError is why the API refused one entry of a batch; RowIndex counts from 0.
type CostValidationError struct {
	RowIndex int    `json:"rowIndex"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

// SubmittedCosts is what a batch committed: the rows the API accepted, and why it refused each of
// the others.
type SubmittedCosts struct {
	BatchID      string                `json:"batchId"`
	TotalRows    int                   `json:"totalRows"`
	AcceptedRows int                   `json:"acceptedRows"`
	RejectedRows int                   `json:"rejectedRows"`
	Errors       []CostValidationError `json:"errors,omitempty"`
	// Replayed is true when the answer is the API's replay of an earlier attempt of the same batch,
	// which a retry receives when the first answer was lost: the rows were committed once.
	Replayed bool `json:"-"`
}

func (s *SubmittedCosts) setReplayed(yes bool) { s.Replayed = yes }

// SubmitCosts writes a batch of costs to the custom connection providerID. Rows the API refuses
// are counted and explained in the result, not returned as an error. A batch another submission or
// upload for the connection overtook commits nothing and returns a conflict (IsConflict); send it
// again. It needs an organization token with syncProviders.
func (c *Client) SubmitCosts(ctx context.Context, providerID string, input *SubmitCostsInput) (*SubmittedCosts, error) {
	req, err := c.newCreateRequest(ctx, "/v1/custom-providers/"+url.PathEscape(providerID)+"/costs", input)
	if err != nil {
		return nil, err
	}

	var submitted SubmittedCosts
	if err := c.do(req, &submitted); err != nil {
		return nil, err
	}
	return &submitted, nil
}
