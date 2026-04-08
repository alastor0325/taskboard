# Taskboard Skill Redesign Investigation

**Document scope:** Full audit of `/taskboard` skill (`~/.claude/skills/taskboard/skill.md`),
covering gaps found, business logic definition, a concrete improvement plan, and a
full three-agent review.

---

## 1. Current State Summary

The `/taskboard` skill defines three actors:

- **Manager session** — coordinator. Reads `team.json`, spawns agents, routes messages.
- **Investigation agent** — researches a bug, writes an investigation file, waits for approval.
- **Build agent** — applies patches, builds, tests, submits to Phabricator.

State is shared via two files:
- `~/.taskboard/{PROJECT}/team.json` — source of truth for agents and tasks
- `~/.taskboard/{PROJECT}/agent-status.json` — TUI feed (written by `taskboard sync`)

The TUI displays task cards with: status badge, bug ID (hyperlinked), summary,
secondary row (note / worktree / BTW), and a detail overlay (agents, files, BTW, links).

---

## 2. Initial Audit — Gaps Found

### 2.1 Critical — `taskboard event` doesn't exist

Both agent prompts use `taskboard event <type> <agent> "<message>"` extensively.
This subcommand does not exist. Every structured milestone call silently fails.

### 2.2 Critical — `set-task` positional form doesn't exist

The build agent MANDATORY rule uses `taskboard set-task {id} "{summary}" running "{note}"`.
The binary only accepts flags. Every phase-change note update fails.

### 2.3 High — Task card never exists during investigation

The investigation agent calls `set-task` only at the very end. The TUI shows
nothing for an active bug during the entire investigation.

### 2.4 High — `claim-task` doesn't update task status

`ClaimTask` only writes to `investigation_agents`. It never touches `tasks`. After
`claim-task`, the card still shows `waiting`.

### 2.5 Medium — `--worktree` never populated in task card

The build agent creates a git worktree but never calls `set-task --worktree`.

### 2.6 Medium — Project detection steps are stale

Step 0 shell script misses Zellij and the single-project `~/.taskboard/` scan.

### 2.7 Medium — `tasks` key missing from team.json schema example

The schema shows four keys but omits `tasks`, which is what the TUI reads.

### 2.8 Medium — No authoritative task lifecycle documentation

No section defines when a task is created, who owns each field, and who updates
what at each phase.

### 2.9 Low — BTW block duplicated in build agent prompt

### 2.10 Low — BTW registration race on agent spawn

Manager writes team.json after spawning; agent's first `btw` can fail.

---

## 3. Business Logic Definition

### 3.1 What a task represents

One task per bug. Created by the manager at investigation spawn. Never created by
an investigation or build agent. Never created twice.

### 3.2 The two agent slots

```
task {bug_id}
  ├── investigation slot → investigation_agents["{bug_id}"]
  └── build slot         → build_agents["{type}"].current_bug == {bug_id}
```

- **Investigation slot**: filled before spawning `inv-{id}`. Status: `running` → `waiting` → `approved`.
- **Build slot**: filled when build agent calls `claim-task`. Cleared when fully done.
- Both slots can be non-empty simultaneously.
- Task agents and utility agents are not linked to a specific task card.

### 3.3 Task field update rules

| Field | Owner | When it changes |
|---|---|---|
| `--summary` | Manager (placeholder) then inv agent (final) | Twice: at spawn, at investigation completion |
| `--status` | Manager and build agent | Only at meaningful phase transitions |
| `--note` | Whoever owns the current phase | On every phase change; always explains why when `waiting` |
| `--worktree` | Build agent | Once, immediately after `git worktree add` |
| `done-task` | Build agent | Once, when fully complete — never use `--status done` |

### 3.4 Complete task lifecycle state machine

