# E2E TUI Slash Command — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a `/e2e-tui` slash command that runs 26 visual test cases against the crex TUI via ttyd + Playwright, reads screenshots, and fixes issues found.

**Architecture:** A Playwright Node.js runner script (`scripts/e2e-tui-runner.js`) captures screenshots mechanically. A Claude Code slash command (`.claude/commands/e2e-tui.md`) orchestrates the full workflow: build, ttyd lifecycle, runner execution, visual inspection, fix cycle, and cleanup.

**Tech Stack:** Node.js, Playwright (from MCP server's `node_modules`), ttyd, Go build toolchain

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `scripts/e2e-tui-runner.js` | Create | Playwright automation: navigate ttyd, send 26 test inputs, screenshot each, write `report.json` |
| `.claude/commands/e2e-tui.md` | Create | Slash command prompt: build/ttyd/runner orchestration, visual inspection, fix workflow |

---

### Task 1: Create the Playwright runner script

**Files:**
- Create: `scripts/e2e-tui-runner.js`

- [ ] **Step 1: Create the runner script with all 26 test cases**

```js
// scripts/e2e-tui-runner.js
//
// E2E TUI test runner — drives crex TUI via ttyd + Playwright.
// Captures a screenshot for each of 26 test cases and writes report.json.
//
// Usage: node scripts/e2e-tui-runner.js [--port 7682] [--cases 1,2,5]
//
// Requires:
//   - ttyd running on the specified port with crex tui
//   - Playwright (uses the MCP server's bundled installation)

const PLAYWRIGHT_PATH = '/Users/txeo/.nvm/versions/node/v23.11.0/lib/node_modules/@automatalabs/mcp-server-playwright/node_modules/playwright';
const { chromium } = require(PLAYWRIGHT_PATH);
const fs = require('fs');
const path = require('path');

// --- CLI args ---

const args = process.argv.slice(2);
let port = 7682;
let caseFilter = null;

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--port' && args[i + 1]) port = parseInt(args[i + 1], 10);
  if (args[i] === '--cases' && args[i + 1]) caseFilter = new Set(args[i + 1].split(',').map(Number));
}

const TTYD_URL = `http://localhost:${port}`;
const OUT_DIR = '/tmp/crex-e2e';
const SHOT_DIR = path.join(OUT_DIR, 'screenshots');

// --- Test definitions ---

const TESTS = [
  // Core Shell (1-6)
  { id: 1,  name: 'launch',              input: null,                        wait: 3000, expected: "Welcome message with 'crex' prompt, help/exit highlighted" },
  { id: 2,  name: 'help',                input: 'help\r',                    wait: 1500, expected: '6 groups with icons, colored headers, aligned columns' },
  { id: 3,  name: 'ls-browse',           input: 'ls\r',                      wait: 1500, expected: 'Numbered items in browse mode with cursor' },
  { id: 4,  name: 'browse-quit',         input: 'q',                         wait: 500,  expected: 'Return to prompt' },
  { id: 5,  name: 'templates-browse',    input: 'templates\r',               wait: 1500, expected: '16 templates in browse mode' },
  { id: 6,  name: 'templates-quit',      input: 'q',                         wait: 500,  expected: 'Return to prompt' },
  // Backend-dependent (7-8)
  { id: 7,  name: 'now-error',           input: 'now\r',                     wait: 3000, expected: 'Error message (no backend in ttyd), prompt recovery' },
  { id: 8,  name: 'save-error',          input: 'save test\r',               wait: 3000, expected: 'Error message, prompt recovery' },
  // Usage errors (9-14)
  { id: 9,  name: 'usage-restore',       input: 'restore\r',                 wait: 1000, expected: 'Red error: Usage: restore <name|#>' },
  { id: 10, name: 'usage-delete',        input: ['delet', 'e\r'],            wait: 1000, expected: 'Red error: Usage: delete <name|#>' },
  { id: 11, name: 'usage-use',           input: 'use\r',                     wait: 1000, expected: 'Red error: Usage: use <template|#>' },
  { id: 12, name: 'usage-bp-add',        input: 'bp add\r',                  wait: 1000, expected: 'Red error: Usage: bp add <name> <path>' },
  { id: 13, name: 'usage-bp-remove',     input: 'bp remove\r',               wait: 1000, expected: 'Red error: Usage: bp remove <name|#>' },
  { id: 14, name: 'usage-bp-toggle',     input: 'bp toggle\r',               wait: 1000, expected: 'Red error: Usage: bp toggle <name|#>' },
  // Features (15-17)
  { id: 15, name: 'watch-status',        input: 'watch status\r',            wait: 1000, expected: 'watch daemon is not running' },
  { id: 16, name: 'bp-list',             input: 'bp list\r',                 wait: 1000, expected: 'Blueprint entries in browse mode or empty message' },
  { id: 17, name: 'unknown-cmd',         input: 'foobar\r',                  wait: 1000, expected: 'Red error: Unknown command: foobar' },
  // Tab Completion (18-21)
  { id: 18, name: 'tab-level1',          input: '\t',                        wait: 1000, expected: 'All commands with icons and descriptions' },
  { id: 19, name: 'tab-escape',          input: '\x1b',                      wait: 500,  expected: 'Completion dismissed, back to prompt' },
  { id: 20, name: 'tab-bp-level2',       input: { pre: 'bp', tab: true },    wait: 1000, expected: 'bp subcommands: add, list, ls, remove, rm, toggle' },
  { id: 21, name: 'tab-settings-level3', input: { pre: 'settings banner', tab: true }, wait: 1000, expected: 'banner subcommands: get, list, set' },
  // Settings Output (22-23)
  { id: 22, name: 'settings-banner-get', input: 'settings banner get\r',     wait: 1000, expected: 'Current banner style: flame (green text)' },
  { id: 23, name: 'settings-banner-list',input: 'settings banner list\r',    wait: 1000, expected: '3 styles with aligned descriptions' },
  // Edge Cases (24-26)
  { id: 24, name: 'history-up',          input: ['\x1b[A', '\x1b[A'],        wait: 500,  expected: 'Recalls previous commands in prompt' },
  { id: 25, name: 'empty-enter',         input: '\r',                        wait: 500,  expected: 'No output, stays in prompt' },
  { id: 26, name: 'exit',                input: 'exit\r',                    wait: 2000, expected: 'Phoenix bye message, ttyd shows reconnect' },
];

// --- Helpers ---

async function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

async function clearPrompt(page) {
  await page.evaluate(() => window.term.input('\x15'));
  await sleep(200);
}

async function sendInput(page, input) {
  if (input === null) return; // launch — no input needed
  if (Array.isArray(input)) {
    for (const chunk of input) {
      await page.evaluate((c) => window.term.input(c), chunk);
      await sleep(300);
    }
  } else if (typeof input === 'object' && input.tab) {
    await page.evaluate((t) => window.term.input(t), input.pre);
    await sleep(300);
    await page.evaluate(() => window.term.input('\t'));
  } else {
    await page.evaluate((t) => window.term.input(t), input);
  }
}

async function shot(page, name) {
  const file = path.join(SHOT_DIR, name);
  await page.screenshot({ path: file, fullPage: true });
}

// --- Main ---

(async () => {
  fs.mkdirSync(SHOT_DIR, { recursive: true });

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1200, height: 800 } });

  console.log(JSON.stringify({ event: 'start', url: TTYD_URL }));

  await page.goto(TTYD_URL);
  await sleep(3000); // wait for xterm.js + crex init

  const tests = TESTS.filter(t => !caseFilter || caseFilter.has(t.id));
  const results = [];

  for (const t of tests) {
    const padId = String(t.id).padStart(2, '0');
    const screenshotName = `${padId}-${t.name}.png`;

    // Clear prompt before each test (except launch and cases that follow browse quit)
    if (t.id > 1 && t.input !== 'q' && !(Array.isArray(t.input) && t.input[0] === '\x1b[A')) {
      await clearPrompt(page);
    }

    await sendInput(page, t.input);
    await sleep(t.wait);
    await shot(page, screenshotName);

    results.push({
      id: t.id,
      name: t.name,
      screenshot: `screenshots/${screenshotName}`,
      expected: t.expected,
      status: 'captured',
    });

    console.log(JSON.stringify({ event: 'captured', id: t.id, name: t.name, screenshot: screenshotName }));
  }

  const report = {
    timestamp: new Date().toISOString(),
    port,
    testCount: results.length,
    tests: results,
  };

  fs.writeFileSync(path.join(OUT_DIR, 'report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ event: 'done', testCount: results.length, reportPath: path.join(OUT_DIR, 'report.json') }));

  await browser.close();
})().catch(err => {
  console.error(JSON.stringify({ event: 'error', message: err.message }));
  process.exit(1);
});
```

- [ ] **Step 2: Verify the script runs against a live ttyd instance**

```bash
go build -o /tmp/crex-e2e/crex-test ./cmd/crex
pkill -f 'ttyd.*7682' 2>/dev/null; sleep 1
ttyd -W -p 7682 /tmp/crex-e2e/crex-test tui > /dev/null 2>&1 &
sleep 2
node scripts/e2e-tui-runner.js
pkill -f 'ttyd.*7682'
```

Expected: 26 lines of `{"event":"captured",...}` JSON followed by `{"event":"done","testCount":26,...}`. Screenshots in `/tmp/crex-e2e/screenshots/`.

- [ ] **Step 3: Spot-check a few screenshots visually**

Read `/tmp/crex-e2e/screenshots/01-launch.png`, `02-help.png`, `18-tab-level1.png`, and `26-exit.png` to confirm they captured the crex TUI (not a bare zsh shell).

- [ ] **Step 4: Commit the runner script**

```bash
git add scripts/e2e-tui-runner.js
git commit -m "feat: add E2E TUI Playwright runner script (26 test cases)"
```

---

### Task 2: Create the slash command

**Files:**
- Create: `.claude/commands/e2e-tui.md`

- [ ] **Step 1: Write the slash command prompt**

```markdown
---
description: Run E2E visual tests against the crex TUI via ttyd + Playwright. Inspects 26 test cases, reports issues, and fixes them.
---

