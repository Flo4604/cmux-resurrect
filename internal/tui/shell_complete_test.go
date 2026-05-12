package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func newTestCompletionEngine(t *testing.T) (*completionEngine, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{
		Name: "default", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 🗑️ Trash", CWD: "/tmp/trash", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "⠐ Claude Code", CWD: "/tmp/claude", Index: 1, Panes: []model.Pane{{Type: "terminal"}, {Type: "terminal", Split: "right"}}},
			{Title: "2 tests", CWD: "/tmp/tests", Index: 2, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	_ = store.Save("default", layout)

	ce := &completionEngine{store: store}
	return ce, dir
}

func TestComplete_RestoreFirstArg_ShowsLayouts(t *testing.T) {
	ce, _ := newTestCompletionEngine(t)

	result := ce.Complete("restore ")
	if len(result.items) != 1 {
		t.Fatalf("expected 1 layout, got %d: %v", len(result.items), result.items)
	}
	if result.items[0].value != "default" {
		t.Errorf("expected layout 'default', got %q", result.items[0].value)
	}
}

func TestComplete_RestoreSecondArg_ShowsWorkspaces(t *testing.T) {
	ce, _ := newTestCompletionEngine(t)

	result := ce.Complete("restore default ")
	if len(result.items) != 3 {
		t.Fatalf("expected 3 workspaces, got %d: %v", len(result.items), result.items)
	}

	// Check workspace titles are present.
	titles := make([]string, len(result.items))
	for i, item := range result.items {
		titles[i] = item.value
	}
	found := false
	for _, title := range titles {
		if strings.Contains(title, "Trash") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'Trash' workspace in completions: %v", titles)
	}
}

func TestComplete_RestoreSecondArg_FiltersWorkspaces(t *testing.T) {
	ce, _ := newTestCompletionEngine(t)

	result := ce.Complete("restore default tra")
	if len(result.items) != 1 {
		t.Fatalf("expected 1 matching workspace, got %d: %v", len(result.items), result.items)
	}
	if !strings.Contains(result.items[0].value, "Trash") {
		t.Errorf("expected 'Trash' workspace, got %q", result.items[0].value)
	}
}

func TestComplete_RestoreSecondArg_InvalidLayout(t *testing.T) {
	ce, _ := newTestCompletionEngine(t)

	result := ce.Complete("restore nonexistent ")
	if len(result.items) != 0 {
		t.Errorf("expected 0 completions for invalid layout, got %d", len(result.items))
	}
}

func TestComplete_RestoreSecondArg_PaneCount(t *testing.T) {
	ce, _ := newTestCompletionEngine(t)

	result := ce.Complete("restore default ")
	for _, item := range result.items {
		if strings.Contains(item.value, "Claude Code") {
			if !strings.Contains(item.desc, "2 panes") {
				t.Errorf("Claude Code should have 2 panes, got desc %q", item.desc)
			}
		}
	}
}
