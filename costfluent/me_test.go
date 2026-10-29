package costfluent

import (
	"context"
	"slices"
	"testing"
)

func TestGetMeDecodesAWorkspaceToken(t *testing.T) {
	c, got := testClient(t, stub{200, `{"organizationId":"org_1","workspaceId":"ws_1","defaultWorkspaceId":"ws_1",
		"tokenType":"workspace","tokenName":"assistant","capabilities":["viewCosts","viewBudgets"]}`})

	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/me")
	if me.OrganizationID != "org_1" || me.WorkspaceID == nil || *me.WorkspaceID != "ws_1" ||
		me.DefaultWorkspaceID == nil || *me.DefaultWorkspaceID != "ws_1" || me.TokenType != TokenTypeWorkspace || me.TokenName != "assistant" {
		t.Fatalf("decoded %+v", me)
	}
	if !me.Can(CapabilityViewCosts) || !me.Can(CapabilityViewBudgets) || me.Can(CapabilityViewAnomalies) {
		t.Errorf("Can disagrees with capabilities %v", me.Capabilities)
	}
}

func TestGetMeLeavesAnOrganizationTokenWithoutAWorkspace(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"organizationId":"org_1","workspaceId":null,"defaultWorkspaceId":"ws_2",
		"tokenType":"organization","tokenName":"ci","capabilities":[]}`})

	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.WorkspaceID != nil || me.TokenType != TokenTypeOrganization || me.Can(CapabilityViewCosts) {
		t.Fatalf("decoded %+v", me)
	}
	if me.DefaultWorkspaceID == nil || *me.DefaultWorkspaceID != "ws_2" {
		t.Fatalf("default workspace = %v, want ws_2", me.DefaultWorkspaceID)
	}
}

func TestGetMeLeavesTheDefaultNilWhenTheOrganizationHasNoWorkspace(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"organizationId":"org_1","workspaceId":null,"defaultWorkspaceId":null,
		"tokenType":"organization","tokenName":"ci","capabilities":[]}`})

	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.DefaultWorkspaceID != nil {
		t.Fatalf("default workspace = %q, want nil", *me.DefaultWorkspaceID)
	}
}

func TestGetMeKeepsACapabilityTheSDKDoesNotList(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"organizationId":"org_1","tokenType":"organization","tokenName":"ci",
		"capabilities":["viewCosts","someLaterCapability"]}`})

	me, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !me.Can(CapabilityViewCosts) || !me.Can(Capability("someLaterCapability")) {
		t.Fatalf("capabilities %v", me.Capabilities)
	}
}

func TestGetMeReturnsTheAPIError(t *testing.T) {
	c, _ := testClient(t, stub{401, `{"title":"Authentication Error","status":401,"code":"unauthorized","retryable":false}`})

	if _, err := c.GetMe(context.Background()); !IsUnauthorized(err) {
		t.Fatalf("err = %v, want unauthorized", err)
	}
}

func TestMissingNamesWhatTheTokenLacks(t *testing.T) {
	me := &Me{Capabilities: []Capability{CapabilityViewCosts, CapabilityCreateTeam}}

	missing := me.Missing(CapabilityCreateTeam, CapabilityManageTeam, CapabilityViewCosts, CapabilityDeleteTeam, CapabilityManageTeam)
	if want := []Capability{CapabilityManageTeam, CapabilityDeleteTeam}; !slices.Equal(missing, want) {
		t.Errorf("Missing = %v, want %v, in the order asked and each once", missing, want)
	}
	if missing := me.Missing(CapabilityViewCosts); missing != nil {
		t.Errorf("Missing = %v for a capability the token carries", missing)
	}
	if missing := me.Missing(); missing != nil {
		t.Errorf("Missing = %v when nothing is needed", missing)
	}
}

func TestTokenTypesMatchTheContract(t *testing.T) {
	wantContractValues(t, "ApiTokenType", []string{TokenTypeOrganization, TokenTypeWorkspace, TokenTypeCluster})
}
