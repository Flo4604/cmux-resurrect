# Restore Workspace Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a two-step restore picker (layout → workspace) with workspace preview, single-workspace restore, and a `restore_mode` setting.

**Architecture:** Extend `LayoutMeta` with workspace titles/pane counts (data layer), add `workspaceFilter` parameter to `Restorer.Restore` (orchestration), add two-level navigation to `BrowseModel` (TUI), replace single huh Select with two-step flow (CLI), and add `restore_mode` to `Config` (settings).

**Tech Stack:** Go, Bubble Tea (TUI), charmbracelet/huh (CLI picker), TOML config

---

### Task 1: Extend LayoutMeta with workspace details

**Files:**
- Modify: `internal/model/layout.go:37-44`
- Modify: `internal/persist/store.go:139-145`
- Modify: `internal/persist/store_test.go:67-109`

- [ ] **Step 1: Write the failing test**

Add to `internal/persist/store_test.go` after the existing `TestFileStore_List` function:

```go
func TestFileStore_List_WorkspaceDetails(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{
		Name:    "details-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "1 docs", CWD: "/tmp", Panes: []model.Pane{{Type: "terminal"}, {Type: "terminal", Split: "right"}}},
		},
	}
	if err := store.Save("details-test", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 layout, got %d", len(metas))
	}

	m := metas[0]
	if len(m.WorkspaceTitles) != 2 {
		t.Fatalf("WorkspaceTitles len = %d, want 2", len(m.WorkspaceTitles))
	}
	if m.WorkspaceTitles[0] != "0 dev" {
		t.Errorf("WorkspaceTitles[0] = %q, want %q", m.WorkspaceTitles[0], "0 dev")
	}
	if m.WorkspaceTitles[1] != "1 docs" {
		t.Errorf("WorkspaceTitles[1] = %q, want %q", m.WorkspaceTitles[1], "1 docs")
	}
	if len(m.WorkspacePanes) != 2 {
		t.Fatalf("WorkspacePanes len = %d, want 2", len(m.WorkspacePanes))
	}
	if m.WorkspacePanes[0] != 1 {
		t.Errorf("WorkspacePanes[0] = %d, want 1", m.WorkspacePanes[0])
	}
	if m.WorkspacePanes[1] != 2 {
		t.Errorf("WorkspacePanes[1] = %d, want 2", m.WorkspacePanes[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/persist/ -run TestFileStore_List_WorkspaceDetails -v`
Expected: FAIL — `WorkspaceTitles` and `WorkspacePanes` don't exist on `LayoutMeta`.

- [ ] **Step 3: Add fields to LayoutMeta**

In `internal/model/layout.go`, add two fields to `LayoutMeta` (after line 42):

```go
type LayoutMeta struct {
	Name            string
	Description     string
	SavedAt         time.Time
	WorkspaceCount  int
	WorkspaceTitles []string // ordered workspace titles for preview
	WorkspacePanes  []int    // pane count per workspace
	FilePath        string
}
```

- [ ] **Step 4: Populate fields in store.List()**

In `internal/persist/store.go`, inside the `List()` loop (lines 139-145), populate the new fields:

```go
titles := make([]string, len(layout.Workspaces))
panes := make([]int, len(layout.Workspaces))
for i, ws := range layout.Workspaces {
	titles[i] = ws.Title
	panes[i] = len(ws.Panes)
}
metas = append(metas, model.LayoutMeta{
	Name:            layout.Name,
	Description:     layout.Description,
	SavedAt:         layout.SavedAt,
	WorkspaceCount:  len(layout.Workspaces),
	WorkspaceTitles: titles,
	WorkspacePanes:  panes,
	FilePath:        s.Path(name),
})
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/persist/ -run TestFileStore_List_WorkspaceDetails -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass (no breakage from adding fields to struct)

- [ ] **Step 7: Commit**

```bash
git add internal/model/layout.go internal/persist/store.go internal/persist/store_test.go
git commit -m "feat: add WorkspaceTitles and WorkspacePanes to LayoutMeta"
```

---

### Task 2: Add workspace filter to Restorer.Restore

**Files:**
- Modify: `internal/orchestrate/restore.go:42`
- Modify: `internal/orchestrate/restore_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/orchestrate/restore_test.go`:

```go
func TestRestore_WorkspaceFilter(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "filter-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 docs", CWD: "/tmp/docs", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 tests", CWD: "/tmp/tests", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("filter-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	// Filter to single workspace.
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

	// Verify only docs workspace commands are present.
	hasNewWorkspace := false
	for _, cmd := range result.Commands {
		if containsStr(cmd, "1 docs") {
			hasNewWorkspace = true
		}
		if containsStr(cmd, "0 dev") || containsStr(cmd, "2 tests") {
			t.Errorf("filtered workspace should not appear: %s", cmd)
		}
	}
	if !hasNewWorkspace {
		t.Error("expected commands for '1 docs' workspace")
	}
}

