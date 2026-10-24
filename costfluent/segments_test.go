package costfluent

import (
	"context"
	"testing"
)

func TestListSegmentsDecodesTheEnvelope(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[{"id":"seg_1","teamLabel":"Platform","product":"API",
		"costCentre":"CC1","isShared":false,"createdAt":"2026-09-01T00:00:00Z"}]}`})

	segments, err := c.ListSegments(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/segments")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	if len(segments) != 1 || segments[0].TeamLabel != "Platform" {
		t.Fatalf("decoded %+v", segments)
	}
}

// A shared segment is created with its label and split, the targets in the order given, since the
// last takes the rounding residual. A percent travels as a JSON number, not a decimal string.
func TestCreateSharedSegmentSendsItsLabelAndSplit(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"seg_3","teamLabel":"","product":"","costCentre":"","isShared":true,
		"label":"Cluster","split":{"method":"percent","targets":[{"segmentId":"seg_1","percent":60.25},
		{"segmentId":"seg_2","percent":39.75}]},"createdAt":"2026-10-20T00:00:00Z"}`})
	first, second := 60.25, 39.75

	segment, err := c.CreateSegment(context.Background(), &CreateSegmentInput{
		WorkspaceID: "ws_1", IsShared: true, Label: "Cluster",
		Split: &SegmentSplit{Method: SegmentSplitPercent, Targets: []SegmentSplitTarget{
			{SegmentID: "seg_1", Percent: &first}, {SegmentID: "seg_2", Percent: &second},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/segments")
	wantBody(t, (*got)[0], "label", "Cluster")
	split, _ := (*got)[0].Body["split"].(map[string]any)
	targets, _ := split["targets"].([]any)
	if split["method"] != "percent" || len(targets) != 2 ||
		targets[0].(map[string]any)["segmentId"] != "seg_1" || targets[0].(map[string]any)["percent"] != 60.25 ||
		targets[1].(map[string]any)["segmentId"] != "seg_2" || targets[1].(map[string]any)["percent"] != 39.75 {
		t.Fatalf("sent split %v", (*got)[0].Body["split"])
	}
	if segment.Label != "Cluster" || segment.Split == nil || segment.Split.Method != SegmentSplitPercent ||
		len(segment.Split.Targets) != 2 || segment.Split.Targets[1].SegmentID != "seg_2" ||
		segment.Split.Targets[1].Percent == nil || *segment.Split.Targets[1].Percent != 39.75 {
		t.Fatalf("decoded %+v", segment)
	}
}

// A direct segment sends no label or split, and reads back with an empty label and no split.
func TestDirectSegmentCarriesNoLabelOrSplit(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"seg_1","teamLabel":"Platform","product":"API","costCentre":"CC1",
		"isShared":false,"label":"","split":null,"createdAt":"2026-10-20T00:00:00Z"}`})
	team := "Platform"

	segment, err := c.CreateSegment(context.Background(), &CreateSegmentInput{
		WorkspaceID: "ws_1", TeamLabel: &team, Product: "API", CostCentre: "CC1",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"label", "split", "isShared"} {
		if _, sent := (*got)[0].Body[absent]; sent {
			t.Errorf("a direct segment sent %s: %v", absent, (*got)[0].Body[absent])
		}
	}
	if segment.Label != "" || segment.Split != nil {
		t.Fatalf("decoded %+v", segment)
	}
}

// An update changes a shared segment's label and split only when it names them: an even split
// sends no percent, an empty label is sent to clear it, and naming neither keeps both.
func TestUpdateSharedSegmentLabelAndSplit(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"seg_3","isShared":true,"label":"Cluster","split":{"method":"even","targets":[
			{"segmentId":"seg_1","percent":null},{"segmentId":"seg_2","percent":null}]}}`},
		stub{200, `{"id":"seg_3","isShared":true,"label":"","split":{"method":"even","targets":[
			{"segmentId":"seg_1"},{"segmentId":"seg_2"}]}}`},
		stub{200, `{"id":"seg_3","isShared":true,"label":"","split":{"method":"proportional","targets":[]}}`},
	)

	even, err := c.UpdateSegment(context.Background(), "ws_1", "seg_3", &UpdateSegmentInput{
		Split: &SegmentSplit{Method: SegmentSplitEven, Targets: []SegmentSplitTarget{{SegmentID: "seg_1"}, {SegmentID: "seg_2"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/segments/seg_3")
	if len((*got)[0].Body) != 1 {
		t.Errorf("a split change sent %v, want the split alone", (*got)[0].Body)
	}
	split, _ := (*got)[0].Body["split"].(map[string]any)
	targets, _ := split["targets"].([]any)
	if split["method"] != "even" || len(targets) != 2 {
		t.Fatalf("sent split %v", (*got)[0].Body["split"])
	}
	for _, target := range targets {
		if _, sent := target.(map[string]any)["percent"]; sent {
			t.Errorf("an even split sent a percent: %v", target)
		}
	}
	if even.Split == nil || even.Split.Method != SegmentSplitEven || len(even.Split.Targets) != 2 ||
		even.Split.Targets[0].Percent != nil {
		t.Fatalf("decoded %+v", even)
	}

	unlabelled := ""
	if _, err := c.UpdateSegment(context.Background(), "ws_1", "seg_3", &UpdateSegmentInput{Label: &unlabelled}); err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[1], "label", "")
	if len((*got)[1].Body) != 1 {
		t.Errorf("a label change sent %v, want the label alone", (*got)[1].Body)
	}

	proportional, err := c.UpdateSegment(context.Background(), "ws_1", "seg_3", &UpdateSegmentInput{
		Split: &SegmentSplit{Method: SegmentSplitProportional},
	})
	if err != nil {
		t.Fatal(err)
	}
	split, _ = (*got)[2].Body["split"].(map[string]any)
	if _, sent := split["targets"]; sent || split["method"] != "proportional" {
		t.Errorf("a proportional split sent %v", (*got)[2].Body["split"])
	}
	if proportional.Split == nil || proportional.Split.Method != SegmentSplitProportional || len(proportional.Split.Targets) != 0 {
		t.Fatalf("decoded %+v", proportional)
	}
}
