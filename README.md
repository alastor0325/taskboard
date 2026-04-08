# taskboard

A CLI tool and terminal dashboard for coordinating multi-agent Firefox bug work.
Tracks tasks, agents, file ownership, and build progress across parallel Claude
sessions, with a live Bubble Tea TUI for at-a-glance status.

![taskboard TUI](screenshot.png)

## Getting started

### 1. Install

**With `go install` (recommended):**

```bash
go install github.com/alastor0325/taskboard/cmd/taskboard@latest
```

Installs to `~/go/bin/taskboard`. Make sure `~/go/bin` is on your `PATH`:

```bash
export PATH="$HOME/go/bin:$PATH"   # add to ~/.zshrc or ~/.bashrc
```

**From source:**

```bash
git clone https://github.com/alastor0325/taskboard
cd taskboard
make install   # builds + installs to ~/.local/bin/taskboard
```

### 2. Initialize

```bash
taskboard init
```

This installs the `/taskboard` Claude skill and verifies that tmux is available
(installing it via `brew` or `apt-get` if not). The checklist output tells you
exactly what happened:

```
Initializing taskboard...
✓ Skill: updated (restart Claude session to pick up changes)
✓ Project: firefox-2026875
✓ Log: reset marker written
✓ Status: synced
✓ tmux: ready
Ready.
```

If tmux is missing and cannot be installed automatically, a `WARNING` is printed
at the end — install tmux manually and re-run `taskboard init` before continuing.

### 3. Open Claude inside a tmux session

> ⚠️ **TMUX IS REQUIRED.** taskboard splits your terminal to show the TUI alongside your Claude session.
>
> Open a tmux session before launching Claude Code:
>
> ```bash
> tmux new-session -s firefox    # or attach: tmux attach -t firefox
> claude                         # launch Claude Code inside tmux
> ```
>
> Add `set -g mouse on` to `~/.tmux.conf` to enable mouse wheel scrolling in the TUI.

### 4. Start the taskboard session in Claude

In the Claude Code session, invoke the skill:

```
/taskboard
```

Claude becomes the manager: it spawns investigation and build agents, opens the
TUI dashboard in a split pane, and routes work between agents. You interact only
with the manager — all taskboard commands are run by Claude on your behalf.

---

## How it works

taskboard coordinates three types of Claude agents:

- **Investigation agents** — research a bug, identify root cause and affected files, write an investigation file, then wait for your approval.
- **Build agents** — apply patches, build, test, and submit to Phabricator.
- **Task/utility agents** — ad-hoc background work (test writing, audits, etc.).

All state is shared through two files:

| File | Purpose |
|---|---|
| `~/.taskboard/{project}/team.json` | Source of truth — agents, tasks, file ownership |
| `~/.taskboard/{project}/agent-status.json` | TUI feed (written by `taskboard sync`) |

The TUI reads `agent-status.json` and shows a live task board with per-task status,
notes, worktree, and agent heartbeats.

---

## TUI

The dashboard shows tasks on the left and a log feed on the right.

### Task cards

Each card shows:
- Status badge (`▶ RUN`, `⏸ WAI`, `✓ DON`, `· IDL`) with a color-coded border
- Bug ID (hyperlinked to Bugzilla) and summary
- Secondary row: done-expiry countdown → note → worktree → live agent heartbeat

### Task detail overlay (`Enter`)

- **Agents** — investigation and build agent IDs, statuses, build type, queue position
- **Live** — current heartbeat from the owning agent
- **Files** — files claimed by this task
- **Note** — current task note
- **Links** — Bugzilla link; review-server link (when worktree has unpushed patches)

### Keyboard shortcuts

| Key | Scope | Action |
|---|---|---|
| `Tab` | Global | Switch focus: TASKS ↔ LOG |
| `↑↓` / `jk` | Focused pane | Scroll |
| `g` / `G` | Focused pane | Jump to top / bottom |
| `Enter` | TASKS | Open task detail overlay |
| `ESC` | Overlay | Close overlay |
| `/` | LOG | Filter log |
| `q` / `ESC` | Global | Quit |
| Mouse wheel | Focused pane | Scroll |

---

## CLI reference

These commands are called by Claude agents — you do not run them directly.

```
taskboard init                           # install skill, reset session
taskboard sync                           # sync team.json → agent-status.json
taskboard set-task <bug_id> [flags]      # create or update a task card
  --summary <text>
  --status  <idle|running|waiting|done>
  --note    <text>
  --worktree <path>
taskboard done-task <bug_id>             # mark task done
taskboard claim-task <bug_id> <agent>    # atomic ownership claim
taskboard who-owns <bug_id>              # ownership query
taskboard file-conflicts <bug_id>        # detect file overlap with other agents
taskboard log <agent> <message>          # append log entry
taskboard btw <agent> <message>          # volatile heartbeat (TTL 120s)
taskboard event <type> <agent> <msg>     # structured milestone (routes to Matrix)
taskboard agent-health <file> [secs]     # liveness check by output file mtime
taskboard check-build-progress <dir>     # build stall detection
taskboard detect                         # print detected project name
taskboard tui                            # launch TUI in current terminal
taskboard open [--width <pct>]           # split tmux pane and launch TUI
taskboard watcher                        # start watcher daemon
taskboard healthcheck                    # run one healthcheck pass

Global flag:
  --project <name>                       # override project detection
```

### Project detection order

1. `--project` flag or `TASKBOARD_PROJECT` env var
2. tmux session name
3. Zellij session name (`$ZELLIJ_SESSION_NAME`)
4. `~/.taskboard/` scan (if exactly one project exists)
5. CWD basename if it starts with `firefox-`
6. Random `session-{hex}` fallback

---

## Development

```bash
make build         # build ./taskboard binary
make test          # go test ./...
make lint          # go vet ./...
make install       # install to ~/.local/bin/taskboard + install skill
make install-skill # install skill only (no binary rebuild)
```
