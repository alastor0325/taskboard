package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type taskListItem struct {
	task taskItem
}

func (t taskListItem) FilterValue() string { return t.task.bugID + " " + t.task.summary }
func (t taskListItem) Title() string       { return t.task.bugID }
func (t taskListItem) Description() string { return t.task.summary }

type taskDelegate struct{}

func newTaskDelegate() taskDelegate { return taskDelegate{} }

// Height is 4: 2 content rows + top border + bottom border.
func (d taskDelegate) Height() int                             { return 4 }
func (d taskDelegate) Spacing() int                            { return 0 }
func (d taskDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

var (
	noteWaitingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	notePlainStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	worktreeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	invStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Faint(true)
	btwCardStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	dimStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

var btwSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

func btwSpinnerChar() string {
	frame := (time.Now().UnixMilli() / 120) % int64(len(btwSpinnerFrames))
	return btwSpinnerFrames[frame]
}

func (d taskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	t, ok := item.(taskListItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	badge := statusStyle(t.task.status).Render(statusBadge(t.task.status))

	dim := lipgloss.NewStyle()
	if t.task.status == "done" || t.task.status == "idle" {
		dim = dimStyle
	}

	var links []string
	if t.task.tryURL != "" {
		links = append(links, hyperlink(t.task.tryURL, "[try]"))
	}
	if t.task.revURL != "" {
		links = append(links, hyperlink(t.task.revURL, "[rev]"))
	}
	linkStr := ""
	if len(links) > 0 {
		linkStr = "  " + strings.Join(links, " ")
	}
	row0 := fmt.Sprintf("%-10s  %s  %s%s",
		dim.Render(t.task.bugID),
		badge,
		dim.Render(t.task.summary),
		linkStr,
	)

	row1 := buildRow1(t.task)

	inner := row0 + "\n" + row1
	cardWidth := m.Width() - 2 // subtract left+right border chars
	if cardWidth < 1 {
		cardWidth = 1
	}
	box := cardBorderStyle(t.task.status, selected).Width(cardWidth).Render(inner)
	fmt.Fprint(w, box)
}

// buildRow1 returns the secondary info line for a card.
// Priority: note > worktree+inv > btw heartbeat > empty.
func buildRow1(t taskItem) string {
	if t.note != "" {
		if t.status == "waiting" {
			return noteWaitingStyle.Render(">> " + t.note)
		}
		return notePlainStyle.Render(t.note)
	}
	var parts []string
	if t.worktree != "" {
		wt := strings.Replace(t.worktree, cachedHomeDir(), "~", 1)
		parts = append(parts, worktreeStyle.Render(wt))
	}
	if t.hasInv {
		invURL := "https://github.com/alastor0325/firefox-bug-investigation/blob/main/bug-" + t.bugID + "-investigation.md"
		parts = append(parts, invStyle.Render(hyperlink(invURL, "[inv]")))
	}
	if len(parts) > 0 {
		return strings.Join(parts, "  ")
	}
	if t.btwMsg != "" {
		return btwCardStyle.Render(btwSpinnerChar() + " " + t.btwMsg)
	}
	return ""
}

// cardBorderStyle returns a border box style for a task card.
// Border type and color signal priority; selected cards get a bold border.
func cardBorderStyle(status string, selected bool) lipgloss.Style {
	var border lipgloss.Border
	var color lipgloss.Color
	switch status {
	case "failed":
		border = lipgloss.ThickBorder()
		color = "196"
	case "waiting":
		border = lipgloss.NormalBorder()
		color = "214"
	case "running":
		border = lipgloss.NormalBorder()
		color = "82"
	case "done":
		border = lipgloss.NormalBorder()
		color = "236"
	default: // idle
		border = lipgloss.NormalBorder()
		color = "238"
	}
	s := lipgloss.NewStyle().Border(border).BorderForeground(lipgloss.Color(color))
	if selected {
		s = s.Bold(true)
	}
	return s
}

// statusBadge returns the display label for a task status, matching the Python STATUS_META symbols.
func statusBadge(status string) string {
	switch status {
	case "running":
		return "▶  RUNNING"
	case "waiting":
		return "⏸  WAITING"
	case "done":
		return "✓  DONE   "
	case "idle":
		return "·  IDLE   "
	case "failed":
		return "✗  FAILED "
	default:
		return strings.ToUpper(status)
	}
}
