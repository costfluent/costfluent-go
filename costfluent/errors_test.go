package costfluent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProblemDetailsDecode(t *testing.T) {
	c, _ := testClient(t, stub{400, `{
		"type": "https://www.rfc-editor.org/rfc/rfc7231#section-6.5.1",
		"title": "One or more validation errors occurred.",
		"status": 400,
		"instance": "/v1/workspaces",
		"traceId": "0HN:00000001",
		"errors": [
			{"name": "currency", "reason": "'Currency' must be 3 characters in length."},
			{"name": "name", "reason": "'Name' must not be empty."}
		]
	}`})

	_, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an APIError", err)
	}
	if !IsBadRequest(err) || StatusCode(err) != 400 {
		t.Errorf("status helpers disagree with %d", apiErr.StatusCode)
	}
	if apiErr.TraceID != "0HN:00000001" || len(apiErr.Errors) != 2 || apiErr.Errors[0].Name != "currency" {
		t.Errorf("decoded %+v", apiErr)
	}
	msg := err.Error()
	if !strings.Contains(msg, "'Currency' must be 3 characters") || !strings.Contains(msg, "traceId=0HN:00000001") {
		t.Errorf("message %q lost the validation errors or trace", msg)
	}
}

// The API's problems as each source writes them: the command validator's generic detail beside
// the fields it refused, the endpoint validator's fields with no detail, and the refusals whose
// detail already states their one entry.
func TestMessageAddsTheFieldErrorsTheDetailLeavesOut(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"command validator": {
			`{"title":"Validation Failed","status":400,"code":"validation_failed",
			"detail":"One or more validation errors occurred",
			"errors":[{"name":"comparisonPeriod","reason":"'comparison Period' must not be empty."},
			{"name":"filter","reason":"Unknown field 'cost.provider' at position 1"}]}`,
			"One or more validation errors occurred; comparisonPeriod: 'comparison Period' must not be empty.; " +
				"filter: Unknown field 'cost.provider' at position 1",
		},
		"endpoint validator": {
			`{"title":"Validation Failed","status":400,"code":"validation_failed",
			"errors":[{"name":"currency","reason":"'Currency' must be 3 characters in length."},
			{"name":"generalErrors","reason":"Exactly one of amount and amountByMonth is required."}]}`,
			"currency: 'Currency' must be 3 characters in length.; Exactly one of amount and amountByMonth is required.",
		},
		"one handler failure": {
			`{"title":"Validation Failed","status":400,"code":"validation_failed",
			"detail":"Unknown app","errors":[{"name":"appIds","reason":"Unknown app"}]}`,
			"Unknown app",
		},
		"missing capability": {
			`{"title":"Authentication Error","status":403,"code":"forbidden",
			"detail":"API token lacks capability: ManageTeam",
			"errors":[{"name":"capability","reason":"manageTeam","code":"missing_capability"}]}`,
			"API token lacks capability: ManageTeam",
		},
		"no detail or errors": {
			`{"title":"Validation Failed","status":400,"code":"validation_failed"}`,
			"Validation Failed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := parseError(&http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": {problemMediaType}},
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}, time.Now())
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want an APIError", err)
			}
			if got := apiErr.Message(); got != tc.want {
				t.Errorf("Message() = %q\nwant        %q", got, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Error() = %q, want it to carry the message", err.Error())
			}
		})
	}
}

func TestErrorCodeAndRetryableDecode(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": {"42"}},
		Body: io.NopCloser(strings.NewReader(`{"title":"Rate Limit Exceeded","status":429,"code":"rate_limit_exceeded",
		"retryable":true,"detail":"API call quota exceeded: at most 100 per 1m."}`)),
	}

	err := parseError(resp, time.Now())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an APIError", err)
	}
	if apiErr.Code != "rate_limit_exceeded" || !IsRetryable(err) || !IsRateLimited(err) {
		t.Errorf("decoded %+v", apiErr)
	}
	if want := "costfluent: API call quota exceeded: at most 100 per 1m. (429); this API token's request quota resets in 42s"; err.Error() != want {
		t.Errorf("message %q, want %q", err, want)
	}
}

func TestProblemDetailPreferred(t *testing.T) {
	c, _ := testClient(t, stub{404, `{"status":404,"title":"Not Found","detail":"Workspace not found"}`})

	_, err := c.GetWorkspace(context.Background(), "ws_1")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if got := apiErr.Message(); got != "Workspace not found" {
		t.Errorf("message %q", got)
	}
}

func TestNonJSONErrorKeepsBody(t *testing.T) {
	c, _ := testClient(t, stub{502, "upstream gone"})

	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	if !IsServerError(err) || !strings.Contains(err.Error(), "upstream gone") {
		t.Fatalf("err = %v", err)
	}
}

