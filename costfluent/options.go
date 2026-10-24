package costfluent

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"
)

// Option configures the Client
type Option func(*Client)

// WithAPIKey sets the API key for authentication
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithWorkspace sets the default workspace ID for workspace-scoped endpoints
func WithWorkspace(id string) Option {
	return func(c *Client) {
		c.workspaceID = id
	}
}

// WithBaseURL sets a custom base URL (useful for testing or on-prem). It must use https, since the
// API key travels with every request; plain http is accepted only for localhost, 127.0.0.1 and ::1.
// Any other base URL makes every request fail with the reason, before a key is sent.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
		c.baseURLErr = checkBaseURL(baseURL)
	}
}

// checkBaseURL is the rule the Terraform provider and the MCP server apply to the same setting.
func checkBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("costfluent: base URL %q is not a URL: %w", raw, err)
	}
	if u.Host == "" {
		return fmt.Errorf("costfluent: base URL %q has no host; write it as https://<host>", raw)
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && slices.Contains([]string{"localhost", "127.0.0.1", "::1"}, u.Hostname()):
		return nil
	}
	return fmt.Errorf("costfluent: base URL %q must use https, because the API key travels with every request; plain http is allowed only for localhost, 127.0.0.1 and ::1", raw)
}

// WithHTTPClient bases the client on a copy of hc: its transport, timeout, redirect policy and
// cookie jar. hc itself is left unchanged.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.callerHTTPClient = hc
	}
}

// WithUserAgent sets a custom User-Agent header
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// WithDebug enables request/response debug logging
func WithDebug(debug bool) Option {
	return func(c *Client) {
		c.debug = debug
	}
}

// WithTimeout bounds each attempt of a request, overriding the timeout a WithHTTPClient client carries
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = &d
	}
}
