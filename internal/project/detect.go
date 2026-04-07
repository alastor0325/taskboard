package project

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Detect returns the active project name using the priority order:
// 1. explicit argument (pass "" to skip)
// 2. TASKBOARD_PROJECT env var
// 3. tmux session name
// 4. Zellij session name
// 5. ~/.taskboard/ directory scan
// 6. CWD basename if it starts with "firefox-"
// 7. random session-{hex} fallback
func Detect(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("TASKBOARD_PROJECT"); v != "" {
		return v
	}
	if name := tmuxSession(); name != "" {
		return name
	}
	if name := os.Getenv("ZELLIJ_SESSION_NAME"); name != "" {
		return name
	}
	if name := firefoxManagerScan(); name != "" {
		return name
	}
	if cwd, err := os.Getwd(); err == nil {
		base := filepath.Base(cwd)
		if strings.HasPrefix(base, "firefox-") {
			return base
		}
	}
	return randomSession()
}

// Sanitize strips path separators and ".." components so the project name is
// safe to embed in /tmp/ filenames.
func Sanitize(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return name
}

// TeamFile returns the path to team.json for the given project.
func TeamFile(project string) string {
	return filepath.Join(os.Getenv("HOME"), ".taskboard", project, "team.json")
}

// StatusFile returns the path to agent-status.json, honouring the
// AGENT_STATUS_FILE env override used by tests.
func StatusFile(project string) string {
	if v := os.Getenv("AGENT_STATUS_FILE"); v != "" {
		return v
	}
	return filepath.Join(os.Getenv("HOME"), ".taskboard", project, "agent-status.json")
}

func tmuxSession() string {
	out, err := exec.Command("tmux", "display-message", "-p", "#S").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firefoxManagerScan() string {
	dir := filepath.Join(os.Getenv("HOME"), ".taskboard")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		return ""
	}
	if entries[0].IsDir() {
		return entries[0].Name()
	}
	return ""
}

func randomSession() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "session-" + hex.EncodeToString(b)
}