func TestRestore_EmptyFilter_RestoresAll(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "all-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestore_WorkspaceFilter -v`
Expected: FAIL — `Restore` doesn't accept 4th argument.

- [ ] **Step 3: Add workspaceFilter parameter**

In `internal/orchestrate/restore.go`, change the `Restore` signature (line 42):

```go
func (r *Restorer) Restore(name string, dryRun bool, mode RestoreMode, workspaceFilter string) (*RestoreResult, error) {
```

After loading the layout and before the result struct (after line 46), add workspace filtering:

```go
	// Filter to a single workspace if requested.
	if workspaceFilter != "" {
		var filtered []model.Workspace
		for _, ws := range layout.Workspaces {
			if ws.Title == workspaceFilter {
				filtered = append(filtered, ws)
				break
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("workspace %q not found in layout %q", workspaceFilter, name)
		}
		layout.Workspaces = filtered
	}
```

- [ ] **Step 4: Fix all callers**

Every call to `Restore` now needs the 4th argument. Update these files:

**`cmd/restore.go:121`** — add `""` as workspace filter:
```go
result, err := restorer.Restore(name, restoreDryRun, mode, "")
```

**`internal/tui/shell_exec.go:164`** — add `""` as workspace filter:
```go
result, err := restorer.Restore(name, false, orchestrate.RestoreModeAdd, "")
```

**`internal/orchestrate/restore_test.go`** — update existing test calls:

Line 47 (`TestRestore_DryRun`):
```go
result, err := restorer.Restore("dry-test", true, RestoreModeAdd, "")
```

Line 111 (`TestRestore_LayoutNotFound`):
```go
_, err := restorer.Restore("nonexistent", false, RestoreModeAdd, "")
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/orchestrate/ -v`
Expected: All pass including new filter tests.

- [ ] **Step 6: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrate/restore.go internal/orchestrate/restore_test.go cmd/restore.go internal/tui/shell_exec.go
git commit -m "feat: add workspaceFilter parameter to Restorer.Restore"
```

---

### Task 3: Add restore_mode to Config

**Files:**
- Modify: `internal/config/config.go:12-19`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestLoad_RestoreMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `restore_mode = "add"`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RestoreMode != "add" {
		t.Errorf("RestoreMode = %q, want %q", cfg.RestoreMode, "add")
	}
}

func TestDefaultConfig_RestoreMode(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RestoreMode != "" {
		t.Errorf("RestoreMode default = %q, want empty (ask)", cfg.RestoreMode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_RestoreMode -v`
Expected: FAIL — `RestoreMode` field doesn't exist.

- [ ] **Step 3: Add RestoreMode to Config**

In `internal/config/config.go`, add the field to `Config` (after line 18, `BannerStyle`):

```go
type Config struct {
	LayoutsDir       string        `toml:"layouts_dir"`
	WorkspaceFile    string        `toml:"workspace_file"`
	WatchInterval    time.Duration `toml:"-"`
	WatchIntervalStr string        `toml:"watch_interval"`
	MaxAutosaves     int           `toml:"max_autosaves"`
	BannerStyle      string        `toml:"banner_style"`
	RestoreMode      string        `toml:"restore_mode"` // "ask" (default when empty), "replace", "add"
}
```

No change needed to `DefaultConfig()` — empty string means "ask".

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add restore_mode setting to Config"
```

---

### Task 4: Wire restore_mode into CLI restore

**Files:**
- Modify: `cmd/restore.go:91-107`

- [ ] **Step 1: Update restore mode resolution**

In `cmd/restore.go`, change the mode resolution block (lines 91-107) to check the config setting before falling through to the interactive prompt:

```go
	// Determine restore mode.
	var mode orchestrate.RestoreMode
	switch {
	case restoreMode == "replace":
		mode = orchestrate.RestoreModeReplace
	case restoreMode == "add":
		mode = orchestrate.RestoreModeAdd
	case restoreDryRun:
		mode = orchestrate.RestoreModeAdd
	default:
		// Check config setting before interactive prompt.
		switch cfg.RestoreMode {
		case "replace":
			mode = orchestrate.RestoreModeReplace
		case "add":
			mode = orchestrate.RestoreModeAdd
		default:
			mode, err = askRestoreMode()
			if err != nil {
				return err
			}
		}
	}
```

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/restore.go
git commit -m "feat: respect restore_mode config setting in CLI restore"
```

---

### Task 5: Add KindWorkspace and KindAllWs to Item, SubItems field

**Files:**
- Modify: `internal/tui/items.go`

- [ ] **Step 1: Write the failing test**

Add to a new file `internal/tui/items_test.go`:

```go
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
	desc := item.Desc()
	if desc != "2 panes" {
		t.Errorf("Desc() = %q, want %q", desc, "2 panes")
	}

	single := Item{Kind: KindWorkspace, Name: "1 docs", Workspaces: 1}
	if single.Desc() != "1 pane" {
		t.Errorf("Desc() = %q, want %q", single.Desc(), "1 pane")
	}
}

func TestAllWsItem_Desc(t *testing.T) {
	item := Item{Kind: KindAllWs, Workspaces: 5}
	desc := item.Desc()
	if desc != "" {
		t.Errorf("AllWs Desc() = %q, want empty", desc)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestItemsFromLayouts_SubItems -v`
Expected: FAIL — `KindWorkspace`, `KindAllWs`, `SubItems` don't exist.

- [ ] **Step 3: Extend Item types and struct**

In `internal/tui/items.go`, extend the constants (lines 10-15):

```go
type ItemKind int

const (
	KindLayout ItemKind = iota
	KindTemplate
	KindWorkspace // individual workspace within a layout
	KindAllWs     // "All workspaces" option
)
```

Add `SubItems` field to `Item` (after line 25, `Category`):

```go
type Item struct {
	Kind        ItemKind
	Name        string
	Description string
	Workspaces  int
	Icon        string
	Category    string
	SubItems    []Item // workspace-level items for layout drill-in
}
```

- [ ] **Step 4: Update Desc() for workspace kinds**

In `internal/tui/items.go`, update the `Desc()` method (lines 44-53) to handle new kinds:

```go
func (i Item) Desc() string {
	switch i.Kind {
	case KindLayout:
		if i.Description != "" {
			return fmt.Sprintf("%d workspaces — %s", i.Workspaces, i.Description)
		}
		return fmt.Sprintf("%d workspaces", i.Workspaces)
	case KindWorkspace:
		if i.Workspaces == 1 {
			return "1 pane"
		}
		return fmt.Sprintf("%d panes", i.Workspaces)
	case KindAllWs:
		return ""
	default:
		return i.Description
	}
}
```

- [ ] **Step 5: Update ItemsFromLayouts to populate SubItems**

In `internal/tui/items.go`, update `ItemsFromLayouts` (lines 55-67):

```go
func ItemsFromLayouts(metas []model.LayoutMeta) []Item {
	items := make([]Item, len(metas))
	for idx, m := range metas {
		var subItems []Item
		for i, title := range m.WorkspaceTitles {
			panes := 0
			if i < len(m.WorkspacePanes) {
				panes = m.WorkspacePanes[i]
			}
			subItems = append(subItems, Item{
				Kind:       KindWorkspace,
				Name:       title,
				Workspaces: panes,
			})
		}
		items[idx] = Item{
			Kind:        KindLayout,
			Name:        m.Name,
			Description: m.Description,
			Workspaces:  m.WorkspaceCount,
			SubItems:    subItems,
		}
	}
	return items
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run "TestItemsFromLayouts_SubItems|TestWorkspaceItem_Desc|TestAllWsItem_Desc" -v`
Expected: All pass.

- [ ] **Step 7: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/items.go internal/tui/items_test.go
git commit -m "feat: add KindWorkspace, KindAllWs, and SubItems to Item"
```

---

### Task 6: Add two-level navigation to BrowseModel

**Files:**
- Modify: `internal/tui/shell_browse.go`
- Modify: `internal/tui/shell_browse_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/shell_browse_test.go`:

```go
func TestBrowseModel_RightDrillsIntoDetail(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws1"},
			{Kind: KindWorkspace, Name: "ws2"},
		}},
	}
	bm := NewBrowseModel(items, "restore")

	// Press right arrow to drill in.
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !bm.inDetail {
		t.Error("expected inDetail=true after right arrow")
	}
	// First item should be "All workspaces".
	if len(bm.visible) != 3 {
		t.Fatalf("visible items = %d, want 3 (all + 2 workspaces)", len(bm.visible))
	}
	if bm.visible[0].Kind != KindAllWs {
		t.Errorf("first item should be KindAllWs, got %v", bm.visible[0].Kind)
	}
}