// TestEveryErrorStatusMapsToItsHelper holds each status to exactly the helpers that should
// recognise it, and to the retry policy: a GET is resent on 429 and 5xx, never on another 4xx.
func TestEveryErrorStatusMapsToItsHelper(t *testing.T) {
	type helpers struct{ badRequest, unauthorized, forbidden, notFound, conflict, rateLimited, server bool }
	cases := []struct {
		status    int
		code      string
		retryable bool
		want      helpers
		attempts  int32
	}{
		{400, "validation_failed", false, helpers{badRequest: true}, 1},
		{401, "unauthorized", false, helpers{unauthorized: true}, 1},
		{403, "forbidden", false, helpers{forbidden: true}, 1},
		{404, "not_found", false, helpers{notFound: true}, 1},
		{409, "conflict", false, helpers{conflict: true}, 1},
		{422, "unprocessable", false, helpers{}, 1},
		{429, "rate_limit_exceeded", true, helpers{rateLimited: true}, 4},
		{500, "internal_error", false, helpers{server: true}, 4},
		{503, "service_unavailable", true, helpers{server: true}, 4},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprintf(w, `{"title":%q,"status":%d,"code":%q,"retryable":%t,"traceId":"00-abc-01"}`,
					http.StatusText(tc.status), tc.status, tc.code, tc.retryable)
			}))
			t.Cleanup(srv.Close)
			c := NewClient(WithBaseURL(srv.URL), WithAPIKey("key"))

			_, err := c.GetWorkspace(context.Background(), "ws_1")
			got := helpers{IsBadRequest(err), IsUnauthorized(err), IsForbidden(err), IsNotFound(err),
				IsConflict(err), IsRateLimited(err), IsServerError(err)}
			if got != tc.want {
				t.Errorf("helpers %+v, want %+v", got, tc.want)
			}
			if StatusCode(err) != tc.status || IsRetryable(err) != tc.retryable {
				t.Errorf("status %d retryable %v", StatusCode(err), IsRetryable(err))
			}
			if !strings.Contains(err.Error(), http.StatusText(tc.status)) || !strings.Contains(err.Error(), "traceId=00-abc-01") {
				t.Errorf("message %q", err.Error())
			}
			if calls.Load() != tc.attempts {
				t.Errorf("sent %d times, want %d", calls.Load(), tc.attempts)
			}
		})
	}
}

func TestMessageFallsBackToTheMostSpecificField(t *testing.T) {
	cases := []struct {
		err  APIError
		want string
	}{
		{APIError{StatusCode: 400, Detail: "d", Title: "t", Errors: []FieldError{{Reason: "r"}}}, "d; r"},
		{APIError{StatusCode: 400, Title: "t", Errors: []FieldError{{Reason: "r1"}, {Reason: "r2"}}}, "r1; r2"},
		{APIError{StatusCode: 409, Title: "Conflict on name"}, "Conflict on name"},
		{APIError{StatusCode: 502}, "Bad Gateway"},
	}
	for _, tc := range cases {
		if got := tc.err.Message(); got != tc.want {
			t.Errorf("%+v: message %q, want %q", tc.err, got, tc.want)
		}
	}
	if got := (&APIError{StatusCode: 404, Detail: "gone"}).Error(); got != "costfluent: gone (404)" {
		t.Errorf("error without a trace %q", got)
	}
}

func TestHelpersIgnoreOtherErrors(t *testing.T) {
	for _, err := range []error{nil, errors.New("dial tcp: refused"), context.Canceled} {
		if IsNotFound(err) || IsConflict(err) || IsUnauthorized(err) || IsForbidden(err) || IsBadRequest(err) ||
			IsRateLimited(err) || IsServerError(err) || IsRetryable(err) || HasCode(err, CodeAwsAccessPending) ||
			StatusCode(err) != 0 {
			t.Errorf("%v matched an API error helper", err)
		}
	}

	wrapped := fmt.Errorf("creating provider: %w", &APIError{StatusCode: 409})
	if !IsConflict(wrapped) || StatusCode(wrapped) != 409 {
		t.Errorf("a wrapped APIError was not recognised")
	}
}

func TestHasCodeReadsTheErrorEntries(t *testing.T) {
	c, _ := testClient(t, stub{409, `{"title":"Provider connection is not ready","status":409,"code":"conflict",
		"retryable":true,"errors":[{"name":"datasetId","reason":"The billing export dataset is not shared yet.",
		"code":"Provider.GcpAccessPending"}]}`})

	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	if !HasCode(err, CodeGcpAccessPending) {
		t.Fatalf("err = %v, want %s", err, CodeGcpAccessPending)
	}
	if HasCode(err, CodeAwsAccessPending) {
		t.Error("matched a code the response did not carry")
	}
	if HasCode(err, "conflict") {
		t.Error("matched the problem's top-level code; HasCode reads the error entries")
	}
}

