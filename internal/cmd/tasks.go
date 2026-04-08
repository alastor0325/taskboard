package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/alastor0325/taskboard/internal/store"
)

func syncStatus(proj string) error {
	team, err := newStore(proj).Load()
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
}

func runInit(args []string) error {
	proj, _ := resolveProject(args)
	fmt.Println("Initializing taskboard...")

	updated, err := installSkill()
	if err != nil {
		return fmt.Errorf("install skill: %w", err)
	}
	if updated {
		fmt.Println("[x] Skill: updated (restart Claude session to pick up changes)")
	} else {
		fmt.Println("[x] Skill: up to date")
	}

	writeLogResetMarker(proj)
	fmt.Printf("[x] Project: %s\n", proj)

	if err := appendLog(logFile(proj), "manager", "session started"); err != nil {
		return err
	}
	fmt.Println("[x] Log: reset marker written")

	if err := syncStatus(proj); err != nil {
		return err
	}
	fmt.Println("[x] Status: synced")

	fmt.Println("Ready.")
	return nil
}

func runSync(args []string) error {
	proj, _ := resolveProject(args)
	return syncStatus(proj)
}

func runSetTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard set-task <bug_id> [--summary S] [--status S] [--note N] [--worktree W]")
	}
	bugID := rest[0]
	if strings.HasPrefix(bugID, "-") {
		return fmt.Errorf("invalid bug_id %q: must not start with '-'", bugID)
	}
	opts := store.SetTaskOpts{}
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case "--summary":
			if i+1 >= len(rest) {
				return fmt.Errorf("--summary requires a value")
			}
			i++
			v := rest[i]
			opts.Summary = &v
		case "--status":
			if i+1 >= len(rest) {
				return fmt.Errorf("--status requires a value")
			}
			i++
			v := rest[i]
			opts.Status = &v
		case "--note":
			if i+1 >= len(rest) {
				return fmt.Errorf("--note requires a value")
			}
			i++
			v := rest[i]
			opts.Note = &v
		case "--worktree":
			if i+1 >= len(rest) {
				return fmt.Errorf("--worktree requires a value")
			}
			i++
			v := rest[i]
			opts.Worktree = &v
		default:
			return fmt.Errorf("unknown flag %q", rest[i])
		}
	}
	if _, err := newStore(proj).SetTask(bugID, opts); err != nil {
		return err
	}
	return syncStatus(proj)
}

func runDoneTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard done-task <bug_id>")
	}
	if _, err := newStore(proj).MarkDone(rest[0]); err != nil {
		return err
	}
	return syncStatus(proj)
}

func runClaimTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard claim-task <bug_id> <agent>")
	}
	bugID, agent := rest[0], rest[1]
	claimed, owner, err := newStore(proj).ClaimTask(bugID, agent)
	if err != nil {
		return err
	}
	result := map[string]any{"claimed": claimed}
	if !claimed {
		result["owner"] = owner
	}
	return printJSON(result)
}

func runWhoOwns(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard who-owns <bug_id>")
	}
	owner, err := newStore(proj).WhoOwns(rest[0])
	if err != nil {
		return err
	}
	var ownerVal any = nil
	if owner != "" {
		ownerVal = owner
	}
	return printJSON(map[string]any{"owner": ownerVal})
}

func runFileConflicts(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard file-conflicts <bug_id>")
	}
	conflicts, err := newStore(proj).FileConflicts(rest[0])
	if err != nil {
		return err
	}
	if conflicts == nil {
		conflicts = []store.FileConflict{}
	}
	return printJSON(map[string]any{"conflicts": conflicts})
}

func runLog(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard log <agent> <message>")
	}
	if err := appendLog(logFile(proj), rest[0], rest[1]); err != nil {
		return err
	}
	return syncStatus(proj)
}

func runBtw(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard btw <agent> <message>")
	}
	agentName := rest[0]
	known, err := newStore(proj).IsKnownAgent(agentName)
	if err != nil {
		return err
	}
	if !known {
		return fmt.Errorf("unknown agent %q: not registered in team.json", agentName)
	}
	if err := appendBtw(logFile(proj), agentName, rest[1]); err != nil {
		return err
	}
	return syncStatus(proj)
}

var eventLevels = map[string]string{
	"build-failed": "alert",
	"test-failed":  "alert",
	"try-auth":     "alert",
	"task-done":    "log",
	"try-pushed":   "log",
	"waiting":      "log",
}

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

func runNotify(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard notify <log|alert|done> <message>")
	}
	level, message := rest[0], rest[1]
	if err := appendLog(logFile(proj), "notify", message); err != nil {
		return err
	}
	exec.Command("matrix-cli", "notify", level, message).Run()
	return syncStatus(proj)
}

func runAgentHealth(args []string) error {
	_, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard agent-health <output_file> [stale_secs]")
	}
	return agentHealth(rest)
}

func runCheckBuildProgress(args []string) error {
	_, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard check-build-progress <obj_dir> [stale_minutes]")
	}
	return checkBuildProgress(rest)
}
