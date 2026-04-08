package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alastor0325/taskboard/internal/store"
	"github.com/alastor0325/taskboard/internal/types"
)

func registerAgent(t *testing.T, proj, agentName string) {
	t.Helper()
	st := newStore(proj)
	team, _ := st.Load()
	team.BuildAgents[agentName] = &store.BuildAgent{AgentID: agentName, Status: "idle"}
	st.Save(team)
}

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
	t.Setenv("TASKBOARD_PROJECT", proj)
	// Also redirect the team.json and log.json under the same tmpdir so we
	// don't touch the real ~/.firefox-manager directory.
	t.Setenv("HOME", dir)
	// Remove the reset marker so that log entries are not suppressed even
	// when a previous test called "init" with the same proj name.
	marker := filepath.Join(os.TempDir(), ".taskboard-"+proj+"-log-reset")
	os.Remove(marker)
	t.Cleanup(func() { os.Remove(marker) })
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
	_, proj := setupProject(t)

	// Register a build agent in team.json so btw can validate it.
	st := newStore(proj)
	team, _ := st.Load()
	agentName := "agent-debug"
	team.BuildAgents[agentName] = &store.BuildAgent{AgentID: "agent-debug", Status: "idle"}
	st.Save(team)

	if err := runArgs(t, "btw", agentName, "tick"); err != nil {
		t.Fatalf("btw returned error: %v", err)
	}
}

