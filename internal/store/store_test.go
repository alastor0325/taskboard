package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tmpStore(t *testing.T) *TaskStore {
	t.Helper()
	dir := t.TempDir()
	return New(filepath.Join(dir, "team.json"))
}

func TestLoadEmpty(t *testing.T) {
	s := tmpStore(t)
	team, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if team.Tasks == nil || team.InvestigationAgents == nil ||
		team.BuildAgents == nil || team.TaskAgents == nil || team.UtilityAgents == nil {
		t.Fatal("Load() must initialise all 5 top-level keys")
	}
}

func TestSaveRoundtrip(t *testing.T) {
	s := tmpStore(t)
	team := emptyTeam()
	team.Tasks["1"] = &Task{Summary: "hello", Status: "running"}
	if err := s.Save(team); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks["1"].Summary != "hello" {
		t.Fatalf("got %q, want %q", loaded.Tasks["1"].Summary, "hello")
	}
}

func TestAtomicWrite(t *testing.T) {
	s := tmpStore(t)
	team := emptyTeam()
	if err := s.Save(team); err != nil {
		t.Fatal(err)
	}
	// Ensure no leftover temp files.
	dir := filepath.Dir(s.teamFile)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "team.json" {
			t.Fatalf("unexpected file after save: %s", e.Name())
		}
	}
}

func TestSetTaskNewTask(t *testing.T) {
	s := tmpStore(t)
	summary := "my task"
	status := "running"
	if _, err := s.SetTask("42", SetTaskOpts{Summary: &summary, Status: &status}); err != nil {
		t.Fatal(err)
	}
	team, _ := s.Load()
	task := team.Tasks["42"]
	if task == nil {
		t.Fatal("task not created")
	}
	if task.Summary != "my task" {
		t.Errorf("summary: got %q, want %q", task.Summary, "my task")
	}
	if task.Status != "running" {
		t.Errorf("status: got %q, want %q", task.Status, "running")
	}
}

func TestSetTaskPartialUpdate(t *testing.T) {
	s := tmpStore(t)
	summary := "original"
	status := "running"
	s.SetTask("1", SetTaskOpts{Summary: &summary, Status: &status})

	note := "updated note"
	if _, err := s.SetTask("1", SetTaskOpts{Note: &note}); err != nil {
		t.Fatal(err)
	}
	team, _ := s.Load()
	task := team.Tasks["1"]
	if task.Summary != "original" {
		t.Errorf("summary should be unchanged, got %q", task.Summary)
	}
	if task.Note != "updated note" {
		t.Errorf("note: got %q, want %q", task.Note, "updated note")
	}
}

func TestValidTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{"", "running", true},
		{"idle", "running", true},
		{"idle", "done", true},
		{"running", "idle", true},
		{"running", "waiting", true},
		{"running", "done", true},
		{"waiting", "running", true},
		{"waiting", "idle", true},
		{"waiting", "done", true},
		{"done", "running", false},
		{"running", "running", false}, // same status, no-op in SetTask
		{"idle", "waiting", false},
	}
	for _, c := range cases {
		s := tmpStore(t)
		team := emptyTeam()
		team.Tasks["1"] = &Task{Status: c.from}
		s.Save(team)

		newStatus := c.to
		_, err := s.SetTask("1", SetTaskOpts{Status: &newStatus})
		if c.ok && err != nil {
			t.Errorf("transition %q→%q should succeed, got error: %v", c.from, c.to, err)
		}
		if !c.ok && err == nil {
			// same-status is silently ignored (not an error), so skip that case
			if c.from != c.to {
				t.Errorf("transition %q→%q should fail, but succeeded", c.from, c.to)
			}
		}
	}
}

