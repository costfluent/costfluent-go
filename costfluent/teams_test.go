package costfluent

import (
	"context"
	"testing"
)

func TestCreateTeamReturnsTheTeamWithoutAReadBack(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"team_1","name":"Platform","isSystemTeam":false,"memberCount":0}`})

	team, err := c.CreateTeam(context.Background(), &CreateTeamInput{Name: "Platform"})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/teams")
	wantBody(t, (*got)[0], "name", "Platform")
	if team.ID != "team_1" || team.Name != "Platform" {
		t.Fatalf("decoded %+v", team)
	}
}

func TestGetTeamDecodesMembersAndAccess(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"id":"team_1","name":"Platform","members":[{"userId":"usr_1","email":"a@b.example",
		"firstName":"A","lastName":"B","role":"editor"}],"workspaceAccess":[{"workspaceId":"wsp_1","workspaceName":"Prod","accessLevel":"canEdit"}]}`})

	team, err := c.GetTeam(context.Background(), "team_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(team.Members) != 1 || team.Members[0].Role != "editor" || team.WorkspaceAccess[0].AccessLevel != "canEdit" {
		t.Fatalf("decoded %+v", team)
	}
}

func TestTeamMembershipAndAccessAddressTheTeamAndTheirTarget(t *testing.T) {
	c, got := testClient(t, stub{204, ``}, stub{204, ``}, stub{204, ``}, stub{204, ``}, stub{204, ``})
	ctx := context.Background()

	if err := c.AddTeamMember(ctx, "team_1", &AddTeamMemberInput{UserID: "usr_1", Role: "viewer"}); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateTeamMember(ctx, "team_1", "usr_1", &UpdateTeamMemberInput{Role: "editor"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveTeamMember(ctx, "team_1", "usr_1"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetTeamWorkspaceAccess(ctx, "team_1", "wsp_1", &SetTeamWorkspaceAccessInput{AccessLevel: "canView"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveTeamWorkspaceAccess(ctx, "team_1", "wsp_1"); err != nil {
		t.Fatal(err)
	}

	wantRequest(t, (*got)[0], "POST", "/v1/teams/team_1/members")
	wantBody(t, (*got)[0], "userId", "usr_1")
	wantRequest(t, (*got)[1], "PUT", "/v1/teams/team_1/members/usr_1")
	wantBody(t, (*got)[1], "role", "editor")
	wantRequest(t, (*got)[2], "DELETE", "/v1/teams/team_1/members/usr_1")
	wantRequest(t, (*got)[3], "PUT", "/v1/teams/team_1/workspaces/wsp_1")
	wantBody(t, (*got)[3], "accessLevel", "canView")
	wantRequest(t, (*got)[4], "DELETE", "/v1/teams/team_1/workspaces/wsp_1")
}

func TestSetTeamDefaultDashboard(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"workspaceId":"wsp_1","workspaceName":"Prod","accessLevel":"canView","defaultDashboardId":"dsh_1"}`},
		stub{200, `{"workspaceId":"wsp_1","workspaceName":"Prod","accessLevel":"canView","defaultDashboardId":null}`},
	)
	ctx := context.Background()

	dashboard := "dsh_1"
	set, err := c.SetTeamDefaultDashboard(ctx, "team_1", "wsp_1", &dashboard)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/teams/team_1/workspaces/wsp_1/default-dashboard")
	wantBody(t, (*got)[0], "dashboardId", "dsh_1")
	if len((*got)[0].Query) != 0 {
		t.Errorf("query = %v, want none: the team and the workspace are in the path", (*got)[0].Query)
	}
	if set.WorkspaceID != "wsp_1" || set.AccessLevel != "canView" || set.DefaultDashboardID == nil || *set.DefaultDashboardID != "dsh_1" {
		t.Fatalf("decoded %+v", set)
	}

	cleared, err := c.SetTeamDefaultDashboard(ctx, "team_1", "wsp_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[1], "PUT", "/v1/teams/team_1/workspaces/wsp_1/default-dashboard")
	if value, sent := (*got)[1].Body["dashboardId"]; !sent || value != nil {
		t.Fatalf("clear sent %v, want an explicit null dashboardId", (*got)[1].Body)
	}
	if cleared.DefaultDashboardID != nil {
		t.Fatalf("cleared grant %+v", cleared)
	}
}

func TestTeamWorkspaceAccessListsCarryTheDefaultDashboard(t *testing.T) {
	c, _ := testClient(t,
		stub{200, `{"data":[{"workspaceId":"wsp_1","workspaceName":"Prod","accessLevel":"canEdit","defaultDashboardId":"dsh_1"},
			{"workspaceId":"wsp_2","workspaceName":"Dev","accessLevel":"canView","defaultDashboardId":null}],"links":{}}`},
	)

	grants, err := c.ListAllTeamWorkspaceAccess(context.Background(), "team_1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 || grants[0].DefaultDashboardID == nil || *grants[0].DefaultDashboardID != "dsh_1" || grants[1].DefaultDashboardID != nil {
		t.Fatalf("decoded %+v", grants)
	}
}
