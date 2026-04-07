# taskboard

CLI task board for multi-agent Firefox bug work. Replaces the Python scripts in `~/.claude/skills/firefox-manager/`.

## Install

```bash
make install
```

Installs `taskboard` to `~/.local/bin/taskboard`.

## Usage

```
taskboard init                        # start session
taskboard sync                        # re-read team.json, update agent-status.json
taskboard set-task <bug_id> [flags]   # partial update task
  --summary <text>
  --status  <idle|running|waiting|done>
  --note    <text>
  --worktree <path>
taskboard done-task <bug_id>          # mark task done
taskboard claim-task <bug_id> <agent> # atomic claim (JSON output)
taskboard who-owns <bug_id>           # ownership query (JSON output)
taskboard file-conflicts <bug_id>     # file conflict check (JSON output)
taskboard log <agent> <message>       # append log entry
taskboard btw <agent> <message>       # volatile heartbeat (TTL 120s)
taskboard notify <log|alert|done> <msg> # Matrix notification
taskboard agent-health <file> [secs]  # liveness check by output file mtime
taskboard check-build-progress <dir> [min] # build stall detection
taskboard tui                         # launch TUI dashboard
taskboard watcher                     # start watcher daemon
taskboard healthcheck                 # run healthcheck pass
taskboard open [--width <pct>]        # split pane + launch tui

Global flag:
  --project <name>                    # override project detection
```

## Project detection

When `--project` is not specified, the project name is detected in this order:

1. `FIREFOX_MANAGER_PROJECT` env var
2. tmux session name (`tmux display-message -p "#S"`)
3. Zellij session name (`$ZELLIJ_SESSION_NAME`)
4. `~/.firefox-manager/` directory scan (only if exactly one project exists)
5. CWD basename if it starts with `firefox-`
6. Random `session-{hex}` fallback

## Data files

- `~/.firefox-manager/{project}/team.json` — task and agent state
- `~/.firefox-manager/{project}/agent-status.json` — TUI data (or `$AGENT_STATUS_FILE`)

## TUI

The TUI dashboard shows tasks and logs in a split layout. Launch with `taskboard tui` or `taskboard open` (which splits the current pane).

### tmux prerequisite

Add to `~/.tmux.conf` for mouse support:

```tmux
set -g mouse on
```

Without this, mouse wheel scrolling in the TUI will not work inside tmux.

After focusing a tmux pane by clicking, the first click is consumed by tmux to switch focus — this is expected behaviour. Subsequent clicks/scrolls work normally.

Text selection in tmux requires `Shift+click` (mouse mode intercepts plain clicks).

## TUI keyboard shortcuts

| Key | Scope | Action |
|---|---|---|
| `Tab` | Global | Switch focus: TASKS ↔ LOG |
| `↑↓` / `jk` | Focused section | Move cursor / scroll |
| `g` / `G` | Focused section | Jump to top / bottom |
| `Enter` | TASKS | Open task detail overlay |
| `ESC` | Overlay | Close overlay |
| `/` | LOG | Activate log filter |
| `q` / `ESC` | Global | Quit |
| Mouse wheel | Focused section | Scroll |

## Development

```bash
make build    # build ./taskboard binary
make test     # go test ./...
make lint     # go vet ./...
make install  # install to ~/.local/bin/taskboard
```
