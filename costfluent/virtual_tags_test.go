package costfluent

import (
	"context"
	"testing"
)

const serviceFilter = `[{"field":"ServiceName","operator":"In","values":["Amazon EC2"]}]`

const virtualTagJSON = `{"id":"vtg_1","key":"team","defaultValue":"unowned",
	"collapsedTagKeys":["team","Team"],
	"rules":[{"value":"platform","filter":"[{\"field\":\"ServiceName\",\"operator\":\"In\",\"values\":[\"Amazon EC2\"]}]"}],
	"createdAt":"2026-09-01T00:00:00Z"}`

func TestCreateVirtualTagReadsItBack(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"vtg_1","key":"team"}`}, stub{200, virtualTagJSON})

	tag, err := c.CreateVirtualTag(context.Background(), &CreateVirtualTagInput{
		Key:              "team",
		CollapsedTagKeys: []string{"team", "Team"},
		Rules:            []VirtualTagRule{{Value: "platform", Filter: serviceFilter}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/virtual-tags")
	wantBody(t, (*got)[0], "key", "team")
	wantRequest(t, (*got)[1], "GET", "/v1/virtual-tags/vtg_1")
	if len(tag.Rules) != 1 || tag.Rules[0].Value != "platform" || tag.Rules[0].Filter != serviceFilter {
		t.Fatalf("decoded rules %+v", tag.Rules)
	}
	if len(tag.CollapsedTagKeys) != 2 || tag.CollapsedTagKeys[1] != "Team" || *tag.DefaultValue != "unowned" {
		t.Fatalf("decoded %+v", tag)
	}
}

func TestUpdateVirtualTagIsPatchWithWorkspaceInBody(t *testing.T) {
	c, got := testClient(t, stub{200, virtualTagJSON})

	value := "none"
	tag, err := c.UpdateVirtualTag(context.Background(), "vtg_1", &UpdateVirtualTagInput{DefaultValue: &value})
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want the PATCH alone", len(*got))
	}
	wantRequest(t, (*got)[0], "PATCH", "/v1/virtual-tags/vtg_1")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "defaultValue", "none")
	if len(tag.Rules) != 1 || tag.Rules[0].Value != "platform" || *tag.DefaultValue != "unowned" {
		t.Fatalf("decoded %+v", tag)
	}
}

func TestListVirtualTagsScopesToTheWorkspace(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"/v1/virtual-tags"},"data":[]}`})

	if _, err := c.ListVirtualTags(context.Background(), "ws_1", nil); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
}
