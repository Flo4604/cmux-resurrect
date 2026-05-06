package orchestrate

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/detect"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// Saver captures the current cmux state and persists it.
type Saver struct {
	Client client.Backend
	Store  persist.Store
}

// Save captures the live cmux state and writes it to the store.
func (s *Saver) Save(name, description string) (*model.Layout, error) {
	tree, err := s.Client.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	if len(tree.Windows) == 0 {
		return nil, fmt.Errorf("no windows found")
	}

	// Use the first (typically only) window.
	win := tree.Windows[0]

	layout := &model.Layout{
		Name:        name,
		Description: description,
		Version:     1,
		SavedAt:     time.Now().UTC(),
	}

	for _, tw := range win.Workspaces {
		ws, err := s.buildWorkspace(tw)
		if err != nil {
			// Log but don't fail — isolate errors per workspace.
			fmt.Fprintf(os.Stderr, "  warning: workspace %q: %v\n", tw.Title, err)
			continue
		}
		layout.Workspaces = append(layout.Workspaces, *ws)
	}

	if len(layout.Workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces could be captured")
	}

	// If a TOML already exists, merge user-edited fields (split direction, commands).
	if existing, err := s.Store.Load(name); err == nil {
		mergeUserEdits(layout, existing)
	}

	// Auto-detect running AI CLI sessions and populate resume commands.
	// Surface titles from the tree confirm which panes actually run an AI CLI,
	// preventing false matches when multiple workspaces share a CWD.
	applyDetectedSessions(layout, win.Workspaces)

	if err := s.Store.Save(name, layout); err != nil {
		return nil, fmt.Errorf("save layout: %w", err)
	}
	return layout, nil
}

func (s *Saver) buildWorkspace(tw client.TreeWorkspace) (*model.Workspace, error) {
	// Get CWD from sidebar-state.
	sidebar, err := s.Client.SidebarState(tw.Ref)
	if err != nil {
		return nil, fmt.Errorf("sidebar-state: %w", err)
	}

	ws := &model.Workspace{
		Title:  tw.Title,
		CWD:    sidebar.CWD,
		Pinned: tw.Pinned,
		Index:  tw.Index,
		Active: tw.Active || tw.Selected,
	}

	// Sort panes by index.
	panes := make([]client.TreePane, len(tw.Panes))
	copy(panes, tw.Panes)
	sort.Slice(panes, func(i, j int) bool {
		return panes[i].Index < panes[j].Index
	})

	for i, tp := range panes {
		pane := model.Pane{
			Type:        "terminal",
			Focus:       tp.Focused,
			Index:       tp.Index,
			FocusTarget: -1, // no refocus needed for saved layouts
		}

		// First pane has no split direction; subsequent default to "right".
		if i > 0 {
			pane.Split = "right"
		}

		// Use surface info for type and URL.
		if len(tp.Surfaces) > 0 {
			surf := tp.Surfaces[0]
			pane.Type = surf.Type
			if surf.URL != nil {
				pane.URL = *surf.URL
			}
		}

		ws.Panes = append(ws.Panes, pane)
	}

	// Ensure at least one pane.
	if len(ws.Panes) == 0 {
		ws.Panes = []model.Pane{{Type: "terminal", Focus: true}}
	}

	return ws, nil
}

// aiTitlePatterns maps AI tool names to substrings found in terminal titles
// when that tool is the active foreground process. These are set by the
// programs themselves via ANSI escape codes — not user-configurable.
var aiTitlePatterns = map[string][]string{
	"claude":   {"Claude Code", "claude"},
	"opencode": {"OpenCode", "opencode", "OC |"},
	"codex":    {"Codex", "codex"},
}

// applyDetectedSessions scans for running AI CLI sessions (Claude Code,
// OpenCode, Codex) and sets the resume command on matching panes.
// Detection is best-effort: if anything fails, panes are left unchanged.
//
// Matching strategy (two passes):
//  1. Title-confirmed: both CWD and surface title agree → highest confidence.
//  2. CWD-only fallback: for tools that don't set a recognizable title,
//     match by CWD alone — but only if no other workspace already claimed
//     that CWD in pass 1.
//
// Each CWD is consumed after the first match to prevent duplicates.
func applyDetectedSessions(layout *model.Layout, treeWorkspaces []client.TreeWorkspace) {
	sessions := detect.AISessions()
	if len(sessions) == 0 {
		return
	}

	// Build a lookup of surface titles by workspace title + pane index.
	type paneKey struct {
		wsTitle string
		paneIdx int
	}
	surfaceTitles := make(map[paneKey]string)
	for _, tw := range treeWorkspaces {
		for _, tp := range tw.Panes {
			for _, s := range tp.Surfaces {
				surfaceTitles[paneKey{tw.Title, tp.Index}] = s.Title
				break // first surface per pane is enough
			}
		}
	}

	consumed := make(map[string]bool)

	// Pass 1: title-confirmed matches (high confidence).
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		s, ok := sessions[ws.CWD]
		if !ok || consumed[ws.CWD] {
			continue
		}
		patterns := aiTitlePatterns[s.Tool]
		for j := range ws.Panes {
			if ws.Panes[j].Type != "terminal" {
				continue
			}
			title := surfaceTitles[paneKey{ws.Title, ws.Panes[j].Index}]
			if !titleMatchesAI(title, patterns) {
				continue
			}
			ws.Panes[j].Command = s.Command
			consumed[ws.CWD] = true
			break
		}
	}

	// Pass 2: CWD-only fallback for sessions not matched in pass 1.
	// Only fires if the CWD wasn't consumed by a title-confirmed match,
	// AND the workspace has exactly one terminal pane (reduces ambiguity).
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		s, ok := sessions[ws.CWD]
		if !ok || consumed[ws.CWD] {
			continue
		}
		termCount := 0
		termIdx := -1
		for j := range ws.Panes {
			if ws.Panes[j].Type == "terminal" {
				termCount++
				termIdx = j
			}
		}
		if termCount == 1 {
			ws.Panes[termIdx].Command = s.Command
			consumed[ws.CWD] = true
		}
	}
}

// titleMatchesAI checks whether a surface title contains any of the
// given AI tool name patterns (case-insensitive).
func titleMatchesAI(title string, patterns []string) bool {
	lower := strings.ToLower(title)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// mergeUserEdits preserves user-edited fields from an existing TOML.
// Fields like split direction, command, and description are kept from existing
// if the user has edited them (since the live tree doesn't expose these).
func mergeUserEdits(live, existing *model.Layout) {
	if live.Description == "" && existing.Description != "" {
		live.Description = existing.Description
	}

	// Build index of existing workspaces by title for matching.
	existByTitle := make(map[string]*model.Workspace)
	for i := range existing.Workspaces {
		existByTitle[existing.Workspaces[i].Title] = &existing.Workspaces[i]
	}

	for i := range live.Workspaces {
		lw := &live.Workspaces[i]
		ew, ok := existByTitle[lw.Title]
		if !ok {
			continue
		}
		// Preserve user-set workspace description (live tree doesn't expose it).
		if lw.Description == "" && ew.Description != "" {
			lw.Description = ew.Description
		}
		// Merge pane-level user edits.
		for j := range lw.Panes {
			if j >= len(ew.Panes) {
				break
			}
			ep := &ew.Panes[j]
			lp := &lw.Panes[j]
			// Preserve user-set split direction.
			if ep.Split != "" && ep.Split != "right" {
				lp.Split = ep.Split
			}
			// Preserve user-set command.
			if ep.Command != "" {
				lp.Command = ep.Command
			}
		}
	}
}
