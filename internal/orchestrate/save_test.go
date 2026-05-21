package orchestrate

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// mockClient implements client.Backend for testing.
type mockClient struct {
	treeResp     *client.TreeResponse
	sidebarCWDs  map[string]string
	pingErr      error
	workspaceSeq int
}

func (m *mockClient) Ping() error { return m.pingErr }

func (m *mockClient) Tree() (*client.TreeResponse, error) {
	return m.treeResp, nil
}

func (m *mockClient) SidebarState(ref string) (*client.SidebarState, error) {
	cwd, ok := m.sidebarCWDs[ref]
	if !ok {
		cwd = "/tmp/unknown"
	}
	return &client.SidebarState{CWD: cwd, FocusedCWD: cwd}, nil
}

func (m *mockClient) ListWorkspaces() ([]client.WorkspaceInfo, error) {
	return nil, nil
}

func (m *mockClient) NewWorkspace(opts client.NewWorkspaceOpts) (string, error) {
	m.workspaceSeq++
	return "workspace:new", nil
}

func (m *mockClient) RenameWorkspace(ref, title string) error  { return nil }
func (m *mockClient) SelectWorkspace(ref string) error         { return nil }
func (m *mockClient) NewSplit(dir, ref string) (string, error) { return "surface:mock", nil }
func (m *mockClient) NewPane(opts client.NewPaneOpts) (string, error) {
	return "surface:new", nil
}
func (m *mockClient) FocusPane(pane, ws string) error         { return nil }
func (m *mockClient) Send(ws, surf, text string) error        { return nil }
func (m *mockClient) PinWorkspace(ref string) error           { return nil }
func (m *mockClient) UnpinWorkspace(ref string) error         { return nil }
func (m *mockClient) CloseWorkspace(ref string) error         { return nil }
func (m *mockClient) DryRunFormatter() client.DryRunFormatter { return client.CmuxDryRun{} }

func TestSave_FromFixture(t *testing.T) {
	// Load tree fixture.
	data, err := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var treeResp client.TreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("test-session", "unit test")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if layout.Name != "test-session" {
		t.Errorf("Name = %q", layout.Name)
	}
	if len(layout.Workspaces) != 3 {
		t.Fatalf("Workspaces = %d, want 3", len(layout.Workspaces))
	}

	// First workspace should have 2 panes (it has 2 in the fixture).
	ws0 := layout.Workspaces[0]
	if ws0.Title != "0 api-server" {
		t.Errorf("ws0.Title = %q", ws0.Title)
	}
	if ws0.CWD != "/home/user/projects/api-server" {
		t.Errorf("ws0.CWD = %q", ws0.CWD)
	}
	if len(ws0.Panes) != 2 {
		t.Errorf("ws0.Panes = %d, want 2", len(ws0.Panes))
	}
	// Second pane should default to split "right".
	if ws0.Panes[1].Split != "right" {
		t.Errorf("ws0.Panes[1].Split = %q, want right", ws0.Panes[1].Split)
	}

	// Verify file was written.
	if !store.Exists("test-session") {
		t.Error("layout file not written")
	}
}

func TestSave_MergePreservesUserEdits(t *testing.T) {
	data, _ := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	var treeResp client.TreeResponse
	_ = json.Unmarshal(data, &treeResp)

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save.
	_, _ = saver.Save("merge-test", "")

	// Manually edit the saved file to add user customizations.
	layout, _ := store.Load("merge-test")
	if len(layout.Workspaces[0].Panes) > 1 {
		layout.Workspaces[0].Panes[1].Split = "down"
		layout.Workspaces[0].Panes[1].Command = "make watch"
	}
	layout.Description = "my custom description"
	_ = store.Save("merge-test", layout)

	// Second save should preserve user edits.
	layout2, err := saver.Save("merge-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if layout2.Description != "my custom description" {
		t.Errorf("Description = %q, want 'my custom description'", layout2.Description)
	}
	if len(layout2.Workspaces[0].Panes) > 1 {
		if layout2.Workspaces[0].Panes[1].Split != "down" {
			t.Errorf("Split = %q, want down (user edit)", layout2.Workspaces[0].Panes[1].Split)
		}
		if layout2.Workspaces[0].Panes[1].Command != "make watch" {
			t.Errorf("Command = %q, want 'make watch' (user edit)", layout2.Workspaces[0].Panes[1].Command)
		}
	}
}

