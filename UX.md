# Taskboard TUI — UX Design Specification

This document specifies the four UX improvements to implement. It is
implementation-ready: every section names the exact Go/lipgloss calls or
terminal commands to use.

---

## 1. `<`/`>` — tmux pane resize (not split ratio)

### Problem
`<` and `>` currently adjust `m.splitRatio`, which shifts the border between
the TASKS and LOG panels. That is the wrong semantic: the user wants to widen
or narrow the whole TUI column inside tmux.

### New behavior

| Key | Action |
|-----|--------|
| `<` | `tmux resize-pane -L 5` (shrink TUI column by 5 cells) |
| `>` | `tmux resize-pane -R 5` (grow TUI column by 5 cells) |

Only execute when `os.Getenv("TMUX") != ""`. Outside tmux, silently ignore.

Implementation in `handleKey`:

```go
case "<":
    if os.Getenv("TMUX") != "" {
        exec.Command("tmux", "resize-pane", "-L", "5").Run()
    }
case ">":
    if os.Getenv("TMUX") != "" {
        exec.Command("tmux", "resize-pane", "-R", "5").Run()
    }
```

### Split-ratio keys (replacement)
Keep the split-ratio feature but move it to `[` (decrease) and `]` (increase).
Update the footer hint: `[/] split  </>  pane width`.

---

## 2. Agent name colors — per-agent palette

### Problem
`updateLogContent` calls `padRight(e.Agent, 16)` with no color. All agent
names look identical.

### Palette (8 distinct 256-color codes)

Choose colors that are visually distinct and readable on both dark and light
terminal backgrounds. Avoid the status colors already in use (82 green, 196
red, 214 amber, 240 dim-grey, 62 purple).

```
Index 0 → "75"   (steel blue)
Index 1 → "208"  (orange)
Index 2 → "141"  (medium orchid)
Index 3 → "43"   (cyan-green)
Index 4 → "204"  (salmon pink)
Index 5 → "185"  (khaki yellow)
Index 6 → "39"   (dodger blue)
Index 7 → "120"  (pale green)
```

### Assignment rule
Assign palette index deterministically by hashing the agent name string so the
same agent always gets the same color across restarts.

```go
var agentColorPalette = []lipgloss.Color{
    "75", "208", "141", "43", "204", "185", "39", "120",
}

func agentColor(name string) lipgloss.Color {
    h := fnv.New32a()
    h.Write([]byte(name))
    return agentColorPalette[h.Sum32()%uint32(len(agentColorPalette))]
}
```

Import `hash/fnv` (stdlib, no new dependency).

### Application points

1. **Log panel** — in `updateLogContent`, wrap the agent column:

```go
agentStyled := lipgloss.NewStyle().
    Foreground(agentColor(e.Agent)).
    Render(padRight(e.Agent, 16))
line := ts + "  " + agentStyled + "  " + renderLogMessage(e.Message)
```

2. **BTW bar** — in `renderBTW`, colorize `e.Agent`:

```go
agentLabel := lipgloss.NewStyle().Foreground(agentColor(e.Agent)).Render(e.Agent)
parts = append(parts, sp+" "+agentLabel+"  "+e.Message)
```

---

## 3. Task card borders

### Problem
Cards are bare rows of text separated only by blank lines. There is no visual
grouping, making the list harder to scan.

### Design
Each card is wrapped in a lipgloss border box. The border style and color vary
by task status, giving instant visual priority signaling.

#### Border styles per status

| Status   | Border type            | Foreground color | Rationale |
|----------|------------------------|------------------|-----------|
| `failed` | `lipgloss.ThickBorder()` | `"196"` (red)  | Highest urgency |
| `waiting`| `lipgloss.NormalBorder()` | `"214"` (amber) | Needs attention |
| `running`| `lipgloss.NormalBorder()` | `"82"` (green) | Active |
| `idle`   | `lipgloss.NormalBorder()` | `"238"` (dim)  | Inactive |
| `done`   | `lipgloss.NormalBorder()` | `"236"` (very dim) | Completed |

Selected (cursor) card additionally uses **bold border** regardless of status:
apply `.Bold(true)` to the border foreground style, or overlay the cursor `▶`
in the top-left corner of the border (see layout below).

