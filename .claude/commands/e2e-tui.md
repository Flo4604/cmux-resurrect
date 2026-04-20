---
description: Run E2E visual tests against the crex TUI via ttyd + Playwright. Inspects 27 test cases, reports issues, and fixes them.
---

Run the full E2E TUI test suite. This builds crex, starts ttyd, drives 27 test cases via Playwright, inspects every screenshot visually, and fixes any issues found.

## Prerequisites

- `ttyd` installed (`brew install ttyd`)
- Playwright available (bundled with the MCP server)
- Go toolchain for building crex

## Workflow

Execute these steps in order:

### 1. Build

```bash
mkdir -p /tmp/crex-e2e
go build -o /tmp/crex-e2e/crex-test ./cmd/crex
```

### 2. Start ttyd

```bash
pkill -f 'ttyd.*7682' 2>/dev/null; sleep 1
ttyd -W -p 7682 /tmp/crex-e2e/crex-test tui > /tmp/crex-e2e/ttyd.log 2>&1 &
```

Wait 2 seconds, then verify with `curl -s -o /dev/null -w "%{http_code}" http://localhost:7682` (expect 200).

### 3. Run the test runner

```bash
node scripts/e2e-tui-runner.js
```

This produces `/tmp/crex-e2e/report.json` and 27 screenshots in `/tmp/crex-e2e/screenshots/`.

### 4. Read and inspect every screenshot

Read the report at `/tmp/crex-e2e/report.json` to get the list of tests and their expected outcomes. Then read each screenshot file using the Read tool and inspect it visually.

For each screenshot, check:

1. **Content correctness** — the expected text, items, or UI elements are present (compare against the `expected` field in the report)
2. **Column alignment** — text columns are visually aligned, no ragged edges from ANSI escape codes
3. **Icon rendering** — emoji icons are visible (note: ttyd/xterm.js may render some emoji differently than Ghostty — flag only if icons are completely missing or replaced with boxes)
4. **Color coding** — green for command names, red for errors, dim/gray for descriptions, yellow/orange for group headers
5. **Prompt recovery** — the `crex ->` prompt is visible after every command (except exit)
6. **No rendering corruption** — no garbled text, no overlapping lines, no missing content

### 5. Report or fix

**If all 27 screenshots pass inspection:**
Report: "All 27 E2E cases clean. No visual issues found."

**If issues are found:**
For each issue:
1. Describe the symptom (e.g., "help text: Settings group header misaligned with Blueprint group")
2. Identify the source file and line responsible
3. Fix the code
4. Rebuild: `go build -o /tmp/crex-e2e/crex-test ./cmd/crex`
5. Restart ttyd: `pkill -f 'ttyd.*7682'; sleep 1; ttyd -W -p 7682 /tmp/crex-e2e/crex-test tui > /tmp/crex-e2e/ttyd.log 2>&1 &`
6. Re-run only the failing cases: `node scripts/e2e-tui-runner.js --cases 2,23` (comma-separated IDs)
7. Re-inspect the new screenshots
8. Maximum 2 fix-retry cycles per issue. If still broken after 2 attempts, report it as unfixed and move on.

**If an issue is a ttyd/xterm.js limitation (not a crex bug):**
Note it as a known rendering difference and skip it.

### 6. Cleanup

Always run this, even if earlier steps failed:

```bash
pkill -f 'ttyd.*7682' 2>/dev/null
```

The `/tmp/crex-e2e/` directory is left in place so screenshots can be reviewed after the command finishes. It is overwritten on the next run.
