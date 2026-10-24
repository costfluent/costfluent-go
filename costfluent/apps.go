package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// App is a notification channel budgets, cost alerts and report notifications deliver to. Key is
// email, slack, teams or webhook. Config comes back redacted: a Slack or Teams webhook URL and a
// webhook's signing secret read "***".
type App struct {
	ID        string         `json:"id"`
	Key       string         `json:"key"`
	Name      string         `json:"name"`
	Config    map[string]any `json:"config"`
	IsEnabled bool           `json:"isEnabled"`
	CreatedAt time.Time      `json:"createdAt"`
	// SigningSecret is the secret Costfluent generated for a webhook created without one. Only
	// CreateApp's answer carries it, once; keep it to check the X-Costfluent-Signature of deliveries.
	SigningSecret string `json:"signingSecret,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
	// Replayed is true when CreateApp's answer is the API's replay of an earlier attempt of the same
	// create, which a retry receives when the first answer was lost. A replay never carries
	// SigningSecret, so a webhook created without a secret then has one nobody can read.
	Replayed bool `json:"-"`
}

func (a *App) setETag(etag string)  { a.ETag = etag }
func (a *App) setReplayed(yes bool) { a.Replayed = yes }

// ListApps returns a page of the organization's notification channels. It needs an organization
// token.
func (c *Client) ListApps(ctx context.Context, opts *PageOptions) (*ListResponse[App], error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/apps", nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp ListResponse[App]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllApps fetches every notification channel across all pages.
func (c *Client) ListAllApps(ctx context.Context, pageSize int) ([]App, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[App], error) {
		return c.ListApps(ctx, p)
	})
}

// GetApp returns a notification channel with its credentials redacted.
func (c *Client) GetApp(ctx context.Context, id string) (*App, error) {
	req, err := c.newRequest(ctx, http.MethodGet, appPath(id), nil)
	if err != nil {
		return nil, err
	}

	var app App
	if err := c.do(req, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// CreateAppInput creates a channel. Config holds recipients for email (one address, or several
// separated by commas), webhookUrl for Slack and Teams, and url with an optional signing secret
// for a webhook, which must be public HTTPS, and its optional events: the state changes it is sent
// (savings_item_status, provider_health, spend_band, subscription_status, anomaly_status).
type CreateAppInput struct {
	Key    string         `json:"key"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

// CreateApp creates a notification channel and returns it as stored, redacted. A webhook created
// without a secret in its config is given one, which this answer alone carries as SigningSecret,
// unless the answer is a replay (Replayed).
func (c *Client) CreateApp(ctx context.Context, input *CreateAppInput) (*App, error) {
	req, err := c.newCreateRequest(ctx, "/v1/apps", input)
	if err != nil {
		return nil, err
	}

	var app App
	if err := c.do(req, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// UpdateAppInput renames a channel or replaces its whole configuration; nil leaves either as it
// is. The kind cannot change.
type UpdateAppInput struct {
	Name   *string        `json:"name,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

// UpdateApp changes a notification channel and returns it, redacted.
func (c *Client) UpdateApp(ctx context.Context, id string, input *UpdateAppInput, opts ...RequestOption) (*App, error) {
	req, err := c.newRequest(ctx, http.MethodPut, appPath(id), input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var app App
	if err := c.do(req, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// DeleteApp removes a notification channel, taking it out of every cost alert, budget, workspace's
// anomaly notifications and report subscription that used it.
func (c *Client) DeleteApp(ctx context.Context, id string, opts ...RequestOption) error {
	req, err := c.newRequest(ctx, http.MethodDelete, appPath(id), nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

func appPath(id string) string { return "/v1/apps/" + url.PathEscape(id) }
