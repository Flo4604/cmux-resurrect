# Browser Pane Restoration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore browser panes (with their URLs) during layout restoration on cmux, with graceful fallback on Ghostty.

**Architecture:** The save side already captures browser pane `Type: "browser"` and `URL` from cmux's tree. The restore side currently treats all panes as terminals — it calls `NewSplit` (which only creates terminal splits) and never reads `pane.URL`. We need to: (1) add a `NewPane` method to the `Backend` interface that supports `--type browser --url <url>`, (2) use it during restore when `pane.Type == "browser"`, (3) fall back gracefully on Ghostty (open a terminal that runs `open <url>` in the default system browser), and (4) update dry-run, show, and tests.

**Tech Stack:** Go 1.26, Cobra, Bubble Tea, cmux CLI (`new-pane --type browser --url <url>`)

**GitHub Issue:** drolosoft/cmux-resurrect#3

---

### Task 1: Add `NewPane` to the Backend interface

The existing `NewSplit` method has no `--type` or `--url` parameters. We need a new method `NewPane` that maps to `cmux new-pane --type <type> --direction <dir> --workspace <ref> --url <url>`.

**Files:**
- Modify: `internal/client/client.go` — add `NewPane` to `Backend` interface
- Modify: `internal/client/types.go` — add `NewPaneOpts` struct

- [ ] **Step 1: Write the failing test**

Create `internal/client/client_test.go` (or add to existing test):

