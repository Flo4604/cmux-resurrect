package orchestrate

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func minimalTree() *client.TreeResponse {
	return &client.TreeResponse{
		Windows: []client.TreeWindow{
			{
				Ref: "window:1",
				Workspaces: []client.TreeWorkspace{
					{
						Ref:   "workspace:1",
						Title: "dev",
						Index: 0,
						Panes: []client.TreePane{
							{
								Index: 0,
								Surfaces: []client.TreeSurface{
									{Type: "terminal", TTY: "/dev/ttys001"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestWatcher_SkipsLogOnSameRevision(t *testing.T) {
	mc := &mockClient{
		treeResp: minimalTree(),
		sidebarCWDs: map[string]string{
			"workspace:1": "/tmp/dev",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	var buf bytes.Buffer
	w := &Watcher{
		Client:    mc,
		Store:     store,
		Name:      "watch-skip-test",
		Interval:  time.Second,
		LogWriter: &buf,
	}

	// First call — should save and log.
	w.saveOnce()
	if !strings.Contains(buf.String(), "saved") {
		t.Errorf("first saveOnce() should log 'saved', got: %q", buf.String())
	}

	// Reset buffer and call again with same state — revision unchanged, should NOT log.
	buf.Reset()
	w.saveOnce()
	if buf.Len() != 0 {
		t.Errorf("second saveOnce() with same revision should not log, got: %q", buf.String())
	}
}

func TestWatcher_LogsOnRevisionChange(t *testing.T) {
	mc := &mockClient{
		treeResp: minimalTree(),
		sidebarCWDs: map[string]string{
			"workspace:1": "/tmp/dev",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	var buf bytes.Buffer
	w := &Watcher{
		Client:    mc,
		Store:     store,
		Name:      "watch-change-test",
		Interval:  time.Second,
		LogWriter: &buf,
	}

	// First call establishes revision.
	w.saveOnce()
	buf.Reset()

	// Change the state so the next save produces a new revision.
	mc.sidebarCWDs["workspace:1"] = "/tmp/other"

	// Second call — revision should increment, should log with "rev".
	w.saveOnce()
	if !strings.Contains(buf.String(), "rev") {
		t.Errorf("saveOnce() after state change should log with 'rev', got: %q", buf.String())
	}
}
