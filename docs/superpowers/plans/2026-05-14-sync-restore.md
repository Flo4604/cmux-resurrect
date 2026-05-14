# Sync-Based Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the destructive replace/add restore modes with a sync-based approach that keeps existing matching workspaces intact, only creates missing ones, and handles pinned workspaces correctly. Add a restore-mode prompt to the TUI (parity with CLI).

**Architecture:** The `Restorer.Restore` method gets a new sync algorithm: snapshot existing workspace titles, compare against layout, skip matching titles (both modes), create missing ones, and only in replace mode close extras (unpinning first). The TUI gets a new `modeRestoreAsk` state that shows a Replace/Add picker before executing the restore. The CLI picker already has this; `crex restore <name>` (no `--mode`, no config) also gets the prompt.

**Tech Stack:** Go, Bubble Tea, cmux CLI, Ghostty AppleScript

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/client/client.go` | Modify | Add `UnpinWorkspace` to Backend interface |
| `internal/client/cli.go` | Modify | Implement `UnpinWorkspace` for cmux |
| `internal/client/ghostty.go` | Modify | Implement `UnpinWorkspace` (no-op, Ghostty has no pinning) |
| `internal/client/dryrun.go` | Modify | Add `FmtUnpinWorkspace` to both formatters |
| `internal/orchestrate/restore.go` | Modify | Rewrite sync algorithm, unpin-before-close |
| `internal/orchestrate/restore_test.go` | Modify | Tests for sync logic, pinned handling, skip-matching |
| `internal/orchestrate/save_test.go` | Modify | Add `UnpinWorkspace` to mockClient |
| `internal/tui/shell.go` | Modify | Add `modeRestoreAsk`, update dispatch, update View |
| `internal/tui/shell_exec.go` | Modify | Rewrite `execRestore` to prompt when mode is "ask"/empty |
| `internal/tui/shell_test.go` | Modify | Tests for TUI restore-mode prompt |
| `cmd/restore.go` | Modify | Add prompt for `crex restore <name>` when no mode specified |
| `cmd/picker.go` | No change | Already has mode picker (used for `crex restore` with no args) |

---

### Task 1: Add `UnpinWorkspace` to Backend Interface

**Files:**
- Modify: `internal/client/client.go:41-44`
- Modify: `internal/client/cli.go:127-130`
- Modify: `internal/client/ghostty.go:71-73`
- Modify: `internal/client/dryrun.go`
- Modify: `internal/orchestrate/save_test.go:52` (mockClient)

- [ ] **Step 1: Write the failing test**

Add to `internal/orchestrate/save_test.go` — the mockClient needs the new method or tests won't compile:

```go
func (m *mockClient) UnpinWorkspace(ref string) error { return nil }
```

Run: `go build ./...`
Expected: Build fails because `UnpinWorkspace` is not in the `Backend` interface yet.

- [ ] **Step 2: Add to the Backend interface**

In `internal/client/client.go`, add after `PinWorkspace`:

```go
	// UnpinWorkspace unpins a workspace so it can be closed.
	UnpinWorkspace(ref string) error
```

- [ ] **Step 3: Implement for cmux CLI client**

In `internal/client/cli.go`, add after `PinWorkspace`:

```go
func (c *CLIClient) UnpinWorkspace(ref string) error {
	_, err := c.run("workspace-action", "--action", "unpin", "--workspace", ref)
	return err
}
```

- [ ] **Step 4: Implement for Ghostty client**

In `internal/client/ghostty.go`, add after `PinWorkspace`:

```go
func (g *GhosttyClient) UnpinWorkspace(ref string) error {
	return nil // Ghostty does not support pinning tabs
}
```

- [ ] **Step 5: Add dry-run formatter**

In `internal/client/dryrun.go`, add `FmtUnpinWorkspace` to both formatters:

```go
func (CmuxDryRun) FmtUnpinWorkspace(ref string) string {
	return fmt.Sprintf("cmux workspace-action --action unpin --workspace %s", ref)
}
```

```go
func (GhosttyDryRun) FmtUnpinWorkspace(ref string) string {
	return "" // Ghostty has no pinning
}
```

Add `FmtUnpinWorkspace(ref string) string` to the `DryRunFormatter` interface.

- [ ] **Step 6: Verify build**

Run: `go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 7: Run existing tests**

