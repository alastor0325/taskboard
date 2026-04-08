# Taskboard Skill Redesign Investigation

**Document scope:** Full audit of `/taskboard` skill (`~/.claude/skills/taskboard/skill.md`),
covering gaps found, business logic definition, and a concrete improvement plan.

**Review method:** Three independent agents (technical, business logic, TUI integration)
reviewed this document against the actual binary source and current skill. Their findings
are consolidated in section 5 (Final Review) and incorporated into the plan in section 4.

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

## 2. Gaps Found (Initial Audit)

### 2.1 Critical — `taskboard event` doesn't exist

Both agent prompts use `taskboard event <type> <agent> "<message>"` extensively:

```bash
taskboard event progress inv-2026875 "Root cause identified"
taskboard event build-started agent-debug "Starting mach build"
taskboard event try-pushed agent-asan "[Try 1](url)"
```

This subcommand does not exist in the binary. Every structured milestone call
silently fails. All key-moment logging is broken.

### 2.2 Critical — `set-task` positional form doesn't exist

The build agent MANDATORY rule and several other calls use:

```bash
taskboard set-task {bug_id} "{summary}" running "{new note}" {worktree}
```

The binary only accepts flags: `--summary`, `--status`, `--note`, `--worktree`.
Positional arguments are rejected. Every phase-change note update from the build
agent will fail.

### 2.3 High — Task card never exists during investigation

The investigation agent calls `set-task` only at the very end. During the entire
investigation the task card does not exist in `team.json`, so the TUI shows nothing
for an active bug.

### 2.4 High — `claim-task` doesn't update task status

The skill states "`claim-task` transitions the task to `running` atomically." The
actual `ClaimTask` implementation only writes to `investigation_agents`. It never
touches `tasks`. After `claim-task`, the card still shows `waiting`.

### 2.5 Medium — `--worktree` never populated in task card

The build agent creates a git worktree but never calls `set-task --worktree`. The
TUI card's secondary row and review-server link stay blank for the entire build.

### 2.6 Medium — Project detection steps are stale

Step 0 contains a hand-written shell script that misses Zellij and the single-project
`~/.taskboard/` scan. The binary's `project.Detect` handles both.

### 2.7 Medium — `tasks` key missing from team.json schema example

The schema example shows four top-level keys but omits `tasks` — the key that
`set-task`, `done-task`, and the TUI all read.

### 2.8 Medium — No authoritative task lifecycle documentation

The skill has no section defining: when a task is created, who creates it, what
fields are set at each phase, and who is responsible for each update.

### 2.9 Low — BTW block duplicated in build agent prompt

The BTW timing rule appears twice with slightly different wording.

### 2.10 Low — BTW registration race on build agent spawn

The manager currently writes the team.json entry after spawning the agent, so the
agent's first `btw` call can fail with "unknown agent".

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
  entry before spawning `inv-{id}`. Status moves from `running` → `waiting` → `approved`.
- **Build slot** is filled when the build agent calls `claim-task`. Only one build
  agent can hold a bug at a time. It is cleared (current_bug → null) when fully done.
- Both slots can be non-empty simultaneously.
- Task agents and utility agents are not linked to a specific bug task and do not
  appear in the task card overlay.

### 3.3 Task field update rules

**`--summary`**
- Set at spawn by manager: `"Bug {id} — investigating"`
- Updated once by investigation agent when root cause is known: final one-liner
- Never changed by the build agent after that

**`--status`**
- Changes only at meaningful phase transitions
- Valid transitions enforced by the binary: `running` → `waiting` → `running` → done
- `idle` is a valid binary transition (running→idle) but is not used in this workflow

**`--note`**
- Changes whenever the work phase changes or the user needs to act
- Always set a note when status goes to `waiting` — it must explain why
- Always replaced (not appended) when changing phase
- Cleared implicitly when `done-task` is called (task disappears)

**`--worktree`**
- Set once by the build agent immediately after `git worktree add`
- Never changed after that
- Worktree directory cleanup (`git worktree remove`) is the build agent's responsibility
  at task completion, but the `--worktree` field in the card is not explicitly cleared
  (the task is done-tasked and removed by the watcher)

**`done-task`**
- The only way to end a task — sets status to `done` and records `done_at`
- Never use `set-task --status done`

### 3.4 Complete task lifecycle state machine

