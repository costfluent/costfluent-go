package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Annotation is a dated marker on the charts of a workspace's cost reports: a member's note, or what
// a budget or cost alert left as it fired
type Annotation struct {
	ID string `json:"id"`
	// Date is the day on the workspace's calendar the marker stands on, YYYY-MM-DD
	Date string `json:"date"`
	// Source is manual, budgetAlert or costAlert
	Source string `json:"source"`
	// Text is a note's text; nil on a budget's or cost alert's annotation
	Text *string `json:"text,omitempty"`
	// CostReportIDs are the cost reports it stands on; deleted reports are left out
	CostReportIDs []string `json:"costReportIds"`
	// BudgetID is the budget that fired; it may since have been deleted
	BudgetID *string `json:"budgetId,omitempty"`
	// CostAlertID is the cost alert that fired; it may since have been deleted
	CostAlertID *string `json:"costAlertId,omitempty"`
	// SourceName is the budget's or cost alert's name when it fired
	SourceName *string `json:"sourceName,omitempty"`
	// ThresholdPercent is the percent of the budget the threshold that fired stands at
	ThresholdPercent *int `json:"thresholdPercent,omitempty"`
	// ThresholdBasis is actual or forecast: what the budget's threshold that fired was held against
	ThresholdBasis  *string `json:"thresholdBasis,omitempty"`
	CreatedByUserID *string `json:"createdByUserId,omitempty"`
	// ResolvedAt is when it was resolved; nil while it is open
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	ResolvedByUserID *string    `json:"resolvedByUserId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (a *Annotation) setETag(etag string) { a.ETag = etag }

// AnnotationList is one page of a workspace's annotations, by date
type AnnotationList = ListResponse[Annotation]

// ListAnnotationsOptions narrows an annotation listing. Dates are calendar days, YYYY-MM-DD, both
// inclusive.
type ListAnnotationsOptions struct {
	// CostReportID lists only the annotations standing on this cost report
	CostReportID string
	StartDate    string
	EndDate      string
	// IncludeResolved lists resolved annotations too; they are left out by default
	IncludeResolved bool
	PageOptions
}

// CreateAnnotationInput is a new note on one to ten of the workspace's cost reports. WorkspaceID
// falls back to the client's default workspace.
type CreateAnnotationInput struct {
	WorkspaceID string `json:"workspaceId"`
	// Date is the day the note stands on, YYYY-MM-DD
	Date string `json:"date"`
	// Text is at most 1,000 characters
	Text          string   `json:"text"`
	CostReportIDs []string `json:"costReportIds"`
}

// UpdateAnnotationInput replaces a note whole: a cost report it leaves out is no longer annotated
type UpdateAnnotationInput struct {
	Date          string   `json:"date"`
	Text          string   `json:"text"`
	CostReportIDs []string `json:"costReportIds"`
}

// ListAnnotations returns one page of the workspace's annotations by date, open ones unless
// IncludeResolved is set
func (c *Client) ListAnnotations(ctx context.Context, workspaceID string, opts *ListAnnotationsOptions) (*AnnotationList, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/annotations", workspaceID, nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		setIf(q, "costReportId", opts.CostReportID)
		setIf(q, "startDate", opts.StartDate)
		setIf(q, "endDate", opts.EndDate)
		if opts.IncludeResolved {
			q.Set("includeResolved", "true")
		}
		req.URL.RawQuery = q.Encode()
		opts.apply(req)
	}

	var resp AnnotationList
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllAnnotations fetches every annotation that matches, across all pages
func (c *Client) ListAllAnnotations(ctx context.Context, workspaceID string, opts *ListAnnotationsOptions, pageSize int) ([]Annotation, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[Annotation], error) {
		page := ListAnnotationsOptions{}
		if opts != nil {
			page = *opts
		}
		page.PageOptions = *p
		return c.ListAnnotations(ctx, workspaceID, &page)
	})
}

// GetAnnotation returns one annotation with its ETag. One whose every cost report is deleted is not
// found.
func (c *Client) GetAnnotation(ctx context.Context, workspaceID, id string) (*Annotation, error) {
	return c.annotationAction(ctx, http.MethodGet, workspaceID, id, "", nil)
}

// CreateAnnotation pins a dated note to one to ten of the workspace's cost reports and returns it as
// stored. A retry of the same call creates nothing more: it carries an Idempotency-Key.
func (c *Client) CreateAnnotation(ctx context.Context, input *CreateAnnotationInput) (*Annotation, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/annotations", &body)
	if err != nil {
		return nil, err
	}

	var annotation Annotation
	if err := c.do(req, &annotation); err != nil {
		return nil, err
	}
	return &annotation, nil
}

// UpdateAnnotation replaces a note whole. A budget's or cost alert's annotation records what fired
// and is not edited: the API answers it with a conflict (IsConflict), and it can only be resolved or
// deleted.
func (c *Client) UpdateAnnotation(ctx context.Context, workspaceID, id string, input *UpdateAnnotationInput, opts ...RequestOption) (*Annotation, error) {
	return c.annotationAction(ctx, http.MethodPut, workspaceID, id, "", input, opts...)
}

// ResolveAnnotation hides an annotation from the default list and returns it with its new ETag. One
// already resolved is returned as it is.
func (c *Client) ResolveAnnotation(ctx context.Context, workspaceID, id string, opts ...RequestOption) (*Annotation, error) {
	return c.annotationAction(ctx, http.MethodPost, workspaceID, id, "/resolve", nil, opts...)
}

// ReopenAnnotation shows a resolved annotation in the default list again and returns it with its new
// ETag. One that is open is returned as it is.
func (c *Client) ReopenAnnotation(ctx context.Context, workspaceID, id string, opts ...RequestOption) (*Annotation, error) {
	return c.annotationAction(ctx, http.MethodPost, workspaceID, id, "/reopen", nil, opts...)
}

// DeleteAnnotation deletes an annotation, a budget's or cost alert's included; the cost reports it
// stood on stay
func (c *Client) DeleteAnnotation(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, annotationPath(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

// annotationAction sends a request on one annotation that the API answers with the annotation.
func (c *Client) annotationAction(
	ctx context.Context, method, workspaceID, id, suffix string, body any, opts ...RequestOption,
) (*Annotation, error) {
	req, err := c.newWorkspaceRequest(ctx, method, annotationPath(id)+suffix, workspaceID, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var annotation Annotation
	if err := c.do(req, &annotation); err != nil {
		return nil, err
	}
	return &annotation, nil
}

func annotationPath(id string) string { return "/v1/annotations/" + url.PathEscape(id) }
