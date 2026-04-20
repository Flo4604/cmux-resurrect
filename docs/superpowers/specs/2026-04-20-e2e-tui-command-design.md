# E2E TUI Slash Command — Design Spec

**Date:** 2026-04-20
**Status:** Approved
**Scope:** Claude Code slash command + Playwright runner script

## Problem

Unit tests verify TUI logic but not rendering. Bubble Tea's inline renderer can silently corrupt output (the `tea.Println` bug in v1.5.0 is the canonical example). The only reliable way to catch visual regressions is to drive the TUI in a real terminal and inspect the rendered output. This is currently a manual, ad-hoc process documented in `docs/tui-testing.md`.

## Solution

A Claude Code slash command `/e2e-tui` that automates the full test matrix: build crex, start ttyd, drive 26 test cases via Playwright, screenshot each step, visually inspect every screenshot, and report or fix issues found.

## Architecture

### Files

| File | Purpose |
|------|---------|
| `.claude/commands/e2e-tui.md` | Slash command prompt — instructs Claude on the full workflow |
| `scripts/e2e-tui-runner.js` | Playwright script — sends inputs, captures screenshots, outputs report |

### Why two files

The runner script handles mechanical work (browser automation, input sequencing, screenshot capture). It outputs structured JSON and screenshots. Claude handles the intelligent part: reading screenshots, judging visual correctness, identifying root causes, and fixing code.

### Dependencies

