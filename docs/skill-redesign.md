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

## 4. Implementation Plan

Phases 1–3 are binary changes (Go). Phase 4 is the skill rewrite. Phases 1–3 can
be done in parallel; Phase 4 depends on all three.

---

### Phase 1 — Binary: skill bundling + `taskboard init` install

**Goal:** The skill ships inside the binary. `taskboard init` always installs the
latest skill to `~/.claude/skills/taskboard/skill.md`. One repo, one `make install`,
everything in sync.

#### 1.1 Move skill into the repo

```
taskboard/
  skills/
    taskboard.md    ← canonical skill source (moved from ~/.claude/skills/taskboard/)
```

The `~/.claude/skills/` repo no longer owns this file. Installed copies are treated
as generated artifacts.

#### 1.2 Embed in binary

In `internal/cmd/skill.go` (new file):
```go
import _ "embed"

//go:embed ../../skills/taskboard.md
var skillContent []byte

func installSkill() error {
    dest := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "skill.md")
    if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
        return err
    }
    return os.WriteFile(dest, skillContent, 0o644)
}
```

#### 1.3 Call from `taskboard init`

In `runInit` (`internal/cmd/tasks.go`), add before the existing log/sync calls:
```go
if err := installSkill(); err != nil {
    return fmt.Errorf("install skill: %w", err)
}
```

`taskboard init` is called at every manager session start — the skill is always
up to date after that.

#### 1.4 Update Makefile

```makefile
install: build
    mkdir -p $(INSTALL_DIR)
    cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
    codesign --sign - $(INSTALL_DIR)/$(BINARY) 2>/dev/null || true
    $(INSTALL_DIR)/$(BINARY) init --project _install  # installs skill as side effect
```

Or simpler — add a dedicated `install-skill` target that `install` depends on:
```makefile
install-skill: build
    $(INSTALL_DIR)/$(BINARY) install-skill

install: build install-skill
    ...
```

#### 1.5 Deliverables

| File | Change |
|---|---|
| `skills/taskboard.md` | New — canonical skill source (initially a copy of current skill) |
| `internal/cmd/skill.go` | New — `installSkill()` + `runInstallSkill()` |
| `internal/cmd/root.go` | Register `install-skill` subcommand; call `installSkill()` in `runInit` |
| `internal/cmd/tasks.go` | Add `installSkill()` call in `runInit` |
| `Makefile` | Add `install-skill` target |
| `internal/cmd/cmd_test.go` | `TestInitInstallsSkill` — verifies skill file written to HOME |

---

### Phase 2 — Binary: `taskboard event` subcommand

**Goal:** Structured milestone logging with conditional Matrix routing.

#### 2.1 Event type routing table

```go
var eventLevels = map[string]string{
    "build-failed": "alert",
    "test-failed":  "alert",
    "try-auth":     "alert",
    "task-done":    "log",
    "try-pushed":   "log",
    "waiting":      "log",
    // progress, build-started, build-done, test-started, test-passed → no Matrix
}
```

#### 2.2 Implementation in `internal/cmd/tasks.go`

```go
func runEvent(args []string) error {
    proj, rest := resolveProject(args)
    if len(rest) < 3 {
        return fmt.Errorf("usage: taskboard event <type> <agent> <message>")
    }
    eventType, agentName, message := rest[0], rest[1], rest[2]

    known, err := newStore(proj).IsKnownAgent(agentName)
    if err != nil {
        return err
    }
    if !known {
        return fmt.Errorf("unknown agent %q: not registered in team.json", agentName)
    }
    if err := appendLog(logFile(proj), agentName, message); err != nil {
        return err
    }
    if level, ok := eventLevels[eventType]; ok {
        exec.Command("matrix-cli", "notify", level, message).Run()
    }
    return syncStatus(proj)
}
```

#### 2.3 Deliverables

| File | Change |
|---|---|
| `internal/cmd/tasks.go` | Add `eventLevels` map, `runEvent` function |
| `internal/cmd/root.go` | Register `event` subcommand |
| `internal/cmd/cmd_test.go` | `TestEventCommand`, `TestEventRejectsUnknownAgent`, `TestEventMatrixAlert`, `TestEventMatrixLog`, `TestEventNoMatrix` |

---

### Phase 3 — Binary: `taskboard detect` + `syncAgentStatus` fix

**Goal:** Expose project detection as a command; fix silent build agent status staleness.

#### 3.1 `taskboard detect`

In `internal/cmd/root.go`:
```go
case "detect":
    fmt.Println(project.Detect(""))
```

One line. No side effects. Replaces the shell detection script in the skill's Step 0.

#### 3.2 `syncAgentStatus` fix

Current code (`internal/store/store.go`) only updates `InvestigationAgents` and
`TaskAgents`. Add a scan of `BuildAgents`:

```go
func (s *TaskStore) syncAgentStatus(team *Team, bugID, status string) {
    if a, ok := team.InvestigationAgents[bugID]; ok {
        a.Status = status
    }
    if a, ok := team.TaskAgents[bugID]; ok {
        a.Status = status
    }
    // NEW: sync build agent whose current_bug matches
    for _, a := range team.BuildAgents {
        if a.CurrentBug != nil && fmt.Sprintf("%d", *a.CurrentBug) == bugID {
            a.Status = status
        }
    }
}
```

#### 3.3 Deliverables

| File | Change |
|---|---|
| `internal/cmd/root.go` | Register `detect` subcommand |
| `internal/store/store.go` | Fix `syncAgentStatus` to scan `BuildAgents` |
| `internal/cmd/cmd_test.go` | `TestDetectCommand` |
| `internal/store/store_test.go` | `TestSyncAgentStatusUpdatesBuildAgent` |

---

### Phase 4 — Skill rewrite (`skills/taskboard.md`)

**Goal:** Rewrite the canonical skill with all business logic fixes. Seven targeted
edits applied to the file now living at `skills/taskboard.md` in this repo.

#### Edit 1 — Add "Task Lifecycle" reference section

Insert after "TUI Dashboard", before "Team Registry Format". Content: the two-slot
agent model, five field rules, full state machine table, and log/btw/event decision
table from section 3 of this document.

This section is the canonical reference. All three agent prompts cite it by name
rather than duplicating rules inline.

#### Edit 2 — Fix team.json schema example

Add `"tasks"` as the first top-level key with a full representative entry:

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

#### Edit 3 — Fix initialization Step 0

Replace the hand-written shell detection script entirely:

```bash
PROJECT=$(taskboard detect)
```

Update the detection order description to match the binary:
`TASKBOARD_PROJECT` → tmux → Zellij → `~/.taskboard/` single-project scan →
CWD `firefox-` prefix → random fallback.

Remove the note "detection order as `status.py`" — that reference is stale.

#### Edit 4 — Fix manager: investigation spawn sequence

Replace the "Investigate a bug" steps with:

```
1. Check team.json — skip if investigation_agents.X already has status running/waiting
2. Write investigation_agents.X entry to team.json:
   { agent_id: "inv-X", status: "running", build_type: "", claimed_files: [] }
3. taskboard sync                          ← must come BEFORE spawn (fixes btw race)
4. taskboard set-task X \
     --summary "Bug X — investigating" \
     --status running \
     --note "Starting investigation"       ← prevents blank secondary row
5. taskboard log manager "Investigating bug X"
6. Spawn inv-X in background (Agent tool, run_in_background=true)
7. taskboard log manager "Spawned inv-X"
```

Same write-before-spawn rule applied to build agent dispatch in Dispatch Logic:
write `build_agents[type]` entry to team.json and call sync BEFORE spawning the
build agent.

#### Edit 5 — Fix investigation agent prompt

**Resumption protocol** (add at top of Steps section):
```
0. Check if ~/firefox-bug-investigation/bug-X-investigation.md already exists.
   If yes: skip steps 1–3, go directly to step 4 (update team.json + set-task).
   This handles session restart without redoing all research.
```

**Root cause update** (add between step 1 and final step):
```bash
# When root cause is identified:
taskboard set-task XXXXXX \
  --summary "<one-line root cause>" \
  --note "Root cause identified — writing investigation file"
taskboard event progress inv-XXXXXX "Root cause: <summary>"
```

**Final set-task** (replace current call — manager already created the card):
```bash
taskboard set-task XXXXXX \
  --summary "<confirmed final one-liner>" \
  --status waiting \
  --note "Awaiting approval — [Investigation](${INV_URL})"
taskboard event waiting inv-XXXXXX "Investigation complete, awaiting approval"
```

Remove the "Old positional form also still works" comment entirely.

#### Edit 6 — Fix build agent prompt

**6a** — After `claim-task` returns `{"claimed": true}`:
```bash
taskboard set-task {bug_id} --status running
```

**6b** — After `git worktree add ~/firefox-{bug_id}`:
```bash
taskboard set-task {bug_id} --worktree ~/firefox-{bug_id}
```

**6c** — Replace ALL positional `set-task` calls with flag form. Every occurrence of:
```bash
taskboard set-task {id} "{summary}" running "{note}" {worktree}
```
becomes:
```bash
taskboard set-task {id} --status running --note "{note}"
```
(`--summary` and `--worktree` only when those fields actually change.)

**6d** — Add retry recovery protocol:
```
On "retry" message from manager:
1. taskboard set-task {bug_id} --status running --note "Retrying"
2. If worktree is clean (no uncommitted changes): re-apply patch from investigation file
   If worktree has changes: build is current, skip to test step
3. If build artifacts exist and source unchanged: skip mach build, go straight to tests
4. Resume from earliest failed step
```

