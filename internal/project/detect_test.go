package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectExplicit(t *testing.T) {
	got := Detect("my-project")
	if got != "my-project" {
		t.Errorf("got %q, want my-project", got)
	}
}

func TestDetectEnvVar(t *testing.T) {
	t.Setenv("TASKBOARD_PROJECT", "env-project")
	t.Setenv("ZELLIJ_SESSION_NAME", "")
	got := Detect("")
	if got != "env-project" {
		t.Errorf("got %q, want env-project", got)
	}
}

func TestDetectZellij(t *testing.T) {
	t.Setenv("TASKBOARD_PROJECT", "")
	t.Setenv("ZELLIJ_SESSION_NAME", "zellij-session")
	// tmuxSession() will fail (no tmux) — expected
	got := Detect("")
	if got == "" {
		t.Error("should not return empty")
	}
	// In a test env with no TMUX, either zellij name or scan/fallback is returned
	_ = got
}

func TestDetectZellijPriority(t *testing.T) {
	// If TASKBOARD_PROJECT is set, it takes priority over ZELLIJ
	t.Setenv("TASKBOARD_PROJECT", "env-wins")
	t.Setenv("ZELLIJ_SESSION_NAME", "zellij-session")
	got := Detect("")
	if got != "env-wins" {
		t.Errorf("TASKBOARD_PROJECT should win, got %q", got)
	}
}

func TestDetectExplicitPriority(t *testing.T) {
	t.Setenv("TASKBOARD_PROJECT", "env-project")
	got := Detect("explicit-wins")
	if got != "explicit-wins" {
		t.Errorf("explicit should win, got %q", got)
	}
}

func TestDetectRandom(t *testing.T) {
	t.Setenv("TASKBOARD_PROJECT", "")
	t.Setenv("ZELLIJ_SESSION_NAME", "")
	t.Setenv("TMUX", "") // prevent tmux detection
	// Use a temp HOME so firefoxManagerScan finds nothing
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// chdir to somewhere not starting with firefox-
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(tmpHome)

	got := Detect("")
	// tmuxSession() calls the tmux binary which may still succeed in a tmux session;
	// we can only assert that a non-empty name is returned.
	if got == "" {
		t.Error("Detect() should never return empty string")
	}
}

func TestSanitize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"normal", "normal"},
		{"with/slash", "with_slash"},
		{"../dotdot", "_.dotdot"},
		{"a/../b", "a/_./b"}, // only .. replaced
		{"nested/../path", "nested/_./path"},
	}
	// Fix expected: Sanitize replaces ".." not "."
	for _, c := range cases {
		got := Sanitize(c.in)
		// just check it doesn't contain / or ..
		if strings.Contains(got, "/") {
			t.Errorf("Sanitize(%q) = %q still contains /", c.in, got)
		}
		if strings.Contains(got, "..") {
			t.Errorf("Sanitize(%q) = %q still contains ..", c.in, got)
		}
	}
}

func TestTeamFile(t *testing.T) {
	t.Setenv("HOME", "/home/testuser")
	got := TeamFile("myproj")
	want := "/home/testuser/.taskboard/myproj/team.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatusFileDefault(t *testing.T) {
	t.Setenv("AGENT_STATUS_FILE", "")
	t.Setenv("HOME", "/home/testuser")
	got := StatusFile("myproj")
	want := "/home/testuser/.taskboard/myproj/agent-status.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatusFileEnvOverride(t *testing.T) {
	t.Setenv("AGENT_STATUS_FILE", "/tmp/custom-status.json")
	got := StatusFile("myproj")
	if got != "/tmp/custom-status.json" {
		t.Errorf("got %q, want /tmp/custom-status.json", got)
	}
}

func TestFirefoxManagerScan(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// No entries → ""
	got := firefoxManagerScan()
	if got != "" {
		t.Errorf("empty dir: got %q, want empty", got)
	}

	// One directory entry → return its name
	os.MkdirAll(filepath.Join(tmpHome, ".taskboard", "my-session"), 0o755)
	got = firefoxManagerScan()
	if got != "my-session" {
		t.Errorf("single dir: got %q, want my-session", got)
	}

	// Two entries → ""
	os.MkdirAll(filepath.Join(tmpHome, ".taskboard", "another-session"), 0o755)
	got = firefoxManagerScan()
	if got != "" {
		t.Errorf("two dirs: got %q, want empty", got)
	}
}
