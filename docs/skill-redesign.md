# Taskboard Skill Redesign Investigation

**Document scope:** Full audit of `/taskboard` skill (`~/.claude/skills/taskboard/skill.md`),
covering gaps found, business logic definition, and a concrete improvement plan.

---

## 1. Current State Summary

The `/taskboard` skill defines three actors:

- **Manager session** — coordinator. Reads `team.json`, spawns agents, routes messages.
- **Investigation agent** — researches a bug, writes an investigation file, waits for approval.
- **Build agent** — applies patches, builds, tests, submits to Phabricator.

State is shared via two files:
- `~/.taskboard/{PROJECT}/team.json` — source of truth for agents and tasks
- `~/.taskboard/{PROJECT}/agent-status.json` — TUI feed (written by `taskboard sync`)

The TUI reads `agent-status.json` and displays task cards with: status badge, bug ID
(hyperlinked), summary, secondary row (note / worktree / BTW), and a detail overlay
(agents, files, BTW, links).

---

## 2. Gaps Found

### 2.1 Critical — `taskboard event` doesn't exist

Both agent prompts use `taskboard event <type> <agent> "<message>"` extensively:

```bash
taskboard event progress inv-2026875 "Root cause identified"
taskboard event build-started agent-debug "Starting mach build"
taskboard event try-pushed agent-asan "[Try 1](url)"
```

**This subcommand does not exist in the binary.** Every structured milestone call
silently fails with an error. All key-moment logging — build started, tests passed,
try push submitted, task done — is completely broken.

The event type table defined in the skill (build-started, build-done, build-failed,
test-started, test-passed, test-failed, try-pushed, try-auth, waiting, progress,
task-done) is correct conceptually; the binary just never implemented it.

### 2.2 Critical — `set-task` positional form doesn't exist

The build agent MANDATORY rule and several other calls use:

```bash
taskboard set-task {bug_id} "{summary}" running "{new note}" {worktree}
```

The binary only accepts flags: `--summary`, `--status`, `--note`, `--worktree`.
Positional arguments are rejected. Every phase-change note update from the build
agent will fail.

### 2.3 High — Task card never exists during investigation

The investigation agent calls `set-task` only at the very end (after writing the
investigation file and pushing to GitHub). During the entire investigation the task
card does not exist in `team.json`, so the TUI shows nothing for an active bug. A
user watching the dashboard during investigation sees a blank task list.

### 2.4 High — `claim-task` doesn't update task status

The build agent prompt states:
> `claim-task` sets `current_bug`, `status: busy`, and transitions the task to
> `running` atomically.

The actual `ClaimTask` implementation only writes to `investigation_agents`. It does
not touch `tasks` at all. After `claim-task`, the task card still shows `waiting`
until someone explicitly calls `set-task --status running`. The skill never says to
do this.

### 2.5 Medium — `--worktree` never populated in task card

The build agent creates a git worktree (`~/firefox-{id}`) but never calls
`set-task --worktree`. The TUI card's secondary row — which shows the worktree path
and uses it to derive the review-server link — stays blank for the entire build.

### 2.6 Medium — Project detection steps are stale

Step 0 of initialization describes the detection order as matching `status.py` and
includes a hand-written shell script. The actual `project.Detect` implementation in
the binary differs:

| Skill says | Binary does |
|---|---|
| `TASKBOARD_PROJECT` env | ✓ same |
| tmux session (not "0") | ✓ same |
| CWD strips `firefox-` prefix | ✓ same |
| uuid fallback | ✓ same |
| *(missing)* | Zellij `$ZELLIJ_SESSION_NAME` |
| *(missing)* | `~/.taskboard/` single-project scan |

The shell script will drift again. The skill should call `taskboard detect` (a new
trivial subcommand) instead of reimplementing the logic.

### 2.7 Medium — `tasks` key missing from team.json schema example

The schema example shows four top-level keys: `investigation_agents`, `build_agents`,
`task_agents`, `utility_agents`. The `tasks` key — which `set-task`, `done-task`, and
the TUI all read — is completely absent. Anyone reading the skill to understand the
data model gets a wrong picture.

### 2.8 Medium — No authoritative task lifecycle documentation

The skill has no section that says: when is a task created, who creates it, what
fields are set at each phase, and who is responsible for each update. This is the
core business logic. Without it, agents make ad-hoc decisions that diverge over time.

### 2.9 Low — BTW block duplicated in build agent prompt

The build agent prompt contains the BTW timing rule twice: once under the `btw`
bullet and again as a MANDATORY block. The two copies are not identical. This bloats
the prompt and creates ambiguity about which rule takes precedence.

### 2.10 Low — BTW registration race on build agent spawn

The build agent sends `btw {agent_name}` immediately on startup. `btw` requires the
agent to be registered in `team.json`. The manager currently writes the team.json
entry *after* spawning the agent, so the agent can win the race and its first
heartbeat fails silently.

---

## 3. Business Logic Definition

### 3.1 What a task represents

A task card represents **one bug being actively tracked**, from the moment the manager
decides to work on it until the watcher removes it after the 5-minute done-TTL.

