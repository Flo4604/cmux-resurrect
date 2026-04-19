# crex quick-start demo — setup + save
#
# Generates a short GIF showing the first-run experience.

Output assets/quickstart.gif
Set FontSize 14
Set Width 1000
Set Height 500
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false
Set Shell "bash"

Hide
Type "shopt -s interactive_comments"
Enter
Sleep 200ms
Type "alias crex='crex --config /tmp/crex-qs/config.toml'"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

# ── Scene 1: Setup wizard ─────────────────────────────────

Type "# First-run wizard"
Enter
Sleep 500ms
Type "crex setup --defaults"
Enter
Sleep 3s

# ── Scene 2: Save with description ────────────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Save your current layout"
Enter
Sleep 500ms
Type "crex save my-day -d 'Friday deep work'"
Enter
Sleep 3s

# ── Scene 3: List ─────────────────────────────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# See what you saved"
Enter
Sleep 500ms
Type "crex list"
Enter
Sleep 3s

Sleep 1s
