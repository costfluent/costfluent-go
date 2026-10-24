package costfluent

import (
	"context"
	"testing"
)

// Coverage reads an inclusive window of days and answers it as it was read: endDate is the last
// day counted, not the day after it.
func TestGetAllocationCoverageReadsAnInclusiveWindow(t *testing.T) {
	c, got := testClient(t, stub{200, `{"startDate":"2026-09-01","endDate":"2026-09-30","costBasis":"AmortizedCost",
		"currencies":[{"billingCurrency":"EUR","totalCost":"100","directCost":"60","sharedCost":"30",
		"undistributedSharedCost":"0","unallocatedCost":"10","unallocatedRecordCount":2,"recordCount":20,
		"coverage":0.9,"segments":[]}]}`})

	coverage, err := c.GetAllocationCoverage(context.Background(), "ws_1", "2026-09-01", "2026-09-30", "AmortizedCost")
	if err != nil {
		t.Fatal(err)
	}
	ex := (*got)[0]
	wantRequest(t, ex, "GET", "/v1/allocation/coverage")
	wantQuery(t, ex, "workspaceId", "ws_1")
	wantQuery(t, ex, "startDate", "2026-09-01")
	wantQuery(t, ex, "endDate", "2026-09-30")
	wantQuery(t, ex, "costBasis", "AmortizedCost")
	for _, gone := range []string{"windowStart", "windowEndExclusive"} {
		if _, sent := ex.Query[gone]; sent {
			t.Errorf("sent %s, which the API no longer reads", gone)
		}
	}
	if coverage.StartDate != "2026-09-01" || coverage.EndDate != "2026-09-30" || coverage.CostBasis != "AmortizedCost" {
		t.Fatalf("decoded window %+v", coverage)
	}
	if len(coverage.Currencies) != 1 || !coverage.Currencies[0].UnallocatedCost.Equal("10") {
		t.Fatalf("decoded currencies %+v", coverage.Currencies)
	}
}

// Each shared pool reads with its method, every share in split order and the residual the last
// share absorbed, all exact.
func TestGetAllocationCoverageReadsThePools(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"startDate":"2026-09-01","endDate":"2026-09-30","costBasis":"ActualCost",
		"currencies":[{"billingCurrency":"EUR","totalCost":"100","directCost":"0","sharedCost":"100",
		"undistributedSharedCost":"0","unallocatedCost":"0","unallocatedRecordCount":0,"recordCount":3,
		"coverage":1,"segments":[],"pools":[{"segmentId":"seg_3","label":"Cluster","method":"even",
		"cost":"100","distributedCost":"100","residual":"0.00000001","recordCount":3,
		"shares":[{"segmentId":"seg_1","share":"33.33333333"},{"segmentId":"seg_2","share":"33.33333333"},
		{"segmentId":"seg_4","share":"33.33333334"}]}]}]}`})

	coverage, err := c.GetAllocationCoverage(context.Background(), "ws_1", "2026-09-01", "2026-09-30", "")
	if err != nil {
		t.Fatal(err)
	}
	pools := coverage.Currencies[0].Pools
	if len(pools) != 1 || pools[0].SegmentID != "seg_3" || pools[0].Label != "Cluster" ||
		pools[0].Method != SegmentSplitEven || pools[0].RecordCount != 3 ||
		pools[0].Cost != "100" || pools[0].DistributedCost != "100" || pools[0].Residual != "0.00000001" {
		t.Fatalf("decoded pools %+v", pools)
	}
	shares := pools[0].Shares
	if len(shares) != 3 || shares[2].SegmentID != "seg_4" || shares[2].Share != "33.33333334" {
		t.Fatalf("decoded shares %+v", shares)
	}
}

func TestGetAllocationCoverageDefaultsToActualCost(t *testing.T) {
	c, got := testClient(t, stub{200, `{"startDate":"2026-09-01","endDate":"2026-09-01","costBasis":"ActualCost","currencies":[]}`})

	if _, err := c.GetAllocationCoverage(context.Background(), "ws_1", "2026-09-01", "2026-09-01", ""); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "costBasis", "ActualCost")
}
