package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Anomaly is a day on which a provider's cost for one service rose unusually, in one workspace
type Anomaly struct {
	ID string `json:"id"`
	// WorkspaceID is the workspace whose cost the anomaly is in; a provider in two workspaces has an anomaly in each
	WorkspaceID string `json:"workspaceId"`
	ProviderID  string `json:"providerId"`
	AnomalyDate string `json:"anomalyDate"`
	ServiceName string `json:"serviceName"`
	// Region is the region with more than half the increase; for new_spend, the new region with more than half the new spend
	Region *string `json:"region,omitempty"`
	// AnomalyType is spike, a day well above its 30-day baseline, or new_spend, a service or a region of one first billed
	// within the seven days up to the anomaly's date after none in the 95 days before it; read any other value as an anomaly
	AnomalyType      string  `json:"anomalyType"`
	Severity         string  `json:"severity"`
	ExpectedCost     Decimal `json:"expectedCost"`
	ActualCost       Decimal `json:"actualCost"`
	DeviationPercent float64 `json:"deviationPercent"`
	Description      *string `json:"description,omitempty"`
	Currency         string  `json:"currency"`
	// Status is open, acknowledged, archived or ignored
	Status string `json:"status"`
	// StatusReason is why detection archived the anomaly by itself: expired or restated
	StatusReason *string `json:"statusReason,omitempty"`
	// IgnoreReason is the reason given for ignoring the anomaly
	IgnoreReason   *string    `json:"ignoreReason,omitempty"`
	IsAcknowledged bool       `json:"isAcknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	DetectedAt     time.Time  `json:"detectedAt"`
	// AttributedResourceID is the resource with more than half the increase
	AttributedResourceID *string `json:"attributedResourceId,omitempty"`
	// Contributors are the resources that added most to the increase, largest first, at most three
	Contributors []AnomalyContributor `json:"contributors"`
	// AttributedSku is the SKU with more than half the increase, named as a cost report's SKU grouping names it
	AttributedSku *string `json:"attributedSku,omitempty"`
	// SkuContributors are the SKUs that added most to the increase, largest first, at most three
	SkuContributors []AnomalySkuContributor `json:"skuContributors"`
}

// AnomalyContributor is one resource's part of an anomaly's increase, in the anomaly's currency
type AnomalyContributor struct {
	// AccountID is the account the resource is billed to; nil when the provider names none
	AccountID  *string `json:"accountId,omitempty"`
	ResourceID string  `json:"resourceId"`
	Increase   Decimal `json:"increase"`
}

// AnomalySkuContributor is one SKU's part of an anomaly's increase, in the anomaly's currency
type AnomalySkuContributor struct {
	Sku      string  `json:"sku"`
	Increase Decimal `json:"increase"`
}

// AnomalyList is one page of the anomalies that match, newest first
type AnomalyList = ListResponse[Anomaly]

// ListAnomaliesOptions narrows an anomaly listing. Dates are calendar days, YYYY-MM-DD.
type ListAnomaliesOptions struct {
	CloudAccountID string
	Severity       string
	// UnacknowledgedOnly lists only open anomalies
	UnacknowledgedOnly bool
	StartDate          string
	EndDate            string
	PageOptions
}

// ListAnomalies returns one page of detected anomalies, newest first
func (c *Client) ListAnomalies(ctx context.Context, opts *ListAnomaliesOptions) (*AnomalyList, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/anomalies", nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		setIf(q, "cloudAccountId", opts.CloudAccountID)
		setIf(q, "severity", opts.Severity)
		if opts.UnacknowledgedOnly {
			q.Set("unacknowledgedOnly", "true")
		}
		setIf(q, "startDate", opts.StartDate)
		setIf(q, "endDate", opts.EndDate)
		req.URL.RawQuery = q.Encode()
		opts.apply(req)
	}

	var resp AnomalyList
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AcknowledgeAnomaly marks an anomaly as acknowledged
func (c *Client) AcknowledgeAnomaly(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/anomalies/"+url.PathEscape(id)+"/acknowledge", nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func setIf(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}
