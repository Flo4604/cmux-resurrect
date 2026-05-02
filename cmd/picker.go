package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
)

// PickResult holds the user's selection from the layout picker.
type PickResult struct {
	Layout    string // layout name
	Workspace string // specific workspace title, or empty for all
}

// pickerModel wraps BrowseModel for standalone CLI use.
type pickerModel struct {
	browse tui.BrowseModel
	done   bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		m.browse, _ = m.browse.Update(msg)
		if m.browse.Done() {
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	return fmt.Sprintf("  📦 Select a layout to restore\n\n%s", m.browse.View())
}

// pickLayout shows an interactive picker using the same BrowseModel as the TUI.
// Returns the selected layout name and optional workspace filter.
func pickLayout(metas []model.LayoutMeta) (*PickResult, error) {
	items := tui.ItemsFromLayouts(metas)
	browse := tui.NewBrowseModel(items, "restore")

	p := tea.NewProgram(pickerModel{browse: browse})
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := finalModel.(pickerModel)
	if !m.done || !m.browse.Selected() {
		return nil, fmt.Errorf("cancelled")
	}

	item := m.browse.SelectedItem()
	result := &PickResult{Layout: item.Name}

	// If user drilled into workspace detail and selected a specific workspace.
	if item.Kind == tui.KindWorkspace {
		result.Workspace = item.Name
		result.Layout = m.browse.LayoutName()
	} else if item.Kind == tui.KindAllWs {
		result.Layout = m.browse.LayoutName()
	}

	return result, nil
}