Run the full E2E TUI test suite. This builds crex, starts ttyd, drives 26 test cases via Playwright, inspects every screenshot visually, and fixes any issues found.

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

This produces `/tmp/crex-e2e/report.json` and 26 screenshots in `/tmp/crex-e2e/screenshots/`.

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

**If all 26 screenshots pass inspection:**
Report: "All 26 E2E cases clean. No visual issues found."

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
```

- [ ] **Step 2: Verify the slash command file is valid**

Read `.claude/commands/e2e-tui.md` and confirm:
- Frontmatter has `description` field
- No `argument-hint` needed (no arguments)
- Instructions are self-contained (Claude can follow them without external context)

- [ ] **Step 3: Commit the slash command**

```bash
git add .claude/commands/e2e-tui.md
git commit -m "feat: add /e2e-tui slash command for visual TUI regression testing"
```

---

### Task 3: Smoke test the full workflow

- [ ] **Step 1: Run `/e2e-tui` end-to-end**

Invoke the slash command and verify:
- Build succeeds
- ttyd starts and responds on port 7682
- Runner captures all 26 screenshots
- Screenshots show the crex TUI (not a bare shell)
- Report JSON is well-formed
- ttyd is cleaned up after

- [ ] **Step 2: Fix any issues discovered during the smoke test**

If the runner or slash command has issues (wrong port, timing problems, missing screenshots), fix them and re-run.

- [ ] **Step 3: Final commit if any fixes were needed**

```bash
git add -u
git commit -m "fix: e2e-tui runner adjustments from smoke test"
```
