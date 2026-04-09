package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestBuildTaskItemsAgentResolution(t *testing.T) {
	bugID := int64(2025475)
	status := types.AgentStatus{
		Tasks: map[string]*store.Task{
			"2025475": {Summary: "test bug", Status: "running"},
			"2025480": {Summary: "inv bug", Status: "waiting"},
		},
		BuildAgents: map[string]*store.BuildAgent{
			"agent-debug": {AgentID: "agent-debug", CurrentBug: &bugID},
		},
		InvestigationAgents: map[string]*store.InvestigationAgent{
			"2025480": {AgentID: "inv-2025480", BuildType: "asan"},
		},
	}
	items := buildTaskItems(status)
	byID := make(map[string]taskItem)
	for _, it := range items {
		byID[it.bugID] = it
	}
	if byID["2025475"].buildAgentName != "agent-debug" {
		t.Errorf("build agent: got %q, want agent-debug", byID["2025475"].buildAgentName)
	}
	if byID["2025475"].buildQueuePos != 0 {
		t.Errorf("build queue pos: got %d, want 0", byID["2025475"].buildQueuePos)
	}
	if byID["2025480"].invAgentID != "inv-2025480" {
		t.Errorf("inv agent: got %q, want inv-2025480", byID["2025480"].invAgentID)
	}
	if byID["2025480"].invBuildType != "asan" {
		t.Errorf("inv build type: got %q, want asan", byID["2025480"].invBuildType)
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

func TestSplitResizeKeys(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	initial := m2.splitRatio

	// "]" increases split ratio
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m3 := updated2.(Model)
	if m3.splitRatio != initial+5 {
		t.Errorf("] key: splitRatio got %d, want %d", m3.splitRatio, initial+5)
	}

	// "[" decreases split ratio
	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	m4 := updated3.(Model)
	if m4.splitRatio != initial {
		t.Errorf("[ key: splitRatio got %d, want %d", m4.splitRatio, initial)
	}
}

func TestSplitRatioClamped(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)

	// Drive ratio to minimum
	m2.splitRatio = 20
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	m3 := updated2.(Model)
	if m3.splitRatio < 20 {
		t.Errorf("splitRatio should not go below 20, got %d", m3.splitRatio)
	}

	// Drive ratio to maximum
	m2.splitRatio = 80
	updated3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m4 := updated3.(Model)
	if m4.splitRatio > 80 {
		t.Errorf("splitRatio should not exceed 80, got %d", m4.splitRatio)
	}
}

func TestAgentColorConsistent(t *testing.T) {
	// Same name always gets the same color.
	c1 := agentColor("inv-2025001")
	c2 := agentColor("inv-2025001")
	if c1 != c2 {
		t.Errorf("agentColor not deterministic: %q != %q", c1, c2)
	}
}

func TestAgentColorDistinct(t *testing.T) {
	// Different names should not all collide to the same color (probabilistic).
	agents := []string{"inv-2025001", "agent-debug", "manager", "inv-2025005", "inv-2025008"}
	seen := map[lipgloss.Color]bool{}
	for _, a := range agents {
		seen[agentColor(a)] = true
	}
	if len(seen) < 2 {
		t.Errorf("all agents got the same color — palette not working")
	}
}

func TestAgentColorInPalette(t *testing.T) {
	c := agentColor("any-agent")
	for _, p := range agentColorPalette {
		if c == p {
			return
		}
	}
	t.Errorf("agentColor returned %q which is not in the palette", c)
}

func TestFooterNormalMode(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)

	footer := m2.renderFooter()
	for _, want := range []string{"Tab", "[TASKS]", "↑↓", "jk", "g/G", ",/.", "filter", "quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer missing %q: %q", want, footer)
		}
	}
	// log not focused
	if !strings.Contains(footer, " log ") {
		t.Errorf("footer should show ' log ' (unfocused) when tasks has focus")
	}
}

func TestFooterLogFocus(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	// Switch to log focus
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated2.(Model)

	footer := m3.renderFooter()
	if !strings.Contains(footer, "[LOG]") {
		t.Errorf("footer should show [LOG] when log has focus: %q", footer)
	}
	if !strings.Contains(footer, " tasks ") {
		t.Errorf("footer should show ' tasks ' (unfocused) when log has focus: %q", footer)
	}
}

