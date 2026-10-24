package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// TagPolicyRule requires every cost row it applies to to carry each of RequiredKeys with a value.
// It applies to the rows of Provider, or of every provider when Provider is nil, and of the
// sub-accounts AccountIDs names, or of every account when it is empty. Rules add up: a row carries
// the keys of every rule that applies to it.
type TagPolicyRule struct {
	Provider     *ProviderKey `json:"provider,omitempty"`
	AccountIDs   []string     `json:"accountIds"`
	RequiredKeys []string     `json:"requiredKeys"`
}

// TagPolicy is a workspace's tag policy: its rules in the order written, empty for a workspace
// without one. ReportID names the cost report saving the first rules created, which lists
// non-compliant cost by account; UpdatedAt is when the rules were last saved. Costfluent measures
// the policy and changes nothing in the cloud.
type TagPolicy struct {
	Rules     []TagPolicyRule `json:"rules"`
	ReportID  *string         `json:"reportId,omitempty"`
	UpdatedAt *time.Time      `json:"updatedAt,omitempty"`
}

// UpdateTagPolicyInput replaces a workspace's whole tag policy with Rules, in order; an empty list
// clears it.
type UpdateTagPolicyInput struct {
	Rules []TagPolicyRule `json:"rules"`
}

// GetTagPolicy returns the workspace's tag policy. The workspace is part of the path, so it is
// named explicitly rather than taken from the client's default.
func (c *Client) GetTagPolicy(ctx context.Context, workspaceID string) (*TagPolicy, error) {
	req, err := c.newRequest(ctx, http.MethodGet, tagPolicyPath(workspaceID), nil)
	if err != nil {
		return nil, err
	}

	var policy TagPolicy
	if err := c.do(req, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// UpdateTagPolicy replaces the workspace's tag policy and returns it as stored. Saving the first
// rules also creates the compliance report the result's ReportID names.
func (c *Client) UpdateTagPolicy(ctx context.Context, workspaceID string, input *UpdateTagPolicyInput) (*TagPolicy, error) {
	// The API requires the list, and reads a null one as a refusal rather than as "clear".
	body := UpdateTagPolicyInput{Rules: make([]TagPolicyRule, 0, len(input.Rules))}
	for _, rule := range input.Rules {
		if rule.AccountIDs == nil {
			rule.AccountIDs = []string{}
		}
		body.Rules = append(body.Rules, rule)
	}
	req, err := c.newRequest(ctx, http.MethodPut, tagPolicyPath(workspaceID), &body)
	if err != nil {
		return nil, err
	}

	var policy TagPolicy
	if err := c.do(req, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

func tagPolicyPath(workspaceID string) string {
	return "/v1/workspaces/" + url.PathEscape(workspaceID) + "/tag-policy"
}