| Phase | Who | status | note |
|---|---|---|---|
| Manager spawns investigation | Manager | `running` | `"Starting investigation"` |
| Inv: root cause found | Inv agent | *(no change)* | `"Root cause: <one line>"` |
| Inv: file written | Inv agent | `waiting` | `"Awaiting approval — [Investigation](url)"` |
| User rejects investigation | Manager | `running` | `"Investigation rejected — revising"` |
| User cancels bug | Manager | — | `done-task` |
| User approves, manager dispatches | Manager | `running` | `"Dispatched to agent-{type}"` |
| Build: claim-task succeeds | Build agent | `running` | *(no change)* |
| Build: worktree created | Build agent | *(no change)* | *(note unchanged; worktree field set)* |
| Build: applying patch | Build agent | *(no change)* | `"Applying patch"` |
| Build: building | Build agent | *(no change)* | `"Building (mach build)"` |
| Build: build failed | Build agent | `waiting` | `"Build failed: <short error>"` |
| Manager approves retry | Manager | `running` | `"Retrying build"` |
| Build: running tests | Build agent | *(no change)* | `"Running mochitest-media"` |
| Build: tests failed | Build agent | `waiting` | `"Tests failed: <test name>"` |
| Manager approves test retry | Manager | `running` | `"Fixing test failure"` |
| Build: plan ready | Build agent | `waiting` | `"Plan ready — awaiting approval"` |
| Build: try push submitted | Build agent | *(no change)* | `"[Try N](url)"` |
| File conflict blocking dispatch | Manager | `waiting` | `"Waiting — file conflict with agent-{x}"` |
| Conflict resolved, re-dispatched | Manager | `running` | `"Dispatched to agent-{type}"` |
| Re-queued after review feedback | Manager | `running` | `"Re-queued after review feedback"` |
| Lock contention on re-queue | Build agent | `waiting` | `"Waiting for build lock (held by bug {x})"` |
| Build fully complete | Build agent | — | `done-task {id}` |

### 3.5 Log vs BTW vs event — when to use each

**`taskboard log`** — free-form narrative, always persisted, no Matrix, safe to call frequently.

**`taskboard btw`** — volatile intention heartbeat (120s TTL). Rules:
- Send immediately before any multi-step scope starts
- Send immediately before EVERY build or test command (even if it's the only command)
- Refresh every ~60s while waiting for a long-running command
- Never rely on "it's one command" to skip refreshes — `./mach build` takes 8–15 min

**`taskboard event`** — structured milestone, always logged, conditionally notifies Matrix:

| Type | When | Matrix |
|---|---|---|
| `progress` | Root cause or notable milestone | none |
| `waiting` | Awaiting user input | none |
| `build-started` | `./mach build` launched | none |
| `build-done` | Build succeeded | none |
| `build-failed` | Build failed | **alert** |
| `test-started` | Test run launched | none |
| `test-passed` | All tests passed | none |
| `test-failed` | Tests failed | **alert** |
| `try-pushed` | Try push submitted | log |
| `try-auth` | Try push needs auth | **alert** |
| `task-done` | Bug fully complete | log |

**`taskboard sync`** — required explicitly after any direct `team.json` write.
`set-task` and `done-task` call sync internally. Direct writes (agent registration
entries in `investigation_agents`, `build_agents`) do NOT auto-sync — manager
must call `taskboard sync` explicitly after writing these.

---

## 4. Improvement Plan

### Phase 0 — Add "Task Lifecycle" section to skill.md

Insert section 3 content (slots, field rules, state machine, log/btw/event table)
into `skill.md` after "TUI Dashboard", before "Team Registry Format". This becomes
the authoritative reference all three agent prompts cite.

### Phase 1 — Binary: implement `taskboard event`

```
taskboard event <type> <agent> "<message>"
```

1. Validate agent is registered (same check as `btw`) — unregistered → error
2. `appendLog` always
3. `syncStatus` always
4. `matrix-cli notify {level}` conditionally per type table in 3.5

Deliverables:
- `internal/cmd/tasks.go`: `runEvent`
- `internal/cmd/root.go`: register subcommand
- Tests: `TestEventCommand`, `TestEventRejectsUnknownAgent`, `TestEventMatrixAlert`,
  `TestEventMatrixLog`, `TestEventNoMatrix`

### Phase 2 — Binary: `taskboard detect` + `syncAgentStatus` fix

**`taskboard detect`**: prints `project.Detect("")`. One-liner.

**`syncAgentStatus` fix**: currently only updates `InvestigationAgents` and
`TaskAgents`. Add scan of `BuildAgents` where `current_bug == bugID` and sync
its `Status` field.

Deliverables:
- `internal/cmd/root.go`: `detect` subcommand
- `internal/store/store.go`: fix `syncAgentStatus`
- Tests: `TestDetectCommand`, `TestSyncAgentStatusUpdatesBuildAgent`

### Phase 3 — Skill: seven targeted edits

**Edit 1 — Add "Task Lifecycle" section** (Phase 0 content)

**Edit 2 — Fix team.json schema**: add `"tasks"` as first key with full example entry.

**Edit 3 — Fix initialization Step 0**: replace shell script with `PROJECT=$(taskboard detect)`.

**Edit 4 — Fix manager investigation spawn**:
```
1. Check team.json — skip if already running/waiting
2. Write investigation_agents.X to team.json (BEFORE spawning)
3. taskboard sync
4. taskboard set-task X --summary "Bug X — investigating" --status running --note "Starting investigation"
5. taskboard log manager "Investigating bug X"
6. Spawn inv-X in background
7. taskboard log manager "Spawned inv-X"
```
Writing team.json and calling sync before spawning fixes the registration race for
investigation agents. Also prevents the blank secondary row (C4) by setting an
initial note.

**Edit 5 — Fix investigation agent prompt**:
- Remove "Old positional form also still works" comment
- Add intermediate note update on root cause:
  `taskboard set-task X --note "Root cause: <one line>"`
- Change final set-task to update only (manager already created the card):
  ```bash
  taskboard set-task XXXXXX --summary "<final one-liner>" --status waiting \
    --note "Awaiting approval — [Investigation](${INV_URL})"
  taskboard event waiting inv-XXXXXX "Investigation complete, awaiting approval"
  ```
- Add resumption protocol: if investigation file `~/firefox-bug-investigation/bug-X-investigation.md`
  already exists when agent starts, skip to step 4 (write file, update team.json).
  Do not restart from step 1.

**Edit 6 — Fix build agent prompt**:

a) After `claim-task` returns `{"claimed": true}`:
   ```bash
   taskboard set-task {bug_id} --status running
   ```

