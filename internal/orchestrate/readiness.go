package orchestrate

import (
	"fmt"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

const (
	// ShellReadyTimeout is the maximum time to wait for a shell to become interactive.
	ShellReadyTimeout = 10 * time.Second

	// ShellReadyPoll is the interval between readiness checks.
	ShellReadyPoll = 150 * time.Millisecond

	// ShellReadySettle is a pause after the shell's CWD appears, giving
	// it time to finish sourcing .zshrc, oh-my-zsh, starship, etc.
	// and enter interactive mode. The CWD is set early in shell startup;
	// the settle covers the gap between "shell process started" and
	// "readline/zle is accepting input."
	ShellReadySettle = 1 * time.Second
)

// waitForShellReady polls the backend until the shell in the target pane
// has initialized. It checks the workspace's working directory via
// SidebarState — when the CWD is populated, the shell process is running.
// A settle delay then covers the remaining init time (.zshrc, etc.).
//
// This approach sends NO text to the terminal — no probe commands, no
// temp files, no shell history pollution, no visible artifacts.
func waitForShellReady(c client.Backend, workspaceRef, surfaceRef string) error {
	deadline := time.Now().Add(ShellReadyTimeout)

	for time.Now().Before(deadline) {
		sidebar, err := c.SidebarState(workspaceRef)
		if err == nil && sidebar != nil && sidebar.CWD != "" {
			// Shell has initialized its CWD. Wait for prompt rendering
			// and .zshrc sourcing to complete before sending commands.
			time.Sleep(ShellReadySettle)
			return nil
		}
		time.Sleep(ShellReadyPoll)
	}

	return fmt.Errorf("shell not ready after %v", ShellReadyTimeout)
}

// noHistoryCmd prefixes a command with a space so shells with
// HIST_IGNORE_SPACE (zsh default, bash with HISTCONTROL=ignorespace)
// don't record it in history. The trailing \\n triggers Enter.
func noHistoryCmd(cmd string) string {
	return " " + cmd + "\\n"
}
