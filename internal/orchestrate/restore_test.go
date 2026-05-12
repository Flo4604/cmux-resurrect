package orchestrate

import (
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func TestRestore_DryRun(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "dry-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title:  "0 dev",
				CWD:    "/tmp/project",
				Pinned: true,
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right", Command: "go test ./..."},
				},
			},
			{
				Title:  "1 docs",
				CWD:    "/tmp/docs",
				Pinned: false,
				Index:  1,
				Panes: []model.Pane{
					{Type: "terminal", Command: "claude"},
				},
			},
		},
	}
	_ = store.Save("dry-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("dry-test", true, RestoreModeAdd, "")
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}

	if !result.DryRun {
		t.Error("DryRun should be true")
	}
	if result.WorkspacesTotal != 2 {
		t.Errorf("WorkspacesTotal = %d, want 2", result.WorkspacesTotal)
	}
	if result.WorkspacesOK != 2 {
		t.Errorf("WorkspacesOK = %d, want 2", result.WorkspacesOK)
	}
	if len(result.Commands) == 0 {
		t.Error("expected dry-run commands")
	}

	// Verify expected commands.
	hasNewWorkspace := false
	hasRename := false
	hasSplit := false
	hasSend := false
	hasSelect := false
	for _, cmd := range result.Commands {
		if containsStr(cmd, "new-workspace") {
			hasNewWorkspace = true
		}
		if containsStr(cmd, "rename-workspace") {
			hasRename = true
		}
		if containsStr(cmd, "new-split") {
			hasSplit = true
		}
		if containsStr(cmd, "send") {
			hasSend = true
		}
		if containsStr(cmd, "select-workspace") {
			hasSelect = true
		}
	}
	if !hasNewWorkspace {
		t.Error("missing new-workspace command")
	}
	if !hasRename {
		t.Error("missing rename-workspace command")
	}
	if !hasSplit {
		t.Error("missing new-split command")
	}
	if !hasSend {
		t.Error("missing send command")
	}
	if !hasSelect {
		t.Error("missing select-workspace command")
	}
}

func TestRestore_LayoutNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	mc := &mockClient{}

	restorer := &Restorer{Client: mc, Store: store}
	_, err := restorer.Restore("nonexistent", false, RestoreModeAdd, "")
	if err == nil {
		t.Error("expected error for nonexistent layout")
	}
}

func TestRestore_WorkspaceFilter(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "filter-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 docs", CWD: "/tmp/docs", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 tests", CWD: "/tmp/tests", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("filter-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("filter-test", true, RestoreModeAdd, "1 docs")
	if err != nil {
		t.Fatalf("restore with filter: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
	if result.WorkspacesOK != 1 {
		t.Errorf("WorkspacesOK = %d, want 1", result.WorkspacesOK)
	}
	hasTarget := false
	for _, cmd := range result.Commands {
		if containsStr(cmd, "1 docs") {
			hasTarget = true
		}
		if containsStr(cmd, "0 dev") || containsStr(cmd, "2 tests") {
			t.Errorf("filtered workspace should not appear: %s", cmd)
		}
	}
	if !hasTarget {
		t.Error("expected commands for '1 docs' workspace")
	}
}

func TestRestore_EmptyFilter_RestoresAll(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "all-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 docs", CWD: "/tmp", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("all-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("all-test", true, RestoreModeAdd, "")
	if err != nil {
		t.Fatalf("restore all: %v", err)
	}
	if result.WorkspacesTotal != 2 {
		t.Errorf("WorkspacesTotal = %d, want 2", result.WorkspacesTotal)
	}
}

func TestRestore_BrowserPane_DryRun(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "browser-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title:  "0 dev",
				CWD:    "/tmp/project",
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "browser", Split: "right", URL: "https://localhost:3000"},
				},
			},
		},
	}
	_ = store.Save("browser-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("browser-test", true, RestoreModeAdd, "")
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}

	hasBrowserCmd := false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "browser") && strings.Contains(cmd, "https://localhost:3000") {
			hasBrowserCmd = true
		}
	}
	if !hasBrowserCmd {
		t.Errorf("expected browser pane command with URL, got commands: %v", result.Commands)
	}
}

func TestRestore_MixedTerminalBrowserPanes_DryRun(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "mixed-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title:  "0 fullstack",
				CWD:    "/tmp/project",
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true, Command: "npm run dev"},
					{Type: "browser", Split: "right", URL: "https://localhost:3000"},
					{Type: "terminal", Split: "down", Command: "npm run test"},
				},
			},
		},
	}
	_ = store.Save("mixed-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("mixed-test", true, RestoreModeAdd, "")
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}

	hasSend := false
	hasBrowser := false
	hasSplit := false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "npm run dev") {
			hasSend = true
		}
		if strings.Contains(cmd, "browser") && strings.Contains(cmd, "https://localhost:3000") {
			hasBrowser = true
		}
		if strings.Contains(cmd, "new-split") && strings.Contains(cmd, "down") {
			hasSplit = true
		}
	}
	if !hasSend {
		t.Error("missing terminal command 'npm run dev'")
	}
	if !hasBrowser {
		t.Error("missing browser pane command with URL")
	}
	if !hasSplit {
		t.Error("missing terminal split 'down' for third pane")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