b) After `git worktree add`:
   ```bash
   taskboard set-task {bug_id} --worktree ~/firefox-{bug_id}
   ```

c) Replace ALL positional `set-task` calls with flag form.

d) Add retry recovery: after build or test failure, once manager sends "retry",
   agent calls `taskboard set-task {bug_id} --status running --note "Retrying"`,
   then resumes from the last successful checkpoint (re-apply patch only if worktree
   is clean; re-run tests only if build is current).

e) Add build lock contention waiting: if another bug holds the lock on re-queue,
   set note and poll:
   ```bash
   taskboard set-task {bug_id} --status waiting --note "Waiting for build lock (held by bug {x})"
   # poll every 60s until lock is released
   taskboard set-task {bug_id} --status running --note "Lock acquired, resuming"
   ```

f) Add worktree cleanup to "When fully done":
   ```bash
   git -C ~/firefox-{bug_id} worktree remove ~/firefox-{bug_id} --force
   ```

g) Consolidate duplicate BTW block into one canonical paragraph citing the decision
   table. Add explicit rule: send `btw` before launching `./mach build` regardless
   of whether it's the first or tenth build command.

h) Add note: `set-task` and `done-task` call sync internally. Direct team.json
   writes (agent registration) require explicit `taskboard sync` afterwards.

i) Add losing-race behavior: if `claim-task` returns `{"claimed": false}`,
   do NOT proceed. SendMessage to manager: "Lost claim on {bug_id} to {owner}."
   Then check queue for next available bug or go idle.

**Edit 7 — Add missing manager user commands**:

Add "Reject a bug" command:
```
Trigger: "reject X", "redo X", "investigation wrong for X"
1. Send "Investigation rejected. Revise based on: {feedback}" to inv-X via SendMessage
2. taskboard set-task X --status running --note "Investigation rejected — revising"
3. taskboard sync
```

Add file conflict dispatch update:
```bash
taskboard set-task X --status waiting --note "Waiting — file conflict with agent-{x}"
taskboard sync
```

Add re-queue after review feedback:
```bash
taskboard set-task X --status running --note "Re-queued after review feedback"
taskboard sync
# write build_agents entry to team.json BEFORE sending message to build agent
taskboard sync
```

For build agent dispatch in Dispatch Logic: write `build_agents[type]` entry to
team.json and call sync BEFORE spawning the build agent (fixes btw registration
race for build agents, same fix as Edit 4 for investigation agents).

---

## 5. Three-Agent Review — Unified Findings

Three agents reviewed independently: **Tech** (binary accuracy), **Business**
(workflow completeness), **Integration** (TUI end-to-end correctness). Findings
are deduplicated and cross-referenced.

### 5.1 Complete findings table

