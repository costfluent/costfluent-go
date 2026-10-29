package costfluent

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

// An amount goes out as the decimal string it was written as, so an eighth decimal a float would
// round survives, and an optional field left unset is not sent at all rather than as a zero.
func TestSubmitCostsSendsTheEntriesAsWrittenAndReturnsTheBatch(t *testing.T) {
	c, got := testClient(t, stub{200, `{"batchId":"cbt_aaaaaaaaaaaaaaaaaaaaaa","totalRows":2,"acceptedRows":2,"rejectedRows":0}`})

	batch, err := c.SubmitCosts(context.Background(), "prv_1", &SubmitCostsInput{Costs: []CostEntry{
		{
			ChargePeriodStart: "2026-10-01", ChargePeriodEnd: ptr("2026-10-03"),
			ChargeCategory: "Usage", ServiceName: "Compute",
			BilledCost: "12.12345678", ListCost: ptr[Decimal]("13.00000001"), EffectiveCost: ptr[Decimal]("0"),
			BillingCurrency: ptr("EUR"), ConsumedQuantity: ptr(0.0), ConsumedUnit: ptr("Hrs"),
			LineItemID: ptr("line-1"), Tags: map[string]string{"team": "platform"},
		},
		{ChargePeriodStart: "2026-10-01", ChargeCategory: "Tax", ServiceName: "Compute", BilledCost: "-1.5"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/custom-providers/prv_1/costs")
	if _, sent := (*got)[0].Body["replaceLines"]; sent {
		t.Errorf("sent replaceLines on a batch that does not replace lines: %v", (*got)[0].Body)
	}
	costs, _ := (*got)[0].Body["costs"].([]any)
	if len(costs) != 2 {
		t.Fatalf("sent costs %v, want two entries", (*got)[0].Body["costs"])
	}
	full, _ := costs[0].(map[string]any)
	for key, want := range map[string]any{
		"chargePeriodStart": "2026-10-01", "chargePeriodEnd": "2026-10-03", "chargeCategory": "Usage",
		"serviceName": "Compute", "billedCost": "12.12345678", "listCost": "13.00000001", "effectiveCost": "0",
		"billingCurrency": "EUR", "consumedQuantity": float64(0), "consumedUnit": "Hrs", "lineItemId": "line-1",
	} {
		if full[key] != want {
			t.Errorf("first entry %s = %#v, want %#v", key, full[key], want)
		}
	}
	if tags, _ := full["tags"].(map[string]any); tags["team"] != "platform" {
		t.Errorf("first entry tags = %v, want team=platform", full["tags"])
	}
	minimal, _ := costs[1].(map[string]any)
	if len(minimal) != 4 || minimal["billedCost"] != "-1.5" {
		t.Errorf("second entry %v, want only its four required fields", minimal)
	}

	if batch.BatchID != "cbt_aaaaaaaaaaaaaaaaaaaaaa" || batch.TotalRows != 2 || batch.AcceptedRows != 2 ||
		batch.RejectedRows != 0 || batch.Errors != nil || batch.Replayed {
		t.Fatalf("decoded %+v", batch)
	}
}

func TestSubmitCostsAsksToReplaceLinesOnlyWhenSet(t *testing.T) {
	c, got := testClient(t, stub{200, `{"batchId":"cbt_aaaaaaaaaaaaaaaaaaaaaa","totalRows":1,"acceptedRows":1,"rejectedRows":0}`})

	_, err := c.SubmitCosts(context.Background(), "prv_1", &SubmitCostsInput{
		Costs:        []CostEntry{{ChargePeriodStart: "2026-10-01", ChargeCategory: "Usage", ServiceName: "Compute", BilledCost: "1"}},
		ReplaceLines: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "replaceLines", true)
}

// The API answers a batch with rows it refused as a success that counts them, so the caller reads
// which rows and why from the result, not from an error.
func TestSubmitCostsReturnsTheRejectedRowsWithoutFailing(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"batchId":"cbt_aaaaaaaaaaaaaaaaaaaaaa","totalRows":2,"acceptedRows":1,"rejectedRows":1,` +
		`"errors":[{"rowIndex":1,"field":"billedCost","message":"At most eight decimals."}]}`})

	batch, err := c.SubmitCosts(context.Background(), "prv_1", &SubmitCostsInput{Costs: []CostEntry{
		{ChargePeriodStart: "2026-10-01", ChargeCategory: "Usage", ServiceName: "Compute", BilledCost: "1"},
		{ChargePeriodStart: "2026-10-01", ChargeCategory: "Usage", ServiceName: "Compute", BilledCost: "1.123456789"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if batch.AcceptedRows != 1 || batch.RejectedRows != 1 || len(batch.Errors) != 1 {
		t.Fatalf("decoded %+v", batch)
	}
	if e := batch.Errors[0]; e.RowIndex != 1 || e.Field != "billedCost" || e.Message != "At most eight decimals." {
		t.Fatalf("row error %+v", e)
	}
}

// A provider ID is one path segment, so a slash or dot in it cannot reach another route.
func TestSubmitCostsEscapesTheProviderID(t *testing.T) {
	var path string
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"batchId":"cbt_aaaaaaaaaaaaaaaaaaaaaa","totalRows":0,"acceptedRows":0,"rejectedRows":0}`)
	})

	if _, err := c.SubmitCosts(context.Background(), "prv/../x", &SubmitCostsInput{}); err != nil {
		t.Fatal(err)
	}
	if path != "/v1/custom-providers/prv%2F..%2Fx/costs" {
		t.Fatalf("request path %s, want the ID escaped into one segment", path)
	}
}

// A batch whose first answer is lost is resent under the same key, and the API's replay of the
// committed batch is marked, so the caller knows the rows went in once.
func TestSubmitCostsMarksAReplayedAnswer(t *testing.T) {
	var posts atomic.Int32
	var mu sync.Mutex
	var keys []string
	c := retryingClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get(idempotencyKeyHeader))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if posts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set(replayedHeader, "true")
		_, _ = io.WriteString(w, `{"batchId":"cbt_aaaaaaaaaaaaaaaaaaaaaa","totalRows":1,"acceptedRows":1,"rejectedRows":0}`)
	})

	batch, err := c.SubmitCosts(context.Background(), "prv_1", &SubmitCostsInput{
		Costs: []CostEntry{{ChargePeriodStart: "2026-10-01", ChargeCategory: "Usage", ServiceName: "Compute", BilledCost: "1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !batch.Replayed || len(keys) != 2 || keys[0] == "" || keys[1] != keys[0] {
		t.Fatalf("batch %+v after keys %q, want the replay marked and one key on both attempts", batch, keys)
	}
}

// A batch another submission or upload overtook committed nothing. The API asks for it to be sent
// again, which the caller decides: the client returns the conflict rather than resending it.
func TestSubmitCostsReturnsAnOvertakenBatchAsAConflict(t *testing.T) {
	var posts atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, _ *http.Request) {
		posts.Add(1)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"title":"Conflict","status":409,"code":"concurrency_conflict"}`)
	})

	_, err := c.SubmitCosts(context.Background(), "prv_1", &SubmitCostsInput{
		Costs: []CostEntry{{ChargePeriodStart: "2026-10-01", ChargeCategory: "Usage", ServiceName: "Compute", BilledCost: "1"}},
	})
	if !IsConflict(err) || posts.Load() != 1 {
		t.Fatalf("err %v after %d posts, want one conflict", err, posts.Load())
	}
}
