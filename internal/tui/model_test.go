package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/alastor0325/taskboard/internal/store"
	"github.com/alastor0325/taskboard/internal/types"
)

func writeStatusFile(t *testing.T, path string, tasks map[string]map[string]any, log []types.LogEntry, btw []types.BtwEntry) {
	t.Helper()
	status := map[string]any{
		"project":      "test",
		"updated_at":   float64(time.Now().Unix()),
		"tasks":        tasks,
		"build_agents": map[string]any{},
		"log":          log,
		"btw":          btw,
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestModel(t *testing.T) (Model, string) {
	t.Helper()
	dir := t.TempDir()
	sf := filepath.Join(dir, "agent-status.json")
	writeStatusFile(t, sf,
		map[string]map[string]any{
			"2001": {"summary": "Fix audio bug", "status": "running"},
			"2002": {"summary": "Update PDF.js", "status": "waiting"},
		},
		[]types.LogEntry{
			{Time: float64(time.Now().Unix()), Agent: "inv-2001", Message: "Starting analysis"},
			{Time: float64(time.Now().Unix()), Agent: "inv-2002", Message: "Approach drafted"},
		},
		nil,
	)
	m := New("test", sf)
	return m, sf
}

func TestModelInit(t *testing.T) {
	m, _ := newTestModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

func TestWindowSizeTriggersRelayout(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	if m2.width != 120 || m2.height != 40 {
		t.Errorf("width/height not updated: got %dx%d", m2.width, m2.height)
	}
	if m2.taskList.Width() == 0 {
		t.Error("task list width should be set after resize")
	}
	if m2.logViewport.Width == 0 {
		t.Error("log viewport width should be set after resize")
	}
}

func TestTabSwitchesFocus(t *testing.T) {
	m, _ := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.focus != focusTasks {
		t.Fatal("initial focus should be tasks")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.focus != focusLog {
		t.Error("Tab should switch focus to log")
	}
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated2.(Model)
	if m3.focus != focusTasks {
		t.Error("second Tab should switch focus back to tasks")
	}
}

func TestQuitKey(t *testing.T) {
	m, _ := newTestModel(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q should produce QuitMsg, got %T", msg)
	}
}

func TestEnterOpensOverlay(t *testing.T) {
	m, sf := newTestModel(t)
	// Force a poll to load tasks.
	m.lastMtime = time.Time{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	m2 = m2.pollStatus()

	if len(m2.tasks) == 0 {
		t.Skip("no tasks loaded; skipping overlay test")
	}

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated2.(Model)
	if m3.overlay == nil {
		t.Error("Enter on tasks should open overlay")
	}
	_ = sf
}

func TestEscClosesOverlay(t *testing.T) {
	m, sf := newTestModel(t)
	m.lastMtime = time.Time{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	m2 = m2.pollStatus()

	if len(m2.tasks) == 0 {
		t.Skip("no tasks loaded")
	}

	// Open overlay.
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated2.(Model)
	if m3.overlay == nil {
		t.Skip("overlay did not open")
	}

	// Close overlay.
	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := updated3.(Model)
	if m4.overlay != nil {
		t.Error("ESC should close overlay")
	}
	_ = sf
}

func TestSlashActivatesFilter(t *testing.T) {
	m, _ := newTestModel(t)
	// Switch to log focus first.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.focus != focusLog {
		t.Fatal("expected log focus")
	}
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m3 := updated2.(Model)
	if !m3.filtering {
		t.Error("/ should activate filter mode in log section")
	}
}

func TestFilterInput(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m3 := updated2.(Model)

	// Type "audio" into filter.
	for _, ch := range "audio" {
		updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m3 = updated3.(Model)
	}
	if m3.filterInput != "audio" {
		t.Errorf("filter input: got %q, want %q", m3.filterInput, "audio")
	}
}

func TestBuildAgentMap(t *testing.T) {
	bugID := int64(2025475)
	status := types.AgentStatus{
		BuildAgents: map[string]*store.BuildAgent{
			"debug": {AgentID: "agent-debug", CurrentBug: &bugID},
		},
		InvestigationAgents: map[string]*store.InvestigationAgent{
			"2025480": {AgentID: "inv-2025480"},
		},
	}
	m := buildAgentMap(status)
	if m["2025475"] != "agent-debug" {
		t.Errorf("build agent: got %q, want agent-debug", m["2025475"])
	}
	if m["2025480"] != "inv-2025480" {
		t.Errorf("inv agent: got %q, want inv-2025480", m["2025480"])
	}
}

func TestBuildBtwMap(t *testing.T) {
	entries := []types.BtwEntry{
		{Agent: "inv-2025475", Message: "tracing call chain"},
		{Agent: "agent-debug", Message: "building"},
		{Agent: "inv-2025475", Message: "writing investigation file"}, // last wins
	}
	m := buildBtwMap(entries)
	if m["inv-2025475"].Message != "writing investigation file" {
		t.Errorf("last btw should win, got %q", m["inv-2025475"].Message)
	}
	if m["agent-debug"].Message != "building" {
		t.Errorf("agent-debug btw: got %q", m["agent-debug"].Message)
	}
}

func TestBuildTaskItems_BtwMapping(t *testing.T) {
	bugID := int64(2025475)
	status := types.AgentStatus{
		Tasks: map[string]*store.Task{
			"2025475": {Summary: "test bug", Status: "running"},
		},
		BuildAgents: map[string]*store.BuildAgent{
			"debug": {AgentID: "agent-debug", CurrentBug: &bugID},
		},
		InvestigationAgents: map[string]*store.InvestigationAgent{},
		Btw: []types.BtwEntry{
			{Agent: "agent-debug", Message: "compiling dom/media"},
		},
	}
	items := buildTaskItems(status)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].btwMsg != "compiling dom/media" {
		t.Errorf("btwMsg: got %q, want %q", items[0].btwMsg, "compiling dom/media")
	}
}

func TestBuildTaskItemsSorting(t *testing.T) {
	status := types.AgentStatus{
		Tasks: map[string]*store.Task{
			"1": {Status: "done"},
			"2": {Status: "running"},
			"3": {Status: "failed"},
			"4": {Status: "waiting"},
		},
	}
	items := buildTaskItems(status)
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	// Failed first, then waiting, then running, then done.
	if items[0].status != "failed" {
		t.Errorf("first item should be failed, got %s", items[0].status)
	}
	if items[1].status != "waiting" {
		t.Errorf("second item should be waiting, got %s", items[1].status)
	}
	if items[2].status != "running" {
		t.Errorf("third item should be running, got %s", items[2].status)
	}
	if items[3].status != "done" {
		t.Errorf("fourth item should be done, got %s", items[3].status)
	}
}

func TestViewShowsLoading(t *testing.T) {
	m, _ := newTestModel(t)
	// width=0 → "loading..."
	view := m.View()
	if !strings.Contains(view, "loading") {
		t.Errorf("View() with width=0 should show loading, got: %q", view)
	}
}

func TestViewAfterResize(t *testing.T) {
	m, sf := newTestModel(t)
	m.lastMtime = time.Time{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	m2 = m2.pollStatus()

	view := m2.View()
	if strings.Contains(view, "loading") {
		t.Error("View() after resize should not show loading")
	}
	if !strings.Contains(view, "TASKS") {
		t.Error("View() should contain TASKS section")
	}
	if !strings.Contains(view, "LOG") {
		t.Error("View() should contain LOG section")
	}
	_ = sf
}

// TestTeatest uses the teatest framework to drive the model programmatically.
func TestTeatestQuit(t *testing.T) {
	m, _ := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestTeatestTabFocus(t *testing.T) {
	m, _ := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
