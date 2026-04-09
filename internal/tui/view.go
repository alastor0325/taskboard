package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/alastor0325/taskboard/internal/types"
)

var (
	borderFocused     = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("62"))
	borderUnfocused   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Underline(true)
	countStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	btwStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	footerFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	overlayStyle      = lipgloss.NewStyle().
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
	footer := m.renderFooter()

	main := lipgloss.JoinVertical(lipgloss.Left,
		header,
		tasksSection,
		logSection,
		btwBar,
		footer,
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
	n := len(m.taskList.Items())
	var countStr string
	if n > 0 {
		countStr = fmt.Sprintf("  (%d/%d)", m.taskList.Index()+1, n)
	} else {
		countStr = "  (0)"
	}
	title := sectionTitleStyle.Render("TASKS") + countStyle.Render(countStr)
	content := lipgloss.NewStyle().Height(m.taskList.Height()).Render(m.taskList.View())
	innerW := m.width - 4 // 2 border chars each side
	if innerW < 1 {
		innerW = 1
	}
	if m.focus == focusTasks {
		return borderFocused.Width(innerW).Render(title + "\n" + content)
	}
	return borderUnfocused.Width(innerW).Render(title + "\n" + content)
}

func (m Model) renderLog() string {
	n := len(m.allLog)
	title := sectionTitleStyle.Render("LOG") + countStyle.Render(fmt.Sprintf("  (%d)", n))
	if m.filtering {
		title += "  [/] filter: \"" + m.filterInput + "_\""
	} else if m.filterInput != "" {
		title += "  [/] filter: \"" + m.filterInput + "\""
	}
	content := m.logViewport.View()
	innerW := m.width - 4
	if innerW < 1 {
		innerW = 1
	}
	if m.focus == focusLog {
		return borderFocused.Width(innerW).Render(title + "\n" + content)
	}
	return borderUnfocused.Width(innerW).Render(title + "\n" + content)
}

// btwSegment is a styled piece of the BTW ticker line.
type btwSegment struct {
	text  string
	color lipgloss.Color // empty = use btwStyle
}

const (
	btwTTLSeconds      = 120
	btwScrollMsPerChar = 80
)

func (m Model) renderBTW() string {
	now := time.Now()
	active := filterBtw(m.btw, now)
	if len(active) == 0 {
		return btwStyle.Render("─")
	}

	sp := btwSpinnerChar()
	sep := "  ·  "

	// Build segments and a parallel plain-text string for scroll-offset calculation.
	var segments []btwSegment
	var plainParts []string
	for i, e := range active {
		if i > 0 {
			segments = append(segments, btwSegment{text: sep})
		}
		segments = append(segments,
			btwSegment{text: sp + " "},
			btwSegment{text: e.Agent, color: agentColor(e.Agent)},
			btwSegment{text: "  " + e.Message},
		)
		plainParts = append(plainParts, sp+" "+e.Agent+"  "+e.Message)
	}
	plainCycle := strings.Join(plainParts, sep) + sep
	cycleLen := len([]rune(plainCycle))
	if cycleLen < 1 {
		cycleLen = 1
	}

	offset := int(now.UnixMilli()/btwScrollMsPerChar) % cycleLen

	// Build visible output by consuming segments, skipping the first `offset` visible chars.
	var b strings.Builder
	remaining := offset
	doubled := append(segments, btwSegment{text: sep})
	doubled = append(doubled, segments...) // wrap-around copy
	for _, seg := range doubled {
		runes := []rune(seg.text)
		if remaining >= len(runes) {
			remaining -= len(runes)
			continue
		}
		// This segment is partially consumed.
		text := string(runes[remaining:])
		remaining = 0
		if seg.color != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(seg.color).Render(text))
		} else {
			b.WriteString(btwStyle.Render(text))
		}
	}

	line := b.String()
	maxW := m.width - 1
	if maxW < 1 {
		maxW = 1
	}
	if visibleWidth(line) > maxW {
		line = ansiTrimRight(line, maxW)
	}
	return line
}

func filterBtw(entries []types.BtwEntry, now time.Time) []types.BtwEntry {
	var out []types.BtwEntry
	for _, e := range entries {
		if now.Unix()-int64(e.Time) <= btwTTLSeconds {
			out = append(out, e)
		}
	}
	return out
}

// renderFooter returns the keybinding help bar. In filter mode it shows the
// current filter input and available actions; otherwise it shows all key bindings.
func (m Model) renderFooter() string {
	if m.filtering {
		return footerFilterStyle.Render(" / " + m.filterInput + "   Enter confirm  Esc clear ")
	}
	tasksLabel := " tasks "
	logLabel := " log "
	if m.focus == focusTasks {
		tasksLabel = "[TASKS]"
	} else {
		logLabel = "[LOG]"
	}
	return footerStyle.Render(" Tab " + tasksLabel + " " + logLabel + "  ↑↓/jk scroll  g/G top/bot  [/] split  ,/.  pane  / filter  q quit ")
}

var (
	overlayLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	overlaySepStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	overlayValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	overlayDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	overlayAgentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	overlayFileStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
)

