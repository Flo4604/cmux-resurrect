package orchestrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// Watcher periodically saves the cmux layout, deduplicating via revision number.
type Watcher struct {
	Client        client.Backend
	Store         persist.Store
	Name          string
	Interval      time.Duration
	WorkspaceFile string    // MD file path; if set, also updates the MD on each tick
	LogWriter     io.Writer // if set, log to this writer instead of stderr

	lastRevision uint64
}

func (w *Watcher) logWriter() io.Writer {
	if w.LogWriter != nil {
		return w.LogWriter
	}
	return os.Stderr
}

// Run starts the watch loop, blocking until interrupted.
func (w *Watcher) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Save immediately on start.
	w.saveOnce()

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_, _ = fmt.Fprintf(w.logWriter(), "\nStopping watcher. Final save...\n")
			w.saveOnce()
			return nil
		case <-ticker.C:
			w.saveOnce()
		}
	}
}

func (w *Watcher) saveOnce() {
	saver := &Saver{Client: w.Client, Store: w.Store}
	layout, err := saver.Save(w.Name, "autosave")
	if err != nil {
		_, _ = fmt.Fprintf(w.logWriter(), "  watch save error: %v\n", err)
		return
	}

	if layout.Revision == w.lastRevision {
		return // no change, skip logging and MD update
	}
	w.lastRevision = layout.Revision

	// Also update the MD file if configured.
	if w.WorkspaceFile != "" {
		exporter := &Exporter{Client: w.Client}
		if err := exporter.ExportToMD(w.WorkspaceFile); err != nil {
			_, _ = fmt.Fprintf(w.logWriter(), "  watch md update error: %v\n", err)
		}
	}

	_, _ = fmt.Fprintf(w.logWriter(), "  saved %d workspaces (rev %d) at %s\n",
		len(layout.Workspaces),
		layout.Revision,
		time.Now().Format("15:04:05"))
}