Run: `go test ./internal/client/ ./internal/orchestrate/ ./internal/tui/ -v`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/client/client.go internal/client/cli.go internal/client/ghostty.go internal/client/dryrun.go internal/orchestrate/save_test.go
git commit -m "$(cat <<'EOF'
feat(client): add UnpinWorkspace to Backend interface

Needed for sync-based restore to unpin workspaces before closing them.
cmux uses workspace-action --action unpin; Ghostty is a no-op.
EOF
)"
```

---

### Task 2: Rewrite Restore to Sync Algorithm

**Files:**
- Modify: `internal/orchestrate/restore.go:44-194`
- Test: `internal/orchestrate/restore_test.go`

The core logic change. Both modes now share a sync foundation:
1. Snapshot existing workspaces (title + ref + pinned)
2. For each layout workspace: if title already exists, **skip it** (both modes)
3. For each existing workspace NOT in the layout: in **replace** mode, unpin + close it; in **add** mode, leave it
4. Create only the missing workspaces

- [ ] **Step 1: Write failing test for sync-skip in replace mode**

Add to `internal/orchestrate/restore_test.go`:

```go
func TestRestore_Replace_SkipsMatchingTitles(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "sync-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "docs", CWD: "/tmp/docs", Index: 1, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "new-tab", CWD: "/tmp/new", Index: 2, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	_ = store.Save("sync-test", layout)

	// Mock has "dev" and "stale" existing — "dev" matches layout, "stale" does not.
	mc := &syncMockClient{
		existingWorkspaces: []client.WorkspaceInfo{
			{Ref: "workspace:1", Title: "dev"},
			{Ref: "workspace:2", Title: "stale"},
		},
		callerRef:   "workspace:1",
		callerTitle: "dev",
	}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("sync-test", false, RestoreModeReplace, "")
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// "dev" should be skipped (already exists and matches layout).
	// "stale" should be closed (exists but not in layout).
	// "docs" and "new-tab" should be created.
	if mc.closedRefs["workspace:1"] {
		t.Error("should NOT close 'dev' — it matches layout")
	}
	if !mc.closedRefs["workspace:2"] {
		t.Error("should close 'stale' — not in layout")
	}
	if result.WorkspacesOK != 2 {
		t.Errorf("WorkspacesOK = %d, want 2 (docs + new-tab)", result.WorkspacesOK)
	}
	if result.WorkspacesClosed != 1 {
		t.Errorf("WorkspacesClosed = %d, want 1 (stale)", result.WorkspacesClosed)
	}
}
```

Also add the `syncMockClient`:

```go
type syncMockClient struct {
	existingWorkspaces []client.WorkspaceInfo
	callerRef          string
	callerTitle        string
	closedRefs         map[string]bool
	unpinnedRefs       map[string]bool
	createdCount       int
}