func TestMarkDone(t *testing.T) {
	s := tmpStore(t)
	status := "running"
	s.SetTask("5", SetTaskOpts{Status: &status})

	before := time.Now().Unix()
	if _, err := s.MarkDone("5"); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()

	team, _ := s.Load()
	task := team.Tasks["5"]
	if task.Status != "done" {
		t.Errorf("status: got %q, want done", task.Status)
	}
	if task.DoneAt == nil {
		t.Fatal("DoneAt should be set")
	}
	ts := int64(*task.DoneAt)
	if ts < before || ts > after {
		t.Errorf("DoneAt %d not in range [%d, %d]", ts, before, after)
	}
}

func TestMarkAgentDead(t *testing.T) {
	s := tmpStore(t)
	team := emptyTeam()
	team.InvestigationAgents["99"] = &InvestigationAgent{AgentID: "inv-99", Status: "running"}
	team.Tasks["99"] = &Task{Status: "running", Summary: "some task"}
	s.Save(team)

	if err := s.MarkAgentDead("investigation_agents", "99"); err != nil {
		t.Fatal(err)
	}
	loaded, _ := s.Load()
	if loaded.InvestigationAgents["99"].Status != "dead" {
		t.Error("agent status should be dead")
	}
	if loaded.Tasks["99"].Status != "failed" {
		t.Error("task status should be failed")
	}
}

func TestCleanupDone(t *testing.T) {
	s := tmpStore(t)
	team := emptyTeam()
	old := float64(time.Now().Unix() - 400)
	recent := float64(time.Now().Unix() - 100)
	team.Tasks["old"] = &Task{Status: "done", DoneAt: &old}
	team.Tasks["recent"] = &Task{Status: "done", DoneAt: &recent}
	team.Tasks["running"] = &Task{Status: "running"}
	s.Save(team)

	modified, err := s.CleanupDone(300 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !modified {
		t.Error("should have modified")
	}
	loaded, _ := s.Load()
	if _, ok := loaded.Tasks["old"]; ok {
		t.Error("old done task should be removed")
	}
	if _, ok := loaded.Tasks["recent"]; !ok {
		t.Error("recent done task should remain")
	}
	if _, ok := loaded.Tasks["running"]; !ok {
		t.Error("running task should remain")
	}
}

func TestClaimTask(t *testing.T) {
	s := tmpStore(t)

	claimed, owner, err := s.ClaimTask("7", "agent-1")
	if err != nil || !claimed || owner != "" {
		t.Fatalf("first claim: got claimed=%v owner=%q err=%v", claimed, owner, err)
	}

	claimed2, owner2, err := s.ClaimTask("7", "agent-2")
	if err != nil {
		t.Fatal(err)
	}
	if claimed2 {
		t.Error("second claim should fail")
	}
	if owner2 != "agent-1" {
		t.Errorf("owner: got %q, want agent-1", owner2)
	}
}

func TestWhoOwns(t *testing.T) {
	s := tmpStore(t)
	owner, _ := s.WhoOwns("99")
	if owner != "" {
		t.Errorf("unowned task should return empty, got %q", owner)
	}

	s.ClaimTask("99", "inv-99")
	owner, _ = s.WhoOwns("99")
	if owner != "inv-99" {
		t.Errorf("got %q, want inv-99", owner)
	}
}

func TestFileConflicts(t *testing.T) {
	s := tmpStore(t)
	team := emptyTeam()
	team.InvestigationAgents["10"] = &InvestigationAgent{
		AgentID:      "inv-10",
		Status:       "running",
		ClaimedFiles: []string{"foo.cpp", "bar.h"},
	}
	team.InvestigationAgents["20"] = &InvestigationAgent{
		AgentID:      "inv-20",
		Status:       "running",
		ClaimedFiles: []string{"bar.h", "baz.cpp"},
	}
	s.Save(team)

	conflicts, err := s.FileConflicts("10")
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Agent != "inv-20" {
		t.Errorf("got agent %q, want inv-20", conflicts[0].Agent)
	}
	if len(conflicts[0].Files) != 1 || conflicts[0].Files[0] != "bar.h" {
		t.Errorf("got files %v, want [bar.h]", conflicts[0].Files)
	}
}
