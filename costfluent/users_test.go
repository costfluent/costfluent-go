package costfluent

import (
	"context"
	"testing"
)

func TestInviteUserReadsTheInvitationBack(t *testing.T) {
	c, got := testClient(t,
		stub{201, `{"id":"usr_1","email":"new@mine.example"}`},
		stub{200, `{"id":"usr_1","email":"new@mine.example","role":"viewer","status":"invited","inviteExpiresAt":"2026-10-20T00:00:00Z"}`},
	)

	user, err := c.InviteUser(context.Background(), &InviteUserInput{Email: "new@mine.example", FirstName: "New", LastName: "Person", Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/users")
	wantBody(t, (*got)[0], "role", "viewer")
	wantRequest(t, (*got)[1], "GET", "/v1/users/usr_1")
	if user.Status != "invited" || user.InviteExpiresAt == nil {
		t.Fatalf("decoded %+v", user)
	}
}

func TestListUsersAsksForRemovedOnlyWhenTold(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{},"data":[]}`}, stub{200, `{"links":{},"data":[]}`})

	if _, err := c.ListUsers(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListUsers(context.Background(), &ListUsersOptions{IncludeRemoved: true, PageOptions: PageOptions{Limit: 5}}); err != nil {
		t.Fatal(err)
	}
	if _, sent := (*got)[0].Query["includeRemoved"]; sent {
		t.Errorf("includeRemoved sent unasked: %v", (*got)[0].Query)
	}
	wantQuery(t, (*got)[1], "includeRemoved", "true")
	wantQuery(t, (*got)[1], "limit", "5")
}

func TestListAllUsersWalksEveryPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"next":"/v1/users?page=2&limit=100"},"data":[{"id":"usr_1","email":"a@example.com"}]}`},
		stub{200, `{"links":{},"data":[{"id":"usr_2","email":"b@example.com"}]}`})

	users, err := c.ListAllUsers(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[1].Email != "b@example.com" {
		t.Fatalf("listed %+v", users)
	}
	wantQuery(t, (*got)[1], "page", "2")
	if _, sent := (*got)[0].Query["includeRemoved"]; sent {
		t.Errorf("includeRemoved sent: %v", (*got)[0].Query)
	}
}
