package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Team is a group of the organization's members that workspace access is granted to. Every
// organization has one system team, Everyone, which cannot be renamed or deleted.
type Team struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	IsSystemTeam bool      `json:"isSystemTeam"`
	MemberCount  int       `json:"memberCount"`
	CreatedAt    time.Time `json:"createdAt"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (t *Team) setETag(etag string) { t.ETag = etag }

// TeamDetail is a team with its members and the workspaces it has an access level on.
type TeamDetail struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	IsSystemTeam    bool                  `json:"isSystemTeam"`
	Members         []TeamMember          `json:"members"`
	WorkspaceAccess []TeamWorkspaceAccess `json:"workspaceAccess"`
	CreatedAt       time.Time             `json:"createdAt"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (t *TeamDetail) setETag(etag string) { t.ETag = etag }

// TeamMember is a member of the organization in a team. Role is viewer or editor.
type TeamMember struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joinedAt"`
}

// TeamWorkspaceAccess is what a team's members may do in one workspace: noAccess, canView or
// canEdit.
type TeamWorkspaceAccess struct {
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
	AccessLevel   string `json:"accessLevel"`
	// DefaultDashboardID is the workspace's dashboard the team's members open after signing in,
	// unless they chose their own; nil for none, or when that dashboard was deleted.
	DefaultDashboardID *string `json:"defaultDashboardId,omitempty"`
}

// ListTeams returns a page of the organization's teams. It needs an organization token.
func (c *Client) ListTeams(ctx context.Context, opts *PageOptions) (*ListResponse[Team], error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/teams", nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp ListResponse[Team]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllTeams fetches every team across all pages.
func (c *Client) ListAllTeams(ctx context.Context, pageSize int) ([]Team, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[Team], error) {
		return c.ListTeams(ctx, p)
	})
}

// GetTeam returns a team with its members and workspace access.
func (c *Client) GetTeam(ctx context.Context, id string) (*TeamDetail, error) {
	req, err := c.newRequest(ctx, http.MethodGet, teamPath(id), nil)
	if err != nil {
		return nil, err
	}

	var team TeamDetail
	if err := c.do(req, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// CreateTeamInput names a new team; names are unique in the organization.
type CreateTeamInput struct {
	Name string `json:"name"`
}

// CreateTeam creates a team with no members and no workspace access.
func (c *Client) CreateTeam(ctx context.Context, input *CreateTeamInput) (*Team, error) {
	req, err := c.newCreateRequest(ctx, "/v1/teams", input)
	if err != nil {
		return nil, err
	}

	var team Team
	if err := c.do(req, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// UpdateTeamInput renames a team.
type UpdateTeamInput struct {
	Name string `json:"name"`
}

// UpdateTeam renames a team. The Everyone team cannot be renamed.
func (c *Client) UpdateTeam(ctx context.Context, id string, input *UpdateTeamInput, opts ...RequestOption) (*Team, error) {
	req, err := c.newRequest(ctx, http.MethodPut, teamPath(id), input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var team Team
	if err := c.do(req, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// DeleteTeam removes a team with its memberships and workspace access.
func (c *Client) DeleteTeam(ctx context.Context, id string, opts ...RequestOption) error {
	req, err := c.newRequest(ctx, http.MethodDelete, teamPath(id), nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

// AddTeamMemberInput names a member of the organization and the role they hold in the team.
type AddTeamMemberInput struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

// AddTeamMember adds a member of the organization to a team.
func (c *Client) AddTeamMember(ctx context.Context, teamID string, input *AddTeamMemberInput) error {
	req, err := c.newRequest(ctx, http.MethodPost, teamPath(teamID)+"/members", input)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// UpdateTeamMemberInput is a team member's new role.
type UpdateTeamMemberInput struct {
	Role string `json:"role"`
}

// UpdateTeamMember changes a team member's role.
func (c *Client) UpdateTeamMember(ctx context.Context, teamID, userID string, input *UpdateTeamMemberInput) error {
	req, err := c.newRequest(ctx, http.MethodPut, teamPath(teamID)+"/members/"+url.PathEscape(userID), input)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// RemoveTeamMember removes a member from a team.
func (c *Client) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, teamPath(teamID)+"/members/"+url.PathEscape(userID), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// ListTeamWorkspaceAccess returns a page of the workspaces a team has an access level on.
func (c *Client) ListTeamWorkspaceAccess(ctx context.Context, teamID string, opts *PageOptions) (*ListResponse[TeamWorkspaceAccess], error) {
	req, err := c.newRequest(ctx, http.MethodGet, teamPath(teamID)+"/workspaces", nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp ListResponse[TeamWorkspaceAccess]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllTeamWorkspaceAccess fetches every workspace access of a team across all pages.
func (c *Client) ListAllTeamWorkspaceAccess(ctx context.Context, teamID string, pageSize int) ([]TeamWorkspaceAccess, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[TeamWorkspaceAccess], error) {
		return c.ListTeamWorkspaceAccess(ctx, teamID, p)
	})
}

// SetTeamWorkspaceAccessInput is the access level a team's members get on a workspace.
type SetTeamWorkspaceAccessInput struct {
	AccessLevel string `json:"accessLevel"`
}

// SetTeamWorkspaceAccess sets a team's access level on a workspace, replacing any it had.
func (c *Client) SetTeamWorkspaceAccess(ctx context.Context, teamID, workspaceID string, input *SetTeamWorkspaceAccessInput) error {
	req, err := c.newRequest(ctx, http.MethodPut, teamPath(teamID)+"/workspaces/"+url.PathEscape(workspaceID), input)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// setTeamDefaultDashboardBody always carries dashboardId, so nil clears the default.
type setTeamDefaultDashboardBody struct {
	DashboardID *string `json:"dashboardId"`
}

// SetTeamDefaultDashboard sets the dashboard of a workspace the team's members open after signing
// in, unless they chose their own, or with a nil dashboardID clears it. The team needs access to
// the workspace other than noAccess to be given one, and the Everyone team never has one. Changing
// the team's access level keeps the default; removing its access ends it.
func (c *Client) SetTeamDefaultDashboard(ctx context.Context, teamID, workspaceID string, dashboardID *string) (*TeamWorkspaceAccess, error) {
	body := setTeamDefaultDashboardBody{DashboardID: dashboardID}
	req, err := c.newRequest(ctx, http.MethodPut, teamPath(teamID)+"/workspaces/"+url.PathEscape(workspaceID)+"/default-dashboard", &body)
	if err != nil {
		return nil, err
	}

	var access TeamWorkspaceAccess
	if err := c.do(req, &access); err != nil {
		return nil, err
	}
	return &access, nil
}

// RemoveTeamWorkspaceAccess removes a team's access level on a workspace.
func (c *Client) RemoveTeamWorkspaceAccess(ctx context.Context, teamID, workspaceID string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, teamPath(teamID)+"/workspaces/"+url.PathEscape(workspaceID), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func teamPath(id string) string { return "/v1/teams/" + url.PathEscape(id) }