func TestBtwRejectsUnknownAgent(t *testing.T) {
	_, _ = setupProject(t)

	err := runArgs(t, "btw", "phantom-agent", "some message")
	if err == nil {
		t.Fatal("btw with unknown agent should return an error")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("expected 'unknown agent' error, got: %v", err)
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

// Integration tests: exercise multiple components together.

func TestCommandFlowUpdatesStatusFile(t *testing.T) {
	statusFile, _ := setupProject(t)

	// init creates agent-status.json
	if err := runArgs(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	// set-task → agent-status.json must reflect the new task
	if err := runArgs(t, "set-task", "9001", "--summary", "crash in MediaDecoder", "--status", "running"); err != nil {
		t.Fatalf("set-task: %v", err)
	}
	data, _ := os.ReadFile(statusFile)
	var status types.AgentStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	task, ok := status.Tasks["9001"]
	if !ok {
		t.Fatal("task 9001 not in agent-status.json after set-task")
	}
	if task.Status != "running" {
		t.Errorf("status: got %q, want running", task.Status)
	}

	// done-task → task shows done in agent-status.json
	if err := runArgs(t, "done-task", "9001"); err != nil {
		t.Fatalf("done-task: %v", err)
	}
	data, _ = os.ReadFile(statusFile)
	json.Unmarshal(data, &status)
	if status.Tasks["9001"].Status != "done" {
		t.Errorf("after done-task: got %q, want done", status.Tasks["9001"].Status)
	}
}

func TestMultiCommandStateConsistency(t *testing.T) {
	statusFile, proj := setupProject(t)

	if err := runArgs(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Claim a task then verify ownership persists through subsequent commands.
	if err := runArgs(t, "set-task", "7777", "--summary", "UAF in CDMProxy", "--status", "running"); err != nil {
		t.Fatalf("set-task: %v", err)
	}

	// Register a build agent so claim-task has somewhere to claim to.
	st := newStore(proj)
	team, _ := st.Load()
	team.BuildAgents["agent-debug"] = &store.BuildAgent{AgentID: "agent-debug", Status: "idle"}
	st.Save(team)

	var claimErr error
	captured := captureStdout(t, func() { claimErr = runArgs(t, "claim-task", "7777", "agent-debug") })
	if claimErr != nil {
		t.Fatalf("claim-task: %v", claimErr)
	}
	var claimResult map[string]any
	json.Unmarshal([]byte(captured), &claimResult)
	if claimResult["claimed"] != true {
		t.Errorf("claim-task result: %v", claimResult)
	}

	// Subsequent who-owns must agree with claim result.
	var ownsErr error
	captured = captureStdout(t, func() { ownsErr = runArgs(t, "who-owns", "7777") })
	if ownsErr != nil {
		t.Fatalf("who-owns: %v", ownsErr)
	}
	var ownsResult map[string]any
	json.Unmarshal([]byte(captured), &ownsResult)
	if ownsResult["owner"] != "agent-debug" {
		t.Errorf("who-owns after claim: got %v, want agent-debug", ownsResult["owner"])
	}

	// done-task removes the task; agent-status.json must reflect it.
	if err := runArgs(t, "done-task", "7777"); err != nil {
		t.Fatalf("done-task: %v", err)
	}
	data, _ := os.ReadFile(statusFile)
	var status types.AgentStatus
	json.Unmarshal(data, &status)
	if status.Tasks["7777"] == nil || status.Tasks["7777"].Status != "done" {
		t.Errorf("task 7777 not marked done in agent-status.json")
	}
}

func TestLogAndBtwAppearsInStatusFile(t *testing.T) {
	statusFile, proj := setupProject(t)

	// Register an agent so btw validates. Do NOT call init — it writes a
	// reset marker that suppresses logs for 10s.
	st := newStore(proj)
	team, _ := st.Load()
	team.BuildAgents["agent-x"] = &store.BuildAgent{AgentID: "agent-x", Status: "idle"}
	st.Save(team)

	if err := runArgs(t, "log", "agent-x", "build started"); err != nil {
		t.Fatalf("log: %v", err)
	}
	if err := runArgs(t, "btw", "agent-x", "compiling"); err != nil {
		t.Fatalf("btw: %v", err)
	}

	data, _ := os.ReadFile(statusFile)
	var status types.AgentStatus
	json.Unmarshal(data, &status)

	if len(status.Log) == 0 {
		t.Error("log entry missing from agent-status.json")
	}
	found := false
	for _, e := range status.Log {
		if e.Agent == "agent-x" && e.Message == "build started" {
			found = true
		}
	}
	if !found {
		t.Error("log entry 'build started' not found in agent-status.json")
	}

	if len(status.Btw) == 0 {
		t.Error("btw entry missing from agent-status.json")
	}
	if status.Btw[0].Message != "compiling" {
		t.Errorf("btw message: got %q, want compiling", status.Btw[0].Message)
	}
}

func TestInitInstallsSkill(t *testing.T) {
	_, _ = setupProject(t)
	if err := runArgs(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	dest := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "SKILL.md")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("skill not installed: %v", err)
	}
}

func TestInstallSkillCommand(t *testing.T) {
	_, _ = setupProject(t)
	out := captureStdout(t, func() {
		if err := runArgs(t, "install-skill"); err != nil {
			t.Errorf("install-skill: %v", err)
		}
	})
	dest := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "SKILL.md")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("skill not installed: %v", err)
	}
	if !strings.Contains(out, "skill") {
		t.Errorf("expected skill output, got: %q", out)
	}
}

func TestEventCommand(t *testing.T) {
	_, proj := setupProject(t)
	registerAgent(t, proj, "agent-debug")

	if err := runArgs(t, "event", "build-done", "agent-debug", "build succeeded"); err != nil {
		t.Fatalf("event: %v", err)
	}
}

func TestEventRejectsUnknownAgent(t *testing.T) {
	_, _ = setupProject(t)
	err := runArgs(t, "event", "build-done", "phantom", "msg")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("want 'unknown agent', got: %v", err)
	}
}

func TestEventRejectsMissingArgs(t *testing.T) {
	_, proj := setupProject(t)
	registerAgent(t, proj, "agent-debug")
	err := runArgs(t, "event", "build-done", "agent-debug")
	if err == nil {
		t.Fatal("expected error for missing message arg")
	}
}

func TestDetectCommand(t *testing.T) {
	_, _ = setupProject(t)
	out := captureStdout(t, func() {
		if err := runArgs(t, "detect"); err != nil {
			t.Errorf("detect: %v", err)
		}
	})
	if strings.TrimSpace(out) == "" {
		t.Error("detect should print a non-empty project name")
	}
}
