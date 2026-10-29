package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// SavingsView is a saved view of a workspace's savings queue: its title and the filters it
// reopens, named as ListSavingsItemsOptions names them
type SavingsView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Tab is toReview, inProgress, verified or closed
	Tab    string  `json:"tab"`
	Source *string `json:"source,omitempty"`
	Kind   *string `json:"kind,omitempty"`
	// CloudAccountID is the connection whose findings the view lists, a removed one's included
	CloudAccountID       *string    `json:"cloudAccountId,omitempty"`
	MinimumMonthlySaving *Decimal   `json:"minimumMonthlySaving,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (v *SavingsView) setETag(etag string) { v.ETag = etag }

// SavingsViewList is one page of a workspace's saved views, by title
type SavingsViewList = ListResponse[SavingsView]

// CreateSavingsViewInput is a new view: its title and the filters it reopens. WorkspaceID falls back
// to the client's default workspace.
type CreateSavingsViewInput struct {
	WorkspaceID string `json:"workspaceId"`
	Title       string `json:"title"`
	// Tab is toReview, inProgress, verified or closed
	Tab                  string   `json:"tab"`
	Source               *string  `json:"source,omitempty"`
	Kind                 *string  `json:"kind,omitempty"`
	CloudAccountID       *string  `json:"cloudAccountId,omitempty"`
	MinimumMonthlySaving *Decimal `json:"minimumMonthlySaving,omitempty"`
}

// UpdateSavingsViewInput replaces a view whole: every filter left unset is cleared
type UpdateSavingsViewInput struct {
	Title                string   `json:"title"`
	Tab                  string   `json:"tab"`
	Source               *string  `json:"source,omitempty"`
	Kind                 *string  `json:"kind,omitempty"`
	CloudAccountID       *string  `json:"cloudAccountId,omitempty"`
	MinimumMonthlySaving *Decimal `json:"minimumMonthlySaving,omitempty"`
}

// ListSavingsViews returns one page of the workspace's saved views
func (c *Client) ListSavingsViews(ctx context.Context, workspaceID string, opts *PageOptions) (*SavingsViewList, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/savings-views", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp SavingsViewList
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllSavingsViews fetches every saved view of the workspace, across all pages
func (c *Client) ListAllSavingsViews(ctx context.Context, workspaceID string, pageSize int) ([]SavingsView, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[SavingsView], error) {
		return c.ListSavingsViews(ctx, workspaceID, p)
	})
}

// GetSavingsView returns one saved view with its ETag
func (c *Client) GetSavingsView(ctx context.Context, workspaceID, id string) (*SavingsView, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/savings-views/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var view SavingsView
	if err := c.do(req, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

// CreateSavingsView saves a set of queue filters as a named view and returns it as stored. A retry
// of the same call creates nothing more: it carries an Idempotency-Key.
func (c *Client) CreateSavingsView(ctx context.Context, input *CreateSavingsViewInput) (*SavingsView, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/savings-views", &body)
	if err != nil {
		return nil, err
	}

	var view SavingsView
	if err := c.do(req, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

// UpdateSavingsView replaces a saved view whole; a filter the input leaves unset is cleared
func (c *Client) UpdateSavingsView(ctx context.Context, workspaceID, id string, input *UpdateSavingsViewInput, opts ...RequestOption) (*SavingsView, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/savings-views/"+url.PathEscape(id), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var view SavingsView
	if err := c.do(req, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

// DeleteSavingsView deletes a saved view; the findings it listed stay
func (c *Client) DeleteSavingsView(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/savings-views/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