func TestMalformedProblemKeepsTheRawBody(t *testing.T) {
	c, _ := testClient(t,
		stub{400, `{"title": "truncated`},
		stub{500, ""},
	)

	_, err := c.TriggerProviderSync(context.Background(), "prv_1", false)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || apiErr.Detail != `{"title": "truncated` {
		t.Fatalf("err = %#v", err)
	}

	_, err = c.TriggerProviderSync(context.Background(), "prv_1", false)
	if !errors.As(err, &apiErr) || apiErr.Message() != "Internal Server Error" {
		t.Fatalf("an empty 500 body gave %v", err)
	}
}

// The seven create endpoints answer with an ID only, so the SDK reads the object back. When that
// read fails the object already exists, and the caller needs its ID to manage or remove it.
func TestCreateWhoseReadFailsReturnsCreatedButUnread(t *testing.T) {
	forbidden := stub{403, `{"title":"Forbidden","status":403,"detail":"The token cannot read it"}`}
	cases := []struct {
		name, path string
		create     func(*Client) error
	}{
		{"saved filter", "/v1/saved-filters/x_1", func(c *Client) error {
			_, err := c.CreateSavedFilter(context.Background(), &CreateSavedFilterInput{})
			return err
		}},
		{"dashboard", "/v1/dashboards/x_1", func(c *Client) error {
			_, err := c.CreateDashboard(context.Background(), &CreateDashboardInput{})
			return err
		}},
		{"workspace", "/v1/workspaces/x_1", func(c *Client) error {
			_, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{})
			return err
		}},
		{"cost alert", "/v1/cost-alerts/x_1", func(c *Client) error {
			_, err := c.CreateCostAlert(context.Background(), &CreateCostAlertInput{})
			return err
		}},
		{"virtual tag", "/v1/virtual-tags/x_1", func(c *Client) error {
			_, err := c.CreateVirtualTag(context.Background(), &CreateVirtualTagInput{})
			return err
		}},
		{"cost report", "/v1/cost-reports/x_1", func(c *Client) error {
			_, err := c.CreateCostReport(context.Background(), &CreateCostReportInput{})
			return err
		}},
		{"folder", "/v1/folders/x_1", func(c *Client) error {
			_, err := c.CreateFolder(context.Background(), &CreateFolderInput{})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, got := testClient(t, stub{200, `{"id":"x_1"}`}, forbidden)

			err := tc.create(c)
			wantRequest(t, (*got)[1], "GET", tc.path)
			if id, ok := CreatedID(err); !ok || id != "x_1" {
				t.Fatalf("CreatedID(%v) = %q, %v; want x_1", err, id, ok)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 || !IsForbidden(err) {
				t.Fatalf("the read's API error is not reachable through %#v", err)
			}
			if !strings.Contains(err.Error(), "x_1") || !strings.Contains(err.Error(), "The token cannot read it") {
				t.Fatalf("message %q names neither the ID nor the cause", err.Error())
			}
		})

		t.Run(tc.name+" whose create fails", func(t *testing.T) {
			c, _ := testClient(t, forbidden)

			err := tc.create(c)
			if id, ok := CreatedID(err); ok {
				t.Fatalf("a failed create returned the ID %q", id)
			}
			if !IsForbidden(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	if id, ok := CreatedID(nil); ok || id != "" {
		t.Fatalf("CreatedID(nil) = %q, %v", id, ok)
	}
}

func TestMissingCapabilityNamesTheCapabilityA403Lacks(t *testing.T) {
	c, _ := testClient(t, stub{403, `{"title":"Forbidden","status":403,"code":"forbidden",
		"detail":"API token lacks capability: ManageTeam",
		"errors":[{"name":"capability","reason":"manageTeam","code":"missing_capability"}]}`})

	_, err := c.CreateWorkspace(context.Background(), &CreateWorkspaceInput{Name: "x", Currency: "EUR"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an APIError", err)
	}
	capability, ok := apiErr.MissingCapability()
	if !ok || capability != CapabilityManageTeam {
		t.Errorf("MissingCapability() = %q, %v; want %q, true", capability, ok, CapabilityManageTeam)
	}
	if !IsMissingCapability(err) || !IsForbidden(err) {
		t.Errorf("IsMissingCapability or IsForbidden false for %+v", apiErr)
	}
}

func TestMissingCapabilityIgnoresEveryOtherRefusal(t *testing.T) {
	for name, err := range map[string]error{
		"a scope 403":                            &APIError{StatusCode: 403, Detail: "Workspace token cannot access other workspaces"},
		"a plan 403":                             &APIError{StatusCode: 403, Errors: []FieldError{{Name: "plan", Reason: "pro", Code: "plan_upgrade_required"}}},
		"a 400 naming a field called capability": &APIError{StatusCode: 400, Errors: []FieldError{{Name: "capability", Reason: "viewCosts", Code: "missing_capability"}}},
		"a 403 whose capability entry is not errors[0]": &APIError{StatusCode: 403, Errors: []FieldError{
			{Name: "plan", Reason: "pro", Code: "plan_upgrade_required"},
			{Name: "capability", Reason: "viewCosts", Code: "missing_capability"},
		}},
		"not an API error": fmt.Errorf("network down"),
	} {
		if IsMissingCapability(err) {
			t.Errorf("%s: IsMissingCapability = true", name)
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if _, ok := apiErr.MissingCapability(); ok {
				t.Errorf("%s: MissingCapability ok = true", name)
			}
		}
	}
}

// Only the API's problem details say an object is gone. A 404 in any other media type is the edge
// answering for a backend that is not routed yet, such as Traefik's "404 page not found" while a
// container is recreated, and a caller that took it as gone would drop a resource that exists.
func TestOnlyAProblemDetails404IsNotFound(t *testing.T) {
	cases := []struct {
		contentType string
		body        string
		notFound    bool
	}{
		{"application/problem+json", `{"status":404,"title":"Not Found"}`, true},
		{"Application/Problem+JSON; charset=utf-8", `{"status":404,"title":"Not Found"}`, true},
		{"text/plain; charset=utf-8", "404 page not found\n", false},
		{"text/html", "<html><body>Not Found</body></html>", false},
		{"application/json", `{"status":404,"title":"Not Found"}`, false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.contentType, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			c := NewClient(WithBaseURL(srv.URL), WithAPIKey("key"))
			slept := recordSleeps(c)

			_, err := c.GetWorkspace(context.Background(), "ws_1")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want an APIError", err)
			}
			if IsNotFound(err) != tc.notFound {
				t.Errorf("IsNotFound = %v, want %v", IsNotFound(err), tc.notFound)
			}
			if apiErr.StatusCode != http.StatusNotFound || apiErr.ContentType != tc.contentType {
				t.Errorf("status %d content type %q, want 404 %q", apiErr.StatusCode, apiErr.ContentType, tc.contentType)
			}
			if tc.notFound {
				if calls.Load() != 1 || IsRetryable(err) {
					t.Errorf("sent %d times, retryable %v: a problem details 404 is final", calls.Load(), IsRetryable(err))
				}
				return
			}
			if calls.Load() != 4 || len(slept.all()) != 3 || !IsRetryable(err) {
				t.Errorf("sent %d times after waits %v, retryable %v: want the edge's 404 retried like a 5xx",
					calls.Load(), slept.all(), IsRetryable(err))
			}
		})
	}
}

func TestPlainTextNotFoundKeepsItsBody(t *testing.T) {
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "404 page not found\n")
	})

	_, err := c.GetWorkspace(context.Background(), "ws_1")
	if err == nil || err.Error() != "costfluent: 404 page not found (404)" {
		t.Fatalf("err = %v", err)
	}
}

func TestAPIErrorRetryAfterIsTheWaitA429AskedFor(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		status int
		header http.Header
		want   time.Duration
	}{
		"Retry-After on a 429":      {http.StatusTooManyRequests, http.Header{"Retry-After": {"120"}}, 2 * time.Minute},
		"RateLimit-Reset alone":     {http.StatusTooManyRequests, http.Header{"Ratelimit-Reset": {"90"}}, 90 * time.Second},
		"a 429 naming no wait":      {http.StatusTooManyRequests, http.Header{}, 0},
		"Retry-After on a 503":      {http.StatusServiceUnavailable, http.Header{"Retry-After": {"30"}}, 0},
		"a 404 with no wait at all": {http.StatusNotFound, http.Header{}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{StatusCode: c.status, Header: c.header, Body: io.NopCloser(strings.NewReader(`{"title":"refused"}`))}
			var ae *APIError
			if !errors.As(parseError(resp, now), &ae) {
				t.Fatal("not an APIError")
			}
			if got := ae.RetryAfter(); got != c.want {
				t.Errorf("RetryAfter() = %s, want %s", got, c.want)
			}
		})
	}
}
