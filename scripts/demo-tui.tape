# crex v1.5.0 interactive shell demo — recorded with github.com/charmbracelet/vhs
#
# Run via: ./scripts/record-demo-tui.sh (needs Ghostty or cmux running)

Output assets/demo-tui.gif
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false
Set Shell "bash"

# Hidden setup: force Ghostty branding + alias to demo binary
Hide
Type "unset CMUX_SOCKET_PATH CMUX_WORKSPACE_ID"
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

# ── Scene 2: Help ─────────────────────────────────────────

Type "help"
Enter
Sleep 3s

# ── Scene 3: List layouts with browse ─────────────────────

Type "ls"
Enter
Sleep 2s
# Browse with arrows
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 1s
# Quit browse
Type "q"
Sleep 1s

# ── Scene 4: Browse template gallery ─────────────────────

Type "templates"
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

# ── Scene 5: Blueprint management ────────────────────────

Type "bp list"
Enter
Sleep 2s
Type "q"
Sleep 1s

# ── Scene 6: Switch banner style ────────────────────────

Type "banner classic"
Enter
Sleep 2s

# ── Scene 7: Watch status ────────────────────────────────

Type "watch status"
Enter
Sleep 2s

# ── Scene 8: Exit ────────────────────────────────────────

Type "exit"
Enter
Sleep 2s
