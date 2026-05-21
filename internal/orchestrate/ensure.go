package orchestrate

import (
	"fmt"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// EnsurePolicy controls how EnsureWorkspace handles existing workspaces.
type EnsurePolicy int

const (
	CreateOnly    EnsurePolicy = iota // error if workspace exists
	CreateOrReuse                     // reuse if exists, create if not
	ReuseOnly                         // error if workspace doesn't exist
	ForceRecreate                     // close existing, recreate from layout
)

// EnsureResult reports what EnsureWorkspace did.
type EnsureResult struct {
	Action  string // "created", "reused", "recreated"
	Ref     string // workspace ref
	Existed bool
}

// EnsureWorkspace creates or reuses a workspace according to the given policy.
// It checks existing workspaces by title and acts accordingly.
func (r *Restorer) EnsureWorkspace(ws model.Workspace, policy EnsurePolicy) (*EnsureResult, error) {
	existing, err := r.Client.ListWorkspaces()
	if err != nil {
		existing = nil // proceed without existing list
	}

	// Find existing workspace by title.
	var existingRef string
	for _, ew := range existing {
		if ew.Title == ws.Title {
			existingRef = ew.Ref
			break
		}
	}

	existed := existingRef != ""

	switch policy {
	case CreateOnly:
		if existed {
			return nil, fmt.Errorf("workspace %q already exists", ws.Title)
		}
		ref, err := r.createWorkspace(ws)
		if err != nil {
			return nil, err
		}
		return &EnsureResult{Action: "created", Ref: ref, Existed: false}, nil

	case CreateOrReuse:
		if existed {
			return &EnsureResult{Action: "reused", Ref: existingRef, Existed: true}, nil
		}
		ref, err := r.createWorkspace(ws)
		if err != nil {
			return nil, err
		}
		return &EnsureResult{Action: "created", Ref: ref, Existed: false}, nil

	case ReuseOnly:
		if !existed {
			return nil, fmt.Errorf("workspace %q not found", ws.Title)
		}
		return &EnsureResult{Action: "reused", Ref: existingRef, Existed: true}, nil

	case ForceRecreate:
		if existed {
			_ = r.Client.UnpinWorkspace(existingRef)
			_ = r.Client.CloseWorkspace(existingRef)
			time.Sleep(DelayAfterClose)
		}
		ref, err := r.createWorkspace(ws)
		if err != nil {
			return nil, err
		}
		action := "created"
		if existed {
			action = "recreated"
		}
		return &EnsureResult{Action: action, Ref: ref, Existed: existed}, nil

	default:
		return nil, fmt.Errorf("unknown ensure policy: %d", policy)
	}
}

// createWorkspace creates a workspace and sets up its panes, commands, and title.
func (r *Restorer) createWorkspace(ws model.Workspace) (string, error) {
	result := &RestoreResult{} // collect errors internally
	ref, err := r.restoreWorkspace(ws, false, result)
	if err != nil {
		return "", err
	}
	return ref, nil
}