| ID | Severity | Finding | Source | Plan coverage |
|---|---|---|---|---|
| F01 | **Critical** | `taskboard event` genuinely absent — all event logging broken | Tech | Phase 1 ✓ |
| F02 | **Critical** | `set-task` positional form absent — build agent note updates fail | Tech | Edit 6c ✓ |
| F03 | **Critical** | `claim-task` never touches `tasks` — card stays `waiting` after claim | Tech + Integration | Edit 6a ✓ |
| F04 | **Critical** | Secondary row blank at spawn (no note, no worktree, no btw yet) | Integration | Edit 4 (initial note) ✓ |
| F05 | **Critical** | Boilerplate summary `"Bug X — investigating"` shows on card for entire investigation | Integration | **Not fixed** — see below |
| F06 | **Critical** | No rejection pathway in skill or state machine | Business | Edit 7 ✓ |
| F07 | **Critical** | Worktree cleanup has no owner — `~/firefox-{id}` never removed | Business | Edit 6f ✓ |
| F08 | **High** | `syncAgentStatus` misses `BuildAgents` — build agent status silently stale | Tech | Phase 2 ✓ |
| F09 | **High** | BTW cadence "every ~60s" misleading — `./mach build` is one command lasting 15 min | Business + Integration | Edit 6g ✓ |
| F10 | **High** | Direct `team.json` writes don't auto-sync — manager must call sync explicitly | Integration | Edit 6h + Edit 7 ✓ |
| F11 | **High** | Build agent btw registration race not fixed for build agent spawn (Edit 4 only covers inv agent) | Integration | Edit 7 ✓ |
| F12 | **High** | Multiple build failure retry flow undefined — what does agent redo vs skip? | Business | Edit 6d ✓ |
| F13 | **High** | Build lock contention on re-queue unmapped — what does agent do while waiting? | Business | Edit 6e ✓ |
| F14 | **High** | Losing-race build agent behavior undefined — agent spawned but claim lost | Business | Edit 6i ✓ |
| F15 | **High** | Session restart mid-investigation: inv agent always restarts from step 1 | Business | Edit 5 (resumption) ✓ |
| F16 | **Medium** | `idle` transition valid in binary but absent from lifecycle — could be accidentally used | Tech | Phase 0 (state machine) ✓ |
| F17 | **Medium** | File conflict `pending_dispatch` state not in state machine | Business | State machine table ✓ |
| F18 | **Medium** | Task agents and utility agents have no lifecycle definition | Business | **Not fixed** — out of scope |
| F19 | **Medium** | `tasks` key missing from team.json schema example | Tech + Integration | Edit 2 ✓ |
| F20 | **Medium** | Agent slot documentation missing from skill | Integration | Edit 1 ✓ |
| F21 | **Medium** | Project detection shell script misses Zellij + single-project scan | Tech | Edit 3 ✓ |
| F22 | **Medium** | `--note` clearing rule unclear — replaced or appended? | Business | Field rules table ✓ |
| F23 | **Low** | `progress` event type definition vague — no concrete examples | Business | Event table examples ✓ |
| F24 | **Low** | Build type not stored in `tasks` — overlay depends on inv slot being populated | Business | **Not fixed** — acceptable |
| F25 | **Low** | BTW block duplicated in build agent prompt | Initial audit | Edit 6g ✓ |

### 5.2 F05 — Boilerplate summary during investigation (unresolved)

**Finding:** Under the new plan, the manager creates the task card immediately with
`--summary "Bug X — investigating"`. This boilerplate stays on the card for the
entire investigation — potentially hours. The user watches useless placeholder text.

**Why it's hard to fix:** The real summary (a precise one-liner about root cause and
affected component) is only known after the investigation completes. There is no
intermediate value that is both accurate and useful.

**Options:**

| Option | Tradeoff |
|---|---|
| Keep boilerplate | Simple. Card exists. User sees something. Summary is wrong until inv completes. |
| Show bug title from Bugzilla | Accurate but requires a `bugzilla-cli` fetch at spawn time. Adds latency. |
| Investigation agent updates summary mid-investigation | More accurate over time. Requires inv agent to call `set-task --summary` when root cause known, not just the note. |

**Recommendation:** Option 3. Add to Edit 5: when root cause is identified, the
investigation agent calls both `--note` and `--summary` updates:
```bash
taskboard set-task XXXXXX \
  --summary "<one-line root cause>" \
  --note "Root cause identified — writing investigation file"
```
This way the card shows a meaningful summary from the moment the root cause is
known, not just at the end.

### 5.3 F18 — Task agents and utility agents (explicitly out of scope)

Task agents (e.g., "write test coverage for bug X") and utility agents (e.g.,
"audit TUI test suite") exist in `team.json` and are health-checked at init. They
are not linked to a specific task card and do not update task fields. Their lifecycle
— spawn triggers, completion handling, re-spawn on crash — is a separate concern
from this redesign and should be a separate document.

### 5.4 F24 — Build type not in `tasks` (acceptable)

Build type lives in `investigation_agents[bugID].build_type`. The TUI overlay reads
this via `invByBug`. As long as the investigation slot is populated (which it always
is once the manager creates it), the overlay shows the correct build type. No fix needed.

