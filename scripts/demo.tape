# crex demo — recorded with github.com/charmbracelet/vhs
#
# Run via: ./scripts/record-demo.sh (needs a supported terminal backend)

Output assets/demo.gif
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 20ms
Set CursorBlink false
Set Shell "bash"

# Hidden setup: alias crex to use demo config; enable bash comments
Hide
Type "shopt -s interactive_comments"
Enter
Sleep 200ms
Type "unset CMUX_SOCKET_PATH CMUX_WORKSPACE_ID"
Enter
Sleep 200ms
Type "alias crex='/usr/local/bin/crex --config /tmp/crex-demo/config.toml'"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

# ── Scene 1: Help ──────────────────────────────────────────

Type "# Show available commands"
Enter
Sleep 500ms
Type "crex --help"
Enter
Sleep 3s

# ── Scene 2: Show the Blueprint ───────────────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# View the Blueprint"
Enter
Sleep 500ms
Type "bat --style=plain /tmp/crex-demo/workspaces.md"
Enter
Sleep 3s

# ── Scene 3: Import from Blueprint (real) ──────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Create workspaces from the Blueprint"
Enter
Sleep 500ms
Type "crex import-from-md"
Enter
Sleep 3s

# ── Scene 4: Save + List ─────────────────────────────────

# Hidden: save once and swap in clean toml (avoids leaking real tabs)
Hide
Type "crex save my-day 2>/dev/null"
Enter
Sleep 2s
Type "cp /tmp/crex-demo/my-day-clean.toml /tmp/crex-demo/layouts/my-day.toml"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

Type "# Save and list layouts"
Enter
Sleep 500ms
Type "crex save my-day -d 'Friday deep work'"
Enter
Sleep 2s

# Hidden: swap clean toml again and clear
Hide
Type "cp /tmp/crex-demo/my-day-clean.toml /tmp/crex-demo/layouts/my-day.toml"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

Type "crex list"
Enter
Sleep 3s

# ── Scene 5: Restore layout (real) ────────────────────────

# Hidden: close workspaces created by import so restore creates them fresh.
Hide
Type "/tmp/crex-demo/close-demo-tabs.sh"
Enter
Sleep 2s
Type "clear"
Enter
Sleep 1s
Show

Type "# Restore a saved layout"
Enter
Sleep 500ms
Type "crex restore my-day --mode add"
Enter
Sleep 3s

# ── Scene 6: Add entry to Blueprint ──────────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Manage the Blueprint"
Enter
Sleep 500ms
Type "crex blueprint add notes ~/docs -t single"
Enter
Sleep 2s
Type "crex blueprint list"
Enter
Sleep 3s

# ── End ───────────────────────────────────────────────────

Sleep 2s
