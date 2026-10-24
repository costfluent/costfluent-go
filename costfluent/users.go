package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// User is a member of the organization or a pending invitation. Role is owner,
// integrationOwner, editor or viewer; Status is invited, active, suspended or removed.
type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	FirstName       string     `json:"firstName"`
	LastName        string     `json:"lastName"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	JoinedAt        time.Time  `json:"joinedAt"`
	LastLoginAt     *time.Time `json:"lastLoginAt,omitempty"`
	InviteExpiresAt *time.Time `json:"inviteExpiresAt,omitempty"`
}

// ListUsersOptions narrows a user listing; IncludeRemoved also lists people removed from the
// organization.
type ListUsersOptions struct {
	PageOptions
	IncludeRemoved bool
}

// ListUsers returns a page of the organization's members and pending invitations. It needs an
// organization token.
func (c *Client) ListUsers(ctx context.Context, opts *ListUsersOptions) (*ListResponse[User], error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/users", nil)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		opts.apply(req)
		if opts.IncludeRemoved {
			q := req.URL.Query()
			q.Set("includeRemoved", "true")
			req.URL.RawQuery = q.Encode()
		}
	}

	var resp ListResponse[User]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllUsers fetches the organization's members and pending invitations across all pages,
// without the people removed from it.
func (c *Client) ListAllUsers(ctx context.Context, pageSize int) ([]User, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[User], error) {
		return c.ListUsers(ctx, &ListUsersOptions{PageOptions: *p})
	})
}

// GetUser returns a member of the organization, or a pending invitation.
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/users/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}

	var user User
	if err := c.do(req, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// InviteUserInput invites a person by email. Role is viewer, editor or integrationOwner; owner is
// granted in the product, never by invitation.
type InviteUserInput struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role"`
}

// InviteUser invites a person to the organization and returns the pending invitation.
func (c *Client) InviteUser(ctx context.Context, input *InviteUserInput) (*User, error) {
	req, err := c.newCreateRequest(ctx, "/v1/users", input)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*User, error) {
		return c.GetUser(ctx, created.ID)
	})
}
