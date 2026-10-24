package costfluent

import (
	"context"
	"net/http"
)

// CostFilterTranslation is a cost filter in both canonical spellings: the JSON document the API
// stores and returns, and CQL text. Both are empty for no filter.
type CostFilterTranslation struct {
	Filter string `json:"filter"`
	Query  string `json:"query"`
}

// TranslateCostFilter reads a filter written as its JSON document or as CQL text and returns both
// canonical spellings. A filter that does not parse is an API error naming the position.
func (c *Client) TranslateCostFilter(ctx context.Context, filter string) (*CostFilterTranslation, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/cost-filters/translate", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	setIf(q, "filter", filter)
	req.URL.RawQuery = q.Encode()

	var resp CostFilterTranslation
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
