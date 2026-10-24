package costfluent

import "net/http"

// RequestOption adjusts one request a write sends, such as IfMatch.
type RequestOption func(*http.Request)

// IfMatch applies a write only while the resource still has etag, the ETag of an earlier read or
// write; otherwise the API answers 412 and changes nothing (IsPreconditionFailed). An empty etag
// sends no condition, and the last write wins.
func IfMatch(etag string) RequestOption {
	return func(req *http.Request) {
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
	}
}

func applyOptions(req *http.Request, opts []RequestOption) {
	for _, opt := range opts {
		opt(req)
	}
}

// versioned is a resource that keeps the ETag of the response that carried it. A list carries no
// versions, so a resource read from one has none.
type versioned interface {
	setETag(etag string)
}
