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

	// Clear stale auto-detected commands — but only for workspaces where
	// the AI process is no longer running. If the process is still active,
	// keep the existing command (its session ID was correct from the first
	// detection; re-detecting could pick a different .jsonl file because
	// Claude touches multiple session files in the background).
	detected := detect.AISessions()
	clearStaleCommands(layout, detected)

	// Auto-detect running AI CLI sessions and populate resume commands.
	// Surface titles from the tree confirm which panes actually run an AI CLI,
	// preventing false matches when multiple workspaces share a CWD.
	if os.Getenv("CREX_DEBUG") != "" {
		debugDetection(layout, win.Workspaces)
	}
	applyDetectedSessions(layout, win.Workspaces, detected)

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

// debugDetection prints detection diagnostics when CREX_DEBUG is set.
func debugDetection(layout *model.Layout, treeWorkspaces []client.TreeWorkspace) {
	detected := detect.AISessions()
	fmt.Fprintf(os.Stderr, "\n  [debug] Detected sessions:\n")
	for cwd, sessions := range detected.ByCWD {
		for _, s := range sessions {
			fmt.Fprintf(os.Stderr, "    tool=%s cwd=%s cmd=%s\n", s.Tool, cwd, s.Command)
		}
	}
	fmt.Fprintf(os.Stderr, "  [debug] Surface titles:\n")
	for _, tw := range treeWorkspaces {
		for _, tp := range tw.Panes {
			for _, s := range tp.Surfaces {
				fmt.Fprintf(os.Stderr, "    ws=%q pane=%d title=%q\n", tw.Title, tp.Index, s.Title)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "  [debug] Layout workspaces:\n")
	for _, ws := range layout.Workspaces {
		fmt.Fprintf(os.Stderr, "    ws=%q cwd=%s panes=%d\n", ws.Title, ws.CWD, len(ws.Panes))
	}
	fmt.Fprintln(os.Stderr)
}

// aiResumePatterns matches commands that were auto-detected in a previous save.
// These are cleared before re-detection to prevent stale commands from persisting.
var aiResumePatterns = []string{
	"claude --resume ",
	"opencode --session ",
	"codex resume ",
}

// clearStaleCommands removes AI resume commands from workspaces where the
// AI process is no longer running. Commands in workspaces where the process
// IS still active are kept — their session IDs are correct from the first
// detection and re-detection could pick a wrong .jsonl file.
func clearStaleCommands(layout *model.Layout, detected detect.DetectedSessions) {
	// Build set of CWDs where AI processes are currently running.
	activeCWDs := make(map[string]bool)
	for cwd := range detected.ByCWD {
		activeCWDs[cwd] = true
	}

	for i := range layout.Workspaces {
		for j := range layout.Workspaces[i].Panes {
			cmd := layout.Workspaces[i].Panes[j].Command
			for _, pattern := range aiResumePatterns {
				if strings.HasPrefix(cmd, pattern) {
					// Only clear if no AI process is running at this CWD.
					if !activeCWDs[layout.Workspaces[i].CWD] {
						layout.Workspaces[i].Panes[j].Command = ""
					}
					break
				}
			}
		}
	}
}

// aiTitlePatterns is populated from the detector registry in the detect package.
// Each tool's title patterns and detection logic are co-located there.
var aiTitlePatterns = detect.TitlePatterns()

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
func applyDetectedSessions(layout *model.Layout, treeWorkspaces []client.TreeWorkspace, detected detect.DetectedSessions) {
	if len(detected.ByCWD) == 0 {
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

	consumed := make(map[string]bool) // consumed session commands (unique per session)

	// findSession returns an unconsumed session for the given tool from a CWD list.
	findSession := func(sessions []detect.Session, tool string) *detect.Session {
		for i := range sessions {
			if sessions[i].Tool == tool && !consumed[sessions[i].Command] {
				return &sessions[i]
			}
		}
		return nil
	}

	// Pass 1: CWD + title confirmed. Workspace CWD matches process CWD
	// and the pane's surface title confirms the tool is running.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		sessions := detected.ByCWD[ws.CWD]
		if len(sessions) == 0 {
			continue
		}
		for j := range ws.Panes {
			if ws.Panes[j].Type != "terminal" {
				continue
			}
			// Use slice position j as the pane index for title lookup,
			// not ws.Panes[j].Index which can be 0 for all panes when
			// the index field was omitted from TOML (omitempty + merge).
			title := surfaceTitles[paneKey{ws.Title, j}]
			for tool, patterns := range aiTitlePatterns {
				if !titleMatchesAI(title, patterns) {
					continue
				}
				if s := findSession(sessions, tool); s != nil {
					ws.Panes[j].Command = s.Command
					consumed[s.Command] = true
				}
				break
			}
		}
	}

	// Pass 2: CWD-only fallback (single-pane workspaces, no title needed).
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		sessions := detected.ByCWD[ws.CWD]
		if len(sessions) == 0 {
			continue
		}
		if len(ws.Panes) != 1 || ws.Panes[0].Type != "terminal" {
			continue
		}
		// Assign the first unconsumed session at this CWD.
		for _, s := range sessions {
			if !consumed[s.Command] {
				ws.Panes[0].Command = s.Command
				consumed[s.Command] = true
				break
			}
		}
	}

	// Pass 3: title-confirmed but CWD mismatch. The pane's title shows
	// an AI tool, but the process CWD doesn't match the workspace CWD
	// (e.g. user cd'd to ~ before launching the tool). Look up the session
	// by tool name instead of CWD.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		for j := range ws.Panes {
			if ws.Panes[j].Type != "terminal" || ws.Panes[j].Command != "" {
				continue
			}
			title := surfaceTitles[paneKey{ws.Title, j}]
			for tool, patterns := range aiTitlePatterns {
				if !titleMatchesAI(title, patterns) {
					continue
				}
				if s := findSession(detected.ByTool[tool], tool); s != nil {
					ws.Panes[j].Command = s.Command
					consumed[s.Command] = true
				}
				break
			}
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
