# crex v1.5.0 interactive shell demo — recorded with github.com/charmbracelet/vhs
#
# Run via: ./scripts/record-demo-tui.sh (needs Ghostty or cmux running)
#
# NOTE: CREX_BRAND=crex forces crex branding while cmux handles live ops.

Output assets/demo-tui.gif
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false
Set Shell "bash"

# Hidden setup: force crex branding; alias to demo binary
Hide
Type "export CREX_BRAND=crex"
Enter
Sleep 200ms
Type "alias crex='/tmp/crex-demo-tui-bin --config /tmp/crex-demo-tui-env/config.toml'"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

# ── Scene 1: Launch the interactive shell ─────────────────

Type "# Interactive shell"
Enter
Sleep 500ms
Type "crex tui"
Enter
Sleep 2s

# Readable typing inside the TUI
Set TypingSpeed 50ms

# ── Scene 2: Help ─────────────────────────────────────────

Type "help"
Sleep 400ms
Enter
Sleep 3s

# ── Scene 3: List layouts with browse ─────────────────────

Type "ls"
Sleep 400ms
Enter
Sleep 2500ms
# Browse with arrows
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 1200ms
# Quit browse
Type "q"
Sleep 1s

# ── Scene 4: Restore a saved layout ──────────────────────

Type "restore my-day"
Sleep 400ms
Enter
Sleep 4s

# ── Scene 5: Show live state ──────────────────────────────

Type "now"
Sleep 400ms
Enter
Sleep 3s

# ── Scene 6: Browse template gallery ─────────────────────

Type "templates"
Sleep 400ms
Enter
Sleep 2s
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 1s
Type "q"
Sleep 1s

# ── Scene 7: Blueprint management ────────────────────────

Type "blueprint list"
Sleep 400ms
Enter
Sleep 2s
Type "q"
Sleep 1s

# ── Scene 8: Watch status ────────────────────────────────

Type "watch status"
Sleep 400ms
Enter
Sleep 2s

# ── Scene 9: Banner preview ─────────────────────────────

Type "banner set flame"
Sleep 400ms
Enter
Sleep 3s

# ── Scene 10: Exit ───────────────────────────────────────

Type "exit"
Sleep 400ms
Enter
Sleep 1s
