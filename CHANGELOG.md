# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.6.0] — 2026-04-20

### Added
- Level-aware tab completion engine in the interactive shell — 3-level depth with icons, descriptions, and argument auto-completion
- Tab/Shift-Tab/Up/Down cycling through completion options; Escape navigates back through levels
- `settings` command group in the TUI — banner configuration moved under `settings > banner > set/get/list`
- Random phoenix-themed farewell messages on exit (15 variants)
- Command header shown in dim after dispatch so you always know what ran
- Usage errors keep the command in the prompt for easy retry
- Confirmation dialogs for destructive operations (`delete`, `bp remove`)
- Prompt always visible at top during browse and confirm modes

### Changed
- Banner commands reorganized from top-level `banner` to `settings banner` inside the TUI (CLI `crex banner` unchanged)
- Bare group commands (`bp`, `settings`) now show subcommands on Enter instead of "Unknown command"
- Shell prompt styled as green `crex` + orange `→`

### Fixed
- Stale completion lists no longer persist after typing or navigating away
- Tab cycling state preserved correctly when using Escape to navigate back

---

## [v1.5.1] — 2026-04-19

### Added
- Quick-start demo GIF showing setup, save, and list in a single flow

### Changed
- CLI output now adds trailing newline spacing for better readability in all contexts
- Shell alias generation updated to reflect the `crex` command name consistently

### Fixed
- Banner was invisible on dark terminal backgrounds — switched to an always-visible color
- Added missing vertical spacing around the banner in several output paths

---

## [v1.5.0] — 2026-04-18

### Added
- Interactive shell mode (`crex shell`) — a persistent REPL with history, prompt, and command dispatch
- `crex tui` — full Bubble Tea launcher with fuzzy filtering, arrow navigation, and action menu
- `crex setup` wizard — detects your terminal backend and writes a config file automatically
- `watch` daemon mode with `--daemon`, `--stop`, and `--status` flags; PID file and log rotation included
- Shell hook generation for zsh, bash, and fish via `crex watch --shell-hook`
- `blueprint` command (replaces `workspace`) with `bp` shorthand alias
- Backend-adaptive labels via `unitName()` — output reads naturally for both cmux and Ghostty users
- Browse model with arrow navigation, live filtering, and cursor highlight inside the shell

### Changed
- `workspace` subcommand renamed to `blueprint`; the old name is no longer supported
- All user-facing output now routes through `unitName()` for consistent backend-aware wording
- Replaced the previous TUI with the new two-level shell/browse architecture
- Bubbletea and Bubbles promoted to direct dependencies (were previously indirect)

### Fixed
- Setup config writer now calls `MkdirAll` before writing to avoid permission errors on fresh installs
- Shell output uses `tea.Println` to prevent inline rendering corruption in Bubble Tea context
- All `golangci-lint` warnings resolved so CI stays green

---

## [v1.4.0] — 2026-04-18

### Added
- Workspace `Description` field — persisted to the Blueprint file and merged on save without overwriting manual edits
- Adaptive theme system with configurable banner styles (light/dark/auto)
- Template shortcut flags for faster one-liner workspace creation from the CLI

### Changed
- `crex template show` output polished — cleaner layout, consistent spacing
- Gallery screenshots added to project docs

---

## [v1.3.0] — 2026-04-17

### Added
- Ghostty backend — save, restore, templates, Workspace Blueprints, watch, and dry-run all work on Ghostty via AppleScript
- Auto-detection of the active terminal; no flags or config changes required
- Backend-aware `--dry-run` — shows actual cmux CLI commands or Ghostty AppleScript depending on your terminal
- 13-method `Backend` interface providing a clean abstraction layer for current and future terminals
- Conditional branding: Ghostty users see just `crex`; cmux users see `crex (cmux-resurrect)`

### Changed
- All core operations delegated through the Backend interface, removing direct cmux assumptions from orchestration logic

---

## [v1.2.0] — 2026-04-17

