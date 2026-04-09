package tui

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alastor0325/taskboard/internal/store"
	"github.com/alastor0325/taskboard/internal/types"
)

type focus int

const (
	focusTasks focus = iota
	focusLog
)

type Model struct {
	proj        string
	statusFile  string
	width       int
	height      int
	focus       focus
	splitRatio  int // percentage of body height allocated to tasks (20–80)
	taskList    list.Model
	logViewport viewport.Model
	btw         []types.BtwEntry
	spinner     spinner.Model
	overlay     *taskDetail
	filterInput string
	filtering   bool
	lastMtime   time.Time
	allLog      []types.LogEntry
	tasks       []taskItem
}

type taskItem struct {
	bugID    string
	summary  string
	status   string
	note     string
	tryURL   string
	revURL   string
	worktree string
	btwMsg   string
	hasInv   bool
	doneAt   float64 // unix timestamp, non-zero only when status=="done"
	// investigation agent
	invAgentID     string
	invAgentStatus string
	invBuildType   string
	claimedFiles   []string
	// build agent
	buildAgentName   string
	buildAgentStatus string
	buildQueuePos    int // -1=not in queue, 0=current build, N=position in queue
}

type taskDetail struct {
	item taskItem
}

var urlRe = regexp.MustCompile(`https?://\S+`)

