package costfluent

import (
	"context"
	"testing"
)

const costAlertJSON = `{"id":"cal_1","name":"Spike","providerIds":["prv_1"],"filter":"service = 'EC2'",
	"thresholdType":"percentageIncrease","thresholdValue":"20.00000000","comparisonPeriod":"previousWeek",
	"minimumAmount":"50.00000000","appIds":[],"evaluationFrequencyMinutes":60,"status":"active","createdAt":"2026-09-01T00:00:00Z"}`

func TestCreateCostAlertReadsItBack(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"cal_1","name":"Spike"}`}, stub{200, costAlertJSON})

	period := CostAlertComparePreviousWeek
	floor := Decimal("50")
	alert, err := c.CreateCostAlert(context.Background(), &CreateCostAlertInput{
		Name: "Spike", ThresholdType: CostAlertThresholdPercentageIncrease, ThresholdValue: "20",
		ComparisonPeriod: &period, MinimumAmount: &floor, ProviderIDs: []string{"prv_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/cost-alerts")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "thresholdType", "percentageIncrease")
	wantBody(t, (*got)[0], "minimumAmount", "50")
	wantRequest(t, (*got)[1], "GET", "/v1/cost-alerts/cal_1")
	wantQuery(t, (*got)[1], "workspaceId", "ws_default")
	if alert.ID != "cal_1" || alert.ProviderIDs[0] != "prv_1" || *alert.ComparisonPeriod != "previousWeek" ||
		!alert.MinimumAmount.Equal("50") || alert.EvaluationFrequencyMinutes != 60 {
		t.Fatalf("decoded %+v", alert)
	}
}

func TestUpdateCostAlertReturnsTheAlertItsPutAnswered(t *testing.T) {
	c, got := testClient(t, stub{200, costAlertJSON})

	value := Decimal("30")
	alert, err := c.UpdateCostAlert(context.Background(), "ws_1", "cal_1", &UpdateCostAlertInput{ThresholdValue: &value})
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want the PUT alone", len(*got))
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/cost-alerts/cal_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	wantBody(t, (*got)[0], "thresholdValue", "30")
	if alert.ID != "cal_1" || alert.ProviderIDs[0] != "prv_1" || alert.EvaluationFrequencyMinutes != 60 {
		t.Fatalf("decoded %+v", alert)
	}
}

// A floor of zero is how an update removes one; leaving MinimumAmount out keeps it.
func TestUpdateCostAlertSendsTheMinimumAmountOnlyWhenSet(t *testing.T) {
	c, got := testClient(t, stub{200, costAlertJSON}, stub{200, costAlertJSON})

	zero := Decimal("0")
	if _, err := c.UpdateCostAlert(context.Background(), "ws_1", "cal_1", &UpdateCostAlertInput{MinimumAmount: &zero}); err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "minimumAmount", "0")

	name := "Renamed"
	if _, err := c.UpdateCostAlert(context.Background(), "ws_1", "cal_1", &UpdateCostAlertInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if _, ok := (*got)[1].Body["minimumAmount"]; ok {
		t.Error("an update that leaves the floor alone sent minimumAmount")
	}
}

func TestPauseCostAlert(t *testing.T) {
	c, got := testClient(t, stub{204, ""})

	if err := c.PauseCostAlert(context.Background(), "ws_1", "cal_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/cost-alerts/cal_1/pause")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
}

// An update leaves the channels and providers alone unless it names them; naming none sends an
// empty list, which removes every channel and watches every provider again.
func TestUpdateCostAlertListsAreSentWhenNamedEvenEmpty(t *testing.T) {
	c, got := testClient(t, stub{200, costAlertJSON}, stub{200, costAlertJSON}, stub{200, costAlertJSON})
	name := "Renamed"
	none := []string{}
	slack, aws := []string{"app_2"}, []string{"prv_2"}

	for _, input := range []*UpdateCostAlertInput{
		{Name: &name},
		{AppIDs: &none, ProviderIDs: &none},
		{AppIDs: &slack, ProviderIDs: &aws},
	} {
		if _, err := c.UpdateCostAlert(context.Background(), "ws_1", "cal_1", input); err != nil {
			t.Fatal(err)
		}
	}

	for _, key := range []string{"appIds", "providerIds"} {
		if value, present := (*got)[0].Body[key]; present {
			t.Errorf("an update naming no %s sent %v", key, value)
		}
		if ids, ok := (*got)[1].Body[key].([]any); !ok || len(ids) != 0 {
			t.Errorf("emptying sent %s = %v, want []", key, (*got)[1].Body[key])
		}
	}
	if ids, ok := (*got)[2].Body["appIds"].([]any); !ok || len(ids) != 1 || ids[0] != "app_2" {
		t.Errorf("replacing sent appIds = %v, want [app_2]", (*got)[2].Body["appIds"])
	}
	if ids, ok := (*got)[2].Body["providerIds"].([]any); !ok || len(ids) != 1 || ids[0] != "prv_2" {
		t.Errorf("replacing sent providerIds = %v, want [prv_2]", (*got)[2].Body["providerIds"])
	}
}

func TestAlertThresholdTypesMatchTheContract(t *testing.T) {
	wantContractValues(t, "AlertThresholdType", []string{
		CostAlertThresholdAbsolute, CostAlertThresholdPercentageIncrease,
		CostAlertThresholdBudgetPercentage, CostAlertThresholdTagCoverageBelow,
	})
}

func TestComparisonPeriodsMatchTheContract(t *testing.T) {
	wantContractValues(t, "ComparisonPeriod", []string{
		CostAlertComparePreviousDay, CostAlertComparePreviousWeek,
		CostAlertComparePreviousMonth, CostAlertCompareSameDayLastMonth,
	}, contractOmission{"previousQuarter", "the API refuses it until a release that reads it is on prod"})
}
