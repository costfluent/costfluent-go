package costfluent

import (
	"context"
	"reflect"
	"testing"
)

const workspaceProviderJSON = `{"id":"wpr_1","workspaceId":"ws_1","providerId":"prv_1","providerName":"Payer",
	"providerKey":"aws","isEnabled":true,"accountIds":["111111111111","222222222222"],"addedAt":"2026-10-12T08:00:00Z"}`

func wantAccountIDs(t *testing.T, got exchange, want ...string) {
	t.Helper()
	raw, ok := got.Body["accountIds"].([]any)
	if !ok {
		t.Fatalf("body accountIds = %#v, want a list", got.Body["accountIds"])
	}
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		s, _ := id.(string)
		ids = append(ids, s)
	}
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("body accountIds = %v, want %v", ids, want)
	}
}

func TestListWorkspaceProvidersDecodesAccounts(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[` + workspaceProviderJSON + `,
		{"id":"wpr_2","workspaceId":"ws_1","providerId":"prv_2","providerName":"GCP","isEnabled":false,
		"accountIds":[],"addedAt":"2026-10-12T08:00:00Z"}]}`})

	links, err := c.ListWorkspaceProviders(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/workspace-providers")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	if len(links) != 2 {
		t.Fatalf("decoded %d links, want 2", len(links))
	}
	first := links[0]
	if first.ID != "wpr_1" || first.ProviderID != "prv_1" || first.ProviderName != "Payer" ||
		first.ProviderKey == nil || *first.ProviderKey != ProviderKeyAws || !first.IsEnabled ||
		!reflect.DeepEqual(first.AccountIDs, []string{"111111111111", "222222222222"}) || first.AddedAt.IsZero() {
		t.Errorf("decoded %+v", first)
	}
	if second := links[1]; second.ProviderKey != nil || second.IsEnabled || len(second.AccountIDs) != 0 {
		t.Errorf("decoded %+v", second)
	}
}

func TestCreateWorkspaceProviderSendsAccounts(t *testing.T) {
	c, got := testClient(t, stub{201, workspaceProviderJSON})

	link, err := c.CreateWorkspaceProvider(context.Background(), &CreateWorkspaceProviderInput{
		ProviderID: "prv_1",
		AccountIDs: []string{"111111111111", "222222222222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want 1: the create answers the stored link", len(*got))
	}
	wantRequest(t, (*got)[0], "POST", "/v1/workspace-providers")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "providerId", "prv_1")
	wantAccountIDs(t, (*got)[0], "111111111111", "222222222222")
	if link.ID != "wpr_1" || len(link.AccountIDs) != 2 {
		t.Fatalf("link %+v", link)
	}
}

func TestCreateWorkspaceProviderOmitsUnsetAccounts(t *testing.T) {
	c, got := testClient(t, stub{201, workspaceProviderJSON})

	if _, err := c.CreateWorkspaceProvider(context.Background(),
		&CreateWorkspaceProviderInput{WorkspaceID: "ws_1", ProviderID: "prv_1"}); err != nil {
		t.Fatal(err)
	}
	wantBody(t, (*got)[0], "workspaceId", "ws_1")
	if v, ok := (*got)[0].Body["accountIds"]; ok {
		t.Errorf("body accountIds = %v, want it omitted", v)
	}
}

func TestUpdateWorkspaceProviderDistinguishesUnchangedFromWholeConnection(t *testing.T) {
	c, got := testClient(t,
		stub{200, workspaceProviderJSON},
		stub{200, workspaceProviderJSON},
		stub{200, workspaceProviderJSON},
	)
	ctx := context.Background()
	off := false

	if _, err := c.UpdateWorkspaceProvider(ctx, "wpr_1", &UpdateWorkspaceProviderInput{IsEnabled: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateWorkspaceProvider(ctx, "wpr_1",
		&UpdateWorkspaceProviderInput{WorkspaceID: "ws_1", AccountIDs: []string{"333333333333"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateWorkspaceProvider(ctx, "wpr_1",
		&UpdateWorkspaceProviderInput{WorkspaceID: "ws_1", AccountIDs: []string{}}); err != nil {
		t.Fatal(err)
	}

	wantRequest(t, (*got)[0], "PUT", "/v1/workspace-providers/wpr_1")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "isEnabled", false)
	if v, ok := (*got)[0].Body["accountIds"]; ok {
		t.Errorf("unchanged accounts sent accountIds = %v", v)
	}
	wantAccountIDs(t, (*got)[1], "333333333333")
	if _, ok := (*got)[1].Body["isEnabled"]; ok {
		t.Error("unchanged isEnabled was sent")
	}
	wantAccountIDs(t, (*got)[2])
}

func TestGetAndDeleteWorkspaceProviderAreWorkspaceScoped(t *testing.T) {
	c, got := testClient(t, stub{200, workspaceProviderJSON}, stub{204, ""})
	ctx := context.Background()

	if _, err := c.GetWorkspaceProvider(ctx, "ws_1", "wpr_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteWorkspaceProvider(ctx, "ws_1", "wpr_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/workspace-providers/wpr_1")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	wantRequest(t, (*got)[1], "DELETE", "/v1/workspace-providers/wpr_1")
	wantQuery(t, (*got)[1], "workspaceId", "ws_1")
}
