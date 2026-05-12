# Single Workspace Restore via CLI

**Date:** 2026-05-12
**Status:** Approved

## Problem

Restoring a single workspace from a layout requires the interactive picker. There is no one-liner CLI syntax to target a specific workspace by name.

## Solution

Add an optional second positional argument to `crex restore` for workspace filtering with case-insensitive substring matching and dynamic tab completion.

## CLI Signature

```
crex restore [layout] [workspace]
```

| Args | Behavior |
|------|----------|
| 0 | Interactive picker (unchanged) |
| 1 | Restore all workspaces in layout (unchanged) |
| 2 | Restore single workspace from layout (new) |

### Examples

```bash
crex restore default trash        # restores "0 🗑️ Trash"
crex restore default "claude"     # restores "⠐ Claude Code"
crex restore default              # all workspaces
crex restore                      # interactive picker
```

## Tab Completion

### Arg 0 (layout name)

Unchanged — `completeLayoutNames` returns saved layouts with descriptions.

### Arg 1 (workspace name) — NEW

New `completeWorkspaceNames` function:

1. Reads `args[0]` (layout name already typed)
2. Loads layout from the store via `store.Load(args[0])`
3. Returns workspace titles with pane count as description:

```
0 🗑️ Trash         1 pane
⠐ Claude Code      1 pane
```

The `ValidArgsFunction` on the restore command dispatches to `completeLayoutNames` when `len(args) == 0` and `completeWorkspaceNames` when `len(args) == 1`. For `len(args) >= 2`, returns no completions.

## Matching

Case-insensitive substring match against workspace titles in `orchestrate.Restore()`.

| Match count | Behavior |
|-------------|----------|
| 1 | Restore that workspace |
| 0 | Error: `workspace "xyz" not found in layout "default"` |
| 2+ | Error listing candidates: `"trash" matches multiple workspaces in layout "default": "0 🗑️ Trash", "1 🗑️ Trash archive"` |

## Restore Mode for Single Workspace

Single workspace restores default to **add mode** — you're adding one workspace to your current session. `--mode replace` still works if explicitly passed but is unlikely for single-workspace use.

When `--mode` is not set and no config `restore_mode` exists, single-workspace restore skips the mode prompt entirely and uses add mode. Full-layout restore behavior is unchanged.

## Files Changed

| File | Change |
|------|--------|
| `cmd/restore.go` | `MaximumNArgs(1)` → `MaximumNArgs(2)`, read `args[1]` as workspace filter, update `ValidArgsFunction` to dispatch based on arg count, default to add mode when workspace filter is set |
| `cmd/completion_helpers.go` | New `completeWorkspaceNames(layoutName, toComplete)` function |
| `internal/orchestrate/restore.go` | Replace exact-match `workspaceFilter` with case-insensitive substring match, return ambiguity error when 2+ match |

No new files. Three existing files modified.

## Testing

- Unit test: substring matching logic (0, 1, 2+ matches)
- Unit test: `completeWorkspaceNames` returns correct titles
- Integration: `crex restore <layout> <workspace>` restores single workspace
- Integration: tab completion on second arg shows workspace list
- Edge cases: empty layout, workspace filter with special chars, ambiguous matches
