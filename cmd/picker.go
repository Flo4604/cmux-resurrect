package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// pickLayout shows an interactive selector and lets the user pick a layout.
// When navigating, workspace names and pane counts appear below as a preview.
func pickLayout(metas []model.LayoutMeta) (string, error) {
	previewByName := make(map[string]string)
	for _, m := range metas {
		var sb strings.Builder
		for i, title := range m.WorkspaceTitles {
			panes := 0
			if i < len(m.WorkspacePanes) {
				panes = m.WorkspacePanes[i]
			}
			paneLabel := fmt.Sprintf("%d panes", panes)
			if panes == 1 {
				paneLabel = "1 pane"
			}
			fmt.Fprintf(&sb, "    %s  (%s)\n", title, paneLabel)
		}
		previewByName[m.Name] = strings.TrimRight(sb.String(), "\n")
	}

	options := make([]huh.Option[string], len(metas))
	for i, m := range metas {
		label := fmt.Sprintf("%s  %d workspaces", m.Name, m.WorkspaceCount)
		if m.Description != "" {
			desc := m.Description
			if len(desc) > 35 {
				desc = desc[:32] + "..."
			}
			label += "  " + desc
		}
		options[i] = huh.NewOption(label, m.Name)
	}

	var selected string
	err := huh.NewSelect[string]().
		Title("📦 Select a layout to restore").
		Options(options...).
		Value(&selected).
		DescriptionFunc(func() string {
			if preview, ok := previewByName[selected]; ok {
				return "\n  Workspaces:\n" + preview
			}
			return ""
		}, &selected).
		Run()
	if err != nil {
		return "", err
	}

	return selected, nil
}
