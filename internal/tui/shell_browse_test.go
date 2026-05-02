package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBrowseModel_NavigateDown(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "a"},
		{Kind: KindLayout, Name: "b"},
		{Kind: KindLayout, Name: "c"},
	}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bm.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", bm.cursor)
	}
}

func TestBrowseModel_NavigateUp(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "a"},
		{Kind: KindLayout, Name: "b"},
	}
	bm := NewBrowseModel(items, "restore")
	bm.cursor = 1

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bm.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", bm.cursor)
	}
}

func TestBrowseModel_ClampTop(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bm.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", bm.cursor)
	}
}

func TestBrowseModel_ClampBottom(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bm.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", bm.cursor)
	}
}

func TestBrowseModel_EnterSelectsItem(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
	}
	bm := NewBrowseModel(items, "restore")
	bm.cursor = 1

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !bm.selected {
		t.Error("expected selected=true after Enter")
	}
	if bm.SelectedItem().Name != "afternoon" {
		t.Errorf("selected item = %q, want %q", bm.SelectedItem().Name, "afternoon")
	}
}

func TestBrowseModel_QuitReturnsToPrompt(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !bm.done {
		t.Error("expected done=true after q")
	}
	if bm.selected {
		t.Error("expected selected=false after q")
	}
}

func TestBrowseModel_LetterExitsBrowse(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !bm.done {
		t.Error("expected done=true after typing a letter")
	}
	if bm.passthrough != 's' {
		t.Errorf("passthrough = %q, want 's'", bm.passthrough)
	}
}

func TestBrowseModel_FilterNarrows(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
		{Kind: KindLayout, Name: "evening"},
	}
	bm := NewBrowseModel(items, "restore")

	// Press / to enter filter
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !bm.filtering {
		t.Error("expected filtering=true after /")
	}

	// Type 'm' — should match "morning"
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if len(bm.visible) != 1 {
		t.Errorf("after filter 'm': visible = %d, want 1", len(bm.visible))
	}
}

func TestBrowseModel_View_ContainsCursor(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning", Description: "test", Workspaces: 2},
	}
	bm := NewBrowseModel(items, "restore")
	view := bm.View()

	if !strings.Contains(view, "▸") {
		t.Error("browse view should contain cursor marker ▸")
	}
	if !strings.Contains(view, "[1]") {
		t.Error("browse view should contain numbered index [1]")
	}
}

func TestBrowseModel_RightDrillsIntoDetail(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws1"},
			{Kind: KindWorkspace, Name: "ws2"},
		}},
	}
	bm := NewBrowseModel(items, "restore")
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !bm.inDetail {
		t.Error("expected inDetail=true after right arrow")
	}
	if len(bm.visible) != 3 {
		t.Fatalf("visible items = %d, want 3 (all + 2 workspaces)", len(bm.visible))
	}
	if bm.visible[0].Kind != KindAllWs {
		t.Errorf("first item should be KindAllWs, got %v", bm.visible[0].Kind)
	}
}

func TestBrowseModel_LeftReturnsFromDetail(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{{Kind: KindWorkspace, Name: "ws1"}}},
		{Kind: KindLayout, Name: "layout-b", SubItems: []Item{{Kind: KindWorkspace, Name: "ws2"}}},
	}
	bm := NewBrowseModel(items, "restore")
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !bm.inDetail {
		t.Error("expected inDetail=true")
	}
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if bm.inDetail {
		t.Error("expected inDetail=false after left arrow")
	}
	if bm.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (restored position)", bm.cursor)
	}
}

func TestBrowseModel_EnterInDetail_Selects(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", Workspaces: 2, SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws1"},
			{Kind: KindWorkspace, Name: "ws2"},
		}},
	}
	bm := NewBrowseModel(items, "restore")
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !bm.selected {
		t.Error("expected selected=true")
	}
	if !bm.done {
		t.Error("expected done=true")
	}
	sel := bm.SelectedItem()
	if sel.Kind != KindWorkspace {
		t.Errorf("selected kind = %v, want KindWorkspace", sel.Kind)
	}
	if sel.Name != "ws1" {
		t.Errorf("selected name = %q, want %q", sel.Name, "ws1")
	}
}

func TestBrowseModel_EnterOnLayout_RestoresAll(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{{Kind: KindWorkspace, Name: "ws1"}}},
	}
	bm := NewBrowseModel(items, "restore")
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if bm.inDetail {
		t.Error("Enter should NOT drill into detail — it should restore all")
	}
	if !bm.selected || !bm.done {
		t.Error("expected selected+done (restore all workspaces)")
	}
}

func TestBrowseModel_EscFromDetail_GoesBack(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{{Kind: KindWorkspace, Name: "ws1"}}},
	}
	bm := NewBrowseModel(items, "restore")
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !bm.inDetail {
		t.Error("expected inDetail=true")
	}
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if bm.inDetail {
		t.Error("Esc should return from detail")
	}
	if bm.done {
		t.Error("Esc from detail should not exit browse entirely")
	}
}
