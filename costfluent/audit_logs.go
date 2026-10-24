package costfluent

import (
	"context"
	"net/http"
	"time"
)

// AuditLogEntry is one event of the organization's audit log: what a member, an API token or the
// system changed. ActorID and TargetUserID are user IDs; Metadata is the event's own JSON value as
// encoding/json decodes it, for an object change the object before and after.
type AuditLogEntry struct {
	ID           string  `json:"id"`
	EventName    string  `json:"eventName"`
	ObjectType   *string `json:"objectType,omitempty"`
	ActorID      *string `json:"actorId,omitempty"`
	ActorName    *string `json:"actorName,omitempty"`
	TargetUserID *string `json:"targetUserId,omitempty"`
	ManagedBy    *string `json:"managedBy,omitempty"`
	// ManagedOrganization is set on a partner's own log when one of its staff acted in an
	// organization it manages: that organization's name, with the person as the actor.
	ManagedOrganization *string   `json:"managedOrganization,omitempty"`
	IPAddress           *string   `json:"ipAddress,omitempty"`
	UserAgent           *string   `json:"userAgent,omitempty"`
	Metadata            any       `json:"metadata,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

// ListAuditLogsOptions narrows an audit log listing. An empty field or a zero time sends no filter.
// From and To bound the event's time as a half-open interval of instants: an event at From is
// listed, and one at exactly To is not.
type ListAuditLogsOptions struct {
	PageOptions
	EventName    string
	ObjectType   string
	ActorID      string
	TargetUserID string
	From         time.Time
	To           time.Time
}

func (o *ListAuditLogsOptions) apply(req *http.Request) {
	o.PageOptions.apply(req)
	q := req.URL.Query()
	for key, value := range map[string]string{
		"eventName": o.EventName, "objectType": o.ObjectType, "actorId": o.ActorID, "targetUserId": o.TargetUserID,
	} {
		if value != "" {
			q.Set(key, value)
		}
	}
	if !o.From.IsZero() {
		q.Set("from", o.From.UTC().Format(time.RFC3339))
	}
	if !o.To.IsZero() {
		q.Set("to", o.To.UTC().Format(time.RFC3339))
	}
	req.URL.RawQuery = q.Encode()
}

// ListAuditLogs returns a page of the organization's audit log, newest first. It needs an
// organization token with CapabilityViewAuditLog.
func (c *Client) ListAuditLogs(ctx context.Context, opts *ListAuditLogsOptions) (*ListResponse[AuditLogEntry], error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/audit-logs", nil)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		opts.apply(req)
	}

	var resp ListResponse[AuditLogEntry]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllAuditLogs fetches every page of the audit log the filters of opts select; its page
// options are ignored.
func (c *Client) ListAllAuditLogs(ctx context.Context, opts *ListAuditLogsOptions, pageSize int) ([]AuditLogEntry, error) {
	filters := ListAuditLogsOptions{}
	if opts != nil {
		filters = *opts
	}
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[AuditLogEntry], error) {
		filters.PageOptions = *p
		return c.ListAuditLogs(ctx, &filters)
	})
}
