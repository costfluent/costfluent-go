package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// VirtualTagRule assigns Value to every cost row its Filter matches. Filter is the cost filter JSON
// document, the same form a cost report and a cost data query take.
type VirtualTagRule struct {
	Value  string `json:"value"`
	Filter string `json:"filter"`
}

// VirtualTag computes a tag value per cost row at query time: the first collapsed provider tag key
// the row carries, then the first rule whose filter matches, then the default.
type VirtualTag struct {
	ID               string           `json:"id"`
	Key              string           `json:"key"`
	Description      *string          `json:"description,omitempty"`
	DefaultValue     *string          `json:"defaultValue,omitempty"`
	CollapsedTagKeys []string         `json:"collapsedTagKeys"`
	Rules            []VirtualTagRule `json:"rules"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        *time.Time       `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (v *VirtualTag) setETag(etag string) { v.ETag = etag }

// VirtualTagsListResponse is the response for listing virtual tags
type VirtualTagsListResponse = ListResponse[VirtualTag]

// ListVirtualTags returns paginated virtual tags
func (c *Client) ListVirtualTags(ctx context.Context, workspaceID string, opts *PageOptions) (*VirtualTagsListResponse, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/virtual-tags", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp VirtualTagsListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllVirtualTags fetches all virtual tags across all pages
func (c *Client) ListAllVirtualTags(ctx context.Context, workspaceID string, pageSize int) ([]VirtualTag, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[VirtualTag], error) {
		return c.ListVirtualTags(ctx, workspaceID, p)
	})
}

// GetVirtualTag returns a single virtual tag by ID
func (c *Client) GetVirtualTag(ctx context.Context, workspaceID, id string) (*VirtualTag, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/virtual-tags/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var tag VirtualTag
	if err := c.do(req, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

// CreateVirtualTagInput for creating a new virtual tag. It needs at least one rule or one collapsed
// tag key. WorkspaceID falls back to the client's default workspace.
type CreateVirtualTagInput struct {
	WorkspaceID      string           `json:"workspaceId"`
	Key              string           `json:"key"`
	Description      *string          `json:"description,omitempty"`
	DefaultValue     *string          `json:"defaultValue,omitempty"`
	CollapsedTagKeys []string         `json:"collapsedTagKeys"`
	Rules            []VirtualTagRule `json:"rules"`
}

// CreateVirtualTag creates a new virtual tag and returns it as stored
func (c *Client) CreateVirtualTag(ctx context.Context, input *CreateVirtualTagInput) (*VirtualTag, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	if body.CollapsedTagKeys == nil {
		body.CollapsedTagKeys = []string{}
	}
	if body.Rules == nil {
		body.Rules = []VirtualTagRule{}
	}
	req, err := c.newCreateRequest(ctx, "/v1/virtual-tags", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*VirtualTag, error) {
		return c.GetVirtualTag(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateVirtualTagInput for updating a virtual tag. A nil field is left as it is; an empty
// description or default clears it, and a non-nil list replaces the whole list. WorkspaceID falls
// back to the client's default workspace.
type UpdateVirtualTagInput struct {
	WorkspaceID       string            `json:"workspaceId"`
	Key               *string           `json:"key,omitempty"`
	Description       *string           `json:"description,omitempty"`
	ClearDescription  bool              `json:"clearDescription,omitempty"`
	DefaultValue      *string           `json:"defaultValue,omitempty"`
	ClearDefaultValue bool              `json:"clearDefaultValue,omitempty"`
	CollapsedTagKeys  *[]string         `json:"collapsedTagKeys,omitempty"`
	Rules             *[]VirtualTagRule `json:"rules,omitempty"`
}

// UpdateVirtualTag updates an existing virtual tag and returns it as stored
func (c *Client) UpdateVirtualTag(ctx context.Context, id string, input *UpdateVirtualTagInput, opts ...RequestOption) (*VirtualTag, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPatch, "/v1/virtual-tags/"+url.PathEscape(id), &body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var tag VirtualTag
	if err := c.do(req, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

// DeleteVirtualTag removes a virtual tag
func (c *Client) DeleteVirtualTag(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/virtual-tags/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