- `ttyd` (installed via Homebrew)
- Playwright (available via the MCP server's `node_modules`)
- Go toolchain (to build crex)

## Workflow

```
/e2e-tui
  |
  1. Build:    go build -o /tmp/crex-e2e/crex-test ./cmd/crex
  2. Start:    ttyd -W -p 7682 /tmp/crex-e2e/crex-test tui
  3. Run:      node scripts/e2e-tui-runner.js
  4. Inspect:  Read each screenshot, compare to expectations
  5. Fix:      If issues found — fix source, rebuild, re-run failing cases
  6. Report:   Summary of results
  7. Cleanup:  Kill ttyd, remove /tmp/crex-e2e
```

## Runner Script

### Input sequencing

The runner navigates to `http://localhost:7682`, waits for xterm.js initialization, then executes each test case in sequence. Each case:

1. Clears the prompt (`\x15` = Ctrl+U)
2. Sends the input (using `window.term.input()`)
3. Waits for rendering (configurable delay per case)
4. Takes a screenshot
5. Records the result in the report

### Known ttyd quirks

- Enter must be `\r`, not `\n` (Bubble Tea raw mode)
- The word `delete` gets swallowed by ttyd/xterm.js — must be sent as split input (`delet` + `e\r`)
- Browse mode quit is `q` without `\r`
- Special keys: Arrow Up = `\x1b[A`, Escape = `\x1b`, Tab = `\t`

### Output

Directory: `/tmp/crex-e2e/`

```
/tmp/crex-e2e/
  crex-test              # built binary
  report.json            # structured test results
  screenshots/
    01-launch.png
    02-help.png
    ...
    26-exit.png
```

`report.json` format:

```json
{
  "timestamp": "2026-04-20T16:00:00Z",
  "binary": "/tmp/crex-e2e/crex-test",
  "tests": [
    {
      "id": 1,
      "name": "launch",
      "screenshot": "screenshots/01-launch.png",
      "input": null,
      "expected": "Welcome message with 'crex' prompt, help/exit highlighted",
      "status": "captured"
    }
  ]
}
```

All tests report `status: "captured"`. The runner does not judge pass/fail — Claude does that by reading the screenshots.

## Test Matrix

26 cases covering the full matrix from `docs/tui-testing.md`:

### Core Shell (1-6)

| # | Input | Expected | Validates |
|---|-------|----------|-----------|
| 1 | *(launch)* | Welcome message + `crex ->` prompt | Init, welcome rendering |
| 2 | `help\r` | 6 groups with icons, colored headers, aligned columns | Multi-line output, ANSI alignment |
| 3 | `ls\r` | Numbered items in browse mode with cursor | Browse mode entry |
| 4 | `q` | Return to prompt (no `\r`) | Browse exit |
| 5 | `templates\r` | 16 templates in browse mode | Template listing |
| 6 | `q` | Return to prompt | Browse exit consistency |

### Backend-dependent (7-8)

| # | Input | Expected in ttyd | Validates |
|---|-------|-----------------|-----------|
| 7 | `now\r` | Error message (no backend in ttyd) | Error display + prompt recovery |
| 8 | `save test\r` | Error message | Save error path |

### Usage errors (9-14)

| # | Input | Expected | Validates |
|---|-------|----------|-----------|
| 9 | `restore\r` | `Usage: restore <name\|#>` | Arg validation |
| 10 | `delet` + `e\r` | `Usage: delete <name\|#>` | Arg validation (split for ttyd quirk) |
| 11 | `use\r` | `Usage: use <template\|#>` | Arg validation |
| 12 | `bp add\r` | `Usage: bp add <name> <path>` | Arg validation |
| 13 | `bp remove\r` | `Usage: bp remove <name\|#>` | Arg validation |
| 14 | `bp toggle\r` | `Usage: bp toggle <name\|#>` | Arg validation |

### Features (15-17)

| # | Input | Expected | Validates |
|---|-------|----------|-----------|
| 15 | `watch status\r` | `watch daemon is not running` | Watch status |
| 16 | `bp list\r` | Entries in browse mode (or empty message) | Blueprint listing |
| 17 | `foobar\r` | `Unknown command: foobar` | Unknown command handling |

### Tab Completion (18-21)

| # | Input | Expected | Validates |
|---|-------|----------|-----------|
| 18 | `\t` (Tab on empty) | All commands with icons and descriptions | Level 1 completion |
| 19 | `\x1b` (Escape) | Completion dismissed | Completion dismiss |
| 20 | `bp` + `\t` | bp subcommands (add, list, ls, remove, rm, toggle) | Level 2 completion |
| 21 | `settings banner` + `\t` | banner subcommands (get, list, set) | Level 3 completion |

### Settings Output (22-23)

| # | Input | Expected | Validates |
|---|-------|----------|-----------|
| 22 | `settings banner get\r` | `Current banner style: flame` (green) | Settings get rendering |
| 23 | `settings banner list\r` | 3 styles with aligned descriptions | Settings list rendering |

### Edge Cases (24-26)

| # | Input | Expected | Validates |
|---|-------|----------|-----------|
| 24 | `\x1b[A` x2 (Arrow Up) | Recalls previous commands in prompt | History navigation |
| 25 | `\r` (empty) | No output, stays in prompt | Empty input handling |
| 26 | `exit\r` | Phoenix bye message, ttyd reconnect | Clean exit |

## Slash Command Behavior

### Visual inspection criteria

For each screenshot, Claude checks:

1. **Content correctness** — expected text/items are present
2. **Column alignment** — no ANSI escape code misalignment
3. **Icon rendering** — emoji icons visible (accounting for ttyd/xterm.js rendering differences vs Ghostty)
4. **Color coding** — green for commands, red for errors, dim for descriptions, yellow/orange for headers
5. **Prompt recovery** — `crex ->` prompt returns after every command
6. **No rendering corruption** — no garbled text, no overlapping lines, no missing content

### Fix workflow

When an issue is found:

1. Identify the symptom (e.g. "help text column 3 misaligned")
2. Trace to the source file and line (e.g. `internal/tui/shell_help.go:45`)
3. Fix the code
4. Rebuild: `go build -o /tmp/crex-e2e/crex-test ./cmd/crex`
5. Restart ttyd (kill + relaunch)
6. Re-run only the failing test cases
7. Re-inspect the new screenshots
8. If fixed, continue. If not, report and move on (don't loop more than twice per issue).

### Reporting

**All pass:** "All 26 E2E cases clean. No visual issues found."

**Issues found and fixed:** List each issue with before/after, plus the fix applied.

**Issues found, not fixable:** List each issue with the screenshot reference and why it couldn't be auto-fixed (e.g. ttyd rendering limitation vs actual bug).

## Constraints

- Port 7682 (avoids conflicts with other services)
- Playwright uses the MCP server's bundled installation (no separate install needed)
- Screenshots stay in `/tmp/crex-e2e/` — not committed to git
- The runner script is committed to the repo (`scripts/e2e-tui-runner.js`)
- The slash command is committed to the repo (`.claude/commands/e2e-tui.md`)
- Maximum 2 fix-retry cycles per issue to avoid infinite loops
- ttyd is always killed on completion (including on error/interrupt)

## Out of Scope

- CI integration (this is an interactive Claude tool, not a CI step)
- Testing in Ghostty directly (ttyd provides the terminal, Ghostty rendering differences are documented as known)
- Testing CLI commands (covered by `scripts/validate-demo.sh`)
- Automated re-recording of demo GIFs (user does this manually in Ghostty)
