package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestDetectRestoreState(t *testing.T) {
	tests := []struct {
		name         string
		existing     []client.WorkspaceInfo
		callerRef    string
		callerTitle  string
		layoutTitles []string
		wantHint     RestoreHint
		wantMatching int
		wantExtras   int
		wantMissing  int
	}{
		{
			name: "FreshTerminal",
			existing: []client.WorkspaceInfo{
				{Ref: "workspace:1", Title: "crex"},
			},
			callerRef:    "workspace:1",
			callerTitle:  "crex",
			layoutTitles: []string{"dev", "docs"},
			wantHint:     HintAutoAdd,
			wantMatching: 0,
			wantExtras:   0,
			wantMissing:  2,
		},
		{
			name: "AlreadyMatches",
			existing: []client.WorkspaceInfo{
				{Ref: "workspace:1", Title: "crex"},
				{Ref: "workspace:2", Title: "dev"},
				{Ref: "workspace:3", Title: "docs"},
			},
			callerRef:    "workspace:1",
			callerTitle:  "crex",
			layoutTitles: []string{"dev", "docs"},
			wantHint:     HintNoop,
			wantMatching: 2,
			wantExtras:   0,
			wantMissing:  0,
		},
		{
			name: "ExtrasNoMatching",
			existing: []client.WorkspaceInfo{
				{Ref: "workspace:1", Title: "crex"},
				{Ref: "workspace:2", Title: "old-project"},
			},
			callerRef:    "workspace:1",
			callerTitle:  "crex",
			layoutTitles: []string{"dev", "docs"},
			wantHint:     HintAskMode,
			wantMatching: 0,
			wantExtras:   1,
			wantMissing:  2,
		},
		{
			name: "AllThreeSets",
			existing: []client.WorkspaceInfo{
				{Ref: "workspace:1", Title: "crex"},
				{Ref: "workspace:2", Title: "dev"},
				{Ref: "workspace:3", Title: "old-stuff"},
			},
			callerRef:    "workspace:1",
			callerTitle:  "crex",
			layoutTitles: []string{"dev", "docs"},
			wantHint:     HintAskBoth,
			wantMatching: 1,
			wantExtras:   1,
			wantMissing:  1,
		},
		{
			name: "ExtrasMatchingNoMissing",
			existing: []client.WorkspaceInfo{
				{Ref: "workspace:1", Title: "crex"},
				{Ref: "workspace:2", Title: "dev"},
				{Ref: "workspace:3", Title: "old-stuff"},
			},
			callerRef:    "workspace:1",
			callerTitle:  "crex",
			layoutTitles: []string{"dev"},
			wantHint:     HintAskMode,
			wantMatching: 1,
			wantExtras:   1,
			wantMissing:  0,
		},
		{
			name: "NoExtras_SomeMatching_SomeMissing",
			existing: []client.WorkspaceInfo{
				{Ref: "workspace:1", Title: "crex"},
				{Ref: "workspace:2", Title: "dev"},
			},
			callerRef:    "workspace:1",
			callerTitle:  "crex",
			layoutTitles: []string{"dev", "docs"},
			wantHint:     HintAutoAdd,
			wantMatching: 1,
			wantExtras:   0,
			wantMissing:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := newSyncMockClient(tc.existing, tc.callerRef, tc.callerTitle)
			got := DetectRestoreState(mc, tc.layoutTitles)

			if got.Hint != tc.wantHint {
				t.Errorf("Hint = %d, want %d", got.Hint, tc.wantHint)
			}
			if got.Matching != tc.wantMatching {
				t.Errorf("Matching = %d, want %d", got.Matching, tc.wantMatching)
			}
			if got.Extras != tc.wantExtras {
				t.Errorf("Extras = %d, want %d", got.Extras, tc.wantExtras)
			}
			if got.Missing != tc.wantMissing {
				t.Errorf("Missing = %d, want %d", got.Missing, tc.wantMissing)
			}
		})
	}
}
