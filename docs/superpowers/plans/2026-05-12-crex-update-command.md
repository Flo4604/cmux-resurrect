# `crex update` Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `crex update` command that checks for new versions and updates crex, eliminating the need for users to manually run `brew upgrade`.

**Architecture:** `crex update` checks the latest GitHub release tag against the current binary version. If a newer version exists, it runs the appropriate update method: `brew upgrade drolosoft/tap/crex` for Homebrew installs, or `go install ...@latest` for Go installs. Includes `--check` flag to just report without updating. The version command also shows when an update is available.

**Tech Stack:** Go 1.26, GitHub API (releases/latest), `os/exec` (brew/go install)

---

## File Structure

| File | Responsibility | Action |
|------|---------------|--------|
| `cmd/update.go` | Cobra command for `crex update` | Create |
| `internal/update/update.go` | Version checking + update logic | Create |
| `internal/update/update_test.go` | Tests for version comparison | Create |
| `cmd/version.go` | Add update-available hint to version output | Modify |

---

### Task 1: Version comparison logic

Create `internal/update/update.go` with functions to check the latest release and compare versions.

**Files:**
- Create: `internal/update/update.go`
- Create: `internal/update/update_test.go`

- [ ] **Step 1: Write the failing test**

```go
package update

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current, latest string
		wantNewer       bool
	}{
		{"v1.11.0", "v1.12.0", true},
		{"v1.11.0", "v1.11.0", false},
		{"v1.12.0", "v1.11.0", false},
		{"v1.11.0", "v1.11.1", true},
		{"v1.9.9", "v1.10.0", true},
		{"dev", "v1.11.0", true},
		{"", "v1.11.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.current+"→"+tt.latest, func(t *testing.T) {
			got := IsNewer(tt.latest, tt.current)
			if got != tt.wantNewer {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.wantNewer)
			}
		})
	}
}
```

Run: `go test ./internal/update/ -run TestCompareVersions -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 2: Implement version comparison**

```go
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "drolosoft"
	repoName  = "cmux-resurrect"
	timeout   = 5 * time.Second
)

// IsNewer returns true if latest is a higher version than current.
// Handles "dev", empty strings, and "vX.Y.Z" format.
func IsNewer(latest, current string) bool {
	latestParts := parseVersion(latest)
	currentParts := parseVersion(current)
	if currentParts == nil {
		return latestParts != nil
	}
	if latestParts == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}

// CheckResult holds the result of a version check.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	UpdateAvailable bool
	ReleaseURL     string
}

// Check queries GitHub for the latest release and compares versions.
func Check(currentVersion string) (*CheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github API: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	return &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: IsNewer(release.TagName, currentVersion),
		ReleaseURL:      release.HTMLURL,
	}, nil
}

// InstallMethod detects how crex was installed.
type InstallMethod int

const (
	InstallBrew InstallMethod = iota
	InstallGo
	InstallUnknown
)

// DetectInstallMethod checks if crex was installed via Homebrew or Go.
func DetectInstallMethod() InstallMethod {
	// Check if managed by Homebrew.
	out, err := exec.Command("brew", "list", "crex").CombinedOutput()
	if err == nil && len(out) > 0 {
		return InstallBrew
	}
	out, err = exec.Command("brew", "list", "cmux-resurrect").CombinedOutput()
	if err == nil && len(out) > 0 {
		return InstallBrew
	}
	return InstallGo
}

// RunUpdate executes the update using the detected install method.
func RunUpdate(method InstallMethod) error {
	switch method {
	case InstallBrew:
		cmd := exec.Command("brew", "upgrade", "drolosoft/tap/crex")
		cmd.Stdout = nil
		cmd.Stderr = nil
		return cmd.Run()
	case InstallGo:
		cmd := exec.Command("go", "install", "github.com/drolosoft/cmux-resurrect/cmd/crex@latest")
		cmd.Stdout = nil
		cmd.Stderr = nil
		return cmd.Run()
	default:
		return fmt.Errorf("could not detect install method — update manually")
	}
}

