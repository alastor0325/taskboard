package tui

import (
	"fmt"
	"io"
	"strings"

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

func (d taskDelegate) Height() int                             { return 1 }
func (d taskDelegate) Spacing() int                           { return 0 }
func (d taskDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

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

	summary := t.task.summary
	// Build link badges.
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

	// Dim done/idle tasks.
	dim := lipgloss.NewStyle()
	if t.task.status == "done" || t.task.status == "idle" {
		dim = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}

	line := fmt.Sprintf("%s%-10s  %s  %s%s",
		cursor,
		dim.Render(t.task.bugID),
		badge,
		dim.Render(summary),
		linkStr,
	)
	fmt.Fprint(w, line)
}
