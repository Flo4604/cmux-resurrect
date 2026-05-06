package orchestrate

import (
	"fmt"
	"os"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

const (
	// ShellReadyTimeout is the maximum time to wait for a shell to become interactive.
	ShellReadyTimeout = 10 * time.Second

	// ShellReadyPoll is the interval between readiness probes.
	ShellReadyPoll = 200 * time.Millisecond

	// ShellReadySettle is a brief pause after the probe succeeds,
	// giving the shell time to render its prompt before the real command
	// is sent. Without this, the command can arrive between "probe executed"
	// and "prompt displayed", causing it to be echoed but not interpreted.
	ShellReadySettle = 300 * time.Millisecond
)

// waitForShellReady polls until the shell in the target pane is interactive.
//
// It repeatedly sends a probe command ("touch <sentinel>") to the pane and
// checks whether the sentinel file was created. During shell initialization
// (sourcing .zshrc, oh-my-zsh, starship, etc.) the probe keystrokes are
// consumed or lost — the file never appears. Once the shell's interactive
// line editor (readline/zle) takes over stdin, the probe executes and creates
// the file, proving the shell is ready to receive the real command.
//
// This approach is:
//   - Universal: works with any shell (bash, zsh, fish) and any backend.
//   - Self-healing: lost probes are retried automatically.
//   - Deterministic: file existence is binary proof, not a timing guess.
func waitForShellReady(c client.Backend, workspaceRef, surfaceRef string) error {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	sentinel := fmt.Sprintf("/tmp/.crex-rdy-%s", id)
	defer os.Remove(sentinel)

	deadline := time.Now().Add(ShellReadyTimeout)
	// Leading space hides the probe from shell history (HISTCONTROL/HIST_IGNORE_SPACE).
	probe := fmt.Sprintf(" touch %s\\n", sentinel)

	for time.Now().Before(deadline) {
		_ = c.Send(workspaceRef, surfaceRef, probe)
		time.Sleep(ShellReadyPoll)
		if _, err := os.Stat(sentinel); err == nil {
			// File exists — shell processed our probe and is interactive.
			// Wait briefly for the prompt to finish rendering.
			time.Sleep(ShellReadySettle)
			return nil
		}
	}
	return fmt.Errorf("shell not ready after %v", ShellReadyTimeout)
}