func (m *syncMockClient) Ping() error { return nil }
func (m *syncMockClient) Tree() (*client.TreeResponse, error) {
	var workspaces []client.TreeWorkspace
	for _, ws := range m.existingWorkspaces {
		workspaces = append(workspaces, client.TreeWorkspace{
			Ref:   ws.Ref,
			Title: ws.Title,
		})
	}
	resp := &client.TreeResponse{
		Windows: []client.TreeWindow{{Workspaces: workspaces}},
	}
	if m.callerRef != "" {
		resp.Caller = &client.CallerInfo{WorkspaceRef: m.callerRef}
	}
	return resp, nil
}
func (m *syncMockClient) SidebarState(ref string) (*client.SidebarState, error) {
	return &client.SidebarState{CWD: "/tmp"}, nil
}
func (m *syncMockClient) ListWorkspaces() ([]client.WorkspaceInfo, error) {
	return m.existingWorkspaces, nil
}
func (m *syncMockClient) NewWorkspace(opts client.NewWorkspaceOpts) (string, error) {
	m.createdCount++
	return fmt.Sprintf("workspace:new_%d", m.createdCount), nil
}
func (m *syncMockClient) RenameWorkspace(ref, title string) error  { return nil }
func (m *syncMockClient) SelectWorkspace(ref string) error         { return nil }
func (m *syncMockClient) NewSplit(dir, ref string) (string, error) { return "surface:mock", nil }
func (m *syncMockClient) NewPane(opts client.NewPaneOpts) (string, error) {
	return "surface:new", nil
}
func (m *syncMockClient) FocusPane(pane, ws string) error { return nil }
func (m *syncMockClient) Send(ws, surf, text string) error { return nil }
func (m *syncMockClient) PinWorkspace(ref string) error    { return nil }
func (m *syncMockClient) UnpinWorkspace(ref string) error {
	if m.unpinnedRefs == nil {
		m.unpinnedRefs = make(map[string]bool)
	}
	m.unpinnedRefs[ref] = true
	return nil
}
func (m *syncMockClient) CloseWorkspace(ref string) error {
	if m.closedRefs == nil {
		m.closedRefs = make(map[string]bool)
	}
	m.closedRefs[ref] = true
	return nil
}
func (m *syncMockClient) DryRunFormatter() client.DryRunFormatter { return client.CmuxDryRun{} }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestore_Replace_SkipsMatchingTitles -v`
Expected: FAIL — current replace mode closes ALL non-caller workspaces including "dev".

- [ ] **Step 3: Write failing test for sync-skip in add mode**

```go
func TestRestore_Add_SkipsMatchingTitles(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "add-sync",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "missing", CWD: "/tmp/m", Index: 1, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	_ = store.Save("add-sync", layout)

	mc := &syncMockClient{
		existingWorkspaces: []client.WorkspaceInfo{
			{Ref: "workspace:1", Title: "dev"},
			{Ref: "workspace:2", Title: "extra"},
		},
	}
	restorer := &Restorer{Client: mc, Store: store}

	var skipped []string
	restorer.OnProgress = func(title string, panes int, err error) {
		if err != nil && strings.Contains(err.Error(), "skipped") {
			skipped = append(skipped, title)
		}
	}

	result, err := restorer.Restore("add-sync", false, RestoreModeAdd, "")
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// "dev" skipped (exists), "extra" kept (add mode), "missing" created.
	if result.WorkspacesOK != 1 {
		t.Errorf("WorkspacesOK = %d, want 1 (missing)", result.WorkspacesOK)
	}
	if len(skipped) != 1 || skipped[0] != "dev" {
		t.Errorf("skipped = %v, want [dev]", skipped)
	}
	if mc.closedRefs != nil && len(mc.closedRefs) > 0 {
		t.Error("add mode should not close any workspaces")
	}
}
```

- [ ] **Step 4: Write failing test for unpin-before-close**

```go
func TestRestore_Replace_UnpinsBeforeClose(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "unpin-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "kept", CWD: "/tmp/kept", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	_ = store.Save("unpin-test", layout)

	mc := &syncMockClient{
		existingWorkspaces: []client.WorkspaceInfo{
			{Ref: "workspace:1", Title: "kept"},
			{Ref: "workspace:2", Title: "pinned-stale"},
		},
		callerRef:   "workspace:1",
		callerTitle: "kept",
	}
	restorer := &Restorer{Client: mc, Store: store}

	_, err := restorer.Restore("unpin-test", false, RestoreModeReplace, "")
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// "pinned-stale" is not in layout, should be unpinned then closed.
	if !mc.unpinnedRefs["workspace:2"] {
		t.Error("should unpin workspace:2 before closing")
	}
	if !mc.closedRefs["workspace:2"] {
		t.Error("should close workspace:2 (not in layout)")
	}
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `go test ./internal/orchestrate/ -run "TestRestore_(Replace_Skips|Add_Skips|Replace_Unpins)" -v`
Expected: All FAIL.

- [ ] **Step 6: Implement the sync algorithm**

Replace the body of `Restorer.Restore` in `internal/orchestrate/restore.go` from the workspace snapshot section through the creation loop. Keep the filter logic, dry-run workspace method, and restoreWorkspace method unchanged.

```go
// Restore loads a layout and recreates it in cmux.
// When workspaceFilter is non-empty, only the workspace matching that title is restored.
func (r *Restorer) Restore(name string, dryRun bool, mode RestoreMode, workspaceFilter string) (*RestoreResult, error) {
	layout, err := r.Store.Load(name)
	if err != nil {
		return nil, fmt.Errorf("load layout: %w", err)
	}

	if workspaceFilter != "" {
		// Try exact match first (case-insensitive).
		var exactMatch *model.Workspace
		var substringMatches []model.Workspace
		filterLower := strings.ToLower(workspaceFilter)

		for i, ws := range layout.Workspaces {
			titleLower := strings.ToLower(ws.Title)
			if titleLower == filterLower {
				exactMatch = &layout.Workspaces[i]
				break
			}
			if strings.Contains(titleLower, filterLower) {
				substringMatches = append(substringMatches, ws)
			}
		}

		switch {
		case exactMatch != nil:
			layout.Workspaces = []model.Workspace{*exactMatch}
		case len(substringMatches) == 1:
			layout.Workspaces = substringMatches
		case len(substringMatches) == 0:
			return nil, fmt.Errorf("workspace %q not found in layout %q", workspaceFilter, name)
		default:
			titles := make([]string, len(substringMatches))
			for i, ws := range substringMatches {
				titles[i] = fmt.Sprintf("%q", ws.Title)
			}
			return nil, fmt.Errorf("%q matches multiple workspaces in layout %q: %s",
				workspaceFilter, name, strings.Join(titles, ", "))
		}
	}

	if !dryRun {
		if err := r.Client.Ping(); err != nil {
			return nil, fmt.Errorf("backend not reachable: %w", err)
		}
	}

	result := &RestoreResult{
		LayoutName:      layout.Name,
		WorkspacesTotal: len(layout.Workspaces),
		DryRun:          dryRun,
	}

	// Build set of layout titles for sync comparison.
	layoutTitles := make(map[string]bool, len(layout.Workspaces))
	for _, ws := range layout.Workspaces {
		layoutTitles[ws.Title] = true
	}

	// Snapshot existing workspace state.
	var callerRef string
	var callerTitle string
	existingTitles := make(map[string]bool)
	if !dryRun {
		if tree, err := r.Client.Tree(); err == nil && tree.Caller != nil {
			callerRef = tree.Caller.WorkspaceRef
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref == callerRef {
						callerTitle = ws.Title
					}
				}
			}
		}
		if existing, err := r.Client.ListWorkspaces(); err == nil {
			for _, ws := range existing {
				existingTitles[ws.Title] = true

				// In replace mode, close workspaces NOT in the layout (sync).
				// Skip the caller workspace — it must survive.
				if mode == RestoreModeReplace && ws.Ref != callerRef && !layoutTitles[ws.Title] {
					// Unpin first to avoid "pinned can't close" errors.
					_ = r.Client.UnpinWorkspace(ws.Ref)
					if err := r.Client.CloseWorkspace(ws.Ref); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("close %s (%s): %v", ws.Ref, ws.Title, err))
					} else {
						result.WorkspacesClosed++
					}
					time.Sleep(DelayAfterClose)
				}
			}
			if result.WorkspacesClosed > 0 {
				time.Sleep(DelayAfterCloseAll)
			}
		}
	} else if mode == RestoreModeReplace {
		result.Commands = append(result.Commands, "# Close workspaces not in layout (sync)")
	}

	// Sort workspaces by index.
	workspaces := make([]model.Workspace, len(layout.Workspaces))
	copy(workspaces, layout.Workspaces)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Index < workspaces[j].Index
	})

	// Create only workspaces that don't already exist (both modes).
	for _, ws := range workspaces {
		if !dryRun && existingTitles[ws.Title] {
			if r.OnProgress != nil {
				r.OnProgress(ws.Title, len(ws.Panes), fmt.Errorf("already open, skipped"))
			}
			continue
		}

		_, err := r.restoreWorkspace(ws, dryRun, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("workspace %q: %v", ws.Title, err))
			if r.OnProgress != nil && !dryRun {
				r.OnProgress(ws.Title, len(ws.Panes), err)
			}
			continue
		}
		result.WorkspacesOK++
		if r.OnProgress != nil && !dryRun {
			r.OnProgress(ws.Title, len(ws.Panes), nil)
		}
	}

	// Return focus to the caller's workspace.
	if callerRef != "" && !dryRun {
		if err := r.Client.SelectWorkspace(callerRef); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("select caller workspace: %v", err))
		}
	} else if dryRun {
		result.Commands = append(result.Commands, r.Client.DryRunFormatter().FmtSelectWorkspace("<caller>"))
	}

	return result, nil
}
```

Key changes from the old algorithm:
- Both modes skip workspaces whose title already exists (sync)
- Replace mode only closes workspaces NOT in the layout (instead of ALL)
- Unpin before close to avoid pinned-workspace errors
- No separate caller-title skip — caller is skipped via `ws.Ref != callerRef`

- [ ] **Step 7: Run the new tests**

Run: `go test ./internal/orchestrate/ -run "TestRestore_(Replace_Skips|Add_Skips|Replace_Unpins)" -v`
Expected: All PASS.

- [ ] **Step 8: Run ALL existing restore tests**

Run: `go test ./internal/orchestrate/ -v`
Expected: All pass (including existing dry-run and filter tests).

- [ ] **Step 9: Commit**

```bash
git add internal/orchestrate/restore.go internal/orchestrate/restore_test.go
git commit -m "$(cat <<'EOF'
feat(restore): sync-based algorithm — skip matching, unpin before close

Replace mode now only closes workspaces NOT in the layout instead of
destroying everything. Both modes skip workspaces that already exist
by title. Unpins workspaces before closing to prevent pinned errors.
EOF
)"
```

---

### Task 3: TUI Restore-Mode Prompt

**Files:**
- Modify: `internal/tui/shell.go:14-20` (add `modeRestoreAsk`)
- Modify: `internal/tui/shell.go:45-62` (add restore-ask state fields)
- Modify: `internal/tui/shell.go:129-160` (route `modeRestoreAsk` in Update)
- Modify: `internal/tui/shell.go:638-658` (render in View)
- Modify: `internal/tui/shell_exec.go:157-193` (trigger prompt or skip)
- Test: `internal/tui/shell_test.go`

- [ ] **Step 1: Write failing test — restore prompts when mode is empty**

Add to `internal/tui/shell_test.go`:

```go
func TestShellModel_RestoreAsk_ShowsPrompt(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	layout := &model.Layout{
		Name:    "test",
		SavedAt: time.Now(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{{Type: "terminal"}},
		}},
	}
	_ = store.Save("test", layout)

	m := NewShellModel(store, nil, client.BackendCmux, "")
	// restoreMode is "" (default) → should prompt

	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	if sm.mode != modeRestoreAsk {
		t.Fatalf("expected modeRestoreAsk, got %d", sm.mode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestShellModel_RestoreAsk -v`
Expected: FAIL — `modeRestoreAsk` doesn't exist yet.

- [ ] **Step 3: Write failing test — pressing 'r' in prompt selects replace**

```go
func TestShellModel_RestoreAsk_RSelectsReplace(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	layout := &model.Layout{
		Name:    "test",
		SavedAt: time.Now(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{{Type: "terminal"}},
		}},
	}
	_ = store.Save("test", layout)

	m := NewShellModel(store, nil, client.BackendCmux, "")
	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Press 'r' for replace
	result2, cmd := sm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	sm2 := result2.(*ShellModel)

	if sm2.mode != modePrompt {
		t.Errorf("expected modePrompt after selection, got %d", sm2.mode)
	}
	// Should return a restore command (tea.Cmd)
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd for async restore")
	}
}
```

- [ ] **Step 4: Write failing test — pressing Escape cancels**

```go
func TestShellModel_RestoreAsk_EscCancels(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	layout := &model.Layout{
		Name:    "test",
		SavedAt: time.Now(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{{Type: "terminal"}},
		}},
	}
	_ = store.Save("test", layout)

	m := NewShellModel(store, nil, client.BackendCmux, "")
	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Press Escape to cancel
	result2, cmd := sm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	sm2 := result2.(*ShellModel)

	if sm2.mode != modePrompt {
		t.Error("expected modePrompt after cancel")
	}
	if cmd != nil {
		t.Error("cancel should not trigger restore")
	}
	if !strings.Contains(sm2.lastOutput, "Cancelled") {
		t.Error("should show Cancelled message")
	}
}
```

- [ ] **Step 5: Write failing test — explicit mode skips prompt**

```go
func TestShellModel_RestoreExplicitMode_SkipsPrompt(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	layout := &model.Layout{
		Name:    "test",
		SavedAt: time.Now(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{{Type: "terminal"}},
		}},
	}
	_ = store.Save("test", layout)

	m := NewShellModel(store, nil, client.BackendCmux, "")
	m.SetRestoreMode("add")

	m.prompt.SetValue("restore test")
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Mode is "add" → should NOT enter modeRestoreAsk, should dispatch directly.
	if sm.mode == modeRestoreAsk {
		t.Error("explicit mode should skip the prompt")
	}
	if cmd == nil {
		t.Error("should return restore command immediately")
	}
}
```

- [ ] **Step 6: Run all new tests to verify they fail**

Run: `go test ./internal/tui/ -run "TestShellModel_Restore" -v`
Expected: FAIL — `modeRestoreAsk` undefined.

- [ ] **Step 7: Add `modeRestoreAsk` constant**

In `internal/tui/shell.go`, update the mode enum:

```go
const (
	modePrompt shellMode = iota
	modeBrowse
	modeConfirm
	modeRestoreAsk
)
```

- [ ] **Step 8: Add restore-ask state fields**

In `internal/tui/shell.go`, add to `ShellModel` struct after the confirmFn field:

```go
	// Restore-ask state (mode picker before restore)
	restoreAskName   string // layout name pending restore
	restoreAskFilter string // workspace filter pending restore
	restoreAskCursor int    // 0 = replace, 1 = add
```

- [ ] **Step 9: Add `updateRestoreAsk` handler**

In `internal/tui/shell.go`, add after `updateConfirm`:

```go
func (m *ShellModel) updateRestoreAsk(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.restoreAskCursor > 0 {
			m.restoreAskCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.restoreAskCursor < 1 {
			m.restoreAskCursor++
		}
		return m, nil
	case tea.KeyEnter:
		mode := orchestrate.RestoreModeReplace
		if m.restoreAskCursor == 1 {
			mode = orchestrate.RestoreModeAdd
		}
		m.mode = modePrompt
		return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, mode)
	case tea.KeyEsc:
		m.mode = modePrompt
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n\n")
		m.flushOutput()
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'r', 'R':
				m.mode = modePrompt
				return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeReplace)
			case 'a', 'A':
				m.mode = modePrompt
				return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeAdd)
			}
		}
	}
	return m, nil
}
```

- [ ] **Step 10: Add `startRestore` helper (extracted from `execRestore`)**

In `internal/tui/shell_exec.go`, add below `execRestore`:

```go
// startRestore launches the async restore with a specific mode.
func (m *ShellModel) startRestore(name, workspaceFilter string, mode orchestrate.RestoreMode) tea.Cmd {
	if workspaceFilter != "" {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q from %q…", workspaceFilter, name)))
	} else {
		action := "Replacing with"
		if mode == orchestrate.RestoreModeAdd {
			action = "Adding from"
		}
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  %s %q…", action, name)))
	}
	m.output.WriteString("\n")
	m.flushOutput()

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

- [ ] **Step 11: Rewrite `execRestore` to prompt when mode is ask/empty**

Replace `execRestore` in `internal/tui/shell_exec.go`:

```go
// execRestore restores a saved layout by name, optionally filtered to a single workspace.
func (m *ShellModel) execRestore(name string, workspaceFilter string) tea.Cmd {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return nil
	}

	// Determine mode from setting.
	switch m.restoreMode {
	case "replace":
		return m.startRestore(name, workspaceFilter, orchestrate.RestoreModeReplace)
	case "add":
		return m.startRestore(name, workspaceFilter, orchestrate.RestoreModeAdd)
	default:
		// "ask" or empty — show the mode picker.
		m.restoreAskName = name
		m.restoreAskFilter = workspaceFilter
		m.restoreAskCursor = 0
		m.mode = modeRestoreAsk
		return nil
	}
}
```

- [ ] **Step 12: Route `modeRestoreAsk` in Update**

In `internal/tui/shell.go`, in the `Update` method's `tea.KeyMsg` switch:

```go
	case tea.KeyMsg:
		switch m.mode {
		case modePrompt:
			return m.updatePrompt(msg)
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeRestoreAsk:
			return m.updateRestoreAsk(msg)
		}