**Rule:** One task per bug. Created by the manager at investigation spawn. Never
created by an investigation or build agent. Never created twice.

### 3.2 The two agent slots

A task has two agent slots that reflect who is working on the bug at any given time:

```
task {bug_id}
  ├── investigation slot → investigation_agents["{bug_id}"]   (keyed by bug_id)
  └── build slot         → build_agents["{type}"].current_bug == {bug_id}
```

- **Investigation slot** is filled when the manager writes the `investigation_agents`
  entry before spawning `inv-{id}`. It is not cleared — status moves from `running`
  → `waiting` → `approved`.
- **Build slot** is filled when the build agent calls `claim-task`. Only one build
  agent can hold a bug at a time. It is cleared (current_bug → null) when the build
  agent finishes.
- Both slots can be non-empty simultaneously (e.g., user is still discussing the
  investigation while the build agent has already been dispatched).
- Task agents and utility agents are not linked to a specific bug task and do not
  appear in the task card overlay.

The TUI detail overlay reads both slots to show agent IDs, statuses, build type, and
queue position. Keeping these entries accurate is what makes the overlay useful.

### 3.3 Task field update rules

Each field has a single owner and a specific set of moments when it changes:

**`--summary`**
- Set at spawn by manager: `"Bug {id} — investigating"`
- Updated once by investigation agent when root cause is known: final one-liner
- Never changed by the build agent
- Never changed after investigation completes

**`--status`**
- Changes only at meaningful phase transitions, not for every minor step
- Valid transitions enforced by the binary: `running` → `waiting` → `running` → done

**`--note`**
- Changes whenever the work phase changes or the user needs to know something
- Always set a note when status goes to `waiting` — it must tell the user *why*
- The build agent updates it on every phase change (building → testing → waiting)
- The note is the primary human-readable progress indicator

**`--worktree`**
- Set once by the build agent immediately after `git worktree add`
- Never changed after that

**`done-task`**
- The only way to end a task — sets status to `done` and records `done_at`
- Never use `set-task --status done`

### 3.4 Complete task lifecycle state machine

| Phase | Who | status | summary | note | worktree |
|---|---|---|---|---|---|
| Manager spawns investigation | Manager | `running` | `"Bug X — investigating"` | *(empty)* | *(empty)* |
| Inv: root cause found | Inv agent | *(no change)* | *(no change)* | `"Root cause: <one line>"` | *(empty)* |
| Inv: file written, awaiting approval | Inv agent | `waiting` | `"<final one-liner>"` | `"Awaiting approval — [Investigation](url)"` | *(empty)* |
| Manager: user approved, dispatching | Manager | `running` | *(no change)* | `"Dispatched to agent-{type}"` | *(empty)* |
| Build: worktree created | Build agent | *(no change)* | *(no change)* | *(no change)* | `~/firefox-{id}` |
| Build: applying patch | Build agent | *(no change)* | *(no change)* | `"Applying patch"` | *(no change)* |
| Build: building | Build agent | *(no change)* | *(no change)* | `"Building (mach build)"` | *(no change)* |
| Build: build failed | Build agent | `waiting` | *(no change)* | `"Build failed: <short error>"` | *(no change)* |
| Build: running tests | Build agent | *(no change)* | *(no change)* | `"Running mochitest-media"` | *(no change)* |
| Build: tests failed | Build agent | `waiting` | *(no change)* | `"Tests failed: <test name>"` | *(no change)* |
| Build: plan ready | Build agent | `waiting` | *(no change)* | `"Plan ready — awaiting approval"` | *(no change)* |
| Build: try push submitted | Build agent | *(no change)* | *(no change)* | `"[Try N](url)"` | *(no change)* |
| Build: work fully complete | Build agent | — | — | — | `done-task {id}` |

### 3.5 Log vs BTW vs event — when to use each

**`taskboard log <agent> "<message>"`**
- Free-form narrative. Always persisted. Use for any step worth recording.
- Does not notify Matrix. Safe to call frequently.
- Examples: "Reading bug report", "Applying part 1 of 3", "All tests passed locally"

**`taskboard btw <agent> "<current intention>"`**
- Volatile heartbeat. Disappears after 120s. Shown in the TUI BTW bar.
- Declare current intention at the START of any multi-step scope.
- Refresh every ~60s while still in that scope, and immediately before any build or test command.
- Never use for facts — only for current intention.
- Examples: "Building (mach build)", "Running mochitest-media suite", "Writing investigation file"

**`taskboard event <type> <agent> "<message>"`** *(to be implemented)*
- Structured milestone. Always logged. Routes to Matrix based on type.
- Use only for key moments that the user may need to act on or be aware of.

| Type | When | Matrix? |
|---|---|---|
| `progress` | Root cause identified / notable milestone | no |
| `waiting` | Awaiting user input | no |
| `build-started` | `./mach build` launched | no |
| `build-done` | Build succeeded | no |
| `build-failed` | Build failed | **alert** |
| `test-started` | Test run launched | no |
| `test-passed` | All tests passed | no |
| `test-failed` | Tests failed | **alert** |
| `try-pushed` | Try push submitted (include `[Try N](url)`) | log |
| `try-auth` | Try push needs authentication (include URL) | **alert** |
| `task-done` | Bug fully complete | log |

