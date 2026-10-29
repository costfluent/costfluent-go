package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Workspace represents a Costfluent workspace.
//
// The currency settings follow one rule: the conversion selection overrides the display
// preference. Currency is the display preference, the currency figures default to where no
// billing currency applies; with EnableCurrencyConversion set, every figure is converted into
// ConversionCurrency by ConversionMethod instead.
//
// EnableAutomaticSyncing is the workspace's switch over the recurring collection Costfluent runs
// against this workspace's data sources. Off, the schedule is skipped for every source no other
// workspace still syncs; stored cost data and an explicitly requested sync are unaffected.
//
// BlendCommitmentDiscounts, when on, spreads each day's AWS Savings Plan and Reserved Instance
// discount evenly across the usage it could cover in every amortized figure of the workspace,
// rather than leave it on the usage AWS applied it to. Billed figures, connection totals and
// commitment coverage do not change. Only an Owner can change it.
//
// TimeZone is the IANA zone the workspace's calendar runs in: its "today", its budget and alert
// periods and the default for its report notifications. Cost rows keep the provider's UTC day.
type Workspace struct {
	ID                       string         `json:"id"`
	Name                     string         `json:"name"`
	Currency                 string         `json:"currency"`
	EnableCurrencyConversion bool           `json:"enableCurrencyConversion"`
	ConversionCurrency       *string        `json:"conversionCurrency,omitempty"`
	ConversionMethod         string         `json:"conversionMethod,omitempty"`
	EnableAutomaticSyncing   bool           `json:"enableAutomaticSyncing"`
	BlendCommitmentDiscounts bool           `json:"blendCommitmentDiscounts"`
	TimeZone                 string         `json:"timeZone"`
	ReportDefaults           ReportDefaults `json:"reportDefaults"`
	// AnomaliesExcludeMarketplace and AnomaliesExcludeCommitmentFees leave those charges out of
	// anomaly detection; AnomalyNotificationThreshold is the smallest increase over its baseline,
	// in the workspace currency, an anomaly must reach to be notified (0 notifies every one).
	AnomaliesExcludeMarketplace    bool       `json:"anomaliesExcludeMarketplace"`
	AnomaliesExcludeCommitmentFees bool       `json:"anomaliesExcludeCommitmentFees"`
	AnomalyNotificationThreshold   Decimal    `json:"anomalyNotificationThreshold"`
	ProviderCount                  int        `json:"providerCount"`
	CreatedAt                      time.Time  `json:"createdAt"`
	UpdatedAt                      *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (w *Workspace) setETag(etag string) { w.ETag = etag }

// ReportDefaults is what a new cost report in the workspace starts from for every part its request
// leaves out. DateInterval is a relative preset, never "custom".
type ReportDefaults struct {
	DateInterval        string `json:"dateInterval"`
	DateBin             string `json:"dateBin"`
	IncludeCredits      bool   `json:"includeCredits"`
	IncludeRefunds      bool   `json:"includeRefunds"`
	IncludeTax          bool   `json:"includeTax"`
	Amortize            bool   `json:"amortize"`
	ShowForecast        bool   `json:"showForecast"`
	CompareToLastPeriod bool   `json:"compareToLastPeriod"`
}

// ReportDefaultsInput sets the parts it names; each left nil keeps the workspace's current value.
type ReportDefaultsInput struct {
	DateInterval        *string `json:"dateInterval,omitempty"`
	DateBin             *string `json:"dateBin,omitempty"`
	IncludeCredits      *bool   `json:"includeCredits,omitempty"`
	IncludeRefunds      *bool   `json:"includeRefunds,omitempty"`
	IncludeTax          *bool   `json:"includeTax,omitempty"`
	Amortize            *bool   `json:"amortize,omitempty"`
	ShowForecast        *bool   `json:"showForecast,omitempty"`
	CompareToLastPeriod *bool   `json:"compareToLastPeriod,omitempty"`
}

// WorkspacesListResponse is the response for listing workspaces
type WorkspacesListResponse = ListResponse[Workspace]

// ListWorkspaces returns paginated workspaces
func (c *Client) ListWorkspaces(ctx context.Context, opts *PageOptions) (*WorkspacesListResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/workspaces", nil)
	if err != nil {
		return nil, err
	}

	opts.apply(req)

	var resp WorkspacesListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllWorkspaces fetches all workspaces across all pages
func (c *Client) ListAllWorkspaces(ctx context.Context, pageSize int) ([]Workspace, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[Workspace], error) {
		return c.ListWorkspaces(ctx, p)
	})
}

// GetWorkspace returns a single workspace by ID
func (c *Client) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/workspaces/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}

	var workspace Workspace
	if err := c.do(req, &workspace); err != nil {
		return nil, err
	}
	return &workspace, nil
}

// CreateWorkspaceInput for creating a new workspace. An empty TimeZone leaves the API's UTC.
type CreateWorkspaceInput struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
	TimeZone string `json:"timeZone,omitempty"`
}

// CreateWorkspace creates a new workspace and returns it as stored
func (c *Client) CreateWorkspace(ctx context.Context, input *CreateWorkspaceInput) (*Workspace, error) {
	req, err := c.newCreateRequest(ctx, "/v1/workspaces", input)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*Workspace, error) {
		return c.GetWorkspace(ctx, created.ID)
	})
}

// UpdateWorkspaceInput fields are applied only when set. The display preference can change only
// while conversion is disabled, and enabling conversion needs a currency to convert into.
type UpdateWorkspaceInput struct {
	Name                           *string              `json:"name,omitempty"`
	Currency                       *string              `json:"currency,omitempty"`
	EnableCurrencyConversion       *bool                `json:"enableCurrencyConversion,omitempty"`
	ConversionCurrency             *string              `json:"conversionCurrency,omitempty"`
	ConversionMethod               *string              `json:"conversionMethod,omitempty"`
	EnableAutomaticSyncing         *bool                `json:"enableAutomaticSyncing,omitempty"`
	BlendCommitmentDiscounts       *bool                `json:"blendCommitmentDiscounts,omitempty"`
	TimeZone                       *string              `json:"timeZone,omitempty"`
	ReportDefaults                 *ReportDefaultsInput `json:"reportDefaults,omitempty"`
	AnomaliesExcludeMarketplace    *bool                `json:"anomaliesExcludeMarketplace,omitempty"`
	AnomaliesExcludeCommitmentFees *bool                `json:"anomaliesExcludeCommitmentFees,omitempty"`
	AnomalyNotificationThreshold   *Decimal             `json:"anomalyNotificationThreshold,omitempty"`
}

// UpdateWorkspace updates an existing workspace
func (c *Client) UpdateWorkspace(ctx context.Context, id string, input *UpdateWorkspaceInput, opts ...RequestOption) (*Workspace, error) {
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/workspaces/"+url.PathEscape(id), input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var workspace Workspace
	if err := c.do(req, &workspace); err != nil {
		return nil, err
	}
	return &workspace, nil
}

// DeleteWorkspace removes a workspace
func (c *Client) DeleteWorkspace(ctx context.Context, id string, opts ...RequestOption) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/v1/workspaces/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