```

- [ ] **Step 13: Render `modeRestoreAsk` in View**

In `internal/tui/shell.go`, in the `View` method's switch:

```go
	case modeRestoreAsk:
		var b strings.Builder
		b.WriteString("  How do you want to restore?\n\n")
		labels := []string{
			fmt.Sprintf("Replace — close non-matching %s, keep matching", unitLabel(m.backend, 2)),
			fmt.Sprintf("Add     — keep all existing %s, add missing", unitLabel(m.backend, 2)),
		}
		keys := []string{"r", "a"}
		for i, label := range labels {
			if i == m.restoreAskCursor {
				fmt.Fprintf(&b, "  %s %s  %s\n", shellSuccessStyle.Render("▸"), shellSuccessStyle.Render("["+keys[i]+"]"), label)
			} else {
				fmt.Fprintf(&b, "    %s  %s\n", shellDimStyle.Render("["+keys[i]+"]"), label)
			}
		}
		b.WriteString("\n")
		b.WriteString(shellDimStyle.Render("  ↑/↓ select · ↵ confirm · r/a shortcut · esc cancel"))
		b.WriteString("\n")
		return prompt + header + "\n\n" + b.String()
```

- [ ] **Step 14: Add missing import**

In `internal/tui/shell.go`, add `"fmt"` to the imports if not already present. Also add `"github.com/drolosoft/cmux-resurrect/internal/orchestrate"` if needed.

- [ ] **Step 15: Run TUI tests**

Run: `go test ./internal/tui/ -run "TestShellModel_Restore" -v`
Expected: All PASS.

- [ ] **Step 16: Run full test suite**

Run: `go test ./... -v`
Expected: All pass.

- [ ] **Step 17: Commit**

```bash
git add internal/tui/shell.go internal/tui/shell_exec.go internal/tui/shell_test.go
git commit -m "$(cat <<'EOF'
feat(tui): add restore-mode prompt when mode is ask/empty

