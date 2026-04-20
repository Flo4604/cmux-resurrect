# crex v1.6.0 interactive shell demo — recorded with github.com/charmbracelet/vhs
#
# Run via: ./scripts/record-demo-tui.sh (needs Ghostty or cmux running)
#
# NOTE: CREX_BRAND=crex forces crex branding while cmux handles live ops.
#
# DEMO NARRATIVE (centered on save / restore / templates + v1.6.0 features):
#   1. Launch          — crex tui, banner + prompt
#   2. Tab completion  — Tab shows all commands with icons (hero feature)
#   3. Save            — save via tab-completed layout name
#   4. List + browse   — ls with arrow navigation
#   5. Restore         — tab-complete into saved layout names
#   6. Templates       — browse gallery, use a template
#   7. Settings depth  — 3-level drill: settings → banner → set flame
#   8. Usage error     — wrong args, command stays in prompt
#   9. Exit            — phoenix bye message

Output assets/demo-tui.gif
Set FontSize 14
Set Width 1000
Set Height 620
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

Type "# Interactive shell with tab completion"
Enter
Sleep 500ms
Type "crex tui"
Enter
Sleep 2s

# Readable typing inside the TUI
Set TypingSpeed 50ms

# ── Scene 2: Tab completion — discover commands ──────────

# Press Tab on empty prompt → full command list with icons
Tab
Sleep 3s

# Cycle through options with arrows
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Up
Sleep 400ms

# Dismiss and type manually
Escape
Sleep 500ms

# ── Scene 3: Save — tab-complete the command + layout name ─

Type "sa"
Sleep 300ms
Tab
Sleep 1500ms
# "save " is filled + layout names shown. Type to filter.
Type "my"
Sleep 300ms
Tab
Sleep 800ms
# "save my-day" filled.
Enter
Sleep 3s

# ── Scene 4: List layouts with browse ────────────────────

Type "ls"
Sleep 300ms
Enter
Sleep 2s
# Browse with arrows
Down
Sleep 500ms
Down
Sleep 500ms
Up
Sleep 800ms
# Quit browse
Type "q"
Sleep 1s

# ── Scene 5: Restore — tab-complete into layout names ────

Type "res"
Sleep 300ms
Tab
Sleep 1500ms
# "restore " filled + layout names shown. Type to filter.
Type "my"
Sleep 300ms
Tab
Sleep 800ms
# "restore my-day " filled.
Enter
Sleep 4s

# ── Scene 6: Templates — browse and use ─────────────────

Type "templates"
Sleep 300ms
Enter
Sleep 2s
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 800ms
Type "q"
Sleep 800ms

# Use a template via tab completion
Type "use "
Sleep 200ms
Tab
Sleep 2s
# Template list shown with real icons. Type to filter.
Type "cl"
Sleep 300ms
Tab
Sleep 800ms
# "use claude " filled.
Enter
Sleep 3s

# ── Scene 7: Settings — 3-level tab completion ──────────

# Type "settings " to enter the group, then Tab to see subcommands.
Type "settings "
Sleep 300ms
Tab
Sleep 1500ms
# Shows "banner" subcommand. Type "b" + Tab to fill "settings banner ".
Type "b"
Sleep 200ms
Tab
Sleep 1500ms
# Prompt is "settings banner ", shows set/get/list (3rd level!).
# Type "s" + Tab → fills "settings banner set " and shows banner styles.
Type "s"
Sleep 200ms
Tab
Sleep 1500ms
# Prompt is "settings banner set ", flame/classic/plain visible.
# Type "fl" + Tab to complete "flame".
Type "fl"
Sleep 200ms
Tab
Sleep 800ms
Enter
Sleep 2s

# ── Scene 8: Usage error — command stays in prompt ───────

Type "restore"
Sleep 300ms
Enter
Sleep 2s
# Prompt still has "restore" — just add the name to fix it
Type " my-day"
Sleep 300ms
Enter
Sleep 3s

# ── Scene 9: Exit — phoenix farewell ────────────────────

Type "exit"
Sleep 400ms
Enter
Sleep 2s