func TestBrowseModel_LeftReturnsFromDetail(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws1"},
		}},
		{Kind: KindLayout, Name: "layout-b", SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws2"},
		}},
	}
	bm := NewBrowseModel(items, "restore")

	// Move to layout-b, then drill in, then go back.
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !bm.inDetail {
		t.Error("expected inDetail=true")
	}
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if bm.inDetail {
		t.Error("expected inDetail=false after left arrow")
	}
	// Cursor should be restored to layout-b (index 1).
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

	// Drill in.
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	// Move to ws1 (index 1, after "All workspaces").
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Select it.
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
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws1"},
		}},
	}
	bm := NewBrowseModel(items, "restore")

	// Enter on a layout in restore mode should select+done (restore all), NOT drill in.
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
		{Kind: KindLayout, Name: "layout-a", SubItems: []Item{
			{Kind: KindWorkspace, Name: "ws1"},
		}},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run "TestBrowseModel_Right|TestBrowseModel_Left|TestBrowseModel_EnterInDetail|TestBrowseModel_EnterOnLayout|TestBrowseModel_NonRestoreAction|TestBrowseModel_EscFromDetail" -v`
Expected: FAIL — `inDetail` field doesn't exist.

- [ ] **Step 3: Add two-level fields to BrowseModel**

In `internal/tui/shell_browse.go`, add fields to `BrowseModel` (after line 20, `passthrough`):

```go
type BrowseModel struct {
	items       []Item
	visible     []Item
	cursor      int
	action      string
	filtering   bool
	filterText  string
	selected    bool
	done        bool
	passthrough rune

	// Two-level drill-in state (restore action only).
	inDetail     bool   // true when showing workspace list
	parentItems  []Item // saved layout items when drilling in
	parentCursor int    // saved cursor position
	layoutName   string // name of the layout being drilled into
}
```

- [ ] **Step 4: Add drillIn and drillOut methods**

Add after the `NewBrowseModel` function:

```go
// drillIn enters workspace detail view for the currently highlighted layout.
func (bm *BrowseModel) drillIn() {
	if bm.cursor >= len(bm.visible) {
		return
	}
	item := bm.visible[bm.cursor]
	if len(item.SubItems) == 0 {
		return
	}

	bm.parentItems = bm.visible
	bm.parentCursor = bm.cursor
	bm.layoutName = item.Name

	// Build detail list: "All workspaces" + individual workspaces.
	detail := make([]Item, 0, len(item.SubItems)+1)
	detail = append(detail, Item{
		Kind:       KindAllWs,
		Name:       fmt.Sprintf("All workspaces (%d)", len(item.SubItems)),
		Workspaces: len(item.SubItems),
	})
	detail = append(detail, item.SubItems...)

	bm.visible = detail
	bm.items = detail
	bm.cursor = 0
	bm.inDetail = true
	bm.filtering = false
	bm.filterText = ""
}

