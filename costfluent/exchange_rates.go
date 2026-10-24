package costfluent

import (
	"context"
	"net/http"
	"time"
)

// ExchangeRate is one published rate: one unit of BaseCurrency buys Rate units of TargetCurrency
// on EffectiveDate, according to Source. Costfluent publishes the European Central Bank's euro
// reference rates, so every pair derives through the euro.
type ExchangeRate struct {
	BaseCurrency   string  `json:"baseCurrency"`
	TargetCurrency string  `json:"targetCurrency"`
	Rate           float64 `json:"rate"`
	EffectiveDate  string  `json:"effectiveDate"`
	Source         string  `json:"source"`
}

// ExchangeRatesResponse is one page of rates over the window that was read.
type ExchangeRatesResponse = ListResponse[ExchangeRate]

// ExchangeRateOptions scopes a rate listing to the publication days from StartDate to EndDate,
// both included; only the calendar day of each is sent. A zero value reads the latest publication
// day.
type ExchangeRateOptions struct {
	StartDate      *time.Time
	EndDate        *time.Time
	BaseCurrency   string
	TargetCurrency string
	PageOptions
}

// ListExchangeRates returns the rates published over a window, so the rates a report converted
// by can be pulled and checked.
func (c *Client) ListExchangeRates(ctx context.Context, opts *ExchangeRateOptions) (*ExchangeRatesResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/exchange-rates", nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		if opts.StartDate != nil {
			q.Set("startDate", opts.StartDate.Format(time.DateOnly))
		}
		if opts.EndDate != nil {
			q.Set("endDate", opts.EndDate.Format(time.DateOnly))
		}
		if opts.BaseCurrency != "" {
			q.Set("baseCurrency", opts.BaseCurrency)
		}
		if opts.TargetCurrency != "" {
			q.Set("targetCurrency", opts.TargetCurrency)
		}
		req.URL.RawQuery = q.Encode()
		opts.apply(req)
	}

	var resp ExchangeRatesResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