func (m InstallMethod) String() string {
	switch m {
	case InstallBrew:
		return "Homebrew"
	case InstallGo:
		return "go install"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/update/ -run TestCompareVersions -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add version checking and update logic"
```

---

### Task 2: Create `crex update` command

**Files:**
- Create: `cmd/update.go`

- [ ] **Step 1: Create the command**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/update"
	"github.com/spf13/cobra"
)

var updateCheckOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and upgrade crex",
	Long:  "Checks GitHub for the latest release. Without --check, downloads and installs the update via Homebrew or go install.",
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "just check for updates without installing")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "  Checking for updates...\n")

	result, err := update.Check(version)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	if !result.UpdateAvailable {
		fmt.Fprintf(os.Stderr, "\n  %s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ crex %s is the latest version", result.CurrentVersion)))
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n  %s → %s\n",
		dimStyle.Render(result.CurrentVersion),
		greenStyle.Render(result.LatestVersion))

	if updateCheckOnly {
		fmt.Fprintf(os.Stderr, "  %s\n\n",
			dimStyle.Render("Run 'crex update' to install"))
		return nil
	}

	method := update.DetectInstallMethod()
	fmt.Fprintf(os.Stderr, "  Updating via %s...\n", method)

	if err := update.RunUpdate(method); err != nil {
		return fmt.Errorf("update failed: %w\n  Try manually: brew upgrade drolosoft/tap/crex", err)
	}

	fmt.Fprintf(os.Stderr, "\n  %s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Updated to %s", result.LatestVersion)))
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: PASS — `version` variable should already be defined in `cmd/version.go` or `cmd/root.go` (check where the goreleaser ldflags set it).

- [ ] **Step 3: Commit**

```bash
git add cmd/update.go
git commit -m "feat: add crex update command with --check flag"
```

---

### Task 3: Add update hint to version output

Show "Update available: vX.Y.Z" when `crex version` detects a newer release (non-blocking, best-effort).

**Files:**
- Modify: `cmd/version.go`

- [ ] **Step 1: Read current version command**

Read `cmd/version.go` to understand the current output format.

- [ ] **Step 2: Add update check to version output**

After displaying the version banner, do a non-blocking check:

```go
// Best-effort update check — don't block or error on failure.
go func() {
	result, err := update.Check(version)
	if err == nil && result.UpdateAvailable {
		fmt.Fprintf(os.Stderr, "  %s\n\n",
			yellowStyle.Render(fmt.Sprintf("⬆ Update available: %s → run 'crex update'", result.LatestVersion)))
	}
}()
```

Note: since this is `version` (quick command), use a short goroutine with a channel/timeout so it doesn't hang if the network is slow. Cap at 2 seconds.

- [ ] **Step 3: Commit**

```bash
git add cmd/version.go
git commit -m "feat(version): show update-available hint"
```

---

### Task 4: Add TUI `update` command

Wire the update command into the interactive shell so `crex❯ update` works.

**Files:**
- Modify: `internal/tui/shell_commands.go` — add "update" to command dispatch
- Modify: `internal/tui/shell_exec.go` — add `execUpdate` method
- Modify: `internal/tui/shell_help.go` — add help entry

- [ ] **Step 1: Add update to shell dispatch**

In `shell_commands.go`, add case for "update" that calls `m.execUpdate()`.

- [ ] **Step 2: Add execUpdate method**

```go
func (m *ShellModel) execUpdate() {
	m.output.WriteString(shellDimStyle.Render("  Checking for updates..."))
	m.output.WriteString("\n")

	result, err := update.Check(version)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	if !result.UpdateAvailable {
		m.output.WriteString(shellSuccessStyle.Render(
			fmt.Sprintf("  ✓ crex %s is the latest version", result.CurrentVersion)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellSuccessStyle.Render(
		fmt.Sprintf("  ⬆ %s available — run 'crex update' from your shell", result.LatestVersion)))
	m.output.WriteString("\n\n")
}
```

Note: the TUI should only check, not run the actual update (which needs to replace the running binary).

- [ ] **Step 3: Add help entry**

In `shell_help.go`, add to the help table:

```go
{"⬆", "update", "", func(b client.DetectedBackend) string { return "Check for updates" }, "System"},
```

- [ ] **Step 4: Run tests**

Run: `go test ./... -count=1`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/shell_commands.go internal/tui/shell_exec.go internal/tui/shell_help.go
git commit -m "feat(tui): add update command to interactive shell"
```

---

### Task 5: Full test suite and verification

- [ ] **Step 1: Run complete test suite**

Run: `go test ./... -v -count=1`
Expected: All PASS

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: Clean

- [ ] **Step 3: Manual test**

```bash
go run ./cmd/crex update --check    # should show "update available" or "latest"
go run ./cmd/crex version           # should show update hint if available
```

- [ ] **Step 4: Commit any fixups**

```bash
git add -A
git commit -m "fix: address issues from update command testing"
```