// drillOut returns to the layout list from workspace detail view.
func (bm *BrowseModel) drillOut() {
	bm.visible = bm.parentItems
	bm.items = bm.parentItems
	bm.cursor = bm.parentCursor
	bm.inDetail = false
	bm.layoutName = ""
	bm.parentItems = nil
	bm.filtering = false
	bm.filterText = ""
}
```

- [ ] **Step 5: Update Update() for two-level navigation**

Replace the `Update` method (lines 43-91) with:

```go
func (bm BrowseModel) Update(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	if bm.filtering {
		return bm.updateFilter(msg)
	}

	// Detail view: different key handling.
	if bm.inDetail {
		return bm.updateDetail(msg)
	}

	switch msg.Type {
	case tea.KeyDown:
		if bm.cursor < len(bm.visible)-1 {
			bm.cursor++
		}
		return bm, nil

	case tea.KeyUp:
		if bm.cursor > 0 {
			bm.cursor--
		}
		return bm, nil

	case tea.KeyRight:
		// Drill into workspace detail (restore action only).
		if bm.action == "restore" && len(bm.visible) > 0 {
			bm.drillIn()
		}
		return bm, nil

	case tea.KeyEnter:
		// Enter always selects (restore all for layouts, direct select for other actions).
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil

	case tea.KeyEsc:
		bm.done = true
		return bm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			switch r {
			case 'q':
				bm.done = true
				return bm, nil
			case '/':
				bm.filtering = true
				bm.filterText = ""
				return bm, nil
			default:
				bm.done = true
				bm.passthrough = r
				return bm, nil
			}
		}
	}
	return bm, nil
}

