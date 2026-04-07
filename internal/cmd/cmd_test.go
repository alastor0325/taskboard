package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alastor0325/taskboard/internal/types"
)

// captureStdout redirects os.Stdout to a pipe for the duration of fn and
// returns whatever was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// setupProject creates a temp dir, sets AGENT_STATUS_FILE and
// FIREFOX_MANAGER_PROJECT, and returns the status file path.
func setupProject(t *testing.T) (statusFile, proj string) {
	t.Helper()
	dir := t.TempDir()
	statusFile = filepath.Join(dir, "agent-status.json")
	proj = "test-proj-" + filepath.Base(dir)
	t.Setenv("AGENT_STATUS_FILE", statusFile)
	t.Setenv("FIREFOX_MANAGER_PROJECT", proj)
	// Also redirect the team.json and log.json under the same tmpdir so we
	// don't touch the real ~/.firefox-manager directory.
	t.Setenv("HOME", dir)
	return statusFile, proj
}

func runArgs(t *testing.T, args ...string) error {
	t.Helper()
	os.Args = append([]string{"taskboard"}, args...)
	return Execute()
}

func TestInitCreatesStatusFile(t *testing.T) {
	statusFile, _ := setupProject(t)

	if err := runArgs(t, "init"); err != nil {
		t.Fatalf("init returned error: %v", err)
	}
	if _, err := os.Stat(statusFile); err != nil {
		t.Fatalf("agent-status.json not created: %v", err)
	}
}

func TestSyncUpdatesStatusFile(t *testing.T) {
	statusFile, _ := setupProject(t)

	if err := runArgs(t, "sync"); err != nil {
		t.Fatalf("sync returned error: %v", err)
	}
	if _, err := os.Stat(statusFile); err != nil {
		t.Fatalf("agent-status.json not created by sync: %v", err)
	}
}

func TestSetTask(t *testing.T) {
	statusFile, _ := setupProject(t)

	if err := runArgs(t, "set-task", "42", "--summary", "foo", "--status", "running"); err != nil {
		t.Fatalf("set-task returned error: %v", err)
	}
	data, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatal(err)
	}
	var status types.AgentStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	task, ok := status.Tasks["42"]
	if !ok {
		t.Fatal("task 42 not found in status")
	}
	if task.Summary != "foo" {
		t.Errorf("summary: got %q, want %q", task.Summary, "foo")
	}
	if task.Status != "running" {
		t.Errorf("status: got %q, want running", task.Status)
	}
}

func TestDoneTask(t *testing.T) {
	statusFile, _ := setupProject(t)

	// First create the task.
	if err := runArgs(t, "set-task", "42", "--status", "running"); err != nil {
		t.Fatalf("set-task: %v", err)
	}
	if err := runArgs(t, "done-task", "42"); err != nil {
		t.Fatalf("done-task returned error: %v", err)
	}
	data, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatal(err)
	}
	var status types.AgentStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	task, ok := status.Tasks["42"]
	if !ok {
		t.Fatal("task 42 not found in status after done-task")
	}
	if task.Status != "done" {
		t.Errorf("status: got %q, want done", task.Status)
	}
}

func TestClaimTask(t *testing.T) {
	setupProject(t)

	out := captureStdout(t, func() {
		if err := runArgs(t, "claim-task", "42", "agent-1"); err != nil {
			t.Errorf("claim-task returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("output is not valid JSON %q: %v", out, err)
	}
	claimed, ok := result["claimed"].(bool)
	if !ok || !claimed {
		t.Errorf(`expected {"claimed":true}, got %s`, out)
	}
}

func TestWhoOwns(t *testing.T) {
	setupProject(t)

	// Claim first.
	captureStdout(t, func() {
		runArgs(t, "claim-task", "42", "agent-1")
	})

	out := captureStdout(t, func() {
		if err := runArgs(t, "who-owns", "42"); err != nil {
			t.Errorf("who-owns returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("output is not valid JSON %q: %v", out, err)
	}
	owner, ok := result["owner"].(string)
	if !ok || owner != "agent-1" {
		t.Errorf(`expected {"owner":"agent-1"}, got %s`, out)
	}
}

func TestFileConflicts(t *testing.T) {
	setupProject(t)

	out := captureStdout(t, func() {
		if err := runArgs(t, "file-conflicts", "42"); err != nil {
			t.Errorf("file-conflicts returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("output is not valid JSON %q: %v", out, err)
	}
	conflicts, ok := result["conflicts"]
	if !ok {
		t.Fatalf("missing 'conflicts' key in output: %s", out)
	}
	arr, ok := conflicts.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf(`expected {"conflicts":[]}, got %s`, out)
	}
}

func TestLogCommand(t *testing.T) {
	_, _ = setupProject(t)

	if err := runArgs(t, "log", "agent-1", "hello"); err != nil {
		t.Fatalf("log returned error: %v", err)
	}
}

func TestBtwCommand(t *testing.T) {
	_, _ = setupProject(t)

	if err := runArgs(t, "btw", "agent-1", "tick"); err != nil {
		t.Fatalf("btw returned error: %v", err)
	}
}

func TestNotifyCommand(t *testing.T) {
	_, _ = setupProject(t)

	// matrix-cli is expected to be absent; the command should still succeed.
	if err := runArgs(t, "notify", "log", "test"); err != nil {
		t.Fatalf("notify returned error: %v", err)
	}
}

func TestAgentHealthDead(t *testing.T) {
	setupProject(t)

	out := captureStdout(t, func() {
		if err := runArgs(t, "agent-health", "/tmp/nonexistent-taskboard-test-file", "30"); err != nil {
			t.Errorf("agent-health returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("output is not valid JSON %q: %v", out, err)
	}
	if result["status"] != "dead" {
		t.Errorf("expected status=dead, got %v", result["status"])
	}
}

func TestCheckBuildProgress(t *testing.T) {
	setupProject(t)
	tmpDir := t.TempDir()

	out := captureStdout(t, func() {
		if err := runArgs(t, "check-build-progress", tmpDir, "5"); err != nil {
			t.Errorf("check-build-progress returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("output is not valid JSON %q: %v", out, err)
	}
	if _, ok := result["status"]; !ok {
		t.Errorf("expected 'status' field in output, got %s", out)
	}
}

func TestBadSubcommand(t *testing.T) {
	setupProject(t)

	err := runArgs(t, "no-such-subcommand-xyz")
	if err == nil {
		t.Fatal("bad subcommand should return a non-nil error")
	}
}

func TestSetTaskRejectsDashPrefix(t *testing.T) {
	setupProject(t)
	err := runArgs(t, "set-task", "--help")
	if err == nil {
		t.Fatal("set-task --help should return an error (-- prefix rejected)")
	}
	err2 := runArgs(t, "set-task", "-flag")
	if err2 == nil {
		t.Fatal("set-task -flag should return an error (- prefix rejected)")
	}
}