### 5.5 Plan coverage summary

| Phase | Addresses | Misses |
|---|---|---|
| Phase 0 (lifecycle doc) | State machine, field rules, log/btw/event table | — |
| Phase 1 (event binary) | F01 | — |
| Phase 2 (detect + syncAgentStatus) | F08, F21 | — |
| Edit 1 (lifecycle section) | F20 | — |
| Edit 2 (schema fix) | F19 | — |
| Edit 3 (detection) | F21 | — |
| Edit 4 (manager spawn) | F03 partial, F04, F11 partial | F05 (boilerplate summary) |
| Edit 5 (inv agent) | F02 partial, F15 | F05 unless option 3 added |
| Edit 6 (build agent) | F02, F03, F05 partial, F09, F10, F12, F13, F14, F25 | — |
| Edit 7 (manager commands) | F06, F10, F11 | — |
| Not addressed | F18 (task/utility agents — out of scope), F24 (build type — acceptable) | |

---

## 6. End-to-End TUI Trace (Under New Plan)

Step-by-step TUI state a user sees from "investigate bug 2026875" to "task done":

| Step | Actor | Action | Card status | Secondary row | Overlay |
|---|---|---|---|---|---|
| 1 | Manager | Creates task card | `running` | `"Starting investigation"` | inv slot: `running` |
| 2 | Manager | Spawns inv-2026875 | *(no change)* | *(no change)* | *(no change)* |
| 3 | Inv agent | First `btw` | *(no change)* | BTW: `"Reading bug report"` | *(no change)* |
| 4 | Inv agent | Traces code, sends periodic btw | *(no change)* | BTW updates every 60s | *(no change)* |
| 5 | Inv agent | Root cause found — updates summary + note | `running` | `"Root cause: keys cleared after CDM destruction"` | *(no change)* |
| 6 | Inv agent | Writes file, sets `waiting` | `waiting` | `"Awaiting approval — [Investigation](url)"` | inv: `waiting` |
| 7 | Manager | User approves, writes build_agents, syncs | `running` | `"Dispatched to agent-asan"` | build slot: appears |
| 8 | Manager | Spawns build agent | *(no change)* | *(no change)* | *(no change)* |
| 9 | Build agent | First `btw`, then `claim-task` | `running` | BTW: `"Reading investigation file"` | build: `busy` |
| 10 | Build agent | `set-task --worktree ~/firefox-2026875` | *(no change)* | `~/firefox-2026875` (worktree beats btw) | worktree shown |
| 11 | Build agent | `set-task --note "Applying patch"` | *(no change)* | `"Applying patch"` | *(no change)* |
| 12 | Build agent | `btw` before mach build, `set-task --note "Building"` | *(no change)* | `"Building (mach build)"` | *(no change)* |
| 13 | Build agent | `./mach build` runs 10 min, btw every 60s | *(no change)* | note stable, BTW updates | *(no change)* |
| 14 | Build agent | Build done, `event build-done`, `set-task --note "Running tests"` | *(no change)* | `"Running mochitest-media"` | *(no change)* |
| 15 | Build agent | Tests pass, plan ready, `set-task --status waiting` | `waiting` | `"Plan ready — awaiting approval"` | *(no change)* |
| 16 | Manager | User approves plan | `running` (manager sets) | `"Retrying build"` or continues | *(no change)* |
| 17 | Build agent | `moz-phab submit`, `event try-pushed`, `set-task --note "[Try 1](url)"` | *(no change)* | `"[Try 1](url)"` | *(no change)* |
| 18 | Build agent | Waits 60 min for CI, btw every 60s | *(no change)* | note stable | *(no change)* |
| 19 | Build agent | CI green, `done-task 2026875` | `done` | `"removing in 300s"` countdown | — |
| 20 | Watcher | 5 min TTL expires, removes task | *(card disappears)* | — | — |

**Remaining blank moments:** Step 3 (1–2s gap between spawn and first btw). Acceptable.
**No other blank moments** under the new plan when all edits are applied.

---

## 7. Execution Order

1. **Phase 1** — implement `event` in binary
2. **Phase 2** — implement `detect`, fix `syncAgentStatus`
3. **Phase 3** — rewrite `skill.md` (all 7 edits)

Phases 1 and 2 in parallel. Phase 3 depends on both (skill references new subcommands).

---

## 8. Out of Scope

- TUI rendering changes
- `team.json` schema structure changes
- New agent types
- Task agent and utility agent lifecycle (separate document)
- Build type storage in `tasks` (F24 — not needed)
