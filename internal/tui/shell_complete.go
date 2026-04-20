package tui

import (
	"fmt"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// completionEngine generates tab-completion candidates for the shell.
type completionEngine struct {
	store  persist.Store
	wsFile string
}

// completionItem pairs a fillable value with display metadata.
type completionItem struct {
	value string // what gets filled into the prompt
	icon  string
	desc  string
}

// --- Level-aware command definitions ----------------------------------------

// level1Commands are the top-level commands (no subcommand expansion).
var level1Commands = []completionItem{
	{"help", "❓", "Show help"},
	{"ls", "📋", "List saved layouts"},
	{"list", "📋", "List saved layouts"},
	{"now", "🖥", "Show current state"},
	{"save", "💾", "Save current layout"},
	{"restore", "🔄", "Restore a saved layout"},
	{"delete", "🗑", "Delete a saved layout"},
	{"templates", "📦", "Browse template gallery"},
	{"use", "🚀", "Create from template"},
	{"watch", "⏱", "Auto-save daemon"},
	{"bp", "📐", "Blueprint commands…"},
	{"blueprint", "📐", "Blueprint commands…"},
	{"settings", "⚙️", "Settings & preferences"},
	{"exit", "👋", "Exit the shell"},
	{"quit", "👋", "Exit the shell"},
}

// level2Subcommands are the subcommands for two-word command groups.
var level2Subcommands = map[string][]completionItem{
	"bp": {
		{"add", "➕", "Add Blueprint entry"},
		{"list", "📋", "List Blueprint entries"},
		{"ls", "📋", "List Blueprint entries"},
		{"remove", "🗑", "Remove Blueprint entry"},
		{"rm", "🗑", "Remove Blueprint entry"},
		{"toggle", "🔀", "Enable/disable entry"},
	},
	"blueprint": {
		{"add", "➕", "Add Blueprint entry"},
		{"list", "📋", "List Blueprint entries"},
		{"remove", "🗑", "Remove Blueprint entry"},
		{"toggle", "🔀", "Enable/disable entry"},
	},
	"settings": {
		{"banner", "🎨", "Banner style"},
	},
	"watch": {
		{"status", "📊", "Show daemon status"},
		{"start", "▶️", "Start daemon"},
		{"stop", "⏹", "Stop daemon"},
	},
}

// nestedGroupPrefixes are level-2 subcommands that expand into level-3 commands.
var nestedGroupPrefixes = map[string]bool{
	"settings banner": true,
}

// level3Subcommands are the subcommands for three-word command groups.
var level3Subcommands = map[string][]completionItem{
	"settings banner": {
		{"set", "🎨", "Set banner style"},
		{"get", "🔍", "Show current style"},
		{"list", "📋", "List available styles"},
	},
}

// singleCommands is the set of single-word commands that take arguments.
var singleCommands = map[string]bool{
	"help": true, "ls": true, "list": true, "now": true,
	"save": true, "restore": true, "delete": true,
	"templates": true, "use": true, "watch": true,
	"exit": true, "quit": true, "?": true,
}

// twoWordPrefixes maps the first word of two-word commands to valid subcommands.
var twoWordPrefixes = map[string]bool{
	"bp": true, "blueprint": true, "settings": true, "watch": true,
}

// --- Candidate generation ---------------------------------------------------

// completionResult holds both the fill values and the display items.
type completionResult struct {
	values []string
	items  []completionItem
}

// Complete returns completion candidates for the current input.
func (ce *completionEngine) Complete(input string) completionResult {
	trimmed := strings.TrimLeft(input, " ")

	// Empty input → level 1 commands.
	if trimmed == "" {
		return ce.level1Completions("")
	}

	parts := strings.Fields(trimmed)
	first := strings.ToLower(parts[0])

	// Check if first word is a two-word command group.
	if twoWordPrefixes[first] {
		trailingSpace := strings.HasSuffix(trimmed, " ")

		if len(parts) == 1 && !trailingSpace {
			// Still typing first word, e.g. "se" → suggest "settings", "save"
			return ce.level1Completions(trimmed)
		}

		// First word complete.
		sub := ""
		if len(parts) >= 2 {
			sub = strings.ToLower(parts[1])
		}

		// Check for nested group (e.g., "settings banner").
		nestedKey := first + " " + sub
		if sub != "" && nestedGroupPrefixes[nestedKey] {
			if len(parts) == 2 && !trailingSpace {
				// Still typing sub-group name: "settings ban"
				return ce.level2Completions(first, sub)
			}
			// Sub-group complete, handle level 3.
			l3partial := ""
			if len(parts) >= 3 {
				l3partial = parts[2]
			}
			if len(parts) >= 3 && trailingSpace {
				// Level 3 command complete → arguments.
				l3cmd := strings.ToLower(parts[2])
				fullCmd := nestedKey + " " + l3cmd
				prefix := nestedKey + " " + l3cmd + " "
				argPartial := ""
				if len(parts) >= 4 {
					argPartial = strings.Join(parts[3:], " ")
				}
				return ce.argCompletions(fullCmd, argPartial, prefix)
			}
			if len(parts) >= 4 {
				// Typing argument after level-3 command.
				l3cmd := strings.ToLower(parts[2])
				fullCmd := nestedKey + " " + l3cmd
				prefix := nestedKey + " " + l3cmd + " "
				argPartial := strings.Join(parts[3:], " ")
				return ce.argCompletions(fullCmd, argPartial, prefix)
			}
			// Show level 3 options.
			return ce.level3Completions(nestedKey, l3partial)
		}

		// Non-nested: original two-level behavior.
		partial := ""
		if len(parts) >= 2 {
			partial = parts[1]
		}
		if len(parts) >= 2 && trailingSpace {
			fullCmd := first + " " + sub
			prefix := first + " " + sub + " "
			argPartial := ""
			if len(parts) >= 3 {
				argPartial = parts[2]
			}
			return ce.argCompletions(fullCmd, argPartial, prefix)
		}
		if len(parts) >= 3 {
			fullCmd := first + " " + sub
			prefix := first + " " + sub + " "
			argPartial := strings.Join(parts[2:], " ")
			return ce.argCompletions(fullCmd, argPartial, prefix)
		}
		return ce.level2Completions(first, partial)
	}

	// Single-word command.
	if singleCommands[first] {
		if !strings.HasSuffix(trimmed, " ") && len(parts) == 1 {
			// Still typing the command name.
			return ce.level1Completions(trimmed)
		}
		// Command complete, suggest arguments.
		prefix := first + " "
		argPartial := ""
		if len(parts) >= 2 {
			argPartial = strings.Join(parts[1:], " ")
		}
		return ce.argCompletions(first, argPartial, prefix)
	}

	// Unknown prefix — try matching level 1.
	return ce.level1Completions(trimmed)
}

func (ce *completionEngine) level1Completions(prefix string) completionResult {
	lower := strings.ToLower(strings.TrimSpace(prefix))
	var items []completionItem
	var values []string
	for _, c := range level1Commands {
		if lower == "" || strings.HasPrefix(c.value, lower) {
			items = append(items, c)
			values = append(values, c.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) level2Completions(group, partial string) completionResult {
	subs, ok := level2Subcommands[group]
	if !ok {
		return completionResult{}
	}
	lower := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, s := range subs {
		if lower == "" || strings.HasPrefix(s.value, lower) {
			items = append(items, s)
			values = append(values, group+" "+s.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) level3Completions(nestedKey, partial string) completionResult {
	subs, ok := level3Subcommands[nestedKey]
	if !ok {
		return completionResult{}
	}
	lower := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, s := range subs {
		if lower == "" || strings.HasPrefix(s.value, lower) {
			items = append(items, s)
			values = append(values, nestedKey+" "+s.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) argCompletions(cmd, partial, prefix string) completionResult {
	var argItems []completionItem

	switch cmd {
	case "restore", "delete", "save":
		for _, n := range ce.layoutNames() {
			argItems = append(argItems, completionItem{n, "📄", ""})
		}
	case "use":
		for _, n := range ce.templateNames() {
			argItems = append(argItems, completionItem{n, "📦", ""})
		}
	case "watch":
		return ce.level2Completions("watch", partial)
	case "settings banner set":
		argItems = []completionItem{
			{"flame", "🔥", "Gradient (ember → gold → green)"},
			{"classic", "🟢", "Solid green"},
			{"plain", "⬜", "Monochrome gray"},
		}
	case "bp remove", "bp rm", "bp toggle",
		"blueprint remove", "blueprint toggle":
		for _, n := range ce.blueprintNames() {
			argItems = append(argItems, completionItem{n, "📐", ""})
		}
	default:
		return completionResult{}
	}

	lowerPartial := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, a := range argItems {
		if lowerPartial == "" || strings.HasPrefix(strings.ToLower(a.value), lowerPartial) {
			items = append(items, a)
			values = append(values, prefix+a.value)
		}
	}
	return completionResult{values: values, items: items}
}

// --- Rendering --------------------------------------------------------------

// RenderItems formats completion items with icons and descriptions.
func RenderItems(items []completionItem) string {
	return RenderItemsHighlighted(items, -1)
}

// RenderItemsHighlighted formats items with one highlighted by index.
func RenderItemsHighlighted(items []completionItem, highlight int) string {
	var b strings.Builder
	b.WriteString("\n")
	for i, it := range items {
		nameStyle := shellSuccessStyle
		descStyle := shellDimStyle
		marker := "  "
		if i == highlight {
			nameStyle = shellPromptStyle // bright green bold for selected
			marker = shellPromptStyle.Render("▸ ")
		}
		if it.desc != "" {
			fmt.Fprintf(&b, "  %s%s  %s  %s\n",
				marker,
				it.icon,
				nameStyle.Render(fmt.Sprintf("%-12s", it.value)),
				descStyle.Render(it.desc))
		} else {
			fmt.Fprintf(&b, "  %s%s  %s\n",
				marker,
				it.icon,
				nameStyle.Render(it.value))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// --- Data source helpers ----------------------------------------------------

func (ce *completionEngine) layoutNames() []string {
	if ce.store == nil {
		return nil
	}
	metas, err := ce.store.List()
	if err != nil {
		return nil
	}
	names := make([]string, len(metas))
	for i, m := range metas {
		names[i] = m.Name
	}
	return names
}

func (ce *completionEngine) templateNames() []string {
	templates := gallery.List()
	names := make([]string, len(templates))
	for i, t := range templates {
		names[i] = t.Name
	}
	return names
}

func (ce *completionEngine) blueprintNames() []string {
	if ce.wsFile == "" {
		return nil
	}
	wf, err := mdfile.Parse(ce.wsFile)
	if err != nil {
		return nil
	}
	names := make([]string, len(wf.Projects))
	for i, p := range wf.Projects {
		names[i] = p.Name
	}
	return names
}

// commonPrefix returns the longest common prefix of all strings.
func commonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}
