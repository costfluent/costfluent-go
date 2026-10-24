package costfluent

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestCreateAppSendsTheConfigAndKeepsTheRedactedAnswer(t *testing.T) {
	c, got := testClient(t, stub{201, `{"id":"app_1","key":"slack","name":"Ops","config":{"webhookUrl":"***"},"isEnabled":true}`})

	app, err := c.CreateApp(context.Background(), &CreateAppInput{
		Key: "slack", Name: "Ops", Config: map[string]any{"webhookUrl": "https://hooks.slack.com/services/T/B/x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/apps")
	wantBody(t, (*got)[0], "key", "slack")
	if app.ID != "app_1" || app.Config["webhookUrl"] != "***" {
		t.Fatalf("decoded %+v", app)
	}
}

// A webhook created without a secret is answered the one Costfluent generated, once; a read never
// carries it.
func TestCreateAppKeepsTheGeneratedSigningSecret(t *testing.T) {
	c, _ := testClient(t,
		stub{201, `{"id":"app_1","key":"webhook","name":"Hook","config":{"url":"https://hooks.example/h","secret":"***"},"isEnabled":true,"signingSecret":"whsec_generated"}`},
		stub{200, `{"id":"app_1","key":"webhook","name":"Hook","config":{"url":"https://hooks.example/h","secret":"***"},"isEnabled":true}`})

	app, err := c.CreateApp(context.Background(), &CreateAppInput{
		Key: "webhook", Name: "Hook", Config: map[string]any{"url": "https://hooks.example/h"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if app.SigningSecret != "whsec_generated" {
		t.Fatalf("signing secret %q, want the generated one", app.SigningSecret)
	}

	read, err := c.GetApp(context.Background(), "app_1")
	if err != nil {
		t.Fatal(err)
	}
	if read.SigningSecret != "" {
		t.Fatalf("a read carried a signing secret: %q", read.SigningSecret)
	}
}

// A create whose first answer is lost is answered by the API's replay of it, which leaves the
// generated secret out; Replayed tells the caller why SigningSecret is empty.
func TestCreateAppMarksAReplayedAnswer(t *testing.T) {
	var posts atomic.Int32
	c := retryingClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if posts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"app_1","key":"webhook","name":"Hook","config":{"url":"https://hooks.example/h","secret":"***"},"isEnabled":true}`)
	})

	app, err := c.CreateApp(context.Background(), &CreateAppInput{
		Key: "webhook", Name: "Hook", Config: map[string]any{"url": "https://hooks.example/h"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !app.Replayed || app.SigningSecret != "" || posts.Load() != 2 {
		t.Fatalf("app %+v after %d posts, want the replay marked and no secret", app, posts.Load())
	}

	first, _ := testClient(t, stub{201, `{"id":"app_2","key":"webhook","name":"Hook","config":{},"isEnabled":true,"signingSecret":"whsec_generated"}`})
	app, err = first.CreateApp(context.Background(), &CreateAppInput{Key: "webhook", Name: "Hook", Config: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if app.Replayed {
		t.Fatal("a first answer was marked as a replay")
	}
}

func TestUpdateAppSendsOnlyWhatIsSet(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"app_1","key":"email","name":"Finance","config":{"recipients":"a@b.example"}}`})

	name := "Finance"
	if _, err := c.UpdateApp(context.Background(), "app_1", &UpdateAppInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/apps/app_1")
	if len((*got)[0].Body) != 1 {
		t.Errorf("unset fields were sent: %v", (*got)[0].Body)
	}
}

// Deleting a channel answers 204 with no body: it reports nothing about the alerts, budgets,
// workspaces and subscriptions it was taken out of.
func TestDeleteAppAcceptsNoContent(t *testing.T) {
	c, got := testClient(t, stub{204, ""})

	if err := c.DeleteApp(context.Background(), "app_1"); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "DELETE", "/v1/apps/app_1")
}
