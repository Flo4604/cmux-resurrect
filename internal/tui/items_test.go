package tui

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestItemsFromLayouts_SubItems(t *testing.T) {
	metas := []model.LayoutMeta{
		{
			Name:            "test-layout",
			Description:     "test",
			WorkspaceCount:  2,
			WorkspaceTitles: []string{"0 dev", "1 docs"},
			WorkspacePanes:  []int{1, 2},
		},
	}
	items := ItemsFromLayouts(metas)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if len(item.SubItems) != 2 {
		t.Fatalf("SubItems len = %d, want 2", len(item.SubItems))
	}
	ws0 := item.SubItems[0]
	if ws0.Kind != KindWorkspace {
		t.Errorf("SubItems[0].Kind = %v, want KindWorkspace", ws0.Kind)
	}
	if ws0.Name != "0 dev" {
		t.Errorf("SubItems[0].Name = %q, want %q", ws0.Name, "0 dev")
	}
	if ws0.Workspaces != 1 {
		t.Errorf("SubItems[0].Workspaces (pane count) = %d, want 1", ws0.Workspaces)
	}
	ws1 := item.SubItems[1]
	if ws1.Name != "1 docs" {
		t.Errorf("SubItems[1].Name = %q, want %q", ws1.Name, "1 docs")
	}
	if ws1.Workspaces != 2 {
		t.Errorf("SubItems[1].Workspaces (pane count) = %d, want 2", ws1.Workspaces)
	}
}

func TestWorkspaceItem_Desc(t *testing.T) {
	item := Item{Kind: KindWorkspace, Name: "0 dev", Workspaces: 2}
	if item.Desc() != "2 panes" {
		t.Errorf("Desc() = %q, want %q", item.Desc(), "2 panes")
	}
	single := Item{Kind: KindWorkspace, Name: "1 docs", Workspaces: 1}
	if single.Desc() != "1 pane" {
		t.Errorf("Desc() = %q, want %q", single.Desc(), "1 pane")
	}
}

func TestAllWsItem_Desc(t *testing.T) {
	item := Item{Kind: KindAllWs, Workspaces: 5}
	if item.Desc() != "" {
		t.Errorf("AllWs Desc() = %q, want empty", item.Desc())
	}
}
