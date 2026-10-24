package costfluent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

// APIError is an error response from the Costfluent API, decoded from its problem details.
type APIError struct {
	StatusCode int    `json:"-"`
	Type       string `json:"type,omitempty"`
	Title      string `json:"title,omitempty"`
	Status     int    `json:"status,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Instance   string `json:"instance,omitempty"`

	// Code is the machine-readable kind of error, such as "not_found" or "validation_failed".
	Code string `json:"code,omitempty"`

	// Retryable is true when repeating the same request later can succeed.
	Retryable bool         `json:"retryable,omitempty"`
	TraceID   string       `json:"traceId,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`

	// ContentType is the response's Content-Type header.
	ContentType string `json:"-"`

	retryAfter time.Duration
}

// FieldError is one entry of a problem's errors: a validation failure on Name, or a general
// error when the API has no field to name.
type FieldError struct {
	Name     string `json:"name"`
	Reason   string `json:"reason"`
	Code     string `json:"code,omitempty"`
	Severity string `json:"severity,omitempty"`
}

// generalErrorsField is the name the API files an error under when it names no request field.
const generalErrorsField = "generalErrors"

// Message is the most specific description the response carried: the detail, then each field
// error the detail does not already state. A refused command's detail is only "One or more
// validation errors occurred", so the fields are what says what to change; a single refusal's
// detail repeats its one entry, and a missing capability's names it in another case.
func (e *APIError) Message() string {
	parts := make([]string, 0, 1+len(e.Errors))
	if e.Detail != "" {
		parts = append(parts, e.Detail)
	}
	detail := strings.ToLower(e.Detail)
	for _, fe := range e.Errors {
		if fe.Reason == "" || strings.Contains(detail, strings.ToLower(fe.Reason)) {
			continue
		}
		if fe.Name == "" || fe.Name == generalErrorsField {
			parts = append(parts, fe.Reason)
		} else {
			parts = append(parts, fe.Name+": "+fe.Reason)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	if e.Title != "" {
		return e.Title
	}
	return http.StatusText(e.StatusCode)
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("costfluent: %s (%d)", e.Message(), e.StatusCode)
	if e.TraceID != "" {
		msg += " traceId=" + e.TraceID
	}
	if e.StatusCode == http.StatusTooManyRequests && e.retryAfter > 0 {
		msg += fmt.Sprintf("; this API token's request quota resets in %s", e.retryAfter)
	}
	return msg
}

func parseError(resp *http.Response, now time.Time) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	apiErr := APIError{StatusCode: resp.StatusCode}
	if err := json.Unmarshal(body, &apiErr); err != nil {
		apiErr = APIError{StatusCode: resp.StatusCode, Detail: strings.TrimSpace(string(body))}
	}
	apiErr.ContentType = resp.Header.Get("Content-Type")
	if edgeNotFound(resp) {
		apiErr.Retryable = true
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		apiErr.retryAfter, _ = retryAfter(resp.Header, now)
	}
	return &apiErr
}

const problemMediaType = "application/problem+json"

func isProblem(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == problemMediaType
}

// edgeNotFound reports a 404 that is not the API's problem details: the reverse proxy in front of
// the API answering for a backend it has no route to yet, as Traefik does with a plain-text
// "404 page not found" while a container is recreated. It says nothing about the object.
func edgeNotFound(resp *http.Response) bool {
	return resp.StatusCode == http.StatusNotFound && !isProblem(resp.Header.Get("Content-Type"))
}

// CodeGcpAccessPending is the error code for a GCP connection whose billing export dataset is not
// shared with the organization's service account yet, or whose grant has not propagated.
const CodeGcpAccessPending = "Provider.GcpAccessPending"

// CodeAwsAccessPending is the error code for an AWS connection whose role does not trust
// Costfluent's principal with the organization's external ID yet, or has not propagated.
const CodeAwsAccessPending = "Provider.AwsAccessPending"

// HasCode returns true if err is an API error carrying code on one of its errors.
func HasCode(err error, code string) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, fe := range apiErr.Errors {
		if fe.Code == code {
			return true
		}
	}
	return false
}

// IsNotFound returns true if err is the API's 404 problem details: the object does not exist or is
// not visible to the token. A 404 in another media type came from the proxy in front of the API,
// not from the API, and is retryable instead.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound && isProblem(apiErr.ContentType)
}

// IsConflict returns true if err is a 409 Conflict error
func IsConflict(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 409
}

// IsPreconditionFailed returns true if a write sent If-Match with an ETag the resource no longer
// has: someone changed it since it was read. Read it again before retrying.
func IsPreconditionFailed(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 412
}

// IsRateLimited returns true if err is a 429 Too Many Requests error
func IsRateLimited(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 429
}

// IsUnauthorized returns true if err is a 401 Unauthorized error
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 401
}

// IsForbidden returns true if err is a 403 Forbidden error
func IsForbidden(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 403
}

// CodeMissingCapability is the code of the first error of a 403 that refuses an API token for a
// capability it lacks; that error's Reason is the capability.
const CodeMissingCapability = "missing_capability"

// MissingCapability returns the capability a 403 says the API token lacks, and false for any other
// error: a token scoped elsewhere, a plan refusal or anything but a 403.
func (e *APIError) MissingCapability() (Capability, bool) {
	if e.StatusCode != http.StatusForbidden || len(e.Errors) == 0 || e.Errors[0].Code != CodeMissingCapability {
		return "", false
	}
	return Capability(e.Errors[0].Reason), true
}

// IsMissingCapability returns true if err is a 403 refusing the API token for a capability it
// lacks; APIError.MissingCapability names it.
func IsMissingCapability(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	_, ok := apiErr.MissingCapability()
	return ok
}

// IsBadRequest returns true if err is a 400 Bad Request error
func IsBadRequest(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 400
}

// IsServerError returns true if err is a 5xx server error
func IsServerError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode >= 500
}

// IsRetryable returns true if err is an API error the API marked as worth repeating later, or a 404
// the proxy in front of the API answered without reaching it
func IsRetryable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Retryable
}

// StatusCode returns the HTTP status code from an API error, or 0 if not an APIError
func StatusCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

// CreatedButUnreadError reports that a create succeeded but reading the new object back failed.
// The object exists under ID, so a caller that drops this error leaves an object nothing manages.
type CreatedButUnreadError struct {
	ID  string
	Err error
}

func (e *CreatedButUnreadError) Error() string {
	return fmt.Sprintf("costfluent: created %s but could not read it back: %v", e.ID, e.Err)
}

func (e *CreatedButUnreadError) Unwrap() error {
	return e.Err
}

// CreatedID returns the ID of the object a create made before its read back failed, and false
// when err does not come from such a create.
func CreatedID(err error) (string, bool) {
	var created *CreatedButUnreadError
	if errors.As(err, &created) {
		return created.ID, true
	}
	return "", false
}