| Phase | Who | status | summary | note | worktree |
|---|---|---|---|---|---|
| Manager spawns investigation | Manager | `running` | `"Bug X — investigating"` | *(empty)* | *(empty)* |
| Inv: root cause found | Inv agent | *(no change)* | *(no change)* | `"Root cause: <one line>"` | *(empty)* |
| Inv: file written, awaiting approval | Inv agent | `waiting` | `"<final one-liner>"` | `"Awaiting approval — [Investigation](url)"` | *(empty)* |
| User rejects investigation | Manager | `running` | *(no change)* | `"Investigation rejected — revising"` | *(empty)* |
| User cancels bug entirely | Manager | — | — | — | `done-task {id}` (or delete from team.json if pre-task) |
| Manager: user approved, dispatching | Manager | `running` | *(no change)* | `"Dispatched to agent-{type}"` | *(empty)* |
| Build agent claims bug (after claim-task) | Build agent | `running` | *(no change)* | *(no change)* | *(empty)* |
| Build: worktree created | Build agent | *(no change)* | *(no change)* | *(no change)* | `~/firefox-{id}` |
| Build: applying patch | Build agent | *(no change)* | *(no change)* | `"Applying patch"` | *(no change)* |
| Build: building | Build agent | *(no change)* | *(no change)* | `"Building (mach build)"` | *(no change)* |
| Build: build failed | Build agent | `waiting` | *(no change)* | `"Build failed: <short error>"` | *(no change)* |
| Manager approves retry after build failure | Manager | `running` | *(no change)* | `"Retrying build"` | *(no change)* |
| Build: running tests | Build agent | *(no change)* | *(no change)* | `"Running mochitest-media"` | *(no change)* |
| Build: tests failed | Build agent | `waiting` | *(no change)* | `"Tests failed: <test name>"` | *(no change)* |
| Manager approves retry after test failure | Manager | `running` | *(no change)* | `"Fixing test failure"` | *(no change)* |
| Build: plan ready | Build agent | `waiting` | *(no change)* | `"Plan ready — awaiting approval"` | *(no change)* |
| Build: try push submitted | Build agent | *(no change)* | *(no change)* | `"[Try N](url)"` | *(no change)* |
| Build: re-queued after review feedback | Manager | `running` | *(no change)* | `"Re-queued after review feedback"` | *(no change)* |
| Build: file conflict, pending dispatch | Manager | `waiting` | *(no change)* | `"Waiting — file conflict with agent-{x}"` | *(empty)* |
| Build: work fully complete | Build agent | — | — | — | `done-task {id}` |

### 3.5 Log vs BTW vs event — when to use each

**`taskboard log <agent> "<message>"`**
- Free-form narrative. Always persisted. Use for any step worth recording.
- Does not notify Matrix. Safe to call frequently.
- Examples: "Reading bug report", "Applying part 1 of 3", "All tests passed locally"

**`taskboard btw <agent> "<current intention>"`**
- Volatile heartbeat. Disappears after 120s. Shown in the TUI BTW bar.
- Declare intention at the START of any multi-step scope.
- Refresh every ~60s OR immediately before each build/test command — whichever is sooner.
- A single `./mach build` takes 8–15 min: send `btw` immediately before launching it,
  then again every 60s while waiting (via separate polling). Never rely on "it's one
  command" to skip heartbeats.
- Never use for facts — only for current intention.

**`taskboard event <type> <agent> "<message>"` (to be implemented)**
- Structured milestone. Always logged. Routes to Matrix based on type.
- Use only for key moments.

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

Add a "Task Lifecycle" section to `skill.md` containing the two-slot agent model,
the five field rules, the full state machine table, and the log/btw/event decision
table from section 3 above. This section becomes the authoritative reference. All
three agent prompts cite it rather than duplicating rules inline.

### Phase 1 — Binary: implement `taskboard event`

New subcommand: `taskboard event <type> <agent> "<message>"`

Behavior:
1. Validate agent is registered (same check as `btw`) — unregistered → error
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
  `TestEventMatrixAlert`, `TestEventMatrixLog`, `TestEventNoMatrix`

### Phase 2 — Binary: implement `taskboard detect` + fix `syncAgentStatus`

**`taskboard detect`:** prints `project.Detect("")`. One-liner, no side effects.
Lets the skill's Step 0 become `PROJECT=$(taskboard detect)`.

**Fix `syncAgentStatus`:** Currently only updates `InvestigationAgents` and
`TaskAgents` when task status changes. It silently misses `BuildAgents`. When
`set-task --status running` is called after `claim-task`, the build agent's entry
in `build_agents[type]` is never updated. Fix: add a scan of `BuildAgents` where
`current_bug == bugID` and update its `Status` field to match.

