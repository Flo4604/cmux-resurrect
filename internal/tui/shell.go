package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

type shellMode int

const (
	modePrompt shellMode = iota
	modeBrowse
	modeConfirm
)

const maxHistory = 50

// BannerCycleFn cycles the banner style and returns (newStyle, preview, error).
// preview is the rendered banner in the new style.
type BannerCycleFn func(explicit string) (string, string, error)

// ShellModel is the main Bubble Tea model for the crex interactive shell.
type ShellModel struct {
	mode       shellMode
	prompt     textinput.Model
	browse     BrowseModel
	output     *strings.Builder // per-command buffer
	lastOutput string           // rendered above the prompt
	lastItems  []Item
	history    []string
	histIdx    int
	backend    client.DetectedBackend
	store      persist.Store
	client     client.Backend
	wsFile     string
	quitting   bool
	byeMsg     string // printed to stdout after alt screen closes
	welcome    string // shown as initial lastOutput
	lastCmd    string // last dispatched command, shown as dim header

	// Banner style cycling (injected by cmd layer).
	BannerCycle BannerCycleFn
	bannerStyle string // current banner style for "banner get"

	// Tab completion
	completer     completionEngine
	tabCandidates []string         // candidates being cycled
	tabItems      []completionItem // display items for current cycle
	tabIndex      int              // current cycle position (-1 = list shown, not yet cycling)

	// Confirmation state
	confirmMsg string
	confirmFn  func()
}

// NewShellModel creates the interactive shell model.
func NewShellModel(store persist.Store, cl client.Backend, backend client.DetectedBackend, wsFile string) ShellModel {
	ti := textinput.New()
	ti.Prompt = "  " + shellSuccessStyle.Render("crex") + " " + shellFlameStyle.Render("→") + " "
	ti.Focus()
	ti.CharLimit = 256
	ti.ShowSuggestions = true
	ti.CompletionStyle = shellCompletionStyle
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("ctrl+n"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("ctrl+p"))
	// Set initial suggestions for level 1 commands.
	var initSuggestions []string
	for _, c := range level1Commands {
		initSuggestions = append(initSuggestions, c.value)
	}
	ti.SetSuggestions(initSuggestions)

	// Build welcome message.
	var w strings.Builder
	w.WriteString("\n")
	w.WriteString(shellDimStyle.Render("  crex interactive shell. Type "))
	w.WriteString(shellSuccessStyle.Render("help"))
	w.WriteString(shellDimStyle.Render(" for commands, "))
	w.WriteString(shellSuccessStyle.Render("exit"))
	w.WriteString(shellDimStyle.Render(" to quit."))
	w.WriteString("\n")

	return ShellModel{
		mode:       modePrompt,
		prompt:     ti,
		output:     &strings.Builder{},
		lastOutput: w.String(),
		backend:    backend,
		store:      store,
		client:     cl,
		wsFile:     wsFile,
		histIdx:    -1,
		welcome:    w.String(),
		completer:  completionEngine{store: store, wsFile: wsFile},
	}
}

// SetBannerStyle sets the current banner style name (for "banner get").
func (m *ShellModel) SetBannerStyle(s string) { m.bannerStyle = s }

// ByeMsg returns the farewell message to print after the TUI exits.
func (m ShellModel) ByeMsg() string { return m.byeMsg }

// Init is the Bubble Tea init function.
func (m ShellModel) Init() tea.Cmd {
	return textinput.Blink
}

// flushOutput drains the per-command buffer into lastOutput.
func (m *ShellModel) flushOutput() {
	text := m.output.String()
	m.output.Reset()
	if text != "" {
		m.lastOutput += text
	}
}

