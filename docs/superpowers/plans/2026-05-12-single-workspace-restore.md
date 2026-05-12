# Single Workspace Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `crex restore <layout> <workspace>` to restore a single workspace by substring match, with tab completion for workspace names.

**Architecture:** Three files changed. The orchestrator gains fuzzy matching logic. The completion helpers gain a workspace name completer. The restore command wires the second arg and dispatches completion by arg position.

**Tech Stack:** Go, Cobra, existing persist.Store and orchestrate.Restorer

---

### Task 1: Substring matching in orchestrate/restore.go

**Files:**
- Modify: `internal/orchestrate/restore.go:42-61`
- Test: `internal/orchestrate/restore_test.go`

- [ ] **Step 1: Write failing test for case-insensitive substring match**

Add to `internal/orchestrate/restore_test.go`:

```go
func TestRestore_WorkspaceFilter_SubstringMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "sub-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 🗑️ Trash", CWD: "/tmp/trash", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "⠐ Claude Code", CWD: "/tmp/claude", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 tests", CWD: "/tmp/tests", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("sub-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	// "trash" should match "0 🗑️ Trash" (case-insensitive substring).
	result, err := restorer.Restore("sub-test", true, RestoreModeAdd, "trash")
	if err != nil {
		t.Fatalf("restore with substring filter: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}

	// "claude" should match "⠐ Claude Code".
	result, err = restorer.Restore("sub-test", true, RestoreModeAdd, "claude")
	if err != nil {
		t.Fatalf("restore with substring filter: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestore_WorkspaceFilter_SubstringMatch -v`
Expected: FAIL — current code does exact match, "trash" won't match "0 🗑️ Trash"

- [ ] **Step 3: Write failing test for no-match error**

Add to `internal/orchestrate/restore_test.go`:

```go
func TestRestore_WorkspaceFilter_NoMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "nomatch-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("nomatch-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	_, err := restorer.Restore("nomatch-test", true, RestoreModeAdd, "zzz")
	if err == nil {
		t.Fatal("expected error for non-matching filter")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}
```