func overlaySection(title string, w int) string {
	bar := strings.Repeat("─", max(0, w-2-len(title)-1))
	return overlaySepStyle.Render("─ " + title + " " + bar)
}

func overlayRow(label, value string) string {
	return overlayLabelStyle.Render(fmt.Sprintf("  %-12s", label)) + overlayValueStyle.Render(value)
}

func renderOverlay(item taskItem, width, height int, behind string) string {
	w := min(68, width-4)
	inner := w - 2 // available width inside border padding

	var lines []string

	// Header
	lines = append(lines,
		fmt.Sprintf("  %s   %s",
			overlayValueStyle.Bold(true).Render("bug-"+item.bugID),
			statusStyle(item.status).Render(strings.ToUpper(item.status)),
		),
		"",
		"  "+overlayDimStyle.Render(item.summary),
		"",
	)

	// Agents section
	hasAgents := item.invAgentID != "" || item.buildAgentName != ""
	if hasAgents {
		lines = append(lines, overlaySection("Agents", inner))
		if item.invAgentID != "" {
			invLine := overlayAgentStyle.Render(item.invAgentID)
			if item.invAgentStatus != "" {
				invLine += overlayDimStyle.Render("  " + item.invAgentStatus)
			}
			if item.invBuildType != "" {
				invLine += overlayDimStyle.Render("  " + item.invBuildType + " build")
			}
			lines = append(lines, overlayLabelStyle.Render("  investigation  ")+invLine)
		}
		if item.buildAgentName != "" {
			buildLine := overlayAgentStyle.Render(item.buildAgentName)
			buildLine += overlayDimStyle.Render("  " + item.buildAgentStatus)
			if item.buildQueuePos == 0 {
				buildLine += overlayDimStyle.Render("  (building now)")
			} else if item.buildQueuePos > 0 {
				buildLine += overlayDimStyle.Render(fmt.Sprintf("  (queued #%d)", item.buildQueuePos))
			}
			lines = append(lines, overlayLabelStyle.Render("  build          ")+buildLine)
		}
		lines = append(lines, "")
	}

	// BTW heartbeat
	if item.btwMsg != "" {
		lines = append(lines, overlaySection("Live", inner))
		lines = append(lines, "  "+btwCardStyle.Render(btwSpinnerChar()+" "+item.btwMsg), "")
	}

	// Claimed files
	if len(item.claimedFiles) > 0 {
		lines = append(lines, overlaySection("Files", inner))
		for _, f := range item.claimedFiles {
			lines = append(lines, "  "+overlayFileStyle.Render(f))
		}
		lines = append(lines, "")
	}

	// Note
	if item.note != "" {
		lines = append(lines, overlaySection("Note", inner))
		lines = append(lines, "  "+overlayValueStyle.Render(item.note), "")
	}

	// Links
	lines = append(lines, overlaySection("Links", inner))
	bugURL := "https://bugzilla.mozilla.org/show_bug.cgi?id=" + item.bugID
	lines = append(lines, overlayRow("bug", hyperlink(bugURL, "[bugzilla "+item.bugID+"]")))
	if item.hasInv {
		invURL := "https://github.com/alastor0325/firefox-bug-investigation/blob/main/bug-" + item.bugID + "-investigation.md"
		lines = append(lines, overlayRow("investigation", hyperlink(invURL, "[github]")))
	}
	if item.worktree != "" {
		wt := strings.Replace(item.worktree, cachedHomeDir(), "~", 1)
		lines = append(lines, overlayRow("worktree", overlayDimStyle.Render(wt)))
	}
	if item.tryURL != "" {
		lines = append(lines, overlayRow("try", hyperlink(item.tryURL, "[treeherder]")))
	}
	if item.revURL != "" {
		lines = append(lines, overlayRow("review", hyperlink(item.revURL, "[review server]")))
	}

	// Footer
	lines = append(lines, "", overlayDimStyle.Render("  Press ESC to close"))

	box := overlayStyle.Width(w).Render(strings.Join(lines, "\n"))

	// Centre the box over the behind content.
	boxLines := strings.Split(box, "\n")
	bgLines := strings.Split(behind, "\n")

	// Pad bgLines to full terminal height so startY indexing never misses.
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	startY := (height - len(boxLines)) / 2
	startX := (width - lipgloss.Width(box)) / 2
	if startX < 0 {
		startX = 0
	}

	for i, bl := range boxLines {
		row := startY + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bgLines[row] = overlayLine(bgLines[row], bl, startX)
	}
	return strings.Join(bgLines, "\n")
}

// overlayLine replaces bg content starting at visible position x with overlay.
// Uses ANSI-aware trimming so existing escape sequences don't corrupt the offset.
func overlayLine(bg, overlay string, x int) string {
	if x < 0 {
		x = 0
	}
	prefix := ansiTrimRight(bg, x)
	prefixW := visibleWidth(prefix)
	if prefixW < x {
		prefix += strings.Repeat(" ", x-prefixW)
	}
	return prefix + overlay
}

func visibleWidth(s string) int {
	return lipgloss.Width(s)
}