// Update handles all incoming messages.
func (m ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case restoreResultMsg:
		m.handleRestoreResult(msg)
		m.flushOutput()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modePrompt:
			return m.updatePrompt(msg)
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		}
	}

	// Pass other messages to the text input
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m ShellModel) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyUp:
		// When completion list is visible, cycle backwards through options.
		if len(m.tabCandidates) > 0 {
			m.tabIndex = (m.tabIndex - 1 + len(m.tabCandidates)) % len(m.tabCandidates)
			m.prompt.SetValue(m.tabCandidates[m.tabIndex])
			m.prompt.CursorEnd()
			m.lastOutput = RenderItemsHighlighted(m.tabItems, m.tabIndex)
			return m, nil
		}
		if len(m.history) > 0 && m.histIdx < len(m.history)-1 {
			m.histIdx++
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		}
		return m, nil

	case tea.KeyDown:
		// When completion list is visible, cycle forwards through options.
		if len(m.tabCandidates) > 0 {
			m.tabIndex = (m.tabIndex + 1) % len(m.tabCandidates)
			m.prompt.SetValue(m.tabCandidates[m.tabIndex])
			m.prompt.CursorEnd()
			m.lastOutput = RenderItemsHighlighted(m.tabItems, m.tabIndex)
			return m, nil
		}
		if m.histIdx > 0 {
			m.histIdx--
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		} else if m.histIdx == 0 {
			m.histIdx = -1
			m.prompt.SetValue("")
		}
		return m, nil

	case tea.KeyEnter:
		input := strings.TrimSpace(m.prompt.Value())
		m.histIdx = -1

		if input == "" {
			return m, nil
		}

		// Bare group prefix (bp, blueprint, settings, settings banner) → treat as space+tab.
		normalized := strings.ToLower(input)
		if normalized == "blueprint" {
			normalized = "bp"
		}
		if twoWordPrefixes[normalized] || nestedGroupPrefixes[normalized] {
			m.lastCmd = ""
			m.prompt.SetValue(normalized + " ")
			m.prompt.CursorEnd()
			result := m.completer.Complete(m.prompt.Value())
			if len(result.values) > 0 {
				m.lastOutput = RenderItems(result.items)
				m.tabCandidates = result.values
				m.tabItems = result.items
				m.tabIndex = -1
			}
			return m, nil
		}

		// Record in history
		m.history = append(m.history, input)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}

		// Clear screen for new output.
		m.lastCmd = input
		m.lastOutput = ""
		m.output.Reset()

		// Dispatch (exec methods write to m.output)
		model, dispatchCmd := m.dispatch(input)

		// Flush buffered output into lastOutput
		sm := model.(ShellModel)
		sm.flushOutput()

		// Keep command in prompt on usage errors so the user can append args.
		if strings.Contains(sm.lastOutput, "Usage:") {
			sm.prompt.SetValue(input + " ")
			sm.prompt.CursorEnd()
		} else {
			sm.prompt.SetValue("")
		}

		return sm, dispatchCmd
	}

	// Escape: remove last level from the command (navigate back).
	if msg.Type == tea.KeyEsc {
		m.lastCmd = ""
		val := strings.TrimRight(m.prompt.Value(), " ")
		if val == "" {
			m.tabCandidates = nil
			m.tabItems = nil
			m.tabIndex = -1
			m.lastOutput = m.welcome
			return m, nil
		}
		// Remove last word.
		lastSpace := strings.LastIndex(val, " ")
		if lastSpace >= 0 {
			m.prompt.SetValue(val[:lastSpace+1])
		} else {
			m.prompt.SetValue("")
		}
		m.prompt.CursorEnd()
		// Show completions for the new level, ready for cycling.
		result := m.completer.Complete(m.prompt.Value())
		if len(result.values) > 1 {
			m.lastOutput = RenderItems(result.items)
			m.tabCandidates = result.values
			m.tabItems = result.items
			m.tabIndex = -1
		} else {
			m.tabCandidates = nil
			m.tabItems = nil
			m.tabIndex = -1
			m.lastOutput = m.welcome
		}
		return m, nil
	}

	// Tab / Shift+Tab: completion cycling.
	if msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab {
		forward := msg.Type == tea.KeyTab

		// If already cycling, advance/reverse through candidates.
		if len(m.tabCandidates) > 0 {
			if forward {
				m.tabIndex = (m.tabIndex + 1) % len(m.tabCandidates)
			} else {
				m.tabIndex = (m.tabIndex - 1 + len(m.tabCandidates)) % len(m.tabCandidates)
			}
			m.prompt.SetValue(m.tabCandidates[m.tabIndex])
			m.prompt.CursorEnd()
			m.lastOutput = RenderItemsHighlighted(m.tabItems, m.tabIndex)
			return m, nil
		}

		result := m.completer.Complete(m.prompt.Value())
		switch len(result.values) {
		case 0:
			// No completions — show brief feedback.
			m.lastOutput = shellErrorStyle.Render("  ✗ No completions") + "\n"
		case 1:
			// Single match — fill it in, add space, and try deeper completions.
			filled := strings.TrimSpace(result.values[0])
			m.prompt.SetValue(filled + " ")
			m.prompt.CursorEnd()
			sub := m.completer.Complete(m.prompt.Value())
			if len(sub.values) > 0 {
				m.lastOutput = RenderItems(sub.items)
				m.tabCandidates = sub.values
				m.tabItems = sub.items
				m.tabIndex = -1
			} else {
				// No deeper completions — clear stale display.
				m.lastOutput = ""
				m.tabCandidates = nil
				m.tabItems = nil
				m.tabIndex = -1
			}
		default:
			// Multiple matches — display them and start cycle state.
			m.lastOutput = RenderItems(result.items)
			m.tabCandidates = result.values
			m.tabItems = result.items
			m.tabIndex = -1
			prefix := commonPrefix(result.values)
			if len(prefix) > len(m.prompt.Value()) {
				m.prompt.SetValue(prefix)
				m.prompt.CursorEnd()
			}
		}
		return m, nil
	}

	// Any non-tab/escape key clears the cycling state and stale list.
	if len(m.tabCandidates) > 0 {
		m.lastOutput = ""
	}
	m.tabCandidates = nil
	m.tabItems = nil
	m.tabIndex = -1

	// Update ghost-text suggestions for inline completion.
	result := m.completer.Complete(m.prompt.Value())
	m.prompt.SetSuggestions(result.values)

	// Pass to text input for line editing
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)

	return m, cmd
}

