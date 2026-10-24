package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// SavedFilter is a named cost filter expression
type SavedFilter struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Filter    string    `json:"filter"`
	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (f *SavedFilter) setETag(etag string) { f.ETag = etag }

// SavedFiltersListResponse is the response for listing saved filters
type SavedFiltersListResponse = ListResponse[SavedFilter]

// ListSavedFilters returns paginated saved filters
func (c *Client) ListSavedFilters(ctx context.Context, workspaceID string, opts *PageOptions) (*SavedFiltersListResponse, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/saved-filters", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp SavedFiltersListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllSavedFilters fetches all saved filters across all pages
func (c *Client) ListAllSavedFilters(ctx context.Context, workspaceID string, pageSize int) ([]SavedFilter, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[SavedFilter], error) {
		return c.ListSavedFilters(ctx, workspaceID, p)
	})
}

// GetSavedFilter returns a single saved filter by ID
func (c *Client) GetSavedFilter(ctx context.Context, workspaceID, id string) (*SavedFilter, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/saved-filters/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var filter SavedFilter
	if err := c.do(req, &filter); err != nil {
		return nil, err
	}
	return &filter, nil
}

// CreateSavedFilterInput for creating a new saved filter. WorkspaceID falls back to the client's
// default workspace.
type CreateSavedFilterInput struct {
	WorkspaceID string `json:"workspaceId"`
	Title       string `json:"title"`
	Filter      string `json:"filter"`
	IsDefault   bool   `json:"isDefault,omitempty"`
}

// CreateSavedFilter creates a new saved filter and returns it as stored
func (c *Client) CreateSavedFilter(ctx context.Context, input *CreateSavedFilterInput) (*SavedFilter, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/saved-filters", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*SavedFilter, error) {
		return c.GetSavedFilter(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateSavedFilterInput for updating a saved filter; unset fields are left as they are
type UpdateSavedFilterInput struct {
	Title     *string `json:"title,omitempty"`
	Filter    *string `json:"filter,omitempty"`
	IsDefault *bool   `json:"isDefault,omitempty"`
}

// UpdateSavedFilter updates an existing saved filter
func (c *Client) UpdateSavedFilter(ctx context.Context, workspaceID, id string, input *UpdateSavedFilterInput, opts ...RequestOption) (*SavedFilter, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/saved-filters/"+url.PathEscape(id), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var filter SavedFilter
	if err := c.do(req, &filter); err != nil {
		return nil, err
	}
	return &filter, nil
}

// DeleteSavedFilter removes a saved filter
func (c *Client) DeleteSavedFilter(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/saved-filters/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
