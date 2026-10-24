package costfluent

import (
	"context"
	"testing"
)

// Every update is a partial PUT: a field left nil or false is not sent, so the API keeps its value,
// and a clear<Field> flag is sent only when set. These cases pin each field the update rule added.
func TestUpdatesSendTheirNewFieldsOnlyWhenSet(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		body  string
		set   func(c *Client) error
		unset func(c *Client) error
		want  map[string]any
	}{
		{
			name: "provider",
			body: providerJSON,
			set: func(c *Client) error {
				_, err := c.UpdateProvider(ctx, "prv_1", &UpdateProviderInput{
					Name: ptr("AWS production (EU)"), ClearDescription: true, ClearSettings: true,
				})
				return err
			},
			unset: func(c *Client) error { _, err := c.UpdateProvider(ctx, "prv_1", &UpdateProviderInput{}); return err },
			want:  map[string]any{"name": "AWS production (EU)", "clearDescription": true, "clearSettings": true},
		},
		{
			name: "budget",
			body: `{"id":"bdg_1"}`,
			set: func(c *Client) error {
				_, err := c.UpdateBudget(ctx, "bdg_1", &UpdateBudgetInput{
					CostReportID: ptr("crp_1"), ClearSegment: true, ClearCostReport: true,
				})
				return err
			},
			unset: func(c *Client) error { _, err := c.UpdateBudget(ctx, "bdg_1", &UpdateBudgetInput{}); return err },
			want:  map[string]any{"costReportId": "crp_1", "clearSegment": true, "clearCostReport": true},
		},
		{
			name: "virtual tag",
			body: `{"id":"vtg_1"}`,
			set: func(c *Client) error {
				_, err := c.UpdateVirtualTag(ctx, "vtg_1", &UpdateVirtualTagInput{
					WorkspaceID: "ws_1", ClearDescription: true, ClearDefaultValue: true,
				})
				return err
			},
			unset: func(c *Client) error {
				_, err := c.UpdateVirtualTag(ctx, "vtg_1", &UpdateVirtualTagInput{WorkspaceID: "ws_1"})
				return err
			},
			want: map[string]any{"clearDescription": true, "clearDefaultValue": true},
		},
		{
			name: "folder",
			body: `{"id":"fld_1"}`,
			set: func(c *Client) error {
				_, err := c.UpdateFolder(ctx, "fld_1", &UpdateFolderInput{WorkspaceID: "ws_1", ClearParent: true})
				return err
			},
			unset: func(c *Client) error {
				_, err := c.UpdateFolder(ctx, "fld_1", &UpdateFolderInput{WorkspaceID: "ws_1"})
				return err
			},
			want: map[string]any{"clearParent": true},
		},
		{
			name: "cost report",
			body: `{"id":"crp_1"}`,
			set: func(c *Client) error {
				_, err := c.UpdateCostReport(ctx, "ws_1", "crp_1", &UpdateCostReportInput{ClearStartDate: true, ClearEndDate: true})
				return err
			},
			unset: func(c *Client) error {
				_, err := c.UpdateCostReport(ctx, "ws_1", "crp_1", &UpdateCostReportInput{})
				return err
			},
			want: map[string]any{"clearStartDate": true, "clearEndDate": true},
		},
		{
			name: "segment",
			body: `{"id":"seg_1"}`,
			set: func(c *Client) error {
				_, err := c.UpdateSegment(ctx, "ws_1", "seg_1", &UpdateSegmentInput{Product: "Payments", CostCentre: "CC-1"})
				return err
			},
			unset: func(c *Client) error {
				_, err := c.UpdateSegment(ctx, "ws_1", "seg_1", &UpdateSegmentInput{})
				return err
			},
			want: map[string]any{"product": "Payments", "costCentre": "CC-1"},
		},
	}

	for _, tc := range cases {
		stubs := []stub{{200, tc.body}}
		t.Run(tc.name+" sends what is set", func(t *testing.T) {
			c, got := testClient(t, stubs...)
			if err := tc.set(c); err != nil {
				t.Fatal(err)
			}
			for key, value := range tc.want {
				wantBody(t, (*got)[0], key, value)
			}
		})
		t.Run(tc.name+" omits what is not", func(t *testing.T) {
			c, got := testClient(t, stubs...)
			if err := tc.unset(c); err != nil {
				t.Fatal(err)
			}
			for key := range tc.want {
				if v, sent := (*got)[0].Body[key]; sent {
					t.Errorf("%s sent as %v, want it omitted", key, v)
				}
			}
		})
	}
}

func TestUpdateCostAlertSendsItsClearFlagsOnlyWhenSet(t *testing.T) {
	ctx := context.Background()
	want := []string{"clearComparisonPeriod", "clearMinimumAmount", "clearFilter"}

	c, got := testClient(t, stub{200, `{"id":"cal_1"}`})
	if _, err := c.UpdateCostAlert(ctx, "ws_1", "cal_1", &UpdateCostAlertInput{
		ClearComparisonPeriod: true, ClearMinimumAmount: true, ClearFilter: true,
	}); err != nil {
		t.Fatal(err)
	}
	for _, key := range want {
		wantBody(t, (*got)[0], key, true)
	}

	c, got = testClient(t, stub{200, `{"id":"cal_1"}`})
	if _, err := c.UpdateCostAlert(ctx, "ws_1", "cal_1", &UpdateCostAlertInput{Name: ptr("Spike")}); err != nil {
		t.Fatal(err)
	}
	for _, key := range want {
		if _, sent := (*got)[0].Body[key]; sent {
			t.Errorf("%s sent, want it omitted", key)
		}
	}
}
