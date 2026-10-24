package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Segment is what an allocation rule claims cost for: a team, a product and a cost centre.
//
// This model was rewritten rather than renamed. It previously carried name/description/rules and
// never matched the Public API, which means it cannot have worked; the filter lives on ordered
// AllocationRule rows, not on the segment.
type Segment struct {
	ID string `json:"id"`

	// TeamID names an existing team row. Exactly one of TeamID and TeamLabel is set on a direct
	// segment; a shared segment sets neither.
	TeamID *string `json:"teamId,omitempty"`

	// TeamLabel names the owning team before it exists as a row.
	TeamLabel string `json:"teamLabel"`

	// TeamName is the referenced team's name, resolved on read. It is never sent.
	TeamName *string `json:"teamName,omitempty"`

	Product    string `json:"product"`
	CostCentre string `json:"costCentre"`

	// IsShared marks a shared-cost bucket. Its cost is spread across direct segments as Split says,
	// so it names no team, product or cost centre.
	IsShared bool `json:"isShared"`

	// Label tells a workspace's shared segments apart: it holds one per label, compared ignoring
	// case and surrounding spaces, and one without. Empty on a direct segment and on the unlabelled
	// shared one.
	Label string `json:"label"`

	// Split is how a shared segment's pool is spread. Nil on a direct segment, which has no pool.
	Split *SegmentSplit `json:"split,omitempty"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version a create or an update answered, to send back with IfMatch. Empty
	// from a list, which carries no versions, and there is no single read.
	ETag string `json:"-"`
}

func (s *Segment) setETag(etag string) { s.ETag = etag }

// How a shared segment's pool is spread across direct segments. More values may be added.
const (
	// SegmentSplitProportional spreads the pool by each direct segment's positive direct cost and
	// names no targets.
	SegmentSplitProportional = "proportional"
	// SegmentSplitEven gives two or more targets the same share.
	SegmentSplitEven = "even"
	// SegmentSplitPercent gives each target its percentage; the percentages sum to exactly 100.
	SegmentSplitPercent = "percent"
)

// SegmentSplit is how a shared segment's pool is spread. Per currency the shares sum to the pool
// exactly: each but the last is rounded to eight decimal places and the last target takes the
// rest, so the order of Targets matters.
type SegmentSplit struct {
	// Method is one of the SegmentSplit constants. Empty is not sent, which the API reads as
	// SegmentSplitProportional.
	Method string `json:"method,omitempty"`

	// Targets are the distinct direct segments of the shared segment's workspace an even or
	// percent split names, at most 100, in order. A proportional split names none.
	Targets []SegmentSplitTarget `json:"targets,omitempty"`
}

// SegmentSplitTarget is one direct segment a split gives a share to.
type SegmentSplitTarget struct {
	SegmentID string `json:"segmentId"`

	// Percent is a percent split's share for this target, above 0 and at most 100 with at most two
	// decimals. Nil for an even split.
	Percent *float64 `json:"percent,omitempty"`
}

// CreateSegmentInput creates an allocation segment. Set IsShared for a shared-cost bucket, in
// which case the identifying fields are left empty and Label and Split may be set; a direct
// segment takes neither.
type CreateSegmentInput struct {
	WorkspaceID string  `json:"workspaceId"`
	TeamID      *string `json:"teamId,omitempty"`
	TeamLabel   *string `json:"teamLabel,omitempty"`
	Product     string  `json:"product,omitempty"`
	CostCentre  string  `json:"costCentre,omitempty"`
	IsShared    bool    `json:"isShared,omitempty"`

	// Label names a shared segment; empty creates the workspace's unlabelled one.
	Label string `json:"label,omitempty"`
	// Split spreads a shared segment's pool; nil is proportional.
	Split *SegmentSplit `json:"split,omitempty"`
}

// UpdateSegmentInput renames a direct segment, or changes a shared segment's label and split. A
// shared segment never becomes a direct one: it takes no team, product or cost centre, and a direct
// segment takes no label or split.
type UpdateSegmentInput struct {
	// TeamID or TeamLabel replaces the owner; naming neither keeps it.
	TeamID    *string `json:"teamId,omitempty"`
	TeamLabel *string `json:"teamLabel,omitempty"`
	// An empty Product or CostCentre is not sent, and the segment keeps its own.
	Product    string `json:"product,omitempty"`
	CostCentre string `json:"costCentre,omitempty"`

	// Label replaces a shared segment's label, an empty one making it the unlabelled shared
	// segment; nil keeps it.
	Label *string `json:"label,omitempty"`
	// Split replaces a shared segment's split; nil keeps it. Allocation is evaluated at query time,
	// so a new split restates every past month at once.
	Split *SegmentSplit `json:"split,omitempty"`
}

// ListSegments returns a workspace's allocation segments.
func (c *Client) ListSegments(ctx context.Context, workspaceID string) ([]Segment, error) {
	return listWorkspace[Segment](c, ctx, "/v1/segments", workspaceID)
}

// CreateSegment creates an allocation segment.
func (c *Client) CreateSegment(ctx context.Context, input *CreateSegmentInput) (*Segment, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/segments", &body)
	if err != nil {
		return nil, err
	}

	var segment Segment
	if err := c.do(req, &segment); err != nil {
		return nil, err
	}
	return &segment, nil
}

// UpdateSegment renames a direct segment, or changes a shared segment's label and split.
func (c *Client) UpdateSegment(
	ctx context.Context, workspaceID, segmentID string, input *UpdateSegmentInput, opts ...RequestOption,
) (*Segment, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/segments/"+url.PathEscape(segmentID), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var segment Segment
	if err := c.do(req, &segment); err != nil {
		return nil, err
	}
	return &segment, nil
}

// DeleteSegment removes an allocation segment. The API refuses while any rule still assigns cost
// to it, a budget measures it, a shared segment's split names it, or a cost report, cost alert or
// saved filter filters to it.
func (c *Client) DeleteSegment(ctx context.Context, workspaceID, segmentID string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/segments/"+url.PathEscape(segmentID), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
