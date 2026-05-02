# Restore Workspace Picker

**Date**: 2026-05-02
**Status**: Approved

## Problem

When running `crex restore`, the layout picker shows layout names with workspace counts and descriptions, but not the actual workspace titles. Users can't preview what's inside a layout before committing to restore it. Additionally, there's no way to restore a single workspace from a layout — it's all or nothing.

## Design

### Two-step picker with workspace preview

**Step 1 — Layout selection** (enhanced from current behavior)

As the user navigates layouts with ↑/↓, a workspace preview appears below the list showing the workspaces inside the highlighted layout. Each workspace entry shows its index, title (which includes the user's icon/emoji), and pane count.

```
📦 Select a layout to restore
▸ autosave  5 workspaces  autosave
  cmux-test  9 workspaces  testing in cmux
  my-day  6 workspaces  Friday deep work

  Workspaces in "autosave":
    0  🧠 drolosoft brain      (1 pane)
    1  🗾 immich-photo-manager  (2 panes)
    2  🐦‍🔥 crex                 (2 panes)
    3  🧩 drolosoft             (1 pane)
    4  🔔 Soundbox              (1 pane)

  ↑/↓ select · ↵ restore · → pick workspace · / filter · esc cancel
```

**Navigation**:
- ↑/↓: navigate layouts, preview updates dynamically
- Enter: restore all workspaces (current behavior, unchanged)
- →: drill into workspace selection (Step 2)
- /: filter layouts
- Esc: cancel

**Step 2 — Workspace selection**

After drilling in, a second picker shows "All workspaces" as the first option followed by individual workspace entries.

```
📦 Restore from "autosave"
▸ All workspaces (5)
  0  🧠 drolosoft brain
  1  🗾 immich-photo-manager
  2  🐦‍🔥 crex
  3  🧩 drolosoft
  4  🔔 Soundbox

  ↑/↓ select · ↵ restore · / filter · ←/esc back
```

**Navigation**:
- ↑/↓: navigate options
- Enter: restore selection ("All" = full layout, specific = single workspace)
- /: filter workspaces
- Esc or ←: back to layout list (Step 1)

### Data flow

**Current state**: `LayoutMeta` only stores `WorkspaceCount` (an int). Workspace titles are only available by loading the full layout via `store.Load(name)`.

**Change**: Add a `WorkspaceTitles []string` field to `LayoutMeta`. Populate it during `store.List()` since the full layout is already loaded there to extract metadata. This avoids lazy-loading on highlight.

```go
type LayoutMeta struct {
    Name             string
    Description      string
    SavedAt          time.Time
    WorkspaceCount   int
    WorkspaceTitles  []string  // NEW: ordered workspace titles for preview
    WorkspacePanes   []int     // NEW: pane count per workspace
    FilePath         string
}
```

### Single-workspace restore

**Orchestration change**: Add a `WorkspaceFilter` field to `Restorer` (or as a parameter to `Restore`) that, when non-empty, restricts restore to that specific workspace title.

```go
func (r *Restorer) Restore(name string, dryRun bool, mode RestoreMode, workspaceFilter string) (*RestoreResult, error)
```

When `workspaceFilter` is set:
- Only the matching workspace is created
- `RestoreResult.WorkspacesTotal` reflects 1 (the filtered set)
- Replace mode still closes all existing workspaces if chosen (the user asked for this explicitly)

### CLI surface (`cmd/picker.go`, `cmd/restore.go`)

**picker.go**: Keep the single `huh.Select` but add workspace preview via `DescriptionFunc`:

1. `huh.Select` with `DescriptionFunc` bound to `&selected` — when the selection changes, the description re-renders showing the workspace list for that layout.
2. Enter picks the layout and restores all workspaces (current behavior, unchanged).
3. The → drill-in to single-workspace selection is a TUI-only feature (huh Select doesn't support custom key bindings for →).

**restore.go**: No change to restore flow — picker still returns a layout name string.

### TUI surface (`internal/tui/`)

**BrowseModel changes**: Add a two-level browse capability.

New fields on `BrowseModel`:
```go
type BrowseModel struct {
    // ... existing fields ...
    detailItems  []Item    // workspace-level items for drill-in
    inDetail     bool      // true when showing workspace list
    parentCursor int       // remember layout cursor when drilling in
    layoutName   string    // currently selected layout name
}
```

**Key handling changes** (only when `action == "restore"`; other actions like "use" and "toggle" keep current behavior):
- → (KeyRight) in layout list: set `inDetail = true`, populate `detailItems` with "All workspaces" + workspace entries, save `parentCursor`
- Enter in layout list: restore all workspaces (unchanged behavior — select + done)
- ← (KeyLeft) or Esc in detail view: set `inDetail = false`, restore `parentCursor`
- Enter in detail view: set `selected = true`, `done = true`

**Item changes**: Add workspace-level items. Extend `ItemKind`:
```go
const (
    KindLayout    ItemKind = "layout"
    KindTemplate  ItemKind = "template"
    KindWorkspace ItemKind = "workspace"  // NEW
    KindAllWs     ItemKind = "all_ws"     // NEW: "All workspaces" option
)
```

**View changes**: When `inDetail`, render the workspace items instead of layout items. The hint line changes to show ←/esc back.

**shell_exec.go**: When browse completes and the selected item is `KindWorkspace`, pass the workspace title to `execRestore`. When `KindAllWs` or `KindLayout`, pass empty string (restore all).

**execRestore signature change**:
```go
func (m *ShellModel) execRestore(name string, workspaceFilter string) tea.Cmd
```

### Workspace preview data in BrowseModel

The TUI needs workspace details for preview. Two options:

**Chosen approach**: Preload all layouts at browse creation time. `store.List()` already loads every layout — extend `LayoutMeta` with `WorkspaceTitles` and `WorkspacePanes`, then carry this through `Item`. The `Item` struct gets a new `SubItems []Item` field for workspace entries.

This keeps the BrowseModel stateless with respect to the store — all data is available at construction time.

### Settings: `restore_mode`

Add `RestoreMode string` to `Config`:

```go
type Config struct {
    // ... existing fields ...
    RestoreMode string `toml:"restore_mode"` // "ask" (default), "replace", "add"
}
```

**Behavior**:
- `"ask"` (default): show the Replace/Add prompt (current behavior)
- `"replace"`: skip prompt, always replace
- `"add"`: skip prompt, always add

**Applies to**: both CLI and TUI restore, regardless of whether restoring all workspaces or a single one. The `--mode` CLI flag overrides the setting.

**TUI settings integration**: Add `settings restore-mode get/set/list` to the TUI settings command group.

### Files to modify

| File | Change |
|------|--------|
| `internal/model/layout.go` | Add `WorkspaceTitles`, `WorkspacePanes` to `LayoutMeta` |
| `internal/persist/store.go` | Populate new `LayoutMeta` fields in `List()` |
| `internal/config/config.go` | Add `RestoreMode` field |
| `cmd/picker.go` | Two-step huh picker with `DescriptionFunc`, return `PickResult` |
| `cmd/restore.go` | Use `PickResult`, pass workspace filter, respect `restore_mode` setting |
| `internal/orchestrate/restore.go` | Add `workspaceFilter` parameter to `Restore()` |
| `internal/tui/items.go` | Add `KindWorkspace`, `KindAllWs`, `SubItems` field |
| `internal/tui/shell_browse.go` | Two-level navigation (detail view, → / ← keys) |
| `internal/tui/shell_exec.go` | Pass workspace filter to `execRestore`, respect `restore_mode` |
| `internal/tui/shell.go` | Handle new item kinds in browse result dispatch |

### Out of scope

- Multi-select (picking several workspaces but not all) — can be added later
- Workspace preview in `delete` or `show` commands — separate feature
- Workspace-level dry-run — uses the same `--dry-run` flag, just filtered

### Testing

- **Unit tests**: `BrowseModel` two-level navigation (→ drills in, ← goes back, Enter selects)
- **Unit tests**: `Restore` with workspace filter — only matching workspace is created
- **Unit tests**: `LayoutMeta` population with titles and pane counts
- **Unit tests**: `restore_mode` setting respected (skip prompt when set)
- **E2E TUI tests**: New test cases for restore workspace selection flow