func (bm BrowseModel) updateDetail(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyDown:
		if bm.cursor < len(bm.visible)-1 {
			bm.cursor++
		}
		return bm, nil

	case tea.KeyUp:
		if bm.cursor > 0 {
			bm.cursor--
		}
		return bm, nil

	case tea.KeyLeft:
		bm.drillOut()
		return bm, nil

	case tea.KeyEsc:
		bm.drillOut()
		return bm, nil

	case tea.KeyEnter:
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			switch r {
			case 'q':
				bm.drillOut()
				return bm, nil
			case '/':
				bm.filtering = true
				bm.filterText = ""
				return bm, nil
			default:
				bm.drillOut()
				bm.passthrough = r
				return bm, nil
			}
		}
	}
	return bm, nil
}
```

- [ ] **Step 6: Update View() for detail mode**

Replace the `View` method (lines 144-176) with:

```go
func (bm BrowseModel) View() string {
	var b strings.Builder

	for i, item := range bm.visible {
		idx := shellDimStyle.Render(fmt.Sprintf("[%d]", i+1))
		name := item.Title()
		desc := item.Desc()

		if i == bm.cursor {
			fmt.Fprintf(&b, "  %s %s %s", shellCursorStyle.Render("▸"), idx, shellSuccessStyle.Render(name))
		} else {
			fmt.Fprintf(&b, "    %s %s", idx, name)
		}
		if desc != "" {
			b.WriteString("  ")
			b.WriteString(shellDimStyle.Render(desc))
		}
		b.WriteString("\n")
	}

	// Workspace preview (layout list only, not in detail).
	if !bm.inDetail && bm.action == "restore" && bm.cursor < len(bm.visible) {
		item := bm.visible[bm.cursor]
		if len(item.SubItems) > 0 {
			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s\n", shellDimStyle.Render(fmt.Sprintf("Workspaces in %q:", item.Name)))
			for _, ws := range item.SubItems {
				desc := ws.Desc()
				if desc != "" {
					fmt.Fprintf(&b, "    %s  %s\n", ws.Name, shellDimStyle.Render("("+desc+")"))
				} else {
					fmt.Fprintf(&b, "    %s\n", ws.Name)
				}
			}
		}
	}

	if bm.filtering {
		fmt.Fprintf(&b, "  / %s", bm.filterText)
		b.WriteString(shellDimStyle.Render("▌"))
		b.WriteString("\n")
	} else if bm.inDetail {
		hint := "  ↑/↓ select · ↵ restore · / filter · ←/esc back"
		b.WriteString(shellDimStyle.Render(hint))
		b.WriteString("\n")
	} else {
		hint := fmt.Sprintf("  ↑/↓ select · ↵ %s · / filter · q back", bm.action)
		if bm.action == "restore" {
			hint = "  ↑/↓ select · ↵ restore · → pick workspace · / filter · esc cancel"
		}
		b.WriteString(shellDimStyle.Render(hint))
		b.WriteString("\n")
	}

	return b.String()
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run "TestBrowseModel" -v`
Expected: All pass (both new and existing).

- [ ] **Step 8: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/shell_browse.go internal/tui/shell_browse_test.go
git commit -m "feat: two-level browse navigation for restore workspace picker"
```

---

### Task 7: Wire TUI browse result to single-workspace restore

**Files:**
- Modify: `internal/tui/shell.go:397-407`
- Modify: `internal/tui/shell_exec.go:146-167`

- [ ] **Step 1: Update handleBrowseSelection**

In `internal/tui/shell.go`, update `handleBrowseSelection` (lines 397-407):

```go
func (m *ShellModel) handleBrowseSelection(item Item) (tea.Model, tea.Cmd) {
	switch m.browse.action {
	case "restore":
		layoutName := m.browse.layoutName
		if layoutName == "" {
			layoutName = item.Name
		}
		wsFilter := ""
		if item.Kind == KindWorkspace {
			wsFilter = item.Name
		}
		return m, m.execRestore(layoutName, wsFilter)
	case "use":
		m.execUse(item.Name)
	case "toggle":
		m.execBpToggle(item.Name)
	}
	return m, nil
}
```

- [ ] **Step 2: Update execRestore signature and caller**

In `internal/tui/shell_exec.go`, update `execRestore` (lines 145-167):

```go
func (m *ShellModel) execRestore(name string, workspaceFilter string) tea.Cmd {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return nil
	}

	if workspaceFilter != "" {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q from %q…", workspaceFilter, name)))
	} else {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q…", name)))
	}
	m.output.WriteString("\n")

	cl := m.client
	store := m.store
	filter := workspaceFilter
	return func() tea.Msg {
		restorer := &orchestrate.Restorer{
			Client: cl,
			Store:  store,
		}
		result, err := restorer.Restore(name, false, orchestrate.RestoreModeAdd, filter)
		return restoreResultMsg{result: result, err: err}
	}
}
```

- [ ] **Step 3: Update the direct command path**

In `internal/tui/shell.go`, update the `case "restore":` in the command dispatch (around line 462-465):

```go
	case "restore":
		if resolved, ok := m.requireResolved(args, "restore <name|#>"); ok {
			return m, m.execRestore(resolved, "")
		}
```

- [ ] **Step 4: Build and run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/shell.go internal/tui/shell_exec.go
git commit -m "feat: wire workspace selection to single-workspace restore in TUI"
```

---

### Task 8: CLI picker with workspace preview

**Files:**
- Modify: `cmd/picker.go`

- [ ] **Step 1: Add workspace preview to pickLayout**

Replace the entire `cmd/picker.go`. The picker keeps its current behavior (Enter = select layout, restore all) but adds a `DescriptionFunc` that shows workspace names for the highlighted layout:

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// pickLayout shows an interactive selector and lets the user pick a layout.
// A workspace preview is shown below the list for the highlighted layout.
func pickLayout(metas []model.LayoutMeta) (string, error) {
	// Build workspace preview map.
	previewByName := make(map[string]string)
	for _, m := range metas {
		var sb strings.Builder
		for i, title := range m.WorkspaceTitles {
			panes := 0
			if i < len(m.WorkspacePanes) {
				panes = m.WorkspacePanes[i]
			}
			paneLabel := fmt.Sprintf("%d panes", panes)
			if panes == 1 {
				paneLabel = "1 pane"
			}
			fmt.Fprintf(&sb, "    %s  (%s)\n", title, paneLabel)
		}
		previewByName[m.Name] = strings.TrimRight(sb.String(), "\n")
	}

	options := make([]huh.Option[string], len(metas))
	for i, m := range metas {
		label := fmt.Sprintf("%s  %d workspaces", m.Name, m.WorkspaceCount)
		if m.Description != "" {
			desc := m.Description
			if len(desc) > 35 {
				desc = desc[:32] + "..."
			}
			label += "  " + desc
		}
		options[i] = huh.NewOption(label, m.Name)
	}

	var selected string
	err := huh.NewSelect[string]().
		Title("📦 Select a layout to restore").
		Options(options...).
		Value(&selected).
		DescriptionFunc(func() string {
			if preview, ok := previewByName[selected]; ok {
				return "\n  Workspaces:\n" + preview
			}
			return ""
		}, &selected).
		Run()
	if err != nil {
		return "", err
	}

	return selected, nil
}
```

Note: The return type stays `(string, error)` — no change to `cmd/restore.go` callers needed since Task 2 already updated the `Restore` call to pass `""` as workspace filter.

- [ ] **Step 2: Build and run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/picker.go
git commit -m "feat: CLI picker with workspace preview via DescriptionFunc"
```

---

### Task 9: Add settings restore-mode commands to TUI

**Files:**
- Modify: `internal/tui/shell.go`
- Modify: `internal/tui/shell_complete.go`
- Modify: `internal/tui/shell_help.go`
- Modify: `internal/tui/shell_help_test.go`

- [ ] **Step 1: Add restoreMode field to ShellModel**

In `internal/tui/shell.go`, add a field after `bannerStyle` (line 49):

```go
	bannerStyle string // current banner style for "banner get"
	restoreMode string // current restore mode for "restore-mode get"
```

Add a setter method after `SetBannerStyle`:

```go
// SetRestoreMode sets the current restore mode (for "restore-mode get").
func (m *ShellModel) SetRestoreMode(mode string) { m.restoreMode = mode }
```

- [ ] **Step 2: Add command handlers**

In `internal/tui/shell.go`, add cases before the `default:` in the command dispatch (before line 565):

```go
	case "settings restore-mode set":
		if len(args) == 0 {
			m.writeError("Usage: settings restore-mode set <ask|replace|add>")
			break
		}
		mode := strings.ToLower(args[0])
		switch mode {
		case "ask", "replace", "add":
			m.restoreMode = mode
			if m.OnSettingChanged != nil {
				m.OnSettingChanged("restore_mode", mode)
			}
			m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Restore mode set to %q", mode)))
			m.output.WriteString("\n\n")
		default:
			m.writeError(fmt.Sprintf("Invalid mode %q — use ask, replace, or add", mode))
		}

	case "settings restore-mode get":
		mode := m.restoreMode
		if mode == "" {
			mode = "ask"
		}
		fmt.Fprintf(m.output, "  Current restore mode: %s\n\n",
			shellSuccessStyle.Render(mode))

	case "settings restore-mode list":
		m.output.WriteString("  Available restore modes:\n")
		fmt.Fprintf(m.output, "    %s  prompt for replace/add each time (default)\n", shellSuccessStyle.Render("ask    "))
		fmt.Fprintf(m.output, "    %s  always replace existing workspaces\n", shellSuccessStyle.Render("replace"))
		fmt.Fprintf(m.output, "    %s  always add alongside existing workspaces\n", shellSuccessStyle.Render("add    "))
		m.output.WriteString("\n")
```

- [ ] **Step 3: Add OnSettingChanged callback**

In `internal/tui/shell.go`, add the callback type and field. After `BannerCycle BannerCycleFn` (line 48):

```go
	BannerCycle      BannerCycleFn
	OnSettingChanged func(key, value string) // called when a setting changes in TUI
```

- [ ] **Step 4: Update completion engine**

In `internal/tui/shell_complete.go`, add the new command group. Find the `"settings banner": true` line (around line 101) and add:

```go
	"settings banner":       true,
	"settings restore-mode": true,
```

Find the `"settings banner"` sub-group definition (around line 106) and add:

```go
	"settings restore-mode": {
		{Value: "set", Icon: "🔧", Desc: "Set restore mode"},
		{Value: "get", Icon: "🔍", Desc: "Show current mode"},
		{Value: "list", Icon: "📋", Desc: "List available modes"},
	},
```

Find the `"settings banner set"` completion case (around line 296) and add after it:

```go
	case "settings restore-mode set":
		return completionResult{
			values: []string{"settings restore-mode set ask", "settings restore-mode set replace", "settings restore-mode set add"},
			items: []completionItem{
				{Value: "ask", Icon: "❓", Desc: "Prompt each time (default)"},
				{Value: "replace", Icon: "🔄", Desc: "Always replace"},
				{Value: "add", Icon: "➕", Desc: "Always add"},
			},
		}
```

Also add `"settings restore-mode"` to the `hasLayoutArg` set (around line 116) — actually no, this doesn't take layout args. Skip this.

And add `"settings restore-mode"` to the `threeWordGroups` map in `internal/tui/shell_commands.go` (line 28):

```go
	threeWordGroups := map[string]bool{"settings banner": true, "settings restore-mode": true}
```

- [ ] **Step 5: Update help table**

In `internal/tui/shell_help.go`, add entries after the banner help lines (after line 39):

```go
	{"🔧", "settings restore-mode set", "<ask|replace|add>", func(b client.DetectedBackend) string { return "Set restore mode" }, "Settings"},
	{"🔍", "settings restore-mode get", "", func(b client.DetectedBackend) string { return "Show current mode" }, "Settings"},
	{"📋", "settings restore-mode list", "", func(b client.DetectedBackend) string { return "List available modes" }, "Settings"},
```

- [ ] **Step 6: Update help test**

In `internal/tui/shell_help_test.go`, add the new commands to the expected list (line 22). Find the existing list and add:

```go
	"settings restore-mode set", "settings restore-mode get", "settings restore-mode list",
```

- [ ] **Step 7: Build and run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/shell.go internal/tui/shell_complete.go internal/tui/shell_help.go internal/tui/shell_help_test.go internal/tui/shell_commands.go
git commit -m "feat: add settings restore-mode commands to TUI"
```

---

### Task 10: Wire OnSettingChanged in cmd/tui.go

**Files:**
- Modify: `cmd/tui.go`

- [ ] **Step 1: Read current tui.go**

Read `cmd/tui.go` to find where ShellModel is configured.

- [ ] **Step 2: Add OnSettingChanged handler**

After the line that sets `m.SetBannerStyle(style)`, add:

```go
	m.SetRestoreMode(cfg.RestoreMode)
	m.OnSettingChanged = func(key, value string) {
		switch key {
		case "restore_mode":
			cfg.RestoreMode = value
		}
		_ = config.Save(config.DefaultConfigPath(), cfg)
	}
```

- [ ] **Step 3: Build and run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/tui.go
git commit -m "feat: persist restore_mode setting changes from TUI to config file"
```

---

### Task 11: Wire restore_mode into TUI restore

**Files:**
- Modify: `internal/tui/shell_exec.go:146-167`

- [ ] **Step 1: Update execRestore to use restoreMode**

In `internal/tui/shell_exec.go`, update `execRestore` to accept and use the restore mode:

```go
func (m *ShellModel) execRestore(name string, workspaceFilter string) tea.Cmd {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return nil
	}

	if workspaceFilter != "" {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q from %q…", workspaceFilter, name)))
	} else {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q…", name)))
	}
	m.output.WriteString("\n")

	// Determine restore mode from setting.
	mode := orchestrate.RestoreModeAdd
	switch m.restoreMode {
	case "replace":
		mode = orchestrate.RestoreModeReplace
	case "add":
		mode = orchestrate.RestoreModeAdd
	}

	cl := m.client
	store := m.store
	filter := workspaceFilter
	restoreMode := mode
	return func() tea.Msg {
		restorer := &orchestrate.Restorer{
			Client: cl,
			Store:  store,
		}
		result, err := restorer.Restore(name, false, restoreMode, filter)
		return restoreResultMsg{result: result, err: err}
	}
}
```

- [ ] **Step 2: Build and run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/shell_exec.go
git commit -m "feat: use restore_mode setting in TUI restore"
```

---

### Task 12: Manual verification

- [ ] **Step 1: Test CLI picker**

Run: `go build -o /tmp/crex ./cmd/crex && /tmp/crex restore`

Verify:
- Layout list shows with workspace preview updating as you arrow through
- Enter drills into workspace list with "All workspaces" at top
- Selecting "All" restores all workspaces
- Selecting a specific workspace restores only that one
- Replace/Add prompt appears (or is skipped if config set)

- [ ] **Step 2: Test TUI**

Run: `go build -o /tmp/crex ./cmd/crex && /tmp/crex`

In the TUI shell, type `restore` and verify:
- Layout list shows with workspace preview
- → and Enter drill into workspace list
- ← and Esc go back to layout list
- Enter on "All workspaces" restores all
- Enter on a specific workspace restores just that one

Test settings:
- `settings restore-mode list` — shows three options
- `settings restore-mode set add` — sets to add
- `settings restore-mode get` — shows "add"

- [ ] **Step 3: Run full test suite one final time**

Run: `go test ./... -count=1`
Expected: All pass.

- [ ] **Step 4: Commit any fixes from manual testing**

If any issues found during manual testing, fix and commit.