---

## 4. Improvement Plan

### Phase 0 — Document task lifecycle in the skill (no code changes)

Add a new "Task Lifecycle" section to `skill.md` containing:
- The two-slot agent model (section 3.2 above)
- The five field rules (section 3.3 above)
- The full state machine table (section 3.4 above)
- The log vs btw vs event decision table (section 3.5 above)

This section becomes the authoritative reference. All three agent prompts cite it
rather than duplicating rules inline.

### Phase 1 — Binary: implement `taskboard event`

New subcommand: `taskboard event <type> <agent> "<message>"`

Behavior:
1. Validate agent is registered (same check as `btw`)
2. `appendLog` — always
3. `syncStatus` — always
4. Conditionally run `matrix-cli notify {level} "{message}"` based on type:
   - `alert`: build-failed, test-failed, try-auth
   - `log`: task-done, try-pushed, waiting
   - *(none)*: progress, build-started, build-done, test-started, test-passed

Deliverables:
- `internal/cmd/tasks.go`: `runEvent(args []string) error`
- `internal/cmd/root.go`: register `event` subcommand
- `internal/cmd/cmd_test.go`: `TestEventCommand`, `TestEventRejectsUnknownAgent`,
  `TestEventMatrixLevels`

### Phase 2 — Binary: implement `taskboard detect`

New subcommand: `taskboard detect`

Prints the detected project name using `project.Detect("")`. One-liner output, no
side effects.

```bash
$ taskboard detect
taskboard
```

Deliverables:
- `internal/cmd/root.go`: register `detect` subcommand calling `project.Detect`
- `internal/cmd/cmd_test.go`: `TestDetectCommand`

### Phase 3 — Skill: six targeted edits to `skill.md`

**Edit 1 — Add "Task Lifecycle" section** (content from Phase 0 above)
Insert after "TUI Dashboard" section, before "Team Registry Format".

**Edit 2 — Fix team.json schema example**
Add `"tasks"` as the first top-level key with a representative entry:
```json
"tasks": {
  "2025475": {
    "summary": "UAF in RemoteCDMChild when keys cleared after object destruction",
    "status": "running",
    "note": "Building (mach build)",
    "worktree": "/Users/you/firefox-2025475"
  }
}
```

**Edit 3 — Fix initialization (Step 0)**
Replace the hand-written shell detection script with:
```bash
PROJECT=$(taskboard detect)
```
Update the detection order description to match the binary's actual order (add
Zellij and single-project scan).

**Edit 4 — Fix manager: create task at investigation spawn**
In the "Investigate a bug" section, rewrite the steps:
```
1. Check team.json — skip if already running/waiting
2. Write investigation_agents.X entry to team.json (BEFORE spawning)
3. set-task X --summary "Bug X — investigating" --status running
4. log manager "Investigating bug X"
5. Spawn inv-X in background
6. log manager "Spawned inv-X"
```
Writing the team.json entry before spawning fixes the btw registration race (gap 2.10).

**Edit 5 — Fix investigation agent prompt**
- Replace `event progress` with `log inv-X "Root cause: …"`
- Replace `event waiting` with `event waiting inv-X "Awaiting approval"` (event now
  implemented)
- Change `set-task` call to only update (not create):
  ```bash
  taskboard set-task XXXXXX \
    --summary "<final one-liner>" \
    --status waiting \
    --note "Awaiting approval — [Investigation](${INV_URL})"
  ```
- Remove the "Old positional form also still works" comment

**Edit 6 — Fix build agent prompt**

Six sub-changes:

a) After `claim-task` returns `{"claimed": true}`, add:
   ```bash
   taskboard set-task {bug_id} --status running
   ```

b) After worktree creation, add:
   ```bash
   taskboard set-task {bug_id} --worktree ~/firefox-{bug_id}
   ```

c) Replace MANDATORY note update rule with flag form:
   ```bash
   taskboard set-task {bug_id} --note "Building (mach build)"
   taskboard set-task {bug_id} --status waiting --note "Build failed: <error>"
   ```
   Remove the positional form entirely.

d) Replace all `event` calls with the now-implemented subcommand (no changes to
   intent, just confirming syntax works).

e) Remove the duplicate BTW paragraph — keep only the MANDATORY block, update it
   to match the canonical decision table from Phase 0.

f) In "When fully done": confirm `done-task {bug_id}` is called (already correct),
   confirm team.json is cleared, confirm build lock is removed.

---

## 5. Execution Order

1. **Phase 0** — write this document, get alignment
2. **Phase 1** — implement `event` in binary (with tests)
3. **Phase 2** — implement `detect` in binary (trivial)
4. **Phase 3** — rewrite `skill.md` with all six edits

Phases 1 and 2 can be done in parallel. Phase 3 depends on both (the skill
references the new subcommands).

---

## 6. Out of Scope

- Changing the TUI rendering (the overlay already shows the right fields once
  the data is populated correctly)
- Changing `team.json` schema structure
- Adding new agent types
- Matrix integration beyond what `event` already handles via `matrix-cli`