func New(proj, statusFile string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	delegate := newTaskDelegate()
	tl := list.New([]list.Item{}, delegate, 0, 0)
	tl.SetShowHelp(false)
	tl.SetShowStatusBar(false)
	tl.SetShowTitle(false)
	tl.SetFilteringEnabled(false)
	tl.SetShowPagination(false)

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return Model{
		proj:        proj,
		statusFile:  statusFile,
		focus:       focusTasks,
		splitRatio:  50,
		taskList:    tl,
		logViewport: vp,
		spinner:     sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("taskboard"),
		tickCmd(),
		m.spinner.Tick,
	)
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.relayout()

	case tickMsg:
		m = m.pollStatus()
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd, tickCmd())

	case tea.KeyMsg:
		if m.overlay != nil {
			if msg.String() == "esc" {
				m.overlay = nil
			}
			return m, tea.Batch(cmds...)
		}
		if m.filtering {
			return m.handleFilterKey(msg, cmds)
		}
		return m.handleKey(msg, cmds)

	case tea.MouseMsg:
		return m.handleMouse(msg, cmds)
	}

	// Propagate to focused component.
	if m.overlay == nil {
		switch m.focus {
		case focusTasks:
			var tlCmd tea.Cmd
			m.taskList, tlCmd = m.taskList.Update(msg)
			cmds = append(cmds, tlCmd)
		case focusLog:
			var vpCmd tea.Cmd
			m.logViewport, vpCmd = m.logViewport.Update(msg)
			cmds = append(cmds, vpCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m, tea.Quit
	case "tab":
		if m.focus == focusTasks {
			m.focus = focusLog
		} else {
			m.focus = focusTasks
		}
	case "enter":
		if m.focus == focusTasks {
			if sel, ok := m.taskList.SelectedItem().(taskListItem); ok {
				m.overlay = &taskDetail{item: sel.task}
			}
		}
	case "g":
		if m.focus == focusTasks {
			m.taskList.Select(0)
		} else {
			m.logViewport.GotoTop()
		}
	case "G":
		if m.focus == focusTasks {
			m.taskList.Select(len(m.taskList.Items()) - 1)
		} else {
			m.logViewport.GotoBottom()
		}
	case "/":
		if m.focus == focusLog {
			m.filtering = true
			m.filterInput = ""
		}
	case "j", "down":
		if m.focus == focusTasks {
			m.taskList, _ = m.taskList.Update(msg)
		} else {
			m.logViewport.LineDown(1)
		}
	case "k", "up":
		if m.focus == focusTasks {
			m.taskList, _ = m.taskList.Update(msg)
		} else {
			m.logViewport.LineUp(1)
		}
	case ",", "<":
		if os.Getenv("TMUX") != "" {
			exec.Command("tmux", "resize-pane", "-L", "5").Run() //nolint:errcheck
		}
	case ".", ">":
		if os.Getenv("TMUX") != "" {
			exec.Command("tmux", "resize-pane", "-R", "5").Run() //nolint:errcheck
		}
	case "[":
		if m.splitRatio > 20 {
			m.splitRatio -= 5
			m = m.relayout()
		}
	case "]":
		if m.splitRatio < 80 {
			m.splitRatio += 5
			m = m.relayout()
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleFilterKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.filtering = false
		m.updateLogContent()
	case "backspace":
		if len(m.filterInput) > 0 {
			m.filterInput = m.filterInput[:len(m.filterInput)-1]
			m.updateLogContent()
		}
	default:
		if len(msg.Runes) > 0 {
			m.filterInput += string(msg.Runes)
			m.updateLogContent()
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleMouse(msg tea.MouseMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.focus == focusTasks {
			m.taskList.CursorUp()
		} else {
			m.logViewport.LineUp(3)
		}
	case tea.MouseButtonWheelDown:
		if m.focus == focusTasks {
			m.taskList.CursorDown()
		} else {
			m.logViewport.LineDown(3)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) relayout() Model {
	// Each bordered section uses 2 border rows + 1 title row = 3 overhead rows.
	// 2 sections (tasks + log) + 1 header + 1 BTW bar + 1 footer = 9 fixed rows.
	const fixedRows = 9 // 1 header + 3 tasks-border + 3 log-border + 1 btw + 1 footer
	remaining := m.height - fixedRows
	if remaining < 4 {
		remaining = 4
	}
	tasksH := remaining * m.splitRatio / 100
	if tasksH < 2 {
		tasksH = 2
	}
	logH := remaining - tasksH
	if logH < 2 {
		logH = 2
	}

	m.taskList.SetSize(m.width-4, tasksH) // -4 for left+right border chars
	m.logViewport.Width = m.width - 4
	m.logViewport.Height = logH
	m.updateLogContent()
	return *m
}

func (m *Model) pollStatus() Model {
	info, err := os.Stat(m.statusFile)
	if err != nil {
		return *m
	}
	if !info.ModTime().After(m.lastMtime) {
		return *m
	}
	m.lastMtime = info.ModTime()

	data, err := os.ReadFile(m.statusFile)
	if err != nil {
		return *m
	}
	var status types.AgentStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return *m
	}

	m.btw = status.Btw
	m.allLog = status.Log
	m.tasks = buildTaskItems(status)
	m.refreshTaskList()
	m.updateLogContent()
	return *m
}

func buildTaskItems(status types.AgentStatus) []taskItem {
	btwMap := buildBtwMap(status.Btw)

	// Index investigation agents and build agents by bugID.
	invByBug := make(map[string]*store.InvestigationAgent)
	for bugID, ia := range status.InvestigationAgents {
		invByBug[bugID] = ia
	}
	type buildPos struct {
		name    string
		agentID string
		status  string
		pos     int // 0=current, N=queue position
	}
	buildByBug := make(map[string]buildPos)
	for agentName, ba := range status.BuildAgents {
		if ba.CurrentBug != nil {
			bugID := strconv.FormatInt(*ba.CurrentBug, 10)
			buildByBug[bugID] = buildPos{name: agentName, agentID: ba.AgentID, status: ba.Status, pos: 0}
		}
		for i, qid := range ba.Queue {
			bugID := strconv.FormatInt(qid, 10)
			if _, already := buildByBug[bugID]; !already {
				buildByBug[bugID] = buildPos{name: agentName, agentID: ba.AgentID, status: ba.Status, pos: i + 1}
			}
		}
	}

	var items []taskItem
	for id, t := range status.Tasks {
		var btwMsg string
		if inv, ok := invByBug[id]; ok && inv.AgentID != "" {
			if e, ok := btwMap[inv.AgentID]; ok {
				btwMsg = e.Message
			}
		}
		if btwMsg == "" {
			if bp, ok := buildByBug[id]; ok {
				if e, ok := btwMap[bp.agentID]; ok {
					btwMsg = e.Message
				}
			}
		}

		var revURL string
		if t.Worktree != "" && worktreeHasPatches(t.Worktree) {
			revURL = reviewURL(t.Worktree)
		}
		var doneAt float64
		if t.DoneAt != nil {
			doneAt = *t.DoneAt
		}

		item := taskItem{
			bugID:         id,
			summary:       t.Summary,
			status:        t.Status,
			note:          t.Note,
			worktree:      t.Worktree,
			revURL:        revURL,
			btwMsg:        btwMsg,
			hasInv:        invFileExists(id),
			doneAt:        doneAt,
			buildQueuePos: -1,
		}
		if inv, ok := invByBug[id]; ok {
			item.invAgentID = inv.AgentID
			item.invAgentStatus = inv.Status
			item.invBuildType = inv.BuildType
			item.claimedFiles = inv.ClaimedFiles
		}
		if bp, ok := buildByBug[id]; ok {
			item.buildAgentName = bp.name
			item.buildAgentStatus = bp.status
			item.buildQueuePos = bp.pos
		}
		items = append(items, item)
	}
	// Sort: FAILED and WAITING first, then RUNNING, then others.
	priority := map[string]int{"failed": 0, "waiting": 1, "running": 2, "idle": 3, "done": 4}
	sort.Slice(items, func(i, j int) bool {
		pi := priority[items[i].status]
		pj := priority[items[j].status]
		if pi != pj {
			return pi < pj
		}
		return items[i].bugID < items[j].bugID
	})
	return items
}

// buildBtwMap returns a map from agentName to the most recent BtwEntry for that agent.
func buildBtwMap(entries []types.BtwEntry) map[string]types.BtwEntry {
	m := make(map[string]types.BtwEntry)
	for _, e := range entries {
		m[e.Agent] = e
	}
	return m
}

var cachedHomeDir = sync.OnceValue(func() string {
	h, _ := os.UserHomeDir()
	return h
})

const worktreePatchCacheTTL = 30 * time.Second

type wtCacheEntry struct {
	has       bool
	checkedAt time.Time
}

var (
	wtCacheMu sync.Mutex
	wtCache   = map[string]wtCacheEntry{}
)

// worktreeHasPatches returns true if the worktree has local commits on top of origin/main.
// Results are cached for worktreePatchCacheTTL.
func worktreeHasPatches(worktree string) bool {
	now := time.Now()
	wtCacheMu.Lock()
	if e, ok := wtCache[worktree]; ok && now.Sub(e.checkedAt) < worktreePatchCacheTTL {
		wtCacheMu.Unlock()
		return e.has
	}
	wtCacheMu.Unlock()

	expanded := strings.ReplaceAll(worktree, "~", cachedHomeDir())
	out, err := exec.Command("git", "-C", expanded,
		"log", "--oneline", "origin/main..HEAD").Output()
	has := err == nil && len(strings.TrimSpace(string(out))) > 0

	wtCacheMu.Lock()
	wtCache[worktree] = wtCacheEntry{has: has, checkedAt: now}
	wtCacheMu.Unlock()
	return has
}

// reviewURL builds the local review-server URL for a worktree path.
func reviewURL(worktree string) string {
	base := filepath.Base(worktree)
	fragment := strings.TrimPrefix(base, "firefox-")
	return fmt.Sprintf("http://localhost:%d/#%s", reviewServerPort, fragment)
}

// reviewServerPort is the platform-specific port used by the review server.
var reviewServerPort = func() int {
	switch runtime.GOOS {
	case "darwin":
		return 7777
	case "windows":
		return 7778
	default:
		return 7779
	}
}()

// invFileExists returns true if a firefox-bug-investigation file exists for the given bug ID.
func invFileExists(bugID string) bool {
	home := cachedHomeDir()
	if home == "" {
		return false
	}
	path := filepath.Join(home, "firefox-bug-investigation", "bug-"+bugID+"-investigation.md")
	_, err := os.Stat(path)
	return err == nil
}

func (m *Model) refreshTaskList() {
	prevIdx := m.taskList.Index()
	var listItems []list.Item
	for _, t := range m.tasks {
		listItems = append(listItems, taskListItem{task: t})
	}
	m.taskList.SetItems(listItems)
	m.taskList.Select(prevIdx) // SetItems resets cursor; restore it
}

var agentColorPalette = []lipgloss.Color{
	"75", "208", "141", "43", "204", "185", "39", "120",
}

// agentColor returns a consistent color for an agent name by hashing it.
func agentColor(name string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	return agentColorPalette[h.Sum32()%uint32(len(agentColorPalette))]
}

func (m *Model) updateLogContent() {
	// Capture scroll position before replacing content so we can restore it.
	// When already at the bottom (including the initial empty-viewport state),
	// auto-scroll to keep the newest entries visible — like tail -f.
	atBottom := m.logViewport.AtBottom()
	var lines []string
	filter := strings.ToLower(m.filterInput)
	for _, e := range m.allLog {
		ts := time.Unix(int64(e.Time), 0).Format("15:04")
		agentStyled := lipgloss.NewStyle().Foreground(agentColor(e.Agent)).Render(padRight(e.Agent, 16))
		line := ts + "  " + agentStyled + "  " + renderLogMessage(e.Message)
		// Strip ANSI for filter matching.
		plain := ts + "  " + padRight(e.Agent, 16) + "  " + e.Message
		if filter == "" || strings.Contains(strings.ToLower(plain), filter) {
			lines = append(lines, line)
		}
	}
	m.logViewport.SetContent(strings.Join(lines, "\n"))
	if atBottom {
		m.logViewport.GotoBottom()
	}
}

var logLinkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

func renderLogMessage(msg string) string {
	return urlRe.ReplaceAllStringFunc(msg, func(url string) string {
		return logLinkStyle.Render(hyperlink(url, "[link]"))
	})
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func hyperlink(url, label string) string {
	// OSC 8 hyperlink: ESC]8;;URL ESC\ label ESC]8;; ESC\
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

var (
	statusStyleBase    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	statusStyleDefault = statusStyleBase.Foreground(lipgloss.Color("255"))
	statusStyleDim     = statusStyleBase.Foreground(lipgloss.Color("240"))
)

// statusStyle returns a lipgloss style for the given task status.
func statusStyle(status string) lipgloss.Style {
	switch status {
	case "done", "idle":
		return statusStyleDim
	default:
		return statusStyleBase.Foreground(statusColor(status))
	}
}