### Added
- Built-in template gallery with 16 embedded workspace layouts (dev, web, data, quad, and more)
- `crex template list` — browse available templates with category, icon, and description
- `crex template show <name>` — display an ASCII diagram of any template layout
- `crex template use <name>` — one-shot workspace creation from a gallery template
- `crex template customize <name>` — copy a gallery template into your Blueprint for local editing
- `FocusTarget` support in the orchestrator for complex layouts (e.g. quad) that need a specific pane focused on launch
- Tag-based completion and styled help output for the template command group

### Changed
- `DefaultTemplates` simplified to `dev` + `single`; the full gallery is now resolved from the embedded package
- `ws add` completion and help updated to reference the template gallery

### Fixed
- `parseTemplatePaneLine` now initializes `FocusTarget` to `-1` to avoid false-zero confusion

---

## [v1.1.1] — 2026-04-15

### Changed
- README: added Blueprint Markdown format snippet showing the actual file structure
- README: added save-my-day screenshot to the Save & Restore section
- README: updated star counts for cmux (14K+) and tmux-resurrect (12.7K) references
- README: one-liner Homebrew install block added; "Try it" order corrected (save before restore)
- README: added macOS-only platform note and data file location
- README: removed unverifiable marketing claim

---

## [v1.1.0] — 2026-04-14

### Added
- Shell completion for bash, zsh, fish, and PowerShell via `crex completion <shell>`

---

## [v1.0.5] — 2026-04-11

### Added
- Buy Me a Coffee support link and GitHub funding config
- Project logo (phoenix) in the README header
- Import success screenshot in the README

### Changed
- Example layout renamed from `work`/`demo` to `my-day` across all docs, help text, and demo recordings
- YouTube demo URL updated to match the new `my-day` layout recording
- CI: switched to golangci-lint v2 built from source for Go 1.26 compatibility

---

## [v1.0.4] — 2026-04-02

### Fixed
- Help text clarified the relationship between `crex` and `cmux-resurrect`, including the `go install` symlink step

---

## [v1.0.3] — 2026-04-02

### Changed
- Help text updated to note that both `crex` and `cmux-resurrect` are valid command names after installation

---

## [v1.0.2] — 2026-04-02

### Changed
- Both `crex` and `cmux-resurrect` binary names are now installed in all installation methods (Homebrew, `go install`)
- README installation instructions unified around the `crex` binary name

---

## [v1.0.1] — 2026-04-02

### Changed
- Moved entrypoint to `cmd/crex` so the binary installs as `crex` consistently across all package managers

---

## [v1.0.0] — 2026-04-02

Initial public release.

### Added
- `crex save <name>` — snapshot all open cmux windows and panes into a named Blueprint file
- `crex restore <name>` — recreate a saved workspace from a Blueprint, including split layout and pane focus
- `crex import-from-md` — create a live workspace directly from a Blueprint Markdown file
- `crex watch` — background file-watcher that auto-imports on Blueprint changes
- `crex ls` — list saved workspaces with metadata
- `crex delete <name>` — remove a saved workspace
- `--dry-run` flag — preview the exact commands that would run without making any changes
- `--workspace-file` global flag — point crex at a custom Blueprint location
- ASCII banner and styled help output
- Interactive restore picker for selecting among saved workspaces
- Workspace Blueprint format (Markdown-based, human-readable and hand-editable)
- Homebrew tap (`drolosoft/tap/cmux-resurrect`) and GoReleaser release workflow
- Cross-platform compilation targets (macOS arm64/amd64, Linux arm64/amd64)
- Shell completion scaffolding

### Fixed
- Restore now targets split surfaces explicitly and preserves the caller's active workspace
- Sync reliability improved: workspace refs, deferred rename, and pin support all stabilized
- Double-`v` in the version string output corrected

---

[v1.6.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.6.0
[v1.5.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.5.1
[v1.5.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.5.0
[v1.4.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.4.0
[v1.3.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.3.0
[v1.2.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.2.0
[v1.1.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.1.1
[v1.1.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.1.0
[v1.0.5]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.5
[v1.0.4]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.4
[v1.0.3]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.3
[v1.0.2]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.2
[v1.0.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.1
[v1.0.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.0
