// Package detect discovers running AI CLI sessions (Claude Code, OpenCode, Codex)
// and returns resume commands for each. All functions are best-effort: if any
// detection step fails, it is silently skipped. The caller never sees an error.
package detect

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Session represents a detected AI CLI session.
type Session struct {
	Tool    string // "claude", "opencode", "codex"
	CWD     string // working directory of the process
	Command string // full resume command (e.g. "claude --resume <id>")
}

// AISessions scans for running AI CLI processes and resolves their session IDs.
// Returns a map of CWD → Session for easy pane matching. If multiple sessions
// share a CWD, the last one detected wins (rare edge case).
//
// This function never returns an error. If detection fails at any stage,
// it returns whatever it found — possibly an empty map.
func AISessions() map[string]Session {
	result := make(map[string]Session)

	procs := listAIProcesses()
	for _, p := range procs {
		var s *Session
		switch p.tool {
		case "claude":
			s = detectClaude(p.cwd)
		case "opencode":
			s = detectOpenCode(p.cwd)
		case "codex":
			s = detectCodex(p.cwd)
		}
		if s != nil {
			result[s.CWD] = *s
		}
	}
	return result
}

// aiProcess holds a running AI CLI process.
type aiProcess struct {
	tool string // "claude", "opencode", "codex"
	pid  string
	cwd  string
}

// listAIProcesses finds running claude, opencode, and codex processes
// and resolves their working directories via lsof.
func listAIProcesses() []aiProcess {
	out, err := exec.Command("ps", "axo", "pid,comm").Output()
	if err != nil {
		return nil
	}

	var pids []struct {
		pid  string
		tool string
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid := fields[0]
		comm := filepath.Base(fields[1])

		switch comm {
		case "claude":
			pids = append(pids, struct {
				pid  string
				tool string
			}{pid, "claude"})
		case "opencode":
			pids = append(pids, struct {
				pid  string
				tool string
			}{pid, "opencode"})
		case "codex":
			pids = append(pids, struct {
				pid  string
				tool string
			}{pid, "codex"})
		}
	}

	var result []aiProcess
	for _, p := range pids {
		cwd := cwdForPID(p.pid)
		if cwd == "" {
			continue
		}
		result = append(result, aiProcess{tool: p.tool, pid: p.pid, cwd: cwd})
	}
	return result
}

// cwdForPID returns the working directory of a process via lsof.
func cwdForPID(pid string) string {
	out, err := exec.Command("lsof", "-p", pid, "-Fn").Output()
	if err != nil {
		return ""
	}
	// lsof -Fn outputs lines like "fcwd\nn/path/to/dir".
	// We look for "fcwd" followed by "n<path>".
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if line == "fcwd" && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "n") {
			return lines[i+1][1:] // strip "n" prefix
		}
	}
	return ""
}

// detectClaude finds the active session ID for a Claude Code instance by CWD.
// Sessions are stored as .jsonl files in ~/.claude/projects/<path-with-dashes>/.
func detectClaude(cwd string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// Claude converts "/" to "-" for the project directory name.
	projectPath := strings.ReplaceAll(cwd, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectPath)

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil
	}

	// Find the most recently modified .jsonl file.
	type fileInfo struct {
		name    string
		modTime int64
	}
	var sessions []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, fileInfo{
			name:    e.Name(),
			modTime: info.ModTime().UnixNano(),
		})
	}
	if len(sessions) == 0 {
		return nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime > sessions[j].modTime
	})

	sessionID := strings.TrimSuffix(sessions[0].name, ".jsonl")
	if !validSessionID.MatchString(sessionID) {
		return nil
	}
	return &Session{
		Tool:    "claude",
		CWD:     cwd,
		Command: "claude --resume " + sessionID,
	}
}

