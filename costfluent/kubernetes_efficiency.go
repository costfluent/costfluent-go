package costfluent

import (
	"context"
	"net/http"
)

// KubernetesEfficiency is the cost, idle cost and CPU and memory efficiency of a workspace's
// Kubernetes clusters over a window, per cluster, namespace or workload, largest cost first
type KubernetesEfficiency struct {
	// Currency is the presentation currency; nil when the window mixes currencies unconverted
	Currency *string `json:"currency,omitempty"`
	// Level is cluster, namespace or workload
	Level string                    `json:"level"`
	Rows  []KubernetesEfficiencyRow `json:"rows"`
	// Truncated is true when more rows matched than the read returns (500); the smallest were left out
	Truncated bool `json:"truncated"`
	// HistoryFloor is the earliest day the plan let the read reach, when it withheld older data
	HistoryFloor *string `json:"historyFloor,omitempty"`
	// Unconverted is the spend of rows read that had no rate into Currency
	Unconverted []CurrencyAmount `json:"unconverted"`
}

// KubernetesEfficiencyRow is one cluster, namespace or workload
type KubernetesEfficiencyRow struct {
	Cluster string `json:"cluster"`
	// Namespace is nil on a cluster row
	Namespace *string `json:"namespace,omitempty"`
	// Workload is a controller (Deployment/api) or a pod without one (Pod/job-x); nil on a cluster
	// or namespace row, empty on the idle namespace's
	Workload *string            `json:"workload,omitempty"`
	Cost     Decimal            `json:"cost"`
	IdleCost Decimal            `json:"idleCost"`
	CPU      ResourceEfficiency `json:"cpu"`
	Memory   ResourceEfficiency `json:"memory"`
}

// ResourceEfficiency is CPU in core-hours or memory in GiB-hours: requested, used and the day peak
// held for the hours observed
type ResourceEfficiency struct {
	Requested float64 `json:"requested"`
	Used      float64 `json:"used"`
	Peak      float64 `json:"peak"`
	// UsedPercent is used over requested; nil when nothing was requested
	UsedPercent *float64 `json:"usedPercent,omitempty"`
	// PeakPercent is peak over requested; nil when nothing was requested
	PeakPercent *float64 `json:"peakPercent,omitempty"`
}

// KubernetesEfficiencyOptions is the window and level efficiency reads. Dates are calendar days,
// YYYY-MM-DD, both included. Level is cluster, namespace or workload. Filter is a cost filter as
// its JSON document or as CQL.
type KubernetesEfficiencyOptions struct {
	StartDate string
	EndDate   string
	Level     string
	Filter    string
}

// GetKubernetesEfficiency reads the workspace's Kubernetes efficiency over a window
func (c *Client) GetKubernetesEfficiency(ctx context.Context, workspaceID string, opts *KubernetesEfficiencyOptions) (*KubernetesEfficiency, error) {
	req, err := c.newWorkspaceRequest(ctx, http.MethodGet, "/v1/kubernetes/efficiency", workspaceID, nil)
	if err != nil {
		return nil, err
	}

	if opts != nil {
		q := req.URL.Query()
		setIf(q, "startDate", opts.StartDate)
		setIf(q, "endDate", opts.EndDate)
		setIf(q, "level", opts.Level)
		setIf(q, "filter", opts.Filter)
		req.URL.RawQuery = q.Encode()
	}

	var efficiency KubernetesEfficiency
	if err := c.do(req, &efficiency); err != nil {
		return nil, err
	}
	return &efficiency, nil
}