func (m ShellModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	bm, _ := m.browse.Update(msg)
	m.browse = bm

	if bm.done {
		m.mode = modePrompt
		if bm.selected {
			model, cmd := m.handleBrowseSelection(bm.SelectedItem())
			sm := model.(ShellModel)
			sm.flushOutput()
			return sm, cmd
		}
		if bm.passthrough != 0 {
			m.prompt.SetValue(string(bm.passthrough))
			m.prompt.CursorEnd()
		}
	}
	return m, nil
}

func (m ShellModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
		if m.confirmFn != nil {
			m.confirmFn()
		}
		m.output.WriteString(shellSuccessStyle.Render("  ✓ Done"))
		m.output.WriteString("\n")
	} else {
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n")
	}
	m.mode = modePrompt
	m.confirmMsg = ""
	m.confirmFn = nil
	m.flushOutput()
	return m, nil
}

func (m ShellModel) handleBrowseSelection(item Item) (tea.Model, tea.Cmd) {
	switch m.browse.action {
	case "restore":
		return m, m.execRestore(item.Name)
	case "use":
		m.execUse(item.Name)
	case "toggle":
		m.execBpToggle(item.Name)
	}
	return m, nil
}

func (m ShellModel) dispatch(input string) (tea.Model, tea.Cmd) {
	// Silently ignore comment lines (useful for demos).
	if strings.HasPrefix(strings.TrimSpace(input), "#") {
		return m, nil
	}

	cmd, args := parseCommand(input)

	switch cmd {
	case "exit", "quit":
		m.byeMsg = randomBye()
		m.quitting = true
		return m, tea.Quit

	case "help", "?":
		m.output.WriteString(renderHelp(m.backend))
		m.output.WriteString("\n")

	case "ls", "list":
		m.execList()

	case "now":
		m.execNow()

	case "save":
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}
		m.execSave(name)

	case "restore":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: restore <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		return m, m.execRestore(resolved)

	case "delete":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: delete <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execDelete(resolved)

	case "templates":
		m.execTemplates()

	case "use":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: use <template|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execUse(resolved)

	case "watch":
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}
		m.execWatch(sub)

	case "bp add":
		if len(args) < 2 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp add <name> <path>"))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpAdd(args[0], args[1])

	case "bp list", "bp ls":
		m.execBpList()

	case "bp remove", "bp rm":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp remove <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpRemove(resolved)

	case "settings banner set":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: settings banner set <flame|classic|plain>"))
			m.output.WriteString("\n\n")
			break
		}
		if m.BannerCycle == nil {
			m.output.WriteString(shellErrorStyle.Render("  ✗ banner not available"))
			m.output.WriteString("\n\n")
			break
		}
		newStyle, preview, err := m.BannerCycle(args[0])
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.bannerStyle = newStyle
		m.output.WriteString(preview)

	case "settings banner get":
		if m.BannerCycle == nil {
			m.output.WriteString(shellErrorStyle.Render("  ✗ banner not available"))
			m.output.WriteString("\n\n")
			break
		}
		m.output.WriteString(fmt.Sprintf("  Current banner style: %s\n\n",
			shellSuccessStyle.Render(m.bannerStyle)))

	case "settings banner list":
		m.output.WriteString("  Available banner styles:\n")
		m.output.WriteString(fmt.Sprintf("    %s  gradient (ember → gold → green)\n", shellSuccessStyle.Render("flame  ")))
		m.output.WriteString(fmt.Sprintf("    %s  solid green\n", shellSuccessStyle.Render("classic")))
		m.output.WriteString(fmt.Sprintf("    %s  monochrome gray\n", shellSuccessStyle.Render("plain  ")))
		m.output.WriteString("\n")

	case "bp toggle":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp toggle <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpToggle(resolved)

	default:
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Unknown command: %s", cmd)))
		m.output.WriteString("\n")
		m.output.WriteString(shellDimStyle.Render("  Type help for available commands."))
		m.output.WriteString("\n\n")
	}

	return m, nil
}

// View renders the full screen: prompt always at top, blank line, then content.
func (m ShellModel) View() string {
	if m.quitting {
		return ""
	}

	prompt := m.prompt.View()
	header := ""
	if m.lastCmd != "" {
		header = "\n" + shellDimStyle.Render("  "+m.lastCmd)
	}

	switch m.mode {
	case modeBrowse:
		return prompt + header + "\n\n" + m.browse.View()
	case modeConfirm:
		return prompt + header + "\n\n" + m.confirmMsg + "\n"
	default:
		return prompt + header + "\n" + m.lastOutput
	}
}

// batchNonNil batches commands, filtering out nils.
func batchNonNil(cmds ...tea.Cmd) tea.Cmd {
	var live []tea.Cmd
	for _, c := range cmds {
		if c != nil {
			live = append(live, c)
		}
	}
	switch len(live) {
	case 0:
		return nil
	case 1:
		return live[0]
	default:
		return tea.Batch(live...)
	}
}