```go
func TestNewPaneOpts_HasRequiredFields(t *testing.T) {
	opts := NewPaneOpts{
		Type:      "browser",
		Direction: "right",
		URL:       "https://example.com",
	}
	if opts.Type != "browser" {
		t.Errorf("Type = %q, want browser", opts.Type)
	}
	if opts.URL != "https://example.com" {
		t.Errorf("URL = %q, want https://example.com", opts.URL)
	}
}
```

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/client/ -run TestNewPaneOpts -v`
Expected: FAIL — `NewPaneOpts` not defined.

- [ ] **Step 2: Add `NewPaneOpts` to types.go**

In `internal/client/types.go`, after the `NewWorkspaceOpts` struct (line 89), add:

```go
// NewPaneOpts for creating a new pane (terminal or browser).
type NewPaneOpts struct {
	Type         string // "terminal" or "browser"
	Direction    string // "left", "right", "up", "down"
	WorkspaceRef string
	URL          string // for browser panes
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/client/ -run TestNewPaneOpts -v`
Expected: PASS

- [ ] **Step 4: Add `NewPane` to `Backend` interface**

In `internal/client/client.go`, add after the `NewSplit` method (line 28):

```go
	// NewPane creates a new pane in a workspace, supporting type (terminal/browser) and URL.
	// Returns the new surface ref.
	NewPane(opts NewPaneOpts) (string, error)
```

- [ ] **Step 5: Verify compilation fails for implementations**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: FAIL — `CLIClient` and `GhosttyClient` do not implement `NewPane`.

- [ ] **Step 6: Commit**

```bash
git add internal/client/client.go internal/client/types.go
git commit -m "feat(client): add NewPane to Backend interface with NewPaneOpts type"
```

---

### Task 2: Implement `NewPane` for cmux (`CLIClient`)

The cmux CLI has `cmux new-pane --type <terminal|browser> --direction <dir> --workspace <ref> --url <url>`. We implement `NewPane` to call this command, polling for the new surface ref afterward (same pattern as `NewSplit`).

**Files:**
- Modify: `internal/client/cli.go` — add `NewPane` method

- [ ] **Step 1: Implement `NewPane` on `CLIClient`**

In `internal/client/cli.go`, add after the `NewSplit` method:

```go
func (c *CLIClient) NewPane(opts NewPaneOpts) (string, error) {
	// Snapshot surface refs before creation so we can detect the new one.
	before := make(map[string]bool)
	if opts.WorkspaceRef != "" {
		if tree, err := c.Tree(); err == nil {
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != opts.WorkspaceRef {
						continue
					}
					for _, p := range ws.Panes {
						for _, s := range p.Surfaces {
							before[s.Ref] = true
						}
					}
				}
			}
		}
	}

	args := []string{"new-pane"}
	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.Direction != "" {
		args = append(args, "--direction", opts.Direction)
	}
	if opts.WorkspaceRef != "" {
		args = append(args, "--workspace", opts.WorkspaceRef)
	}
	if opts.URL != "" {
		args = append(args, "--url", opts.URL)
	}
	if _, err := c.run(args...); err != nil {
		return "", err
	}

	// Find the new surface by diffing against the snapshot.
	if opts.WorkspaceRef != "" {
		deadline := time.Now().Add(NewSplitDeadline)
		for time.Now().Before(deadline) {
			time.Sleep(PollInterval)
			tree, err := c.Tree()
			if err != nil {
				continue
			}
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != opts.WorkspaceRef {
						continue
					}
					for _, p := range ws.Panes {
						for _, s := range p.Surfaces {
							if !before[s.Ref] {
								return s.Ref, nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("new pane created but could not determine new surface ref")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./internal/client/`
Expected: FAIL — `GhosttyClient` still missing `NewPane`. That's expected (fixed in Task 3).

- [ ] **Step 3: Commit**

```bash
git add internal/client/cli.go
git commit -m "feat(client): implement NewPane for cmux CLIClient"
```

---

### Task 3: Implement `NewPane` fallback for Ghostty (`GhosttyClient`)

Ghostty has no browser pane concept. For `Type: "browser"`, we fall back to creating a terminal split and running `open <url>` to open the URL in the system browser. For `Type: "terminal"`, we delegate to the existing `NewSplit` logic.

**Files:**
- Modify: `internal/client/ghostty.go` — add `NewPane` method

- [ ] **Step 1: Implement `NewPane` on `GhosttyClient`**

In `internal/client/ghostty.go`, add after the `NewSplit` method:

```go
func (g *GhosttyClient) NewPane(opts NewPaneOpts) (string, error) {
	direction := opts.Direction
	if direction == "" {
		direction = "right"
	}

	// Ghostty has no browser pane — create a terminal split instead.
	ref, err := g.NewSplit(direction, opts.WorkspaceRef)
	if err != nil {
		return "", err
	}

	// For browser panes, open the URL in the system browser.
	if opts.Type == "browser" && opts.URL != "" {
		g.waitForShellReady(opts.WorkspaceRef)
		_ = g.Send(opts.WorkspaceRef, ref, fmt.Sprintf("open %q\\n", opts.URL))
	}

	return ref, nil
}
```

- [ ] **Step 2: Verify full project compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: PASS — both backends now implement `Backend`.

- [ ] **Step 3: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(client): implement NewPane fallback for Ghostty (open URL in system browser)"
```

---

### Task 4: Implement `NewPane` on the `mockClient` (test support)

The `mockClient` in `internal/orchestrate/save_test.go` implements `Backend`. It needs `NewPane` so tests compile. The mock in `cmd/newcmds_test.go` (if any) may also need updating.

**Files:**
- Modify: `internal/orchestrate/save_test.go` — add `NewPane` to `mockClient`
- Modify: any other test mocks that implement `Backend`

- [ ] **Step 1: Find all mock clients**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && grep -rn 'func.*mockClient.*NewSplit\|func.*Mock.*NewSplit\|func.*stub.*NewSplit' --include='*.go'`

- [ ] **Step 2: Add `NewPane` to each mock**

In each mock that implements `Backend`, add:

```go
func (m *mockClient) NewPane(opts client.NewPaneOpts) (string, error) {
	return "surface:new", nil
}
```

- [ ] **Step 3: Verify all tests compile**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./... -count=0`
Expected: All packages compile (tests may run but no new failures).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test: add NewPane to mock clients for compilation"
```

---

### Task 5: Add `FmtNewPane` to `DryRunFormatter`

Dry-run mode needs to render browser pane creation commands. Add `FmtNewPane` to the `DryRunFormatter` interface and both implementations.

**Files:**
- Modify: `internal/client/dryrun.go` — add `FmtNewPane` to interface + both implementations

- [ ] **Step 1: Add `FmtNewPane` to `DryRunFormatter` interface**

In `internal/client/dryrun.go`, add to the interface:

```go
	FmtNewPane(paneType, direction, ref, url string) string
```

- [ ] **Step 2: Implement for `CmuxDryRun`**

```go
func (CmuxDryRun) FmtNewPane(paneType, direction, ref, url string) string {
	cmd := fmt.Sprintf("cmux new-pane --type %s --direction %s --workspace %s", paneType, direction, ref)
	if url != "" {
		cmd += fmt.Sprintf(" --url %q", url)
	}
	return cmd
}
```

- [ ] **Step 3: Implement for `GhosttyDryRun`**

```go
func (GhosttyDryRun) FmtNewPane(paneType, direction, ref, url string) string {
	if paneType == "browser" && url != "" {
		return fmt.Sprintf("osascript: split %s in %s + open %q in system browser", direction, ref, url)
	}
	return fmt.Sprintf("osascript: split %s in %s", direction, ref)
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/client/dryrun.go
git commit -m "feat(client): add FmtNewPane to DryRunFormatter for browser dry-run output"
```

---

### Task 6: Update `restoreWorkspace` to handle browser panes

This is the core change. In `internal/orchestrate/restore.go`, the pane loop at line 219 currently uses `NewSplit` for all non-first panes, and sends `Command` as text. For browser panes, we use `NewPane` instead.

**Files:**
- Modify: `internal/orchestrate/restore.go` — update `restoreWorkspace` pane loop

- [ ] **Step 1: Write the failing test**

In `internal/orchestrate/restore_test.go`, add:

```go
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
```

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestRestore_BrowserPane_DryRun -v`
Expected: FAIL — dry-run still uses `FmtNewSplit`, no browser or URL in output.

- [ ] **Step 2: Update `restoreWorkspace` for browser panes (live mode)**

In `internal/orchestrate/restore.go`, replace the split-creation block for non-first panes (the `else` branch starting around line 244) with browser-aware logic. Replace the section from `direction := pane.Split` through `continue` (lines 245-253) with:

```go
		direction := pane.Split
		if direction == "" {
			direction = "right"
		}

		var surfaceRef string
		if pane.Type == "browser" {
			surfaceRef, err = r.Client.NewPane(client.NewPaneOpts{
				Type:         "browser",
				Direction:    direction,
				WorkspaceRef: ref,
				URL:          pane.URL,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d browser: %v", i, err))
				continue
			}
		} else {
			surfaceRef, err = r.Client.NewSplit(direction, ref)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d split: %v", i, err))
				continue
			}
		}
```

Then update the command-sending block that follows — browser panes don't need shell commands sent to them. Wrap the existing command-sending in a `pane.Type != "browser"` check:

```go
		if pane.Type != "browser" && pane.Command != "" {
			if err := waitForShellReady(r.Client, ref, surfaceRef); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d shell not ready: %v", i, err))
			} else if err := r.Client.Send(ref, surfaceRef, pane.Command+"\\n"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
			}
		}
```

- [ ] **Step 3: Update `restoreWorkspace` for browser first pane**

The first pane (i == 0) is the default pane created with the workspace. If the first pane is a browser pane, we need to handle it differently — skip command sending (since the workspace was created as a terminal, we can't convert pane 0 to a browser). For now, if pane 0 is browser type, send `open <url>` as a command fallback. Update the `i == 0` block:

```go
		if i == 0 {
			if pane.Type == "browser" && pane.URL != "" {
				// First pane is always a terminal. Open URL as fallback.
				if err := waitForShellReady(r.Client, ref, ""); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d shell not ready: %v", i, err))
				} else if err := r.Client.Send(ref, "", fmt.Sprintf("open %q\\n", pane.URL)); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d open url: %v", i, err))
				}
			} else if pane.Command != "" {
				if err := waitForShellReady(r.Client, ref, ""); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d shell not ready: %v", i, err))
				} else if err := r.Client.Send(ref, "", pane.Command+"\\n"); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
				}
			}
			if i < lastPane {
				time.Sleep(DelayAfterSplit)
			}
			continue
		}
```

- [ ] **Step 4: Update `dryRunWorkspace` for browser panes**

In `internal/orchestrate/restore.go`, update the `dryRunWorkspace` method. Replace the pane loop body for non-first panes to use `FmtNewPane` for browser panes:

```go
	for i, pane := range ws.Panes {
		if i == 0 {
			if pane.Type == "browser" && pane.URL != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, fmt.Sprintf("open %q", pane.URL)))
			} else if pane.Command != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
			}
			continue
		}
		if pane.FocusTarget >= 0 {
			result.Commands = append(result.Commands,
				f.FmtFocusPane(fmt.Sprintf("pane:%d", pane.FocusTarget), ref))
		}
		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		if pane.Type == "browser" {
			result.Commands = append(result.Commands, f.FmtNewPane(pane.Type, direction, ref, pane.URL))
		} else {
			result.Commands = append(result.Commands, f.FmtNewSplit(direction, ref))
			if pane.Command != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
			}
		}
	}
```

- [ ] **Step 5: Run the browser test**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestRestore_BrowserPane_DryRun -v`
Expected: PASS

- [ ] **Step 6: Run all existing restore tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestRestore -v`
Expected: All PASS (no regressions).

- [ ] **Step 7: Commit**

```bash
git add internal/orchestrate/restore.go internal/orchestrate/restore_test.go
git commit -m "feat(restore): support browser pane restoration with URL"
```

---

### Task 7: Add test for browser-only workspace restoration

Test a workspace that has both terminal and browser panes to verify the mixed-pane case works correctly in dry-run.

**Files:**
- Modify: `internal/orchestrate/restore_test.go`

- [ ] **Step 1: Write the mixed-pane test**

```go
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
```

- [ ] **Step 2: Run the test**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestRestore_MixedTerminalBrowserPanes -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/orchestrate/restore_test.go
git commit -m "test(restore): add mixed terminal+browser pane restoration test"
```

---

### Task 8: Update `show` command to display browser panes

The `show` command currently shows "shell" or the command for each pane. Browser panes should show the URL with a globe icon.

**Files:**
- Modify: `cmd/show.go` — update pane rendering

- [ ] **Step 1: Update pane display in `runShow`**

In `cmd/show.go`, update the pane description block (around line 82-96). Replace:

```go
			var desc string
			if p.Split != "" {
				desc = magentaStyle.Render("→"+p.Split) + " "
			}
			if p.Command != "" {
				cmd := p.Command
				if len(cmd) > 50 {
					cmd = cmd[:47] + "..."
				}
				desc += cyanStyle.Render(cmd)
			} else {
				desc += dimStyle.Render("shell")
			}
			if p.Focus {
				desc += " " + yellowStyle.Render("★")
			}
```

With:

```go
			var desc string
			if p.Split != "" {
				desc = magentaStyle.Render("→"+p.Split) + " "
			}
			if p.Type == "browser" {
				url := p.URL
				if len(url) > 50 {
					url = url[:47] + "..."
				}
				if url != "" {
					desc += cyanStyle.Render("🌐 " + url)
				} else {
					desc += dimStyle.Render("🌐 browser")
				}
			} else if p.Command != "" {
				cmd := p.Command
				if len(cmd) > 50 {
					cmd = cmd[:47] + "..."
				}
				desc += cyanStyle.Render(cmd)
			} else {
				desc += dimStyle.Render("shell")
			}
			if p.Focus {
				desc += " " + yellowStyle.Render("★")
			}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./cmd/crex/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/show.go
git commit -m "feat(show): display browser panes with globe icon and URL"
```

---

### Task 9: Update TUI save output to show browser pane count

When the TUI or CLI save reports "Saved X — N workspaces", it's useful to also indicate if browser panes were captured. This is optional polish but helps the user verify browser state was saved.

**Files:**
- Modify: `cmd/save.go` — update save output to mention browser panes

- [ ] **Step 1: Read `cmd/save.go`**

Read the file to see the current output format.

- [ ] **Step 2: Update save output**

In `cmd/save.go`, in the `runSave` function, after the save succeeds, count browser panes and mention them:

```go
	// Count browser panes for user feedback.
	browserCount := 0
	for _, ws := range layout.Workspaces {
		for _, p := range ws.Panes {
			if p.Type == "browser" {
				browserCount++
			}
		}
	}

	count := len(layout.Workspaces)
	label := unitName(count)
	msg := fmt.Sprintf("  ✓ Saved %q — %d %s", name, count, label)
	if browserCount > 0 {
		msg += fmt.Sprintf(" (%d browser)", browserCount)
	}
```

- [ ] **Step 3: Verify it compiles and run existing tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./cmd/crex/ && go test ./cmd/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/save.go
git commit -m "feat(save): show browser pane count in save output"
```

---

### Task 10: Full test suite and integration verification

Run the complete test suite to catch any regressions, then do a manual smoke test.

**Files:**
- No new files — verification only

- [ ] **Step 1: Run complete test suite**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./... -v -count=1`
Expected: All PASS

- [ ] **Step 2: Run `go vet`**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go vet ./...`
Expected: No issues

- [ ] **Step 3: Manual smoke test (if cmux is running)**

```bash
# Save current layout
crex save browser-smoke

# Show it to verify browser panes appear
crex show browser-smoke

# Dry-run restore to verify browser commands
crex restore --dry-run browser-smoke
```

- [ ] **Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: address test suite issues from browser pane restoration"
```
