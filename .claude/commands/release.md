---
description: Run the full crex release workflow — pre-flight, changelog, tag, CI release, post-release verification, and downstream updates.
argument-hint: "version (e.g. 1.7.0)"
---

Run the full crex release workflow for version **v$ARGUMENTS**. Execute every phase in order — do not skip steps or proceed past a failing gate.

## Phase 1: Pre-flight checks

All must pass before continuing.

```bash
go test ./... -count=1
```

```bash
go vet ./...
```

```bash
gofmt -l .
# Must produce no output
```

```bash
git status
# Must be clean (nothing uncommitted)
```

```bash
git log --oneline -1
# Must be on main, up to date with remote
```

If any check fails, fix the issue first. Do NOT proceed.

## Phase 2: Breaking change analysis

Compare all changes since the last tag:

```bash
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```

For each change, evaluate:
- **Renamed commands or flags?** — breaking unless old name kept as alias
- **Changed config format?** — breaking unless old format still accepted
- **Changed default behavior?** — document prominently
- **Removed functionality?** — breaking

Report the analysis to the user. If any breaking changes exist, ask for explicit confirmation before continuing.

## Phase 3: Changelog

Update `CHANGELOG.md`:

1. Add a new `## [v$ARGUMENTS] — YYYY-MM-DD` section after the `---` below the header
2. Categorize changes into Added, Changed, Fixed, Removed sections
3. Write entries in past tense, starting with bold component name
4. Add the version link at the bottom of the file: `[v$ARGUMENTS]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v$ARGUMENTS`
5. Commit: `git commit -am "docs: changelog for v$ARGUMENTS"`

## Phase 4: Tag and push

```bash
git tag -a v$ARGUMENTS -m "Release v$ARGUMENTS"
```

Ask the user for confirmation before pushing the tag. Then:

```bash
git push origin main
git push origin v$ARGUMENTS
```

This triggers GitHub Actions (`.github/workflows/release.yml`):
- Runs `go test ./...`
- Runs goreleaser: builds darwin amd64/arm64 binaries, creates GitHub Release, updates Homebrew tap

**NEVER run goreleaser locally** — it requires CI-only tokens.

## Phase 5: Wait for CI

Monitor the release workflow:

```bash
gh run list --workflow=release.yml --limit 3
```

Wait until the run completes. If it fails, investigate with:

```bash
gh run view <run-id> --log-failed
```

## Phase 6: Post-release verification

### 6a. Homebrew

```bash
brew update
brew upgrade drolosoft/tap/crex 2>/dev/null || brew install drolosoft/tap/crex
crex version
# Must show v$ARGUMENTS
```

### 6b. CLI validation

```bash
scripts/validate-demo.sh
# All checks must pass
```

### 6c. TUI visual tests

Run `/e2e-tui` — the full 27-case Playwright suite against the TUI. All screenshots must pass visual inspection.

## Phase 7: Downstream updates

### 7a. Drolosoft website

Remind the user:
> "Check the Drolosoft product page at `/Users/txeo/Git/mac/go/drolosoft` — update `web/templates/site/pages/products/cmux-resurrect.html` if any user-facing content changed (help text, commands, screenshots, demo GIF). Copy updated `assets/demo.gif` to `public/assets/crex-demo.gif` if re-recorded."

### 7b. Demo GIFs

If any user-facing output changed (help text, commands, TUI rendering):
> "Demo GIFs may need re-recording. The VHS tape files are ready — record them in Ghostty when convenient."

Do NOT run the recording scripts — the user records GIFs manually in Ghostty.

### 7c. Obsidian notes

Remind the user:
> "Update the crex version in your Obsidian vault if you track releases there."

## Phase 8: Final report

Summarize:
- Version released
- Breaking changes (if any)
- CI status
- Post-release verification results
- Downstream items that need manual attention
