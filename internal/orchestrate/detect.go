package orchestrate

import "github.com/drolosoft/cmux-resurrect/internal/client"

// RestoreHint tells the UI which prompts to show based on pre-detection.
type RestoreHint int

const (
	HintNoop    RestoreHint = iota // layout already matches — nothing to do
	HintAutoAdd                    // no extras, Replace=Add — skip all prompts
	HintAskMode                    // ask Replace/Add only, skip Skip/Fresh
	HintAskBoth                    // ask Replace/Add, then Skip/Fresh
)

// RestoreState holds the pre-detection results.
type RestoreState struct {
	Hint     RestoreHint
	Matching int // titles in both existing and layout
	Extras   int // existing titles NOT in layout (excluding caller)
	Missing  int // layout titles NOT in existing
}

// DetectRestoreState compares existing tabs against a layout to determine
// which restore prompts are needed.
func DetectRestoreState(cl client.Backend, layoutTitles []string) RestoreState {
	layoutSet := make(map[string]bool, len(layoutTitles))
	for _, t := range layoutTitles {
		layoutSet[t] = true
	}

	var callerTitle string
	if tree, err := cl.Tree(); err == nil && tree.Caller != nil {
		for _, w := range tree.Windows {
			for _, ws := range w.Workspaces {
				if ws.Ref == tree.Caller.WorkspaceRef {
					callerTitle = ws.Title
				}
			}
		}
	}

	existing, err := cl.ListWorkspaces()
	if err != nil {
		return RestoreState{Hint: HintAskBoth, Missing: len(layoutTitles)}
	}

	existingSet := make(map[string]bool, len(existing))
	for _, ws := range existing {
		existingSet[ws.Title] = true
	}

	var matching, extras, missing int
	for _, ws := range existing {
		if ws.Title == callerTitle {
			continue
		}
		if layoutSet[ws.Title] {
			matching++
		} else {
			extras++
		}
	}
	for _, t := range layoutTitles {
		if !existingSet[t] {
			missing++
		}
	}

	state := RestoreState{Matching: matching, Extras: extras, Missing: missing}

	switch {
	case extras == 0 && missing == 0:
		state.Hint = HintNoop
	case extras == 0:
		state.Hint = HintAutoAdd
	case matching == 0:
		state.Hint = HintAskMode
	case missing > 0:
		state.Hint = HintAskBoth
	default:
		state.Hint = HintAskMode
	}

	return state
}
