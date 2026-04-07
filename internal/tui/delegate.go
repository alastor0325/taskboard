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

// Height is 4: top-border/accent + 2 content rows + bottom-border/padding.
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

	// Build link set: [try] [rev] always [bug]; [inv] if investigation file exists.
	bugURL := "https://bugzilla.mozilla.org/show_bug.cgi?id=" + t.task.bugID
	var links []string
	if t.task.tryURL != "" {
		links = append(links, hyperlink(t.task.tryURL, "[try]"))
	}
	if t.task.revURL != "" {
		links = append(links, hyperlink(t.task.revURL, "[rev]"))
	}
	links = append(links, hyperlink(bugURL, "[bug]"))
	if t.task.hasInv {
		invURL := "https://github.com/alastor0325/firefox-bug-investigation/blob/main/bug-" + t.task.bugID + "-investigation.md"
		links = append(links, invStyle.Render(hyperlink(invURL, "[inv]")))
	}
	linkStr := "  " + strings.Join(links, " ")

	row0 := fmt.Sprintf("%-10s  %s  %s%s",
		dim.Render(t.task.bugID),
		badge,
		dim.Render(t.task.summary),
		linkStr,
	)
	row1 := buildRow1(t.task)

	cardWidth := m.Width() - 2
	if cardWidth < 1 {
		cardWidth = 1
	}

	if selected {
		// Truncate rows so the border box never wraps and corrupts the fixed height.
		inner := ansiTrimRight(row0, cardWidth) + "\n" + ansiTrimRight(row1, cardWidth)
		box := cardBorderStyle(t.task.status).Width(cardWidth).Render(inner)
		fmt.Fprint(w, box)
	} else {
		// Colored left-accent bar for unselected cards.
		accent := lipgloss.NewStyle().Foreground(statusAccentColor(t.task.status)).Render("▌ ")
		fmt.Fprintln(w, accent+ansiTrimRight(row0, m.Width()-2))
		fmt.Fprintln(w, "  "+ansiTrimRight(row1, m.Width()-2))
		fmt.Fprintln(w, "")
		fmt.Fprint(w, "")
	}
}

// buildRow1 returns the secondary info line for a card.
// Priority: note > worktree > btw heartbeat > empty.
// [inv] and [bug] links are always shown in row0, not here.
func buildRow1(t taskItem) string {
	if t.note != "" {
		if t.status == "waiting" {
			return noteWaitingStyle.Render(">> " + t.note)
		}
		return notePlainStyle.Render(t.note)
	}
	if t.worktree != "" {
		wt := strings.Replace(t.worktree, cachedHomeDir(), "~", 1)
		return worktreeStyle.Render(wt)
	}
	if t.btwMsg != "" {
		return btwCardStyle.Render(btwSpinnerChar() + " " + t.btwMsg)
	}
	return ""
}

// cardBorderStyle returns a colored border box for the selected task card.
func cardBorderStyle(status string) lipgloss.Style {
	var color lipgloss.Color
	switch status {
	case "failed":
		color = "196"
	case "waiting":
		color = "214"
	case "running":
		color = "82"
	case "done":
		color = "236"
	default: // idle
		color = "238"
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(color)).
		Bold(true)
}

// statusAccentColor returns the left-accent bar color for an unselected card.
func statusAccentColor(status string) lipgloss.Color {
	switch status {
	case "failed":
		return "196"
	case "waiting":
		return "214"
	case "running":
		return "82"
	case "done":
		return "236"
	default:
		return "238"
	}
}

// statusBadge returns the display label for a task status.
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