TUI now shows a Replace/Add picker before restoring, matching
the CLI picker behavior. Explicit mode (settings restore-mode set)
skips the prompt.
EOF
)"
```

---

### Task 4: CLI Prompt for `crex restore <name>` Without Mode

**Files:**
- Modify: `cmd/restore.go:113-135`
- Test: manual (interactive prompt requires stdin)

Currently `crex restore <name>` (with a name but no `--mode`) silently defaults to replace (zero value). It should prompt like the picker does, unless config or flag is set.

- [ ] **Step 1: Add a `promptRestoreMode` helper**

In `cmd/restore.go`, add a helper function:

```go
func promptRestoreMode() (orchestrate.RestoreMode, error) {
	fmt.Fprintf(os.Stderr, "How do you want to restore?\n")
	fmt.Fprintf(os.Stderr, "  %s  Close non-matching %s, keep matching\n", cyanStyle.Render("[r]eplace"), unitName(2))
	fmt.Fprintf(os.Stderr, "  %s  Keep all existing %s, add missing\n", cyanStyle.Render("[a]dd"), unitName(2))
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Choice [r/a]: ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	switch answer {
	case "r", "replace":
		return orchestrate.RestoreModeReplace, nil
	case "a", "add":
		return orchestrate.RestoreModeAdd, nil
	default:
		return 0, fmt.Errorf("cancelled")
	}
}
```

- [ ] **Step 2: Add `bufio` import**

In `cmd/restore.go`, add `"bufio"` to the import block.

- [ ] **Step 3: Update the mode determination in `runRestore`**

Replace the `default` case of the outer switch in `runRestore` (lines ~123-135):

```go
	default:
		switch cfg.RestoreMode {
		case "replace":
			mode = orchestrate.RestoreModeReplace
		case "add":
			mode = orchestrate.RestoreModeAdd
		default:
			// Single workspace restore defaults to add mode (no prompt).
			if workspaceFilter != "" {
				mode = orchestrate.RestoreModeAdd
			} else {
				// Interactive prompt.
				prompted, err := promptRestoreMode()
				if err != nil {
					fmt.Fprintln(os.Stderr, dimStyle.Render("Cancelled."))
					return nil
				}
				mode = prompted
			}
		}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 5: Run all tests**

