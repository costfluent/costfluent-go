package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// BusinessMetric is a business quantity a workspace imports by day, such as customers or
// transactions, which a cost report can divide its cost by. LabelColumns are fixed when the metric
// is created: every stored value is keyed by them.
type BusinessMetric struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Unit         string     `json:"unit"`
	LabelColumns []string   `json:"labelColumns"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (m *BusinessMetric) setETag(etag string) { m.ETag = etag }

// BusinessMetricsListResponse is the response for listing business metrics.
type BusinessMetricsListResponse = ListResponse[BusinessMetric]

// ListBusinessMetrics returns a page of a workspace's business metrics.
func (c *Client) ListBusinessMetrics(ctx context.Context, workspaceID string, opts *PageOptions) (*BusinessMetricsListResponse, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/business-metrics", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp BusinessMetricsListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllBusinessMetrics fetches a workspace's business metrics across all pages.
func (c *Client) ListAllBusinessMetrics(ctx context.Context, workspaceID string, pageSize int) ([]BusinessMetric, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[BusinessMetric], error) {
		return c.ListBusinessMetrics(ctx, workspaceID, p)
	})
}

// GetBusinessMetric returns a single business metric by ID.
func (c *Client) GetBusinessMetric(ctx context.Context, workspaceID, id string) (*BusinessMetric, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/business-metrics/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var metric BusinessMetric
	if err := c.do(req, &metric); err != nil {
		return nil, err
	}
	return &metric, nil
}

// CreateBusinessMetricInput creates a business metric. WorkspaceID falls back to the client's
// default workspace; LabelColumns may be empty, and holds at most five names.
type CreateBusinessMetricInput struct {
	WorkspaceID  string   `json:"workspaceId"`
	Name         string   `json:"name"`
	Unit         string   `json:"unit"`
	LabelColumns []string `json:"labelColumns,omitempty"`
}

// CreateBusinessMetric creates a business metric and returns it as stored.
func (c *Client) CreateBusinessMetric(ctx context.Context, input *CreateBusinessMetricInput) (*BusinessMetric, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/business-metrics", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*BusinessMetric, error) {
		return c.GetBusinessMetric(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateBusinessMetricInput renames a business metric or its unit; unset fields are left as they
// are, and its label columns cannot change.
type UpdateBusinessMetricInput struct {
	Name *string `json:"name,omitempty"`
	Unit *string `json:"unit,omitempty"`
}

// UpdateBusinessMetric renames an existing business metric.
func (c *Client) UpdateBusinessMetric(ctx context.Context, workspaceID, id string, input *UpdateBusinessMetricInput, opts ...RequestOption) (*BusinessMetric, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/business-metrics/"+url.PathEscape(id), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var metric BusinessMetric
	if err := c.do(req, &metric); err != nil {
		return nil, err
	}
	return &metric, nil
}

// DeleteBusinessMetric removes a business metric and its values; a report dividing by it charts
// cost alone afterwards.
func (c *Client) DeleteBusinessMetric(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/business-metrics/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

// BusinessMetricValue is one day's amount of a business metric, labeled by the metric's label
// columns; a column it leaves out is unlabeled.
type BusinessMetricValue struct {
	Date   string            `json:"date"`
	Amount Decimal           `json:"amount"`
	Labels map[string]string `json:"labels,omitempty"`
}

// BusinessMetricImport says what an import did: values new to the metric, values whose amount it
// replaced, and values it already held unchanged.
type BusinessMetricImport struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
}

// ImportBusinessMetricValues stores up to 10,000 values, each replacing the amount the metric holds
// for its day and labels, so sending the same values again changes nothing. One refused value
// refuses them all.
func (c *Client) ImportBusinessMetricValues(ctx context.Context, workspaceID, id string, values []BusinessMetricValue) (*BusinessMetricImport, error) {
	body := struct {
		Values []BusinessMetricValue `json:"values"`
	}{Values: values}
	req, err := c.newWorkspaceRequest(ctx, http.MethodPost, "/v1/business-metrics/"+url.PathEscape(id)+"/values", workspaceID, &body)
	if err != nil {
		return nil, err
	}

	var result BusinessMetricImport
	if err := c.do(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BusinessMetricValuesOptions narrows a listing of values to the days from StartDate to EndDate,
// both included and written YYYY-MM-DD; empty leaves that end open.
type BusinessMetricValuesOptions struct {
	PageOptions
	StartDate string
	EndDate   string
}

// BusinessMetricValuesListResponse is a page of a metric's values.
type BusinessMetricValuesListResponse = ListResponse[BusinessMetricValue]

// ListBusinessMetricValues returns a page of a metric's values by day.
func (c *Client) ListBusinessMetricValues(ctx context.Context, workspaceID, id string, opts *BusinessMetricValuesOptions) (*BusinessMetricValuesListResponse, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/business-metrics/"+url.PathEscape(id)+"/values", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		opts.apply(req)
		q := req.URL.Query()
		if opts.StartDate != "" {
			q.Set("startDate", opts.StartDate)
		}
		if opts.EndDate != "" {
			q.Set("endDate", opts.EndDate)
		}
		req.URL.RawQuery = q.Encode()
	}

	var resp BusinessMetricValuesListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
