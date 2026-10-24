package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// CostReportSubscription delivers a saved cost report on a schedule to email, Slack or Teams
// apps. ScheduleType is daily, weekly or monthly; DeliveryHour is the hour, 0 to 23, on the wall
// clock of TimeZone.
type CostReportSubscription struct {
	ID              string     `json:"id"`
	CostReportID    string     `json:"costReportId"`
	Name            string     `json:"name"`
	ScheduleType    string     `json:"scheduleType"`
	DeliveryHour    int        `json:"deliveryHour"`
	TimeZone        string     `json:"timeZone"`
	AppIDs          []string   `json:"appIds"`
	IsActive        bool       `json:"isActive"`
	LastSentAt      *time.Time `json:"lastSentAt,omitempty"`
	NextScheduledAt *time.Time `json:"nextScheduledAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (s *CostReportSubscription) setETag(etag string) { s.ETag = etag }

// ListCostReportSubscriptions returns a page of a workspace's report notifications, of one report
// when costReportID is set. An empty workspaceID uses the client's default workspace.
func (c *Client) ListCostReportSubscriptions(
	ctx context.Context, workspaceID, costReportID string, opts *PageOptions,
) (*ListResponse[CostReportSubscription], error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/cost-report-subscriptions", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	if costReportID != "" {
		q := req.URL.Query()
		q.Set("costReportId", costReportID)
		req.URL.RawQuery = q.Encode()
	}
	opts.apply(req)

	var resp ListResponse[CostReportSubscription]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCostReportSubscription returns one report notification of a workspace.
func (c *Client) GetCostReportSubscription(ctx context.Context, workspaceID, id string) (*CostReportSubscription, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, subscriptionPath(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var subscription CostReportSubscription
	if err := c.do(req, &subscription); err != nil {
		return nil, err
	}
	return &subscription, nil
}

// CreateCostReportSubscriptionInput schedules a report to email, Slack or Teams apps.
// DeliveryHour defaults to 9 and TimeZone to the workspace's zone; WorkspaceID falls back to the
// client's default workspace.
type CreateCostReportSubscriptionInput struct {
	WorkspaceID  string   `json:"workspaceId"`
	CostReportID string   `json:"costReportId"`
	Name         string   `json:"name"`
	ScheduleType string   `json:"scheduleType"`
	DeliveryHour *int     `json:"deliveryHour,omitempty"`
	TimeZone     string   `json:"timeZone,omitempty"`
	AppIDs       []string `json:"appIds"`
}

// CreateCostReportSubscription creates a report notification and returns it as stored.
func (c *Client) CreateCostReportSubscription(
	ctx context.Context, input *CreateCostReportSubscriptionInput,
) (*CostReportSubscription, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/cost-report-subscriptions", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*CostReportSubscription, error) {
		return c.GetCostReportSubscription(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateCostReportSubscriptionInput changes the fields it sets; nil leaves each as it is, and
// AppIDs replaces the destinations. WorkspaceID falls back to the client's default workspace.
type UpdateCostReportSubscriptionInput struct {
	WorkspaceID  string    `json:"workspaceId"`
	Name         *string   `json:"name,omitempty"`
	ScheduleType *string   `json:"scheduleType,omitempty"`
	DeliveryHour *int      `json:"deliveryHour,omitempty"`
	TimeZone     *string   `json:"timeZone,omitempty"`
	AppIDs       *[]string `json:"appIds,omitempty"`
}

// UpdateCostReportSubscription changes a report notification and returns it as stored.
func (c *Client) UpdateCostReportSubscription(
	ctx context.Context, id string, input *UpdateCostReportSubscriptionInput, opts ...RequestOption,
) (*CostReportSubscription, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPatch, subscriptionPath(id), &body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var subscription CostReportSubscription
	if err := c.do(req, &subscription); err != nil {
		return nil, err
	}
	return &subscription, nil
}

// DeleteCostReportSubscription removes a report notification.
func (c *Client) DeleteCostReportSubscription(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	return c.subscriptionAction(ctx, http.MethodDelete, workspaceID, id, "", opts...)
}

// ActivateCostReportSubscription resumes a paused report notification. The answer carries no
// subscription, and the version changes: read it again for the ETag a later write sends.
func (c *Client) ActivateCostReportSubscription(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	return c.subscriptionAction(ctx, http.MethodPost, workspaceID, id, "/activate", opts...)
}

// DeactivateCostReportSubscription pauses a report notification without deleting it. The answer
// carries no subscription, and the version changes: read it again for the ETag a later write sends.
func (c *Client) DeactivateCostReportSubscription(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	return c.subscriptionAction(ctx, http.MethodPost, workspaceID, id, "/deactivate", opts...)
}

func (c *Client) subscriptionAction(ctx context.Context, method, workspaceID, id, suffix string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, method, subscriptionPath(id)+suffix, workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

func subscriptionPath(id string) string { return "/v1/cost-report-subscriptions/" + url.PathEscape(id) }