Run: `go test ./... -v`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/restore.go
git commit -m "$(cat <<'EOF'
feat(cli): prompt for restore mode when no --mode flag or config

crex restore <name> now asks Replace/Add interactively instead of
silently defaulting to replace. --mode flag and config still skip
the prompt. Single-workspace restore defaults to add.
EOF
)"
```

---

### Task 5: Update CLI Restore Output to Reflect Sync Behavior

**Files:**
- Modify: `cmd/restore.go:141-147`

The current output says "Replacing with" or "Adding from" which implies destructive replace. Update to reflect sync behavior.

- [ ] **Step 1: Update action labels**

In `cmd/restore.go`, update the action label logic (around lines 141-147):

```go
	} else {
		action := "🔄 Syncing (replace)"
		if mode == orchestrate.RestoreModeAdd {
			action = "➕ Syncing (add)"
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render(action), greenStyle.Render(name))
	}
```

- [ ] **Step 2: Verify build and tests**

Run: `go build ./... && go test ./...`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/restore.go
git commit -m "$(cat <<'EOF'
chore(cli): update restore output labels to reflect sync behavior
EOF
)"
```

---

### Task 6: Fix `updateConfirm` False Success Message

**Files:**
- Modify: `internal/tui/shell.go:384-399`

Currently `updateConfirm` writes "Done" even if `confirmFn` wrote an error. Fix it.