**6e** — Add build lock contention protocol:
```bash
# On re-queue when lock is held by another bug:
taskboard set-task {bug_id} --status waiting \
  --note "Waiting for build lock (held by bug {x})"
# Poll every 60s:
while [ -f "{obj_dir}/.build.lock" ]; do sleep 60; done
taskboard set-task {bug_id} --status running --note "Lock acquired, resuming"
```

**6f** — Add worktree cleanup to "When fully done":
```bash
git worktree remove ~/firefox-{bug_id} --force 2>/dev/null || true
```
(After `done-task`, before clearing team.json build slot.)

**6g** — BTW: remove duplicate paragraph. Replace both with one canonical block:
```
Send btw:
- Immediately on startup (before reading anything)
- Immediately before EVERY build or test command (even if it's the only one)
- Every 60s while waiting for a long-running command
- When switching intention

A single ./mach build takes 8–15 min. Send btw before launching it, then refresh
every 60s while it runs via a background polling loop or between polling intervals.
Never assume "it's one command" exempts you from refreshing.
```

**6h** — Add sync discipline note:
```
sync is baked into set-task and done-task — no explicit taskboard sync needed after them.
Direct team.json writes (registering agents) require explicit taskboard sync afterwards.
```

**6i** — Add losing-race behavior:
```
If claim-task returns {"claimed": false, "owner": "..."}:
- Do NOT proceed with this bug
- SendMessage to manager: "Lost claim on {bug_id} to {owner}. Standing by."
- Check build_agents[type].queue for next available bug
- If queue empty: set status = "idle", wait for SendMessage
```

#### Edit 7 — Add missing manager user commands

**Add "Reject a bug" command** (new section in User Commands):
```
Trigger: "reject X", "redo X", "investigation wrong for X", "revise X"

1. SendMessage to inv-X: "Investigation rejected. Revise based on: {user feedback}"
2. taskboard set-task X --status running --note "Investigation rejected — revising"
3. taskboard sync
4. Update investigation_agents.X.status = "running" in team.json
5. taskboard sync
```

**Update "Approve a bug"** — add file conflict and re-queue card updates:
```bash
# If file conflicts detected:
taskboard set-task X --status waiting \
  --note "Waiting — file conflict with {agent} on {files}"
taskboard sync

# When re-queuing after review feedback:
taskboard set-task X --status running --note "Re-queued after review feedback"
taskboard sync
# Write build_agents[type] entry to team.json BEFORE sending message to build agent:
taskboard sync
```

**Update Dispatch Logic** — write-before-spawn rule:
```
For all build agent spawns:
1. Write build_agents[type] entry to team.json first
2. taskboard sync
3. THEN spawn the build agent
This ensures the agent's btw and event calls succeed from the first line.
```

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

| Phase / Edit | Addresses | Notes |
|---|---|---|
| Phase 1 (skill bundling + init) | Skill version sync, install on every init | New — not in original plan |
| Phase 2 (event binary) | F01 | Prerequisite for Phase 4 |
| Phase 3 (detect + syncAgentStatus) | F08, F21 | Can run in parallel with Phase 2 |
| Phase 4 Edit 1 (lifecycle section) | F16, F17, F20, F22 | Authoritative reference |
| Phase 4 Edit 2 (schema fix) | F19 | — |
| Phase 4 Edit 3 (detection) | F21 | — |
| Phase 4 Edit 4 (manager spawn) | F04, F10, F11 | Write-before-spawn rule |
| Phase 4 Edit 5 (inv agent) | F05, F15 | Resumption + mid-investigation summary |
| Phase 4 Edit 6 (build agent) | F02, F03, F09, F10, F12, F13, F14, F25 | — |
| Phase 4 Edit 7 (manager commands) | F06, F10, F11 | Reject + conflict + re-queue |
| Not addressed | F18 — out of scope; F24 — acceptable as-is | |

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

```
Phase 1 (skill bundling + init install)  ─┐
Phase 2 (taskboard event)                ─┼─→ Phase 4 (skill rewrite, 7 edits)
Phase 3 (detect + syncAgentStatus fix)   ─┘
```

Phases 1, 2, 3 are independent — run in parallel.
Phase 4 depends on all three: the skill references `taskboard event` and
`taskboard detect`, and is installed via the Phase 1 mechanism.

Within Phase 4: write Edits 1–3 first (structure, schema, detection), then
Edits 4–7 (agent prompts) which reference the lifecycle section from Edit 1.

---

## 8. Out of Scope

- TUI rendering changes
- `team.json` schema structure changes
- New agent types
- Task agent and utility agent lifecycle (separate document)
- Build type storage in `tasks` (F24 — not needed)