// detectOpenCode finds the active session for an OpenCode instance by CWD.
// Sessions are stored in ~/.local/share/opencode/opencode.db (SQLite).
// We shell out to the sqlite3 CLI (ships with macOS) to avoid CGO dependencies.
func detectOpenCode(cwd string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	// Use the sqlite3 CLI to query the database in read-only mode.
	query := `SELECT id FROM session WHERE directory = '` + escapeSQLite(cwd) + `' ORDER BY time_updated DESC LIMIT 1;`
	out, err := exec.Command("sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return nil
	}

	sessionID := strings.TrimSpace(string(out))
	if sessionID == "" || !validSessionID.MatchString(sessionID) {
		return nil
	}

	return &Session{
		Tool:    "opencode",
		CWD:     cwd,
		Command: "opencode --session " + sessionID,
	}
}

// escapeSQLite escapes single quotes for SQLite string literals.
// The input comes from lsof output (machine-sourced), not user input.
func escapeSQLite(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// validSessionID checks that a session ID contains only safe characters.
// This prevents corrupted or crafted IDs from injecting shell commands
// when the resume command is sent to a terminal via Send.
var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// detectCodex finds the active session for a Codex instance by CWD.
// Codex 0.128+ stores sessions as JSONL files under ~/.codex/sessions/YYYY/MM/DD/.
// The first line of each file contains session metadata with "id" and "cwd".
// Falls back to the legacy rollout-*.json format in ~/.codex/sessions/ root.
func detectCodex(cwd string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sessDir := filepath.Join(home, ".codex", "sessions")

	// Try new format first: dated subdirectories with .jsonl files.
	if s := detectCodexNew(sessDir, cwd); s != nil {
		return s
	}
	// Fall back to legacy format: rollout-*.json in root.
	return detectCodexLegacy(sessDir, cwd)
}

// detectCodexNew handles Codex 0.128+ sessions stored as .jsonl in dated subdirs.
// Each file's first line has type "session_meta" with payload.id and payload.cwd.
// Walks reverse-chronologically from today, bounded to 30 days, to avoid scanning
// the entire session history.
func detectCodexNew(sessDir, cwd string) *Session {
	now := time.Now()
	for daysBack := 0; daysBack < 30; daysBack++ {
		d := now.AddDate(0, 0, -daysBack)
		dayDir := filepath.Join(sessDir, d.Format("2006"), d.Format("01"), d.Format("02"))
		entries, err := os.ReadDir(dayDir)
		if err != nil {
			continue
		}

		// Collect rollout files with their mod times.
		type candidate struct {
			path    string
			modTime int64
		}
		var files []candidate
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "rollout-") || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, candidate{
				path:    filepath.Join(dayDir, e.Name()),
				modTime: info.ModTime().UnixNano(),
			})
		}

		// Sort most recent first within the day.
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime > files[j].modTime
		})

		for _, f := range files {
			id, dir := readCodexJSONLMeta(f.path)
			if id != "" && dir == cwd {
				return &Session{
					Tool:    "codex",
					CWD:     cwd,
					Command: "codex resume " + id,
				}
			}
		}
	}
	return nil
}

// readCodexJSONLMeta reads the first line of a Codex JSONL session file
// and extracts the session ID and CWD from the session_meta payload.
func readCodexJSONLMeta(path string) (id, cwd string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "", ""
	}

	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			CWD string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return "", ""
	}
	if meta.Type != "session_meta" {
		return "", ""
	}
	return meta.Payload.ID, meta.Payload.CWD
}

// detectCodexLegacy handles pre-0.128 Codex sessions (rollout-*.json in root).
func detectCodexLegacy(sessDir, cwd string) *Session {
	matches, err := filepath.Glob(filepath.Join(sessDir, "rollout-*.json"))
	if err != nil || len(matches) == 0 {
		return nil
	}

	// Find the most recently modified rollout file.
	type fileInfo struct {
		path    string
		modTime int64
	}
	var files []fileInfo
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: m, modTime: info.ModTime().UnixNano()})
	}
	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	data, err := os.ReadFile(files[0].path)
	if err != nil {
		return nil
	}

	var rollout struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := json.Unmarshal(data, &rollout); err != nil || rollout.Session.ID == "" {
		return nil
	}
	if !validSessionID.MatchString(rollout.Session.ID) {
		return nil
	}

	return &Session{
		Tool:    "codex",
		CWD:     cwd,
		Command: "codex resume " + rollout.Session.ID,
	}
}
