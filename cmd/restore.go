package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var restoreDryRun bool
var restoreMode string

// errGoBack signals the user wants to return to the layout picker.
var errGoBack = errors.New("go back")

var restoreCmd = &cobra.Command{
	Use:   "restore [name]",
	Short: "Restore a saved layout",
	Long:  "Recreates tabs, pane arrangements, and sends commands from a saved layout.\n\nYou will be asked whether to replace your current tabs or add to them.\nUse --mode to skip the interactive prompt (useful for scripts).\n\nIf no layout name is given, an interactive picker is shown.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "show commands without executing")
	restoreCmd.Flags().StringVar(&restoreMode, "mode", "", "restore mode: \"replace\" or \"add\" (skip interactive prompt)")
	restoreCmd.ValidArgsFunction = completeLayoutNames
	_ = restoreCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"replace\tClose existing " + unitName(2) + " first",
			"add\tKeep existing " + unitName(2),
		}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	var name string
	var workspaceFilter string
	var earlyMode orchestrate.RestoreMode
	var earlyModeSet bool
	if len(args) == 1 {
		name = args[0]
	} else {
		// Interactive picker with retry loop (user can go back from mode prompt).
		store, err := newStore()
		if err != nil {
			return err
		}
		metas, err := store.List()
		if err != nil {
			return err
		}
		if len(metas) == 0 {
			fmt.Fprintln(os.Stderr, dimStyle.Render("  No saved layouts. Use 'crex save <name>' to create one."))
			return nil
		}
		for {
			pick, err := pickLayout(metas)
			if err != nil {
				return err
			}
			name = pick.Layout
			workspaceFilter = pick.Workspace

			// If mode needs interactive prompt, ask now so user can go back.
			if restoreMode == "" && !restoreDryRun && cfg.RestoreMode == "" {
				m, err := askRestoreMode()
				if errors.Is(err, errGoBack) {
					continue // back to picker
				}
				if err != nil {
					return err
				}
				earlyMode = m
				earlyModeSet = true
			}
			break
		}
	}

	// Validate --mode flag value early.
	if restoreMode != "" && restoreMode != "replace" && restoreMode != "add" {
		return fmt.Errorf("invalid --mode %q: must be \"replace\" or \"add\"", restoreMode)
	}

	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	restorer := &orchestrate.Restorer{
		Client: cl,
		Store:  store,
		OnProgress: func(title string, panes int, err error) {
			t := padTitle(title)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "skipped") {
					fmt.Fprintf(os.Stderr, "  %s  %s %s\n", dimStyle.Render("SKIP"), t, dimStyle.Render("("+errMsg+")"))
				} else {
					fmt.Fprintf(os.Stderr, "  %s  %s: %v\n", yellowStyle.Render("FAIL"), t, err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "  %s  %s (%d panes)\n", greenStyle.Render("OK"), t, panes)
			}
		},
	}

	// Determine restore mode.
	var mode orchestrate.RestoreMode
	switch {
	case restoreMode == "replace":
		mode = orchestrate.RestoreModeReplace
	case restoreMode == "add":
		mode = orchestrate.RestoreModeAdd
	case restoreDryRun:
		mode = orchestrate.RestoreModeAdd
	case earlyModeSet:
		mode = earlyMode
	default:
		switch cfg.RestoreMode {
		case "replace":
			mode = orchestrate.RestoreModeReplace
		case "add":
			mode = orchestrate.RestoreModeAdd
		default:
			mode, err = askRestoreMode()
			if err != nil {
				return err
			}
		}
	}

	if restoreDryRun {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("👁️  Dry-run restore of"), greenStyle.Render(name))
	} else {
		action := "🔄 Replacing with"
		if mode == orchestrate.RestoreModeAdd {
			action = "➕ Adding from"
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render(action), greenStyle.Render(name))
	}

	result, err := restorer.Restore(name, restoreDryRun, mode, workspaceFilter)
	if err != nil {
		return err
	}

	if restoreDryRun {
		fmt.Fprintln(os.Stderr)
		for _, c := range result.Commands {
			switch {
			case c == "":
				fmt.Println()
			case strings.HasPrefix(c, "#"):
				fmt.Println(yellowStyle.Render(c))
			default:
				// Color the cmux prefix dim, highlight the action
				parts := strings.SplitN(c, " ", 3)
				if len(parts) >= 2 {
					fmt.Printf("%s %s", dimStyle.Render(parts[0]), cyanStyle.Render(parts[1]))
					if len(parts) == 3 {
						fmt.Printf(" %s", dimStyle.Render(parts[2]))
					}
					fmt.Println()
				} else {
					fmt.Println(c)
				}
			}
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ %d commands for %d %s", len(result.Commands)-countBlanks(result.Commands), result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
		return nil
	}

	fmt.Fprintln(os.Stderr)
	if result.WorkspacesClosed > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render(fmt.Sprintf("  Closed %d existing %s", result.WorkspacesClosed, unitName(result.WorkspacesClosed))))
	}
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Restored %d/%d %s", result.WorkspacesOK, result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", yellowStyle.Render("⚠️  Errors:"))
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", dimStyle.Render("• "+e))
		}
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

func countBlanks(cmds []string) int {
	n := 0
	for _, c := range cmds {
		if c == "" || strings.HasPrefix(c, "#") {
			n++
		}
	}
	return n
}

// modePickerModel is a Bubble Tea model for the replace/add prompt.
type modePickerModel struct {
	cursor int // 0 = replace, 1 = add
	done   bool
	back   bool
}

func (m modePickerModel) Init() tea.Cmd { return nil }

func (m modePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.back = true
			m.done = true
			return m, tea.Quit
		case tea.KeyCtrlC:
			m.done = true
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < 1 {
				m.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'r', 'R':
					m.cursor = 0
					m.done = true
					return m, tea.Quit
				case 'a', 'A':
					m.cursor = 1
					m.done = true
					return m, tea.Quit
				case 'q':
					m.back = true
					m.done = true
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m modePickerModel) View() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(headingStyle.Render("How do you want to restore?"))
	b.WriteString("\n")

	labels := []string{
		fmt.Sprintf("Replace — close all current %s, then restore", unitName(2)),
		fmt.Sprintf("Add     — keep current %s, add restored ones", unitName(2)),
	}
	keys := []string{"r", "a"}

	for i, label := range labels {
		if i == m.cursor {
			fmt.Fprintf(&b, "  %s %s  %s\n", greenStyle.Render("▸"), cyanStyle.Render("["+keys[i]+"]"), label)
		} else {
			fmt.Fprintf(&b, "    %s  %s\n", cyanStyle.Render("["+keys[i]+"]"), label)
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ select · ↵ confirm · esc back"))
	b.WriteString("\n")
	return b.String()
}

// askRestoreMode prompts the user to choose between replacing or adding workspaces.
func askRestoreMode() (orchestrate.RestoreMode, error) {
	p := tea.NewProgram(modePickerModel{})
	finalModel, err := p.Run()
	if err != nil {
		return orchestrate.RestoreModeReplace, err
	}

	m := finalModel.(modePickerModel)
	if m.back || !m.done {
		return 0, errGoBack
	}

	if m.cursor == 1 {
		return orchestrate.RestoreModeAdd, nil
	}
	return orchestrate.RestoreModeReplace, nil
}