- [ ] **Step 1: Write failing test**

Add to `internal/tui/shell_test.go`:

```go
func TestShellModel_ConfirmFnError_NoSuccessMsg(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendCmux, "")

	// Set up a confirmFn that writes an error.
	m.mode = modeConfirm
	m.confirmMsg = "Delete?"
	errWritten := false
	m.confirmFn = func() {
		m.output.WriteString(shellErrorStyle.Render("  ✗ something failed"))
		m.output.WriteString("\n")
		errWritten = true
	}

	// Press 'y'
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	sm := result.(*ShellModel)

	if !errWritten {
		t.Fatal("confirmFn should have run")
	}
	if strings.Contains(sm.lastOutput, "Done") {
		t.Error("should not show 'Done' when confirmFn wrote an error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/ -run TestShellModel_ConfirmFnError -v`
Expected: FAIL — currently always writes "Done".

- [ ] **Step 3: Fix `updateConfirm`**

In `internal/tui/shell.go`, replace `updateConfirm`:

```go
func (m *ShellModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
		if m.confirmFn != nil {
			before := m.output.Len()
			m.confirmFn()
			// Only show success if confirmFn didn't write an error.
			if m.output.Len() == before {
				m.output.WriteString(shellSuccessStyle.Render("  ✓ Done"))
				m.output.WriteString("\n")
			}
		}
	} else {
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n")
	}
	m.mode = modePrompt
	m.confirmMsg = ""
	m.confirmFn = nil
	m.flushOutput()
	return m, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/ -v`
