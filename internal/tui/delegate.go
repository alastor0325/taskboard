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

func (d taskDelegate) Height() int                             { return 4 }
func (d taskDelegate) Spacing() int                            { return 1 }
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

	cursor := "  "
	if index == m.Index() {
		cursor = "▶ "
	}

	badge := statusStyle(t.task.status).Render(strings.ToUpper(t.task.status))

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
	row0 := fmt.Sprintf("%s%-10s  %s  %s%s",
		cursor,
		dim.Render(t.task.bugID),
		badge,
		dim.Render(t.task.summary),
		linkStr,
	)

	row1 := ""
	if t.task.note != "" {
		if t.task.status == "waiting" {
			row1 = "  " + noteWaitingStyle.Render(">> "+t.task.note)
		} else {
			row1 = "  " + notePlainStyle.Render(t.task.note)
		}
	}

	row2 := ""
	var row2Parts []string
	if t.task.worktree != "" {
		wt := strings.Replace(t.task.worktree, cachedHomeDir(), "~", 1)
		row2Parts = append(row2Parts, worktreeStyle.Render(wt))
	}
	if t.task.hasInv {
		invURL := "https://github.com/alastor0325/firefox-bug-investigation/blob/main/bug-" + t.task.bugID + "-investigation.md"
		row2Parts = append(row2Parts, invStyle.Render(hyperlink(invURL, "[inv]")))
	}
	if len(row2Parts) > 0 {
		row2 = "  " + strings.Join(row2Parts, "  ")
	}

	row3 := ""
	if t.task.btwMsg != "" {
		row3 = "  " + btwCardStyle.Render(btwSpinnerChar()+" "+t.task.btwMsg)
	}

	fmt.Fprintln(w, row0)
	fmt.Fprintln(w, row1)
	fmt.Fprintln(w, row2)
	fmt.Fprint(w, row3)
}
