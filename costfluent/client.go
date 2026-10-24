package costfluent

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.costfluent.com"
	DefaultTimeout = 30 * time.Second
)

// DefaultUserAgent is sent unless WithUserAgent replaces it: costfluent-go and the module version
// the build recorded, or costfluent-go alone when it recorded none.
var DefaultUserAgent = userAgent(debug.ReadBuildInfo())

const modulePath = "github.com/costfluent/costfluent-go"

// userAgent finds this module among the build's modules. A program depending on a release lists it
// with that version; a build inside the module itself, or of a copy of this package under another
// module path, carries no version for it.
func userAgent(info *debug.BuildInfo, ok bool) string {
	if ok {
		for _, module := range append([]*debug.Module{&info.Main}, info.Deps...) {
			if module.Path == modulePath && strings.HasPrefix(module.Version, "v") {
				return "costfluent-go/" + strings.TrimPrefix(module.Version, "v")
			}
		}
	}
	return "costfluent-go"
}

// Client is the Costfluent API client
type Client struct {
	httpClient *http.Client
	// callerHTTPClient and timeout hold WithHTTPClient and WithTimeout until NewClient builds
	// httpClient, so the two apply in either order and the caller's client is never changed.
	callerHTTPClient *http.Client
	timeout          *time.Duration
	baseURL          string
	baseURLErr       error
	apiKey           string
	workspaceID      string
	userAgent        string
	debug            bool
}

// NewClient creates a new Costfluent API client
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:   DefaultBaseURL,
		userAgent: DefaultUserAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	// A copy, because the middleware below would otherwise retry and log every request the caller
	// sends through its own client, to any host.
	hc := &http.Client{}
	timeout := DefaultTimeout
	if c.callerHTTPClient != nil {
		copied := *c.callerHTTPClient
		hc = &copied
		timeout = hc.Timeout
	}
	if c.timeout != nil {
		timeout = *c.timeout
	}
	// The timeout bounds each attempt in the retry transport; on the http.Client it would bound
	// the whole retry loop, backoff included.
	hc.Timeout = 0
	c.httpClient = hc

	// Apply transport middleware (order: debug -> retry -> base)
	transport := c.httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	transport = &retryTransport{base: transport, attemptTimeout: timeout}
	if c.debug {
		transport = &debugTransport{base: transport, logger: log.Printf}
	}
	c.httpClient.Transport = transport

	return c
}

// Workspace returns a new client whose default workspace is workspaceID
func (c *Client) Workspace(workspaceID string) *Client {
	clone := *c
	clone.workspaceID = workspaceID
	return &clone
}

// BaseURL returns the base URL of the client
func (c *Client) BaseURL() string {
	return c.baseURL
}

var errNoWorkspace = errors.New("costfluent: this operation is workspace-scoped: pass a workspace ID or set WithWorkspace")

// ResolveWorkspace returns the workspace a workspace-scoped operation given workspaceID targets:
// workspaceID itself, or the client's default when it is empty. Empty when neither is set.
func (c *Client) ResolveWorkspace(workspaceID string) string {
	return c.workspace(workspaceID)
}

// workspace resolves the workspace an operation targets: an explicit ID wins over the default.
func (c *Client) workspace(workspaceID string) string {
	if workspaceID != "" {
		return workspaceID
	}
	return c.workspaceID
}

func (c *Client) requireWorkspace(workspaceID string) (string, error) {
	if id := c.workspace(workspaceID); id != "" {
		return id, nil
	}
	return "", errNoWorkspace
}

// createdRef is the part of a *Created response the SDK reads before fetching the full object.
type createdRef struct {
	ID string `json:"id"`
}

// readCreated reads back the object a create made, and reports a failed read as
// CreatedButUnreadError so the caller keeps the ID of an object that exists.
func readCreated[T any](id string, read func() (*T, error)) (*T, error) {
	obj, err := read()
	if err != nil {
		return nil, &CreatedButUnreadError{ID: id, Err: err}
	}
	return obj, nil
}
