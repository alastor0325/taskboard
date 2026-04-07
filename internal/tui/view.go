package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	borderFocused   = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("62"))
	borderUnfocused = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
	headerStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	btwStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	overlayStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
)

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	header := m.renderHeader()
	btwBar := m.renderBTW()
	tasksSection := m.renderTasks()
	logSection := m.renderLog()

	main := lipgloss.JoinVertical(lipgloss.Left,
		header,
		tasksSection,
		logSection,
		btwBar,
	)

	if m.overlay != nil {
		return renderOverlay(m.overlay.item, m.width, m.height, main)
	}
	return main
}

func (m Model) renderHeader() string {
	now := time.Now().Format("2006-01-02  15:04:05")
	left := headerStyle.Render("taskboard  [" + m.proj + "]")
	right := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(now)
	gap := m.width - visibleWidth(left) - visibleWidth(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderTasks() string {
	title := "TASKS"
	content := lipgloss.NewStyle().Height(m.taskList.Height()).Render(m.taskList.View())
	if m.focus == focusTasks {
		return borderFocused.Width(m.width - 2).Render(title + "\n" + content)
	}
	return borderUnfocused.Width(m.width - 2).Render(title + "\n" + content)
}

func (m Model) renderLog() string {
	title := "LOG"
	if m.filtering {
		title += "  [/] filter: \"" + m.filterInput + "_\""
	} else if m.filterInput != "" {
		title += "  [/] filter: \"" + m.filterInput + "\""
	}
	content := m.logViewport.View()
	if m.focus == focusLog {
		return borderFocused.Width(m.width - 2).Render(title + "\n" + content)
	}
	return borderUnfocused.Width(m.width - 2).Render(title + "\n" + content)
}

func (m Model) renderBTW() string {
	if len(m.btw) == 0 {
		return btwStyle.Render("─")
	}
	var parts []string
	sp := m.spinner.View()
	for _, e := range m.btw {
		parts = append(parts, sp+" "+e.Agent+"  "+e.Message)
	}
	line := strings.Join(parts, "  ·  ")
	// Truncate to terminal width with ellipsis.
	ellipsis := "…"
	if visibleWidth(line) > m.width-1 {
		for visibleWidth(line)+utf8.RuneCountInString(ellipsis) > m.width-1 && len(line) > 0 {
			_, size := utf8.DecodeLastRuneInString(line)
			line = line[:len(line)-size]
		}
		line += ellipsis
	}
	return btwStyle.Render(line)
}

func renderOverlay(item taskItem, width, height int, behind string) string {
	w := min(60, width-4)
	var lines []string
	lines = append(lines,
		fmt.Sprintf("  bug-%s    %s", item.bugID, statusStyle(item.status).Render(strings.ToUpper(item.status))),
		"",
		"  "+item.summary,
		"",
	)
	if item.note != "" {
		lines = append(lines, "  note:      "+item.note, "")
	}
	if item.worktree != "" {
		lines = append(lines, "  worktree:  "+item.worktree)
	}
	if item.tryURL != "" {
		lines = append(lines, "  try:       "+hyperlink(item.tryURL, "[treeherder]"))
	}
	if item.revURL != "" {
		lines = append(lines, "  review:    "+hyperlink(item.revURL, "[localhost:7777]"))
	}
	bugURL := "https://bugzilla.mozilla.org/show_bug.cgi?id=" + item.bugID
	lines = append(lines, "  bug:       "+hyperlink(bugURL, "[bugzilla "+item.bugID+"]"))
	lines = append(lines, "", "                         [ESC] close")

	box := overlayStyle.Width(w).Render(strings.Join(lines, "\n"))

	// Centre the box over the behind content.
	boxLines := strings.Split(box, "\n")
	bgLines := strings.Split(behind, "\n")
	startY := (height - len(boxLines)) / 2
	startX := (width - w - 4) / 2

	for i, bl := range boxLines {
		row := startY + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bg := bgLines[row]
		bgLines[row] = overlayLine(bg, bl, startX, width)
	}
	return strings.Join(bgLines, "\n")
}

func overlayLine(bg, overlay string, x, maxW int) string {
	if x < 0 {
		x = 0
	}
	bgRunes := []rune(bg)
	ovRunes := []rune(overlay)
	// Pad bg if needed.
	for len(bgRunes) < x {
		bgRunes = append(bgRunes, ' ')
	}
	result := make([]rune, len(bgRunes))
	copy(result, bgRunes)
	for i, r := range ovRunes {
		pos := x + i
		if pos >= len(result) {
			result = append(result, r)
		} else {
			result[pos] = r
		}
	}
	return string(result)
}

func visibleWidth(s string) int {
	// Strip ANSI escape sequences for width calculation.
	inEsc := false
	w := 0
	for _, r := range s {
		if inEsc {
			if r == 'm' || r == 'K' || r == 'H' || r == 'J' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		w++
	}
	return w
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