#### lipgloss call pattern

```go
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
    default: // idle, done
        border = lipgloss.NormalBorder()
        color = "238"
        if status == "done" {
            color = "236"
        }
    }
    s := lipgloss.NewStyle().
        Border(border).
        BorderForeground(lipgloss.Color(color))
    if selected {
        s = s.Bold(true)
    }
    return s
}
```

The cursor `▶` is dropped from row0 content. Selection is indicated solely by
the bold border.

---

## 4. Card layout and spacing

### Problem
`Height() = 4` plus `Spacing() = 1` equals 5 rows per card before borders.
With borders that adds 2 more rows (top + bottom) = 7 rows per card total.
That is far too tall.

### Compact layout

Cards contain at most 3 content rows. With a border the total card height is
**3 content rows + 2 border rows = 5 terminal rows**.

Set `Spacing() = 0`. The border provides natural visual separation.

#### Row assignments

```
┌─ border (top) ──────────────────────────────────────────────┐
│ BUG-XXXXXX  [STATUS]  summary text                [try][rev] │  ← row 0 (always)
│ >> waiting note  OR  worktree path  [inv]                    │  ← row 1 (optional, shown if non-empty)
│ ⣾ btw message text...                                        │  ← row 2 (optional, shown if btwMsg set)
└─ border (bottom) ───────────────────────────────────────────┘
```

**Height calculation:**
- If neither row1 nor row2 has content: 1 content row → card height = 3 (1 + 2 border).
- If only row1 has content: 2 content rows → card height = 4.
- If row1 and row2 both have content: 3 content rows → card height = 5.

Because `list.Delegate` requires a fixed `Height()`, use **3** (the maximum
useful content rows), and pad short cards with empty lines so the border box
stays the same height. Set `Spacing() = 0`.

```go
func (d taskDelegate) Height() int  { return 3 }
func (d taskDelegate) Spacing() int { return 0 }
```

The `Render` method builds the three-line inner string and wraps it:

```go
func (d taskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
    t := item.(taskListItem)
    selected := index == m.Index()
    cardWidth := m.Width() - 2 // subtract 2 for left+right border chars

    // Row 0: bugID + badge + summary + links (always present)
    row0 := fmt.Sprintf("%-10s  %s  %s%s",
        dim.Render(t.task.bugID),
        badge,
        dim.Render(t.task.summary),
        linkStr,
    )

    // Row 1: note (if waiting) OR worktree + inv link (first non-empty wins)
    row1 := buildRow1(t.task)

    // Row 2: btw spinner message
    row2 := ""
    if t.task.btwMsg != "" {
        row2 = btwCardStyle.Render(btwSpinnerChar() + " " + t.task.btwMsg)
    }

    inner := strings.Join([]string{row0, row1, row2}, "\n")
    box := cardBorderStyle(t.task.status, selected).Width(cardWidth).Render(inner)
    fmt.Fprint(w, box)
}

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
    return strings.Join(parts, "  ")
}
```

### relayout adjustment
With `Height() = 3` and `Spacing() = 0`, each card takes `3 + 2 = 5` terminal
rows (content height + border). The `list.Model` measures content height via
`delegate.Height()` and border is added inside `Render`. Verify that
`m.taskList.SetSize` still receives the correct height after this change: the
inner list height should equal `tasksH` (unchanged), and the border box around
the whole TASKS section still uses the existing `borderFocused`/`borderUnfocused`
styles in `renderTasks`.

---

## 5. Footer hint update

Update the footer string in `renderFooter` to reflect all changes:

```
Tab [TASKS] [LOG]  ↑↓/jk scroll  g/G top/bot  [/] split  </>  pane  / filter  q quit
```

---

## 6. Summary of all changes

| File | Change |
|------|--------|
| `internal/tui/model.go` | `<`/`>` → tmux resize-pane; `[`/`]` → split ratio; add `agentColor()` helper with `hash/fnv` |
| `internal/tui/delegate.go` | `Height()=3`, `Spacing()=0`; border-per-card in `Render`; remove cursor prefix from row0; consolidate row1 to note-or-worktree; add `cardBorderStyle()` |
| `internal/tui/view.go` | `renderBTW` colorizes agent name; `updateLogContent` colorizes agent column; footer text updated |