Note: add `"strings"` to the import block at the top of the test file.

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestore_WorkspaceFilter_NoMatch -v`
Expected: FAIL — current code returns "workspace \"zzz\" not found" but using exact match

- [ ] **Step 5: Write failing test for ambiguous match error**

Add to `internal/orchestrate/restore_test.go`:

```go
func TestRestore_WorkspaceFilter_AmbiguousMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "ambig-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev-api", CWD: "/tmp/api", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 dev-web", CWD: "/tmp/web", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 docs", CWD: "/tmp/docs", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("ambig-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	_, err := restorer.Restore("ambig-test", true, RestoreModeAdd, "dev")
	if err == nil {
		t.Fatal("expected error for ambiguous filter")
	}
	if !strings.Contains(err.Error(), "matches multiple") {
		t.Errorf("error = %q, want 'matches multiple'", err.Error())
	}
	if !strings.Contains(err.Error(), "0 dev-api") || !strings.Contains(err.Error(), "1 dev-web") {
		t.Errorf("error should list matching titles: %q", err.Error())
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestore_WorkspaceFilter_AmbiguousMatch -v`
Expected: FAIL — current code finds no exact match and returns "not found"

- [ ] **Step 7: Write failing test for exact match taking priority over substring**

Add to `internal/orchestrate/restore_test.go`:

```go
func TestRestore_WorkspaceFilter_ExactMatchPriority(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "exact-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "dev-tools", CWD: "/tmp/devtools", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("exact-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	// "dev" exactly matches "dev", should NOT be ambiguous even though "dev-tools" also contains "dev".
	result, err := restorer.Restore("exact-test", true, RestoreModeAdd, "dev")
	if err != nil {
		t.Fatalf("exact match should not be ambiguous: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
}
```

- [ ] **Step 8: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestore_WorkspaceFilter_ExactMatchPriority -v`
Expected: FAIL — current exact match returns "dev" but the new substring logic (once written) needs to prefer exact over substring

- [ ] **Step 9: Implement substring matching in restore.go**

Replace lines 49-61 in `internal/orchestrate/restore.go` (the `workspaceFilter` block):

```go
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
```

Add `"strings"` to the import block in `restore.go`.

- [ ] **Step 10: Run all restore tests**

Run: `go test ./internal/orchestrate/ -run TestRestore -v`
Expected: ALL PASS (existing tests + 4 new tests)

- [ ] **Step 11: Commit**

```bash
git add internal/orchestrate/restore.go internal/orchestrate/restore_test.go
git commit -m "feat(restore): case-insensitive substring matching for workspace filter"
```

---

### Task 2: Workspace name completion helper

**Files:**
- Modify: `cmd/completion_helpers.go`
- Test: `cmd/completion_test.go`

- [ ] **Step 1: Write failing unit test for completeWorkspaceNames**

Add to `cmd/completion_test.go` in a new section after the `completeLayoutNames` tests (after line 247):

```go
// ---------------------------------------------------------------------------
// 1b. Unit tests: completeWorkspaceNames
// ---------------------------------------------------------------------------

func TestCompleteWorkspaceNames_ReturnsWorkspaceTitles(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)

	// Create a layout with named workspaces.
	store, _ := persist.NewFileStore(layoutsDir)
	layout := &model.Layout{
		Name: "my-day", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 🗑️ Trash", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "⠐ Claude Code", CWD: "/tmp", Index: 1, Panes: []model.Pane{{Type: "terminal"}, {Type: "terminal", Split: "right"}}},
			{Title: "2 tests", CWD: "/tmp", Index: 2, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	_ = store.Save("my-day", layout)

	completions, directive := completeWorkspaceNames(nil, []string{"my-day"}, "")
	if len(completions) != 3 {
		t.Fatalf("expected 3 completions, got %d: %v", len(completions), completions)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}

	// Check titles and descriptions.
	if !strings.Contains(completions[0], "0 🗑️ Trash") {
		t.Errorf("completions[0] = %q, expected '0 🗑️ Trash'", completions[0])
	}
	if !strings.Contains(completions[1], "⠐ Claude Code") && !strings.Contains(completions[1], "2 panes") {
		t.Errorf("completions[1] = %q, expected Claude Code with 2 panes", completions[1])
	}
}

func TestCompleteWorkspaceNames_InvalidLayout(t *testing.T) {
	setupTestConfig(t)

	completions, directive := completeWorkspaceNames(nil, []string{"nonexistent"}, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for invalid layout, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestCompleteWorkspaceNames_ThirdArgBlocked(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "my-day", "", 2)

	completions, directive := completeWorkspaceNames(nil, []string{"my-day", "something"}, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for third arg, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestCompleteWorkspaceNames -v`
Expected: FAIL — `completeWorkspaceNames` doesn't exist yet

- [ ] **Step 3: Implement completeWorkspaceNames**

Add to `cmd/completion_helpers.go` after `completeLayoutNames` (after line 37):

```go
// completeWorkspaceNames provides dynamic completion of workspace titles within a layout.
// Used by: restore (second positional arg).
func completeWorkspaceNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	store, err := newStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	layout, err := store.Load(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(layout.Workspaces))
	for _, ws := range layout.Workspaces {
		paneCount := len(ws.Panes)
		desc := fmt.Sprintf("%d %s", paneCount, unitName(paneCount))
		names = append(names, fmt.Sprintf("%s\t%s", ws.Title, desc))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/ -run TestCompleteWorkspaceNames -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/completion_helpers.go cmd/completion_test.go
git commit -m "feat(completion): add workspace name completer for restore second arg"
```

---

### Task 3: Wire second arg into restore command

**Files:**
- Modify: `cmd/restore.go:15-68`
- Test: `cmd/completion_test.go`

- [ ] **Step 1: Write failing E2E completion test**

Add to `cmd/completion_test.go` in the E2E section (after `TestE2E_RestoreLayoutNames_PartialMatch`, around line 511):

```go
func TestE2E_RestoreWorkspaceNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)

	// Create a layout with named workspaces.
	store, _ := persist.NewFileStore(layoutsDir)
	layout := &model.Layout{
		Name: "my-day", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "1 docs", CWD: "/tmp", Index: 1, Panes: []model.Pane{{Type: "terminal"}, {Type: "terminal", Split: "right"}}},
		},
	}
	_ = store.Save("my-day", layout)

	output := executeComplete(t, "restore", "my-day", "")
	names := completionNames(output)
	assertContains(t, names, "0 dev")
	assertContains(t, names, "1 docs")

	// Check descriptions include pane counts.
	lines := completionLines(output)
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "1 docs\t") && strings.Contains(line, "2") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected '1 docs' with pane count in: %v", lines)
	}

	// Directive should be NoFileComp.
	if d := completionDirective(output); d != 4 {
		t.Errorf("directive = %d, want 4 (NoFileComp)", d)
	}
}

func TestE2E_RestoreThirdArgBlocked(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)

	store, _ := persist.NewFileStore(layoutsDir)
	layout := &model.Layout{
		Name: "my-day", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	_ = store.Save("my-day", layout)

	output := executeComplete(t, "restore", "my-day", "0 dev", "")
	names := completionNames(output)
	if len(names) != 0 {
		t.Errorf("expected 0 completions after workspace name, got %d: %v", len(names), names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestE2E_RestoreWorkspaceNames -v`
Expected: FAIL — restore currently returns 0 completions for second arg

- [ ] **Step 3: Update restore command to accept 2 args and dispatch completion**

In `cmd/restore.go`, make these changes:

1. Change `Args` from `MaximumNArgs(1)` to `MaximumNArgs(2)` (line 19)
2. Replace `ValidArgsFunction` with a dispatcher (line 26)
3. In `runRestore`, read `args[1]` as workspace filter and default to add mode (lines 41-43)

Updated `cmd/restore.go` init function:

```go
func init() {
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "show commands without executing")
	restoreCmd.Flags().StringVar(&restoreMode, "mode", "", "restore mode: \"replace\" or \"add\" (skip interactive prompt)")
	restoreCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeLayoutNames(cmd, args, toComplete)
		case 1:
			return completeWorkspaceNames(cmd, args, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	_ = restoreCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"replace\tClose existing " + unitName(2) + " first",
			"add\tKeep existing " + unitName(2),
		}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(restoreCmd)
}
```

Updated `runRestore` top section (replace lines 36-68):

```go
func runRestore(cmd *cobra.Command, args []string) error {
	var name string
	var workspaceFilter string
	var pickerMode orchestrate.RestoreMode
	var pickerModeSet bool
	switch len(args) {
	case 2:
		name = args[0]
		workspaceFilter = args[1]
	case 1:
		name = args[0]
	default:
		store, err := newStore()
		if err != nil {
			return err
		}
		metas, err := store.List()
		if err != nil {
			return err
		}
		if len(metas) == 0 {
			fmt.Fprintln(os.Stderr, dimStyle.Render("  No saved layouts. Use 'crex save <name>' to create one."))
			return nil
		}
		// Skip the mode step inside the picker if it's already determined.
		skipMode := restoreMode != "" || restoreDryRun || cfg.RestoreMode != ""
		pick, err := pickLayout(metas, skipMode)
		if err != nil {
			return err
		}
		name = pick.Layout
		workspaceFilter = pick.Workspace
		if !skipMode {
			pickerMode = pick.Mode
			pickerModeSet = true
		}
	}
```

- [ ] **Step 4: Default single-workspace restore to add mode**

In the mode determination section of `runRestore` (around line 100), add a case for single-workspace defaulting to add mode. Replace the `default:` block in the mode switch:

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
	case pickerModeSet:
		mode = pickerMode
	default:
		switch cfg.RestoreMode {
		case "replace":
			mode = orchestrate.RestoreModeReplace
		case "add":
			mode = orchestrate.RestoreModeAdd
		default:
			// Single workspace restore defaults to add mode (no prompt needed).
			if workspaceFilter != "" {
				mode = orchestrate.RestoreModeAdd
			}
		}
	}
```

- [ ] **Step 5: Run E2E completion tests**

Run: `go test ./cmd/ -run "TestE2E_Restore" -v`
Expected: ALL PASS

- [ ] **Step 6: Update TestE2E_LayoutNoSecondArg to exclude restore**

The existing test at line 560 checks that ALL layout commands return 0 completions for the second arg. Now `restore` should return workspace names instead. Update the test:

```go
func TestE2E_LayoutNoSecondArg(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "alpha", "", 1)

	// After providing the layout name, no further completions (except restore, which completes workspace names).
	for _, cmd := range []string{"delete", "show", "edit", "save", "watch"} {
		t.Run(cmd, func(t *testing.T) {
			output := executeComplete(t, cmd, "alpha", "")
			names := completionNames(output)
			if len(names) != 0 {
				t.Errorf("%s: expected 0 completions after layout name, got %d: %v", cmd, len(names), names)
			}
			if d := completionDirective(output); d != 4 {
				t.Errorf("%s: directive = %d, want 4 (NoFileComp)", cmd, d)
			}
		})
	}
}
```

- [ ] **Step 7: Run full test suite**

Run: `go test ./cmd/ -v`
Expected: ALL PASS

Run: `go test ./internal/orchestrate/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add cmd/restore.go cmd/completion_test.go
git commit -m "feat(restore): accept workspace name as second arg with tab completion"
```

---

### Task 4: Manual smoke test

**Files:** None (verification only)

- [ ] **Step 1: Build crex**

Run: `go build -o crex .`

- [ ] **Step 2: Test one-liner restore**

Run: `./crex restore default trash --dry-run`
Expected: Dry-run output showing only the workspace matching "trash"

- [ ] **Step 3: Test ambiguous match**

Run: `./crex restore default "Claude" --dry-run`
Expected: Restores the single matching workspace, or error if ambiguous

- [ ] **Step 4: Test no-match error**

Run: `./crex restore default "zzz_nonexistent"`
Expected: Error message: `workspace "zzz_nonexistent" not found in layout "default"`

- [ ] **Step 5: Test full layout restore unchanged**

Run: `./crex restore default --dry-run`
Expected: Dry-run showing all workspaces (same as before)

- [ ] **Step 6: Test completion output**

Run: `./crex __complete restore default ""`
Expected: List of workspace titles from the "default" layout with pane counts

- [ ] **Step 7: Clean up build artifact**

Run: `rm ./crex`

- [ ] **Step 8: Commit (if any fixes were needed)**

Only if smoke testing revealed issues that required code changes.