// TestSave_PreservesWorkspaceDescription verifies that a user-edited
// per-workspace description survives a re-save. cmux itself doesn't
// expose descriptions through Tree/SidebarState, so crex keeps them as
// a user-annotated field (aligned with cmux v0.63.2's "editable
// workspace descriptions" feature).
func TestSave_PreservesWorkspaceDescription(t *testing.T) {
	data, _ := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	var treeResp client.TreeResponse
	_ = json.Unmarshal(data, &treeResp)

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save.
	if _, err := saver.Save("desc-test", ""); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Annotate workspace[0] with a description.
	layout, _ := store.Load("desc-test")
	layout.Workspaces[0].Description = "backend API — reads postgres"
	if err := store.Save("desc-test", layout); err != nil {
		t.Fatalf("annotated save: %v", err)
	}

	// Re-save — the live tree doesn't expose descriptions, so the
	// merge must preserve the annotation.
	layout2, err := saver.Save("desc-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if got := layout2.Workspaces[0].Description; got != "backend API — reads postgres" {
		t.Errorf("Workspaces[0].Description = %q, want preserved annotation", got)
	}
	// Other workspaces should remain empty (no bleed).
	for i := 1; i < len(layout2.Workspaces); i++ {
		if got := layout2.Workspaces[i].Description; got != "" {
			t.Errorf("Workspaces[%d].Description = %q, want empty", i, got)
		}
	}
}

// --- Revision tracking tests ---

func TestSave_RevisionIncrements(t *testing.T) {
	data, err := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var treeResp client.TreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save → Revision should be 1.
	layout1, err := saver.Save("rev-test", "")
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if layout1.Revision != 1 {
		t.Errorf("first save: Revision = %d, want 1", layout1.Revision)
	}

	// Second save with same state → Revision should stay 1 (no content change).
	layout2, err := saver.Save("rev-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if layout2.Revision != 1 {
		t.Errorf("second save (same state): Revision = %d, want 1", layout2.Revision)
	}
}

func TestSave_RevisionIncrementsOnChange(t *testing.T) {
	data, err := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var treeResp client.TreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save → Revision = 1.
	layout1, err := saver.Save("rev-change-test", "")
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if layout1.Revision != 1 {
		t.Errorf("first save: Revision = %d, want 1", layout1.Revision)
	}

	// Change CWD for workspace:1 — this will produce different content.
	mc.sidebarCWDs["workspace:1"] = "/home/user/projects/different-path"

	// Second save → Revision should be 2.
	layout2, err := saver.Save("rev-change-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if layout2.Revision != 2 {
		t.Errorf("second save (changed state): Revision = %d, want 2", layout2.Revision)
	}
}

// --- layoutContentChanged tests ---

func baseLayout() *model.Layout {
	return &model.Layout{
		Name:        "test",
		Description: "desc",
		Version:     1,
		Revision:    0,
		SavedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Workspaces: []model.Workspace{
			{
				Title:  "ws1",
				CWD:    "/home/user",
				Pinned: false,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Split: "", Command: "vim", URL: "", Focus: true},
					{Type: "terminal", Split: "right", Command: "make watch", URL: "", Focus: false},
				},
			},
		},
	}
}

func TestLayoutContentChanged_Identical(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	if layoutContentChanged(a, b) {
		t.Error("identical layouts should not be changed")
	}
}

func TestLayoutContentChanged_DifferentWorkspaceCount(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Workspaces = append(b.Workspaces, model.Workspace{
		Title: "ws2",
		CWD:   "/home/user/extra",
		Panes: []model.Pane{{Type: "terminal"}},
	})
	if !layoutContentChanged(a, b) {
		t.Error("different workspace count should report changed")
	}
}

func TestLayoutContentChanged_DifferentCommand(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Workspaces[0].Panes[0].Command = "nvim"
	if !layoutContentChanged(a, b) {
		t.Error("different pane command should report changed")
	}
}

func TestLayoutContentChanged_DifferentPaneCount(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Workspaces[0].Panes = b.Workspaces[0].Panes[:1]
	if !layoutContentChanged(a, b) {
		t.Error("different pane count should report changed")
	}
}

func TestLayoutContentChanged_IgnoresDescription(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Description = "completely different description"
	b.Workspaces[0].Description = "pane-level description"
	if layoutContentChanged(a, b) {
		t.Error("description-only difference should not report changed")
	}
}

func TestLayoutContentChanged_IgnoresSavedAt(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.SavedAt = time.Now().Add(24 * time.Hour)
	b.Revision = 42
	if layoutContentChanged(a, b) {
		t.Error("SavedAt/Revision-only difference should not report changed")
	}
}

func TestLayoutContentChanged_NilHandling(t *testing.T) {
	a := baseLayout()
	if !layoutContentChanged(nil, a) {
		t.Error("nil vs non-nil should report changed")
	}
	if !layoutContentChanged(a, nil) {
		t.Error("non-nil vs nil should report changed")
	}
	if layoutContentChanged(nil, nil) {
		t.Error("nil vs nil should not report changed")
	}
}