Deliverables:
- `internal/cmd/root.go`: register `detect` subcommand
- `internal/store/store.go`: fix `syncAgentStatus` to also scan `BuildAgents`
- `internal/cmd/cmd_test.go`: `TestDetectCommand`
- `internal/store/store_test.go`: `TestSyncAgentStatusUpdatesBuildAgent`

### Phase 3 — Skill: seven targeted edits to `skill.md`

**Edit 1 — Add "Task Lifecycle" section** (content from section 3 above)
Insert after "TUI Dashboard", before "Team Registry Format".

**Edit 2 — Fix team.json schema example**
Add `"tasks"` as the first top-level key with a representative entry showing all
fields: `summary`, `status`, `note`, `worktree`, `done_at`.

**Edit 3 — Fix initialization (Step 0)**
Replace the hand-written shell script with `PROJECT=$(taskboard detect)`.
Update the detection order to: TASKBOARD_PROJECT → tmux → Zellij → single-project
scan → CWD `firefox-` prefix → random fallback.

**Edit 4 — Fix manager: create task at investigation spawn + fix registration race**
In "Investigate a bug", rewrite steps:
```
1. Check team.json — skip if already running/waiting
2. Write investigation_agents.X entry to team.json (BEFORE spawning — fixes btw race)
3. set-task X --summary "Bug X — investigating" --status running
4. log manager "Investigating bug X"
5. Spawn inv-X in background
6. log manager "Spawned inv-X"
```
Same pattern for build agent dispatch: write `build_agents` entry to team.json
BEFORE spawning the build agent. This fixes the btw registration race for both
agent types.

**Edit 5 — Fix investigation agent prompt**
- Remove "Old positional form also still works" comment
- Replace `event progress` with `log inv-X "Root cause: …"`
- Keep `event waiting` (now implemented in Phase 1)
- Change `set-task` to update only (manager already created the card):
  ```bash
  taskboard set-task XXXXXX \
    --summary "<final one-liner>" \
    --status waiting \
    --note "Awaiting approval — [Investigation](${INV_URL})"
  ```
- Add intermediate note update when root cause is found:
  ```bash
  taskboard set-task XXXXXX --note "Root cause: <one line>"
  ```

**Edit 6 — Fix build agent prompt**
Six sub-changes:

a) After `claim-task` returns `{"claimed": true}`, explicitly call:
   ```bash
   taskboard set-task {bug_id} --status running
   ```

b) After worktree creation, immediately call:
   ```bash
   taskboard set-task {bug_id} --worktree ~/firefox-{bug_id}
   ```

c) Replace ALL positional `set-task` calls with flag form:
   ```bash
   taskboard set-task {bug_id} --note "Building (mach build)"
   taskboard set-task {bug_id} --status waiting --note "Build failed: <error>"
   ```

d) Add manager note update when approving retry after failure:
   The build agent sets `--status waiting` on failure. On manager approval to retry,
   manager calls `set-task {bug_id} --status running --note "Retrying build"`.

e) Remove the duplicate BTW paragraph. Keep one canonical block citing the
   decision table from Phase 0.

f) Note explicitly that `taskboard set-task` and `taskboard done-task` call
   `syncStatus` internally — agents do NOT need to call `taskboard sync` separately
   after these commands.

**Edit 7 — Add rejection, conflict, and re-queue paths to manager commands**
In "Approve a bug": add a rejection branch — if user rejects, send "Revise
investigation" to the inv agent and update task:
```bash
taskboard set-task X --status running --note "Investigation rejected — revising"
```

In "Dispatch Logic": after a file conflict, update the task card:
```bash
taskboard set-task X --status waiting --note "Waiting — file conflict with agent-{x}"
```

In "Re-add a bug for rebuild": manager updates task status before re-queuing:
```bash
taskboard set-task X --status running --note "Re-queued after review feedback"
```

---

## 5. Final Review — Consolidated Findings

Three agents reviewed this document independently. New issues found (not in initial
section 2 audit) are marked ★.

### Critical

| # | Finding | Source |
|---|---|---|
| C1 | `taskboard event` genuinely absent from binary — all event logging broken | Tech |
| C2 | `set-task` positional form absent — build agent note updates fail | Tech |
| C3 | `claim-task` never touches `tasks` — card stays `waiting` after claim | Tech + Integration |
| C4 ★ | Task card secondary row blank at spawn (no note, no worktree, btw not yet sent) | Integration |

