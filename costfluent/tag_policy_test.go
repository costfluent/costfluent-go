package costfluent

import (
	"context"
	"testing"
)

const tagPolicyJSON = `{"rules":[
	{"provider":null,"accountIds":[],"requiredKeys":["team","environment"]},
	{"provider":"aws","accountIds":["123456789012"],"requiredKeys":["cost-centre"]}],
	"reportId":"rpt_1","updatedAt":"2026-10-20T08:00:00Z"}`

func TestGetTagPolicyNamesTheWorkspaceInThePath(t *testing.T) {
	c, got := testClient(t, stub{200, tagPolicyJSON})

	policy, err := c.GetTagPolicy(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/workspaces/ws_1/tag-policy")
	if len((*got)[0].Query) != 0 {
		t.Errorf("sent a query: %v", (*got)[0].Query)
	}
	if len(policy.Rules) != 2 || policy.Rules[0].Provider != nil || len(policy.Rules[0].RequiredKeys) != 2 {
		t.Fatalf("decoded rules %+v", policy.Rules)
	}
	second := policy.Rules[1]
	if second.Provider == nil || *second.Provider != ProviderKeyAws || second.AccountIDs[0] != "123456789012" ||
		second.RequiredKeys[0] != "cost-centre" {
		t.Fatalf("decoded %+v", second)
	}
	if policy.ReportID == nil || *policy.ReportID != "rpt_1" || policy.UpdatedAt == nil {
		t.Fatalf("decoded %+v", policy)
	}
}

func TestGetTagPolicyWithoutAPolicyDecodesNoRules(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"rules":[],"reportId":null,"updatedAt":null}`})

	policy, err := c.GetTagPolicy(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Rules) != 0 || policy.ReportID != nil || policy.UpdatedAt != nil {
		t.Fatalf("decoded %+v", policy)
	}
}

func TestUpdateTagPolicyIsPutOfTheWholeRuleList(t *testing.T) {
	c, got := testClient(t, stub{200, tagPolicyJSON})

	aws := ProviderKeyAws
	policy, err := c.UpdateTagPolicy(context.Background(), "ws_1", &UpdateTagPolicyInput{Rules: []TagPolicyRule{
		{RequiredKeys: []string{"team", "environment"}},
		{Provider: &aws, AccountIDs: []string{"123456789012"}, RequiredKeys: []string{"cost-centre"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want the PUT alone", len(*got))
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/workspaces/ws_1/tag-policy")
	if _, sent := (*got)[0].Body["workspaceId"]; sent {
		t.Errorf("the workspace was sent in the body as well: %v", (*got)[0].Body)
	}
	rules, _ := (*got)[0].Body["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("sent rules %v", (*got)[0].Body["rules"])
	}
	first, _ := rules[0].(map[string]any)
	if _, sent := first["provider"]; sent {
		t.Errorf("a rule for every provider sent one: %v", first)
	}
	if accounts, _ := first["accountIds"].([]any); accounts == nil || len(accounts) != 0 {
		t.Errorf("a rule for every account sent accountIds %v, want []", first["accountIds"])
	}
	second, _ := rules[1].(map[string]any)
	if second["provider"] != "aws" {
		t.Errorf("sent provider %v", second["provider"])
	}
	if policy.ReportID == nil || *policy.ReportID != "rpt_1" {
		t.Fatalf("decoded %+v", policy)
	}
}

func TestUpdateTagPolicyWithNoRulesSendsAnEmptyList(t *testing.T) {
	c, got := testClient(t, stub{200, `{"rules":[],"reportId":"rpt_1","updatedAt":"2026-10-20T08:00:00Z"}`})

	if _, err := c.UpdateTagPolicy(context.Background(), "ws_1", &UpdateTagPolicyInput{}); err != nil {
		t.Fatal(err)
	}
	rules, ok := (*got)[0].Body["rules"].([]any)
	if !ok || len(rules) != 0 {
		t.Fatalf("sent rules %v, want [] to clear the policy", (*got)[0].Body["rules"])
	}
}

func TestUpdateTagPolicyReturnsTheRefusal(t *testing.T) {
	c, _ := testClient(t, stub{400, `{"title":"Bad Request","status":400,"code":"validation_failed",
		"errors":[{"name":"rules","reason":"A tag policy rule requires one to 20 tag keys."}]}`})

	_, err := c.UpdateTagPolicy(context.Background(), "ws_1", &UpdateTagPolicyInput{Rules: []TagPolicyRule{{}}})
	if !IsBadRequest(err) {
		t.Fatalf("err = %v, want a bad request", err)
	}
}
