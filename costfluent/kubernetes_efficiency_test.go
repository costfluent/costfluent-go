package costfluent

import (
	"context"
	"testing"
)

func TestGetKubernetesEfficiency(t *testing.T) {
	c, got := testClient(t, stub{200, `{"currency":"USD","level":"namespace","rows":[
		{"cluster":"prod-eu","namespace":"api","workload":null,"cost":"600.00000000","idleCost":"0.00000000",
		"cpu":{"requested":72,"used":18,"peak":36,"usedPercent":25,"peakPercent":50},
		"memory":{"requested":96,"used":72,"peak":90,"usedPercent":75,"peakPercent":93.75}},
		{"cluster":"prod-eu","namespace":"__idle__","cost":"300.00000000","idleCost":"300.00000000",
		"cpu":{"requested":0,"used":0,"peak":0,"usedPercent":null,"peakPercent":null},
		"memory":{"requested":0,"used":0,"peak":0}}],
		"truncated":true,"historyFloor":null,"unconverted":[]}`})

	efficiency, err := c.GetKubernetesEfficiency(context.Background(), "ws_1", &KubernetesEfficiencyOptions{
		StartDate: "2026-03-01", EndDate: "2026-03-31", Level: "namespace", Filter: "costs.cluster = 'prod-eu'",
	})
	if err != nil {
		t.Fatal(err)
	}
	g := (*got)[0]
	wantRequest(t, g, "GET", "/v1/kubernetes/efficiency")
	wantQuery(t, g, "workspaceId", "ws_1")
	wantQuery(t, g, "startDate", "2026-03-01")
	wantQuery(t, g, "endDate", "2026-03-31")
	wantQuery(t, g, "level", "namespace")
	wantQuery(t, g, "filter", "costs.cluster = 'prod-eu'")
	if !efficiency.Truncated || efficiency.HistoryFloor != nil || len(efficiency.Rows) != 2 {
		t.Fatalf("decoded %+v", efficiency)
	}
	api, idle := efficiency.Rows[0], efficiency.Rows[1]
	if api.Namespace == nil || *api.Namespace != "api" || api.Workload != nil || api.Cost != "600.00000000" ||
		api.CPU.UsedPercent == nil || *api.CPU.UsedPercent != 25 || *api.Memory.PeakPercent != 93.75 {
		t.Fatalf("decoded %+v", api)
	}
	if idle.IdleCost != "300.00000000" || idle.CPU.UsedPercent != nil || idle.Memory.PeakPercent != nil {
		t.Fatalf("decoded %+v", idle)
	}
}

func TestGetKubernetesEfficiencyWithoutAWorkspaceUsesTheDefault(t *testing.T) {
	c, got := testClient(t, stub{200, `{"currency":null,"level":"cluster","rows":[],"truncated":false,"unconverted":[]}`})

	if _, err := c.GetKubernetesEfficiency(context.Background(), "", &KubernetesEfficiencyOptions{Level: "cluster"}); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "workspaceId", "ws_default")
}