Expected: All pass including new test.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/shell.go internal/tui/shell_test.go
git commit -m "$(cat <<'EOF'
fix(tui): don't show success after confirmFn error

updateConfirm now checks if confirmFn wrote to the output buffer.
If it did (error message), the "Done" message is suppressed.
EOF
)"
```

---

### Task 7: Integration Verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All pass.

- [ ] **Step 2: Build binary**

Run: `go build -o /tmp/crex-test ./cmd/crex`
Expected: Clean build.

- [ ] **Step 3: Verify help text mentions sync**

Run: `/tmp/crex-test restore --help`
Expected: Help text describes restore behavior.

- [ ] **Step 4: Manual test — TUI restore ask mode**

1. Run `/tmp/crex-test tui`
2. Type `restore <existing-layout>`
3. Verify Replace/Add picker appears
4. Press `r` → verify sync behavior (matching tabs kept, extras closed)
5. Repeat with `a` → verify only missing tabs created

- [ ] **Step 5: Manual test — CLI restore ask mode**

1. Run `/tmp/crex-test restore <existing-layout>`
2. Verify r/a prompt appears
3. Choose `r` → verify sync behavior
4. Repeat with `--mode add` → verify prompt is skipped

- [ ] **Step 6: Manual test — pinned workspace handling**

1. Pin a workspace
2. Run restore in replace mode
3. Verify the pinned workspace is unpinned and closed (if not in layout) without errors

- [ ] **Step 7: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: integration test fixups for sync restore"
```
