package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// devWorkspace returns a minimal workspace used by all EnsureWorkspace tests.
func devWorkspace() model.Workspace {
	return model.Workspace{
		Title: "dev",
		CWD:   "/tmp/dev",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
		},
	}
}

// TestEnsureWorkspace_CreateOnly_NoExisting verifies CreateOnly creates when no workspace exists.
func TestEnsureWorkspace_CreateOnly_NoExisting(t *testing.T) {
	mc := newSyncMockClient(nil, "", "")
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	result, err := r.EnsureWorkspace(devWorkspace(), CreateOnly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "created" {
		t.Errorf("Action = %q, want %q", result.Action, "created")
	}
	if result.Existed {
		t.Error("Existed should be false")
	}
	if mc.createdCount != 1 {
		t.Errorf("createdCount = %d, want 1", mc.createdCount)
	}
}

// TestEnsureWorkspace_CreateOnly_Existing verifies CreateOnly errors when workspace already exists.
func TestEnsureWorkspace_CreateOnly_Existing(t *testing.T) {
	mc := newSyncMockClient(
		[]client.WorkspaceInfo{{Ref: "workspace:1", Title: "dev"}},
		"", "",
	)
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	_, err := r.EnsureWorkspace(devWorkspace(), CreateOnly)
	if err == nil {
		t.Fatal("expected error for existing workspace with CreateOnly")
	}
}

// TestEnsureWorkspace_CreateOrReuse_Existing verifies CreateOrReuse reuses existing workspace.
func TestEnsureWorkspace_CreateOrReuse_Existing(t *testing.T) {
	mc := newSyncMockClient(
		[]client.WorkspaceInfo{{Ref: "workspace:1", Title: "dev"}},
		"", "",
	)
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	result, err := r.EnsureWorkspace(devWorkspace(), CreateOrReuse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "reused" {
		t.Errorf("Action = %q, want %q", result.Action, "reused")
	}
	if result.Ref != "workspace:1" {
		t.Errorf("Ref = %q, want %q", result.Ref, "workspace:1")
	}
	if !result.Existed {
		t.Error("Existed should be true")
	}
	if mc.createdCount != 0 {
		t.Errorf("createdCount = %d, want 0 (should reuse, not create)", mc.createdCount)
	}
}

// TestEnsureWorkspace_CreateOrReuse_NotExisting verifies CreateOrReuse creates when workspace is absent.
func TestEnsureWorkspace_CreateOrReuse_NotExisting(t *testing.T) {
	mc := newSyncMockClient(nil, "", "")
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	result, err := r.EnsureWorkspace(devWorkspace(), CreateOrReuse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "created" {
		t.Errorf("Action = %q, want %q", result.Action, "created")
	}
	if result.Existed {
		t.Error("Existed should be false")
	}
	if mc.createdCount != 1 {
		t.Errorf("createdCount = %d, want 1", mc.createdCount)
	}
}

// TestEnsureWorkspace_ReuseOnly_Existing verifies ReuseOnly returns existing workspace ref.
func TestEnsureWorkspace_ReuseOnly_Existing(t *testing.T) {
	mc := newSyncMockClient(
		[]client.WorkspaceInfo{{Ref: "workspace:1", Title: "dev"}},
		"", "",
	)
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	result, err := r.EnsureWorkspace(devWorkspace(), ReuseOnly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "reused" {
		t.Errorf("Action = %q, want %q", result.Action, "reused")
	}
	if result.Ref != "workspace:1" {
		t.Errorf("Ref = %q, want %q", result.Ref, "workspace:1")
	}
}

// TestEnsureWorkspace_ReuseOnly_NotExisting verifies ReuseOnly errors when workspace is absent.
func TestEnsureWorkspace_ReuseOnly_NotExisting(t *testing.T) {
	mc := newSyncMockClient(nil, "", "")
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	_, err := r.EnsureWorkspace(devWorkspace(), ReuseOnly)
	if err == nil {
		t.Fatal("expected error for missing workspace with ReuseOnly")
	}
}

// TestEnsureWorkspace_ForceRecreate_Existing verifies ForceRecreate closes and recreates.
func TestEnsureWorkspace_ForceRecreate_Existing(t *testing.T) {
	mc := newSyncMockClient(
		[]client.WorkspaceInfo{{Ref: "workspace:1", Title: "dev"}},
		"", "",
	)
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	result, err := r.EnsureWorkspace(devWorkspace(), ForceRecreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "recreated" {
		t.Errorf("Action = %q, want %q", result.Action, "recreated")
	}
	if !result.Existed {
		t.Error("Existed should be true")
	}
	if !mc.closedRefs["workspace:1"] {
		t.Error("workspace:1 should have been closed")
	}
	if mc.createdCount != 1 {
		t.Errorf("createdCount = %d, want 1", mc.createdCount)
	}
}

// TestEnsureWorkspace_ForceRecreate_NotExisting verifies ForceRecreate creates when absent.
func TestEnsureWorkspace_ForceRecreate_NotExisting(t *testing.T) {
	mc := newSyncMockClient(nil, "", "")
	store, _ := persist.NewFileStore(t.TempDir())
	r := &Restorer{Client: mc, Store: store}

	result, err := r.EnsureWorkspace(devWorkspace(), ForceRecreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "created" {
		t.Errorf("Action = %q, want %q", result.Action, "created")
	}
	if result.Existed {
		t.Error("Existed should be false")
	}
	if mc.createdCount != 1 {
		t.Errorf("createdCount = %d, want 1", mc.createdCount)
	}
}