func TestFooterFilterMode(t *testing.T) {
	m, _ := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m3 := updated2.(Model)
	// Type a filter term
	for _, ch := range "audio" {
		updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m3 = updated3.(Model)
	}

	footer := m3.renderFooter()
	if !strings.Contains(footer, "audio") {
		t.Errorf("footer filter mode should show filter input, got: %q", footer)
	}
	if !strings.Contains(footer, "Enter confirm") {
		t.Errorf("footer filter mode should show confirm hint: %q", footer)
	}
	if !strings.Contains(footer, "Esc clear") {
		t.Errorf("footer filter mode should show clear hint: %q", footer)
	}
}

func TestViewContainsFooter(t *testing.T) {
	m, sf := newTestModel(t)
	m.lastMtime = time.Time{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	m2 = m2.pollStatus()

	view := m2.View()
	if !strings.Contains(view, "quit") {
		t.Errorf("View() should contain footer with 'quit': %q", view[:min(200, len(view))])
	}
	_ = sf
}

func TestFilterBtwTTL(t *testing.T) {
	now := time.Now()
	entries := []types.BtwEntry{
		{Time: float64(now.Unix() - 60), Agent: "inv-1", Message: "recent"},
		{Time: float64(now.Unix() - 200), Agent: "inv-2", Message: "expired"},
		{Time: float64(now.Unix() - 10), Agent: "inv-3", Message: "very recent"},
	}
	got := filterBtw(entries, now)
	if len(got) != 2 {
		t.Fatalf("filterBtw: got %d entries, want 2", len(got))
	}
	for _, e := range got {
		if e.Agent == "inv-2" {
			t.Error("expired entry should be filtered out")
		}
	}
}

func TestViewTaskCount(t *testing.T) {
	m, sf := newTestModel(t)
	m.lastMtime = time.Time{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	m2 = m2.pollStatus()

	view := m2.View()
	if !strings.Contains(view, "TASKS") {
		t.Error("View() should contain TASKS")
	}
	// Should show count like "(2)" since newTestModel sets 2 tasks.
	if !strings.Contains(view, "(2)") {
		t.Errorf("View() should show task count (2), got: %q", view[:min(300, len(view))])
	}
	_ = sf
}

func TestViewLogCount(t *testing.T) {
	m, sf := newTestModel(t)
	m.lastMtime = time.Time{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	m2 = m2.pollStatus()

	view := m2.View()
	// newTestModel has 2 log entries.
	if !strings.Contains(view, "(2)") {
		t.Errorf("View() should show log count, got: %q", view[:min(300, len(view))])
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

func TestReviewURL(t *testing.T) {
	cases := []struct {
		worktree string
		wantFrag string
	}{
		{"~/firefox-2026875", "2026875"},
		{"/home/user/firefox-myfeature", "myfeature"},
		{"/home/user/firefox-", ""},
		{"/home/user/somedir", "somedir"}, // no firefox- prefix: use basename as-is
	}
	for _, c := range cases {
		url := reviewURL(c.worktree)
		if !strings.Contains(url, "#"+c.wantFrag) {
			t.Errorf("reviewURL(%q) = %q, want fragment %q", c.worktree, url, c.wantFrag)
		}
		if !strings.HasPrefix(url, "http://localhost:") {
			t.Errorf("reviewURL(%q) = %q, want http://localhost: prefix", c.worktree, url)
		}
	}
}

func TestBuildTaskItemsQueuePosition(t *testing.T) {
	currentBug := int64(2025001)
	status := types.AgentStatus{
		Tasks: map[string]*store.Task{
			"2025001": {Status: "running"},
			"2025002": {Status: "waiting"},
			"2025003": {Status: "waiting"},
		},
		BuildAgents: map[string]*store.BuildAgent{
			"agent-debug": {
				AgentID:    "agent-debug",
				Status:     "busy",
				CurrentBug: &currentBug,
				Queue:      []int64{2025002, 2025003},
			},
		},
	}
	items := buildTaskItems(status)
	byID := make(map[string]taskItem)
	for _, it := range items {
		byID[it.bugID] = it
	}
	if byID["2025001"].buildQueuePos != 0 {
		t.Errorf("current bug should have pos 0, got %d", byID["2025001"].buildQueuePos)
	}
	if byID["2025002"].buildQueuePos != 1 {
		t.Errorf("first queued bug should have pos 1, got %d", byID["2025002"].buildQueuePos)
	}
	if byID["2025003"].buildQueuePos != 2 {
		t.Errorf("second queued bug should have pos 2, got %d", byID["2025003"].buildQueuePos)
	}
}

func TestBuildTaskItemsClaimedFiles(t *testing.T) {
	status := types.AgentStatus{
		Tasks: map[string]*store.Task{
			"2025475": {Status: "running"},
		},
		InvestigationAgents: map[string]*store.InvestigationAgent{
			"2025475": {
				AgentID:      "inv-2025475",
				BuildType:    "asan",
				ClaimedFiles: []string{"dom/media/ipc/RemoteCDMChild.cpp", "dom/media/ipc/RemoteCDMChild.h"},
			},
		},
	}
	items := buildTaskItems(status)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.invBuildType != "asan" {
		t.Errorf("invBuildType: got %q, want asan", it.invBuildType)
	}
	if len(it.claimedFiles) != 2 {
		t.Errorf("claimedFiles: got %v, want 2 entries", it.claimedFiles)
	}
	if it.claimedFiles[0] != "dom/media/ipc/RemoteCDMChild.cpp" {
		t.Errorf("claimedFiles[0]: got %q", it.claimedFiles[0])
	}
}

func TestRefreshTaskListPreservesCursor(t *testing.T) {
	m, sf := newTestModel(t)
	writeStatusFile(t, sf, map[string]map[string]any{
		"2025001": {"status": "running", "summary": "A"},
		"2025002": {"status": "running", "summary": "B"},
		"2025003": {"status": "running", "summary": "C"},
	}, nil, nil)
	m = m.pollStatus()
	m.taskList.Select(2)
	if m.taskList.Index() != 2 {
		t.Fatalf("pre-condition: cursor should be 2, got %d", m.taskList.Index())
	}
	// refreshTaskList should preserve cursor at 2
	m.refreshTaskList()
	if m.taskList.Index() != 2 {
		t.Errorf("cursor reset after refresh: got %d, want 2", m.taskList.Index())
	}
}

func TestPollStatusDetectsTaskChange(t *testing.T) {
	m, sf := newTestModel(t)

	// Start with one running task.
	writeStatusFile(t, sf, map[string]map[string]any{
		"5001": {"status": "running", "summary": "initial task"},
	}, nil, nil)
	m = m.pollStatus()
	if len(m.taskList.Items()) != 1 {
		t.Fatalf("expected 1 task, got %d", len(m.taskList.Items()))
	}

	// Add a second task — poll must detect the file change.
	writeStatusFile(t, sf, map[string]map[string]any{
		"5001": {"status": "running", "summary": "initial task"},
		"5002": {"status": "waiting", "summary": "new task"},
	}, nil, nil)
	m = m.pollStatus()
	if len(m.taskList.Items()) != 2 {
		t.Errorf("expected 2 tasks after update, got %d", len(m.taskList.Items()))
	}

	// Mark first task done — poll must reflect the status change.
	writeStatusFile(t, sf, map[string]map[string]any{
		"5001": {"status": "done", "summary": "initial task"},
		"5002": {"status": "waiting", "summary": "new task"},
	}, nil, nil)
	m = m.pollStatus()
	items := m.taskList.Items()
	found := false
	for _, it := range items {
		if tli, ok := it.(taskListItem); ok && tli.task.bugID == "5001" {
			if tli.task.status != "done" {
				t.Errorf("task 5001: got status %q, want done", tli.task.status)
			}
			found = true
		}
	}
	if !found {
		t.Error("task 5001 not found in list")
	}
}

func TestPollStatusIgnoresUnchangedFile(t *testing.T) {
	m, sf := newTestModel(t)

	writeStatusFile(t, sf, map[string]map[string]any{
		"6001": {"status": "running", "summary": "task"},
	}, nil, nil)
	m = m.pollStatus()
	firstMtime := m.lastMtime

	// Poll again without changing the file — lastMtime must not change.
	m = m.pollStatus()
	if m.lastMtime != firstMtime {
		t.Error("pollStatus re-read unchanged file (lastMtime changed)")
	}
}

// makeLogEntries builds N log entries for use in scroll tests.
func makeLogEntries(n int) []types.LogEntry {
	var entries []types.LogEntry
	for i := 0; i < n; i++ {
		entries = append(entries, types.LogEntry{
			Time:    float64(time.Now().Unix()),
			Agent:   "test-agent",
			Message: fmt.Sprintf("log line %d", i),
		})
	}
	return entries
}

// TestLogViewportAutoScrollsOnInitialLoad verifies that the first content load
// scrolls the viewport to the bottom so the newest entries are immediately visible.
func TestLogViewportAutoScrollsOnInitialLoad(t *testing.T) {
	m, sf := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m2 := updated.(Model)
	m2.lastMtime = time.Time{}

	writeStatusFile(t, sf, map[string]map[string]any{}, makeLogEntries(30), nil)
	m2 = m2.pollStatus()

	if !m2.logViewport.AtBottom() {
		t.Error("log viewport should auto-scroll to bottom on first content load")
	}
}

// TestLogViewportSticksToBottom verifies that while the user stays at the bottom,
// each new poll keeps the viewport anchored to the newest entries.
func TestLogViewportSticksToBottom(t *testing.T) {
	m, sf := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m2 := updated.(Model)
	m2.lastMtime = time.Time{}

	entries := makeLogEntries(30)
	writeStatusFile(t, sf, map[string]map[string]any{}, entries, nil)
	m2 = m2.pollStatus()
	if !m2.logViewport.AtBottom() {
		t.Fatal("pre-condition: should be at bottom after initial load")
	}

	// Append one more entry and poll again — must stay at bottom.
	entries = append(entries, types.LogEntry{
		Time:    float64(time.Now().Unix() + 1),
		Agent:   "test-agent",
		Message: "new entry",
	})
	writeStatusFile(t, sf, map[string]map[string]any{}, entries, nil)
	m2 = m2.pollStatus()

	if !m2.logViewport.AtBottom() {
		t.Error("log viewport should stay at bottom when new entries arrive while already at bottom")
	}
}

// TestLogViewportPreservesScrollWhenNotAtBottom verifies that if the user has
// scrolled up, a new poll does NOT force them back to the bottom.
func TestLogViewportPreservesScrollWhenNotAtBottom(t *testing.T) {
	m, sf := newTestModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m2 := updated.(Model)
	m2.lastMtime = time.Time{}

	entries := makeLogEntries(30)
	writeStatusFile(t, sf, map[string]map[string]any{}, entries, nil)
	m2 = m2.pollStatus()
	if !m2.logViewport.AtBottom() {
		t.Fatal("pre-condition: should be at bottom after initial load")
	}
	if m2.logViewport.YOffset == 0 {
		t.Skip("content fits in viewport; scroll test not applicable")
	}

	// Scroll up, away from the bottom.
	m2.logViewport.LineUp(5)
	savedOffset := m2.logViewport.YOffset
	if m2.logViewport.AtBottom() {
		t.Fatal("pre-condition: should not be at bottom after scrolling up")
	}

	// Add more entries and poll — position should be preserved.
	entries = append(entries, types.LogEntry{
		Time:    float64(time.Now().Unix() + 1),
		Agent:   "test-agent",
		Message: "new entry",
	})
	writeStatusFile(t, sf, map[string]map[string]any{}, entries, nil)
	m2 = m2.pollStatus()

	if m2.logViewport.YOffset != savedOffset {
		t.Errorf("viewport should preserve scroll position when not at bottom: got offset %d, want %d",
			m2.logViewport.YOffset, savedOffset)
	}
}
