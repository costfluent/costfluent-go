package costfluent

import "testing"

func TestResolveWorkspacePrefersTheExplicitID(t *testing.T) {
	cases := []struct {
		name     string
		client   *Client
		explicit string
		want     string
	}{
		{"explicit over default", NewClient(WithWorkspace("ws_default")), "ws_explicit", "ws_explicit"},
		{"default when empty", NewClient(WithWorkspace("ws_default")), "", "ws_default"},
		{"explicit without default", NewClient(), "ws_explicit", "ws_explicit"},
		{"neither", NewClient(), "", ""},
		{"scoped clone's default", NewClient(WithWorkspace("ws_a")).Workspace("ws_b"), "", "ws_b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.client.ResolveWorkspace(tc.explicit); got != tc.want {
				t.Errorf("ResolveWorkspace(%q) = %q, want %q", tc.explicit, got, tc.want)
			}
		})
	}
}
