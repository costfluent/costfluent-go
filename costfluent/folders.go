package costfluent

import (
	"context"
	"net/http"
	"net/url"
)

// Folder organizes a workspace's cost reports. Folders nest; Children holds the folders directly
// inside this one.
type Folder struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	ParentID    *string  `json:"parentId,omitempty"`
	ReportCount int      `json:"reportCount"`
	Children    []Folder `json:"children,omitempty"`
	// SavedFilterID is the saved filter every report in this folder and its subfolders is read
	// through, on top of the report's own filter; nil for none.
	SavedFilterID *string `json:"savedFilterId,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (f *Folder) setETag(etag string) { f.ETag = etag }

// ListFolders returns a workspace's top-level folders, each with its subfolders.
func (c *Client) ListFolders(ctx context.Context, workspaceID string) ([]Folder, error) {
	return listWorkspace[Folder](c, ctx, "/v1/folders", workspaceID)
}

// GetFolder returns a single folder by ID
func (c *Client) GetFolder(ctx context.Context, workspaceID, id string) (*Folder, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/folders/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var folder Folder
	if err := c.do(req, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// CreateFolderInput for creating a new folder. WorkspaceID falls back to the client's default
// workspace.
type CreateFolderInput struct {
	WorkspaceID string  `json:"workspaceId"`
	Title       string  `json:"title"`
	ParentID    *string `json:"parentId,omitempty"`
}

// CreateFolder creates a new folder and returns it as stored
func (c *Client) CreateFolder(ctx context.Context, input *CreateFolderInput) (*Folder, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/folders", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*Folder, error) {
		return c.GetFolder(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateFolderInput for renaming or moving a folder; unset fields are left as they are.
// WorkspaceID falls back to the client's default workspace.
type UpdateFolderInput struct {
	WorkspaceID string  `json:"workspaceId"`
	Title       *string `json:"title,omitempty"`
	ParentID    *string `json:"parentId,omitempty"`
	// ClearParent moves the folder to the top of the tree; a nil ParentID keeps its parent.
	ClearParent bool `json:"clearParent,omitempty"`
}

// UpdateFolder updates an existing folder
func (c *Client) UpdateFolder(ctx context.Context, id string, input *UpdateFolderInput, opts ...RequestOption) (*Folder, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/folders/"+url.PathEscape(id), &body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var folder Folder
	if err := c.do(req, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// setFolderSavedFilterBody always carries savedFilterId, so nil detaches the filter.
type setFolderSavedFilterBody struct {
	WorkspaceID   string  `json:"workspaceId"`
	SavedFilterID *string `json:"savedFilterId"`
}

// SetFolderSavedFilter attaches a saved filter of the same workspace to a folder, or with a nil
// savedFilterID detaches the one attached. WorkspaceID falls back to the client's default workspace.
func (c *Client) SetFolderSavedFilter(ctx context.Context, workspaceID, id string, savedFilterID *string, opts ...RequestOption) (*Folder, error) {
	ws, err := c.requireWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	body := setFolderSavedFilterBody{WorkspaceID: ws, SavedFilterID: savedFilterID}
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/folders/"+url.PathEscape(id)+"/saved-filter", &body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var folder Folder
	if err := c.do(req, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// DeleteFolder removes a folder
func (c *Client) DeleteFolder(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/folders/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
