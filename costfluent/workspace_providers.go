package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// WorkspaceProvider is a connection enabled in a workspace. AccountIDs are the cloud provider's
// own account ids the workspace reads (AWS account numbers, Azure subscription ids); empty, the
// workspace reads the whole connection, including accounts discovered later. One connection can
// be enabled in several workspaces, each for its own accounts.
type WorkspaceProvider struct {
	ID           string       `json:"id"`
	WorkspaceID  string       `json:"workspaceId"`
	ProviderID   string       `json:"providerId"`
	ProviderName string       `json:"providerName"`
	ProviderKey  *ProviderKey `json:"providerKey,omitempty"`
	IsEnabled    bool         `json:"isEnabled"`
	AccountIDs   []string     `json:"accountIds"`
	AddedAt      time.Time    `json:"addedAt"`
}

// ListWorkspaceProviders returns every connection enabled in a workspace, across all pages.
func (c *Client) ListWorkspaceProviders(ctx context.Context, workspaceID string) ([]WorkspaceProvider, error) {
	return listWorkspace[WorkspaceProvider](c, ctx, "/v1/workspace-providers", workspaceID)
}

// GetWorkspaceProvider returns one connection enabled in a workspace.
func (c *Client) GetWorkspaceProvider(ctx context.Context, workspaceID, id string) (*WorkspaceProvider, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/workspace-providers/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var link WorkspaceProvider
	if err := c.do(req, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// CreateWorkspaceProviderInput enables a connection in a workspace. Empty AccountIDs reads the
// whole connection; only AWS and Azure connections list accounts, and naming one the connection
// does not cover is refused. WorkspaceID falls back to the client's default workspace.
type CreateWorkspaceProviderInput struct {
	WorkspaceID string   `json:"workspaceId"`
	ProviderID  string   `json:"providerId"`
	AccountIDs  []string `json:"accountIds,omitempty"`
}

// CreateWorkspaceProvider enables a connection in a workspace and returns the link as stored. It
// needs an organization token with AddWorkspaceProvider and ManageProviders.
func (c *Client) CreateWorkspaceProvider(ctx context.Context, input *CreateWorkspaceProviderInput) (*WorkspaceProvider, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/workspace-providers", &body)
	if err != nil {
		return nil, err
	}

	var link WorkspaceProvider
	if err := c.do(req, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// UpdateWorkspaceProviderInput turns a connection on or off in a workspace, or replaces the
// accounts it reads. A nil field is left as it is; an empty, non-nil AccountIDs reads the whole
// connection again. A non-nil AccountIDs needs an organization token with ManageProviders, a
// workspace token only turns the link on or off. WorkspaceID falls back to the client's default
// workspace.
type UpdateWorkspaceProviderInput struct {
	WorkspaceID string   `json:"workspaceId"`
	IsEnabled   *bool    `json:"isEnabled,omitempty"`
	AccountIDs  []string `json:"accountIds,omitzero"`
}

// UpdateWorkspaceProvider updates a connection enabled in a workspace.
func (c *Client) UpdateWorkspaceProvider(ctx context.Context, id string, input *UpdateWorkspaceProviderInput) (*WorkspaceProvider, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/workspace-providers/"+url.PathEscape(id), &body)
	if err != nil {
		return nil, err
	}

	var link WorkspaceProvider
	if err := c.do(req, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// DeleteWorkspaceProvider stops a workspace reading a connection. The connection and its cost
// stay, and other workspaces that enable it are unaffected.
func (c *Client) DeleteWorkspaceProvider(ctx context.Context, workspaceID, id string) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/workspace-providers/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}
