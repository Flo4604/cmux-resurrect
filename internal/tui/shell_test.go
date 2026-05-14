package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func TestShellModel_WelcomeInInit(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	if !strings.Contains(m.welcome, "crex") {
		t.Error("welcome should contain 'crex'")
	}
	if !strings.Contains(m.welcome, "help") {
		t.Error("welcome should mention 'help'")
	}
}

func TestShellModel_StartsInPromptMode(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	if m.mode != modePrompt {
		t.Errorf("expected modePrompt, got %v", m.mode)
	}
}

func TestShellModel_ViewShowsPromptAndWelcome(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	view := m.View()
	if !strings.Contains(view, "crex") {
		t.Error("view should show the prompt with crex")
	}
	// Welcome is rendered as part of lastOutput in alt screen mode.
	if !strings.Contains(view, "interactive shell") {
		t.Error("view should contain welcome text in lastOutput")
	}
}

func TestShellModel_ExitQuits(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("exit")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	if !sm.quitting {
		t.Error("expected quitting=true after 'exit'")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestShellModel_HelpProducesOutput(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("help")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Help output is flushed into lastOutput and rendered in View().
	view := sm.View()
	if !strings.Contains(view, "Layouts") {
		t.Error("help content should appear in View() via lastOutput")
	}
}

func TestShellModel_UnknownCommand(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("wat")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Unknown command error is flushed into lastOutput.
	if !strings.Contains(sm.lastOutput, "Unknown command") {
		t.Error("expected unknown command error in lastOutput")
	}
}

func TestShellModel_EmptyEnterDoesNothing(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	if sm.quitting {
		t.Error("empty enter should not quit")
	}
	if sm.mode != modePrompt {
		t.Error("should stay in prompt mode")
	}
}

func TestShellModel_HistoryRecordsCommands(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")

	m.prompt.SetValue("help")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	if len(sm.history) != 1 {
		t.Errorf("history length = %d, want 1", len(sm.history))
	}
	if sm.history[0] != "help" {
		t.Errorf("history[0] = %q, want %q", sm.history[0], "help")
	}
}

func TestShellModel_CtrlCQuits(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	sm := result.(*ShellModel)

	if !sm.quitting {
		t.Error("ctrl+c should quit")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func saveTestLayout(t *testing.T, dir string) persist.Store {
	t.Helper()
	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	layout := &model.Layout{
		Name:    "test",
		SavedAt: time.Now(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{{Type: "terminal"}},
		}},
	}
	if err := store.Save("test", layout); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return store
}

func TestShellModel_RestoreAsk_ShowsPrompt(t *testing.T) {
	store := saveTestLayout(t, t.TempDir())
	m := NewShellModel(store, nil, client.BackendCmux, "")
	// restoreMode is "" (default) → should prompt

	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	if sm.mode != modeRestoreAsk {
		t.Fatalf("expected modeRestoreAsk (%d), got %d", modeRestoreAsk, sm.mode)
	}
}

func TestShellModel_RestoreAsk_RSelectsReplace(t *testing.T) {
	store := saveTestLayout(t, t.TempDir())
	m := NewShellModel(store, nil, client.BackendCmux, "")

	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	if sm.mode != modeRestoreAsk {
		t.Fatalf("expected modeRestoreAsk, got %d", sm.mode)
	}

	// Press 'r' for replace — should transition to modeRestoreSkip (second question).
	result2, _ := sm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	sm2 := result2.(*ShellModel)

	if sm2.mode != modeRestoreSkip {
		t.Errorf("expected modeRestoreSkip after 'r', got %d", sm2.mode)
	}

	// Press 's' for skip — should start restore.
	result3, _ := sm2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	sm3 := result3.(*ShellModel)

	if sm3.mode != modePrompt {
		t.Errorf("expected modePrompt after 's', got %d", sm3.mode)
	}
}

func TestShellModel_RestoreAsk_EscCancels(t *testing.T) {
	store := saveTestLayout(t, t.TempDir())
	m := NewShellModel(store, nil, client.BackendCmux, "")

	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Press Escape to cancel
	result2, cmd := sm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	sm2 := result2.(*ShellModel)

	if sm2.mode != modePrompt {
		t.Error("expected modePrompt after cancel")
	}
	if cmd != nil {
		t.Error("cancel should not trigger restore")
	}
	if !strings.Contains(sm2.lastOutput, "Cancelled") {
		t.Error("should show Cancelled message")
	}
}

func TestShellModel_RestoreExplicitMode_SkipsPrompt(t *testing.T) {
	store := saveTestLayout(t, t.TempDir())
	m := NewShellModel(store, nil, client.BackendCmux, "")
	m.SetRestoreMode("add")

	m.prompt.SetValue("restore test")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(*ShellModel)

	// Mode is "add" → should NOT enter modeRestoreAsk, should dispatch directly.
	if sm.mode == modeRestoreAsk {
		t.Error("explicit mode should skip the prompt")
	}
	// With no client connected, startRestore returns nil cmd (error printed),
	// but the key point is that modeRestoreAsk was never entered.
	if sm.mode != modePrompt {
		t.Errorf("expected modePrompt, got %d", sm.mode)
	}
}

func TestShellModel_ConfirmFnError_NoSuccessMsg(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendCmux, "")

	// Set up a confirmFn that writes an error.
	m.mode = modeConfirm
	m.confirmMsg = "Delete?"
	m.confirmFn = func() {
		m.output.WriteString(shellErrorStyle.Render("  ✗ something failed"))
		m.output.WriteString("\n")
	}

	// Press 'y'
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	sm := result.(*ShellModel)

	if strings.Contains(sm.lastOutput, "Done") {
		t.Error("should not show 'Done' when confirmFn wrote an error")
	}
	if !strings.Contains(sm.lastOutput, "something failed") {
		t.Error("should show the error from confirmFn")
	}
}