**C4 detail:** At the moment the manager creates the task card (Phase 3 Edit 4),
note is empty, worktree is empty, and the investigation agent hasn't sent its first
`btw` yet. `buildRow1` returns an empty string. The card looks broken until the
agent's first `btw` arrives. Fix: manager sets `--note "Starting investigation"` at
task creation so there is always something in the secondary row.

### High

| # | Finding | Source |
|---|---|---|
| H1 | Task lifecycle missing rejection, retry, worktree cleanup, re-queue paths | Business |
| H2 ★ | BTW cadence "every ~60s" is misleading — a single `./mach build` is one command that takes 8–15 min with no natural 60s checkpoints | Business + Integration |
| H3 ★ | `syncAgentStatus` in binary only updates `InvestigationAgents` and `TaskAgents`, never `BuildAgents` — build agent status in team.json silently out of sync | Tech |
| H4 | Build agent btw registration race NOT fixed by Edit 4 (Edit 4 fixes inv agent only; dispatch logic for build agents is a separate code path) | Integration |

**H4 detail:** Edit 4 writes the investigation_agents entry before spawning inv-X.
But the build agent is spawned from Dispatch Logic (a different section of the skill),
which also has the same race. Edit 4 must be applied to the build agent spawn path too,
or added as Edit 7 sub-item.

### Medium

| # | Finding | Source |
|---|---|---|
| M1 | `idle` status is valid in binary (`running → idle` transition allowed) but never appears in lifecycle table — could be accidentally used | Tech |
| M2 | Task agents and utility agents have no lifecycle definition — spawned separately but not integrated into task card | Business |
| M3 | File conflict `pending_dispatch` state not in state machine table | Business |
| M4 | Summary stays as boilerplate `"Bug X — investigating"` for entire investigation; first useful content only appears at investigation completion | Integration |
| M5 ★ | `sync` discipline: agents may add explicit `taskboard sync` calls after `set-task` even though it's redundant (sync is baked into set-task). Skill should state this clearly to avoid confusion | Integration |

### Low

| # | Finding | Source |
|---|---|---|
| L1 | `progress` event type definition is vague ("notable milestone") — needs concrete examples | Business |
| L2 | Build type not stored in `tasks` — it lives in `investigation_agents`, making the overlay's build_type display dependent on the investigation slot being populated | Business |

### What the plan already covers correctly

- `taskboard event` implementation (Phase 1) ✓
- `taskboard detect` implementation (Phase 2) ✓
- Task created at manager spawn not at investigation end ✓
- Explicit `set-task --status running` after `claim-task` (Edit 6a) ✓
- `set-task --worktree` after worktree creation (Edit 6b) ✓
- All positional `set-task` replaced (Edit 6c) ✓
- `tasks` key added to schema example (Edit 2) ✓
- Detection order corrected (Edit 3) ✓

### Additions to the plan from review findings

The following items are added to the plan to address findings C4, H1–H4, M3, M5:

1. **C4 fix:** Manager sets `--note "Starting investigation"` at task creation (Edit 4)
2. **H1 fix:** Rejection, retry, file conflict, and re-queue rows added to state machine table (section 3.4 above, now complete)
3. **H2 fix:** BTW cadence rule reworded: "immediately before each build/test command, then every 60s while waiting" (Edit 6e)
4. **H3 fix:** `syncAgentStatus` binary fix added to Phase 2
5. **H4 fix:** Build agent dispatch path in Dispatch Logic must also write team.json entry before spawning — covered by Edit 7 (rejection/conflict/re-queue paths now include the write-before-spawn requirement)
6. **M3 fix:** File conflict `pending_dispatch` state added to state machine table (Edit 7)
7. **M5 fix:** Edit 6f explicitly states sync is baked into set-task/done-task

---

## 6. Execution Order

1. **Phase 1** — implement `event` in binary (with tests)
2. **Phase 2** — implement `detect` + fix `syncAgentStatus` in binary (with tests)
3. **Phase 3** — rewrite `skill.md` with all seven edits

Phases 1 and 2 can be done in parallel. Phase 3 depends on both.

---

## 7. Out of Scope

- Changing the TUI rendering
- Changing `team.json` schema structure
- Adding new agent types
- Matrix integration beyond what `event` handles via `matrix-cli`
- Task agent and utility agent lifecycle (separate concern, separate document)
