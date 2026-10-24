package costfluent

import (
	"context"
	"testing"
	"time"
)

func TestListAuditLogsSendsItsFiltersAndWalksPages(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{},"data":[{"id":"aud_1","eventName":"team.created","objectType":"team","actorId":"usr_1",
			"actorName":"Ada","targetUserId":null,"metadata":{"teamName":"Platform"},"createdAt":"2026-10-14T09:00:00Z"}]}`},
		stub{200, `{"links":{"next":"/v1/audit-logs?page=2&limit=100"},"data":[{"id":"aud_2","eventName":"user.invited",
			"actorName":"Erin","managedOrganization":"Customer One","createdAt":"2026-10-14T10:00:00Z"}]}`},
		stub{200, `{"links":{},"data":[{"id":"aud_3","eventName":"team.deleted","createdAt":"2026-10-14T11:00:00Z"}]}`},
	)

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 15, 12, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	page, err := c.ListAuditLogs(context.Background(), &ListAuditLogsOptions{
		PageOptions: PageOptions{Page: 1, Limit: 10},
		EventName:   "team.created", ObjectType: "team", ActorID: "usr_1", TargetUserID: "usr_2",
		From: from, To: to,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/audit-logs")
	wantQuery(t, (*got)[0], "eventName", "team.created")
	wantQuery(t, (*got)[0], "objectType", "team")
	wantQuery(t, (*got)[0], "actorId", "usr_1")
	wantQuery(t, (*got)[0], "targetUserId", "usr_2")
	wantQuery(t, (*got)[0], "from", "2026-10-01T00:00:00Z")
	wantQuery(t, (*got)[0], "to", "2026-10-15T10:30:00Z")
	wantQuery(t, (*got)[0], "page", "1")
	wantQuery(t, (*got)[0], "limit", "10")
	entry := page.Data[0]
	if entry.ID != "aud_1" || entry.EventName != "team.created" || entry.ObjectType == nil || *entry.ObjectType != "team" ||
		entry.ActorID == nil || *entry.ActorID != "usr_1" || entry.TargetUserID != nil ||
		entry.Metadata.(map[string]any)["teamName"] != "Platform" || !entry.CreatedAt.Equal(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("decoded %+v", entry)
	}

	all, err := c.ListAllAuditLogs(context.Background(), &ListAuditLogsOptions{EventName: "user.invited"}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[1].ID != "aud_3" {
		t.Fatalf("listed %+v", all)
	}
	if all[0].ManagedOrganization == nil || *all[0].ManagedOrganization != "Customer One" || entry.ManagedOrganization != nil {
		t.Fatalf("managed organization decoded as %v and %v", all[0].ManagedOrganization, entry.ManagedOrganization)
	}
	wantQuery(t, (*got)[1], "eventName", "user.invited")
	wantQuery(t, (*got)[2], "eventName", "user.invited")
	wantQuery(t, (*got)[2], "page", "2")
}

func TestListAuditLogsSendsNoFilterItWasNotGiven(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{},"data":[]}`})

	if _, err := c.ListAuditLogs(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if q := (*got)[0].Query; len(q) != 0 {
		t.Errorf("sent query %v unasked", q)
	}
}
