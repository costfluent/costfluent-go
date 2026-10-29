package costfluent

import (
	"context"
	"net/http"
	"slices"
)

// The token types GET /v1/me reports.
const (
	TokenTypeOrganization = "organization"
	TokenTypeWorkspace    = "workspace"
	TokenTypeCluster      = "cluster"
)

// Me is the API token a client authenticates with: its organization, its workspace when it is a
// workspace token, the workspace to act in when none is named, and the capabilities it can use now,
// named as the API names them, such as CapabilityViewCosts ("viewCosts"). An organization
// capability only an Owner holds is left out while the token's creator is no longer an Owner.
//
// DefaultWorkspaceID is a workspace token's own workspace; for an organization token, its creator's
// default while the creator is an active member who can view it, else the organization's oldest
// workspace. It is nil when the organization has no workspace.
type Me struct {
	OrganizationID     string       `json:"organizationId"`
	WorkspaceID        *string      `json:"workspaceId,omitempty"`
	DefaultWorkspaceID *string      `json:"defaultWorkspaceId,omitempty"`
	TokenType          string       `json:"tokenType"`
	TokenName          string       `json:"tokenName"`
	Capabilities       []Capability `json:"capabilities"`
}

// Can reports whether the token carries capability.
func (m *Me) Can(capability Capability) bool {
	return slices.Contains(m.Capabilities, capability)
}

// Missing returns the capabilities of needs the token does not carry, each once and in the order
// asked, or nil when it carries them all: what a tool names before it calls what needs them.
func (m *Me) Missing(needs ...Capability) []Capability {
	var missing []Capability
	for _, need := range needs {
		if !m.Can(need) && !slices.Contains(missing, need) {
			missing = append(missing, need)
		}
	}
	return missing
}

// GetMe describes the token the client authenticates with. It needs no capability, so it is how a
// client learns what it may call before calling it.
func (c *Client) GetMe(ctx context.Context) (*Me, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/me", nil)
	if err != nil {
		return nil, err
	}

	var me Me
	if err := c.do(req, &me); err != nil {
		return nil, err
	}
	return &me, nil
}
