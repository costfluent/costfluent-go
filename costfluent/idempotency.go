package costfluent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
)

// idempotencyKeyHeader names the key a create carries, so the API answers a retry of it with the
// first response instead of creating a second object.
const idempotencyKeyHeader = "Idempotency-Key"

// replayedHeader marks an answer the API replayed for a create it had already run under the same
// Idempotency-Key.
const replayedHeader = "Idempotent-Replayed"

// replayable is a create result that records whether its answer was a replay.
type replayable interface {
	setReplayed(yes bool)
}

// newCreateRequest is newRequest for a POST that creates an object. It carries a new
// Idempotency-Key, which the transport resends unchanged on each of its own retries, so a create
// whose first answer was lost runs once.
func (c *Client) newCreateRequest(ctx context.Context, path string, body any) (*http.Request, error) {
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(idempotencyKeyHeader, newIdempotencyKey())
	return req, nil
}

func newIdempotencyKey() string {
	b := make([]byte, 16)
	// crypto/rand.Read never returns an error since Go 1.24; it crashes the program instead.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// IsIdempotencyKeyReused returns true if err is the API refusing a create whose Idempotency-Key
// was already used with another request.
func IsIdempotencyKeyReused(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == "idempotency_key_reused"
}
