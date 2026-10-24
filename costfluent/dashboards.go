package costfluent

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Dashboard represents a custom dashboard
type Dashboard struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	IsDefault    bool   `json:"isDefault"`
	DateInterval string `json:"dateInterval"`
	DateBin      string `json:"dateBin"`
	// StartDate and EndDate are a custom range's first and last day, YYYY-MM-DD; nil for a preset.
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	// SavedFilterID is the saved filter every report widget is read through, on top of its
	// report's own filter; nil for none.
	SavedFilterID *string   `json:"savedFilterId,omitempty"`
	Widgets       []Widget  `json:"widgets"`
	CreatedAt     time.Time `json:"createdAt"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (d *Dashboard) setETag(etag string) { d.ETag = etag }

// Widget types, spelled the same by a create and a read.
const (
	WidgetTypeCostReport = "costReport"
	WidgetTypeText       = "text"
)

// Widget displays: how a report widget draws its report.
const (
	WidgetDisplayChart = "chart"
	WidgetDisplayTable = "table"
	WidgetDisplayKPI   = "kpi"
)

// Widget represents a dashboard widget: a saved cost report drawn as a chart, a table or a KPI,
// or a text widget's words.
type Widget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// CostReportID is the report a report widget renders; nil on a text widget or once the report
	// is deleted.
	CostReportID *string `json:"costReportId,omitempty"`
	// Display is how a report widget draws its report; nil on a text widget.
	Display *string `json:"display,omitempty"`
	// Content is a text widget's words; nil on a report widget.
	Content  *string        `json:"content,omitempty"`
	Title    string         `json:"title"`
	Position WidgetPosition `json:"position"`
	Config   map[string]any `json:"config,omitempty"`
	// ETag is the stored version this response carried, to send back with IfMatch. Empty from a
	// list, which carries no versions.
	ETag string `json:"-"`
}

func (w *Widget) setETag(etag string) { w.ETag = etag }

// WidgetPosition defines widget placement on the dashboard grid
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// DashboardsListResponse is the response for listing dashboards
type DashboardsListResponse = ListResponse[Dashboard]

// ListDashboards returns paginated dashboards
func (c *Client) ListDashboards(ctx context.Context, workspaceID string, opts *PageOptions) (*DashboardsListResponse, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/dashboards", workspaceID, nil)
	if err != nil {
		return nil, err
	}
	opts.apply(req)

	var resp DashboardsListResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAllDashboards fetches all dashboards across all pages
func (c *Client) ListAllDashboards(ctx context.Context, workspaceID string, pageSize int) ([]Dashboard, error) {
	return listAll(pageSize, func(p *PageOptions) (*ListResponse[Dashboard], error) {
		return c.ListDashboards(ctx, workspaceID, p)
	})
}

// GetDashboard returns a single dashboard by ID
func (c *Client) GetDashboard(ctx context.Context, workspaceID, id string) (*Dashboard, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/dashboards/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var dashboard Dashboard
	if err := c.do(req, &dashboard); err != nil {
		return nil, err
	}
	return &dashboard, nil
}

// CreateDashboardInput for creating a new dashboard. WorkspaceID falls back to the client's
// default workspace.
type CreateDashboardInput struct {
	WorkspaceID  string  `json:"workspaceId"`
	Title        string  `json:"title"`
	IsDefault    bool    `json:"isDefault,omitempty"`
	DateInterval *string `json:"dateInterval,omitempty"`
	DateBin      *string `json:"dateBin,omitempty"`
	// StartDate and EndDate are required with a "custom" DateInterval and ignored otherwise.
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
}

// CreateDashboard creates a new dashboard and returns it as stored
func (c *Client) CreateDashboard(ctx context.Context, input *CreateDashboardInput) (*Dashboard, error) {
	body := *input
	var err error
	if body.WorkspaceID, err = c.requireWorkspace(body.WorkspaceID); err != nil {
		return nil, err
	}
	req, err := c.newCreateRequest(ctx, "/v1/dashboards", &body)
	if err != nil {
		return nil, err
	}

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*Dashboard, error) {
		return c.GetDashboard(ctx, body.WorkspaceID, created.ID)
	})
}

// UpdateDashboardInput for updating a dashboard; unset fields are left as they are
type UpdateDashboardInput struct {
	Title        *string `json:"title,omitempty"`
	DateInterval *string `json:"dateInterval,omitempty"`
	DateBin      *string `json:"dateBin,omitempty"`
	// StartDate and EndDate are required with a "custom" DateInterval and ignored otherwise.
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
}

// UpdateDashboard updates an existing dashboard
func (c *Client) UpdateDashboard(ctx context.Context, workspaceID, id string, input *UpdateDashboardInput, opts ...RequestOption) (*Dashboard, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, "/v1/dashboards/"+url.PathEscape(id), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var dashboard Dashboard
	if err := c.do(req, &dashboard); err != nil {
		return nil, err
	}
	return &dashboard, nil
}

// DeleteDashboard removes a dashboard
func (c *Client) DeleteDashboard(ctx context.Context, workspaceID, id string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, "/v1/dashboards/"+url.PathEscape(id), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}

// setDashboardSavedFilterBody always carries savedFilterId, so nil detaches the filter.
type setDashboardSavedFilterBody struct {
	WorkspaceID   string  `json:"workspaceId"`
	SavedFilterID *string `json:"savedFilterId"`
}

// SetDashboardSavedFilter attaches a saved filter of the same workspace to a dashboard, or with a
// nil savedFilterID detaches the one attached. WorkspaceID falls back to the client's default
// workspace.
func (c *Client) SetDashboardSavedFilter(ctx context.Context, workspaceID, id string, savedFilterID *string, opts ...RequestOption) (*Dashboard, error) {
	ws, err := c.requireWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	body := setDashboardSavedFilterBody{WorkspaceID: ws, SavedFilterID: savedFilterID}
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/dashboards/"+url.PathEscape(id)+"/saved-filter", &body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var dashboard Dashboard
	if err := c.do(req, &dashboard); err != nil {
		return nil, err
	}
	return &dashboard, nil
}

// setDashboardDefaultBody always carries isDefault: the API refuses a body without it.
type setDashboardDefaultBody struct {
	WorkspaceID string `json:"workspaceId"`
	IsDefault   bool   `json:"isDefault"`
}

// SetDashboardDefault makes a dashboard the workspace's only default, or with isDefault false
// clears it when it is the default, leaving the workspace without one. WorkspaceID falls back to
// the client's default workspace.
func (c *Client) SetDashboardDefault(ctx context.Context, workspaceID, id string, isDefault bool, opts ...RequestOption) (*Dashboard, error) {
	ws, err := c.requireWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	body := setDashboardDefaultBody{WorkspaceID: ws, IsDefault: isDefault}
	req, err := c.newRequest(ctx, http.MethodPut, "/v1/dashboards/"+url.PathEscape(id)+"/default", &body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var dashboard Dashboard
	if err := c.do(req, &dashboard); err != nil {
		return nil, err
	}
	return &dashboard, nil
}

func widgetPath(dashboardID, widgetID string) string {
	return "/v1/dashboards/" + url.PathEscape(dashboardID) + "/widgets/" + url.PathEscape(widgetID)
}

// GetDashboardWidget returns one widget of a dashboard
func (c *Client) GetDashboardWidget(ctx context.Context, workspaceID, dashboardID, widgetID string) (*Widget, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, widgetPath(dashboardID, widgetID), workspaceID, nil)
	if err != nil {
		return nil, err
	}

	var widget Widget
	if err := c.do(req, &widget); err != nil {
		return nil, err
	}
	return &widget, nil
}

// CreateDashboardWidgetInput adds a widget to a dashboard. A report widget (Type empty or
// WidgetTypeCostReport) names CostReportID and may set Display, which defaults to chart; a text
// widget (WidgetTypeText) sets Content, at most 10,000 characters, and neither of the others.
type CreateDashboardWidgetInput struct {
	Type         string         `json:"type,omitempty"`
	CostReportID *string        `json:"costReportId,omitempty"`
	Display      *string        `json:"display,omitempty"`
	Content      *string        `json:"content,omitempty"`
	Title        string         `json:"title"`
	Position     WidgetPosition `json:"position"`
}

// CreateDashboardWidget adds a widget to a dashboard and returns it as stored
func (c *Client) CreateDashboardWidget(
	ctx context.Context, workspaceID, dashboardID string, input *CreateDashboardWidgetInput,
) (*Widget, error) {
	ws, err := c.requireWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	req, err := c.newWorkspaceRequest(ctx, http.MethodPost, "/v1/dashboards/"+url.PathEscape(dashboardID)+"/widgets", ws, input)
	if err != nil {
		return nil, err
	}
	req.Header.Set(idempotencyKeyHeader, newIdempotencyKey())

	var created createdRef
	if err := c.do(req, &created); err != nil {
		return nil, err
	}
	return readCreated(created.ID, func() (*Widget, error) {
		return c.GetDashboardWidget(ctx, ws, dashboardID, created.ID)
	})
}

// UpdateDashboardWidgetInput changes a widget; unset fields are left as they are. Display is a
// report widget's and Content a text widget's; the type and the report cannot change, so a
// different report is a new widget.
type UpdateDashboardWidgetInput struct {
	Title    *string         `json:"title,omitempty"`
	Display  *string         `json:"display,omitempty"`
	Content  *string         `json:"content,omitempty"`
	Position *WidgetPosition `json:"position,omitempty"`
}

// UpdateDashboardWidget updates a widget of a dashboard
func (c *Client) UpdateDashboardWidget(
	ctx context.Context, workspaceID, dashboardID, widgetID string, input *UpdateDashboardWidgetInput, opts ...RequestOption,
) (*Widget, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodPut, widgetPath(dashboardID, widgetID), workspaceID, input)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)

	var widget Widget
	if err := c.do(req, &widget); err != nil {
		return nil, err
	}
	return &widget, nil
}

// DeleteDashboardWidget removes a widget from a dashboard
func (c *Client) DeleteDashboardWidget(ctx context.Context, workspaceID, dashboardID, widgetID string, opts ...RequestOption) error {
	req, err := c.newWorkspaceRequest(ctx, http.MethodDelete, widgetPath(dashboardID, widgetID), workspaceID, nil)
	if err != nil {
		return err
	}
	applyOptions(req, opts)
	return c.do(req, nil)
}
