package costfluent

import (
	"context"
	"testing"
)

func TestListFoldersDecodesTree(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"s"},"data":[{"id":"fld_1","title":"Teams","reportCount":1,
		"children":[{"id":"fld_2","title":"Platform","parentId":"fld_1","reportCount":3}]}]}`})

	folders, err := c.ListFolders(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/folders")
	wantQuery(t, (*got)[0], "workspaceId", "ws_1")
	child := folders[0].Children[0]
	if child.ID != "fld_2" || *child.ParentID != "fld_1" || child.ReportCount != 3 {
		t.Fatalf("decoded %+v", folders)
	}
}

func TestCreateFolderSendsWorkspaceInBody(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"fld_1","title":"Teams"}`},
		stub{200, `{"id":"fld_1","title":"Teams","reportCount":0}`},
	)

	folder, err := c.CreateFolder(context.Background(), &CreateFolderInput{Title: "Teams"})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/folders")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "title", "Teams")
	wantRequest(t, (*got)[1], "GET", "/v1/folders/fld_1")
	wantQuery(t, (*got)[1], "workspaceId", "ws_default")
	if folder.ID != "fld_1" {
		t.Fatalf("folder %+v", folder)
	}
}

func TestUpdateFolderIsPut(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"fld_1","title":"Renamed"}`})

	title := "Renamed"
	if _, err := c.UpdateFolder(context.Background(), "fld_1", &UpdateFolderInput{WorkspaceID: "ws_1", Title: &title}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/folders/fld_1")
	wantBody(t, (*got)[0], "workspaceId", "ws_1")
}

func TestSetFolderSavedFilterAttachesAndDetaches(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"id":"fld_1","title":"Teams","reportCount":0,"savedFilterId":"flt_1"}`},
		stub{200, `{"id":"fld_1","title":"Teams","reportCount":0}`},
	)

	filter := "flt_1"
	folder, err := c.SetFolderSavedFilter(context.Background(), "", "fld_1", &filter)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/folders/fld_1/saved-filter")
	wantBody(t, (*got)[0], "workspaceId", "ws_default")
	wantBody(t, (*got)[0], "savedFilterId", "flt_1")
	if folder.SavedFilterID == nil || *folder.SavedFilterID != "flt_1" {
		t.Fatalf("folder %+v", folder)
	}

	detached, err := c.SetFolderSavedFilter(context.Background(), "ws_1", "fld_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value, sent := (*got)[1].Body["savedFilterId"]; !sent || value != nil {
		t.Fatalf("detach sent %v, want an explicit null savedFilterId", (*got)[1].Body)
	}
	if detached.SavedFilterID != nil {
		t.Fatalf("detached folder %+v", detached)
	}
}
