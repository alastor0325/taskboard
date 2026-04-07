package cmd

import (
	"fmt"
	"os/exec"

	"github.com/alastor0325/taskboard/internal/store"
)

func runInit(args []string) error {
	proj, _ := resolveProject(args)
	writeLogResetMarker(proj)
	st := newStore(proj)
	team, err := st.Load()
	if err != nil {
		return err
	}
	if err := appendLog(logFile(proj), "manager", "session started"); err != nil {
		return err
	}
	return writeStatus(proj, team)
}

func runSync(args []string) error {
	proj, _ := resolveProject(args)
	st := newStore(proj)
	team, err := st.Load()
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
}

func runSetTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard set-task <bug_id> [--summary S] [--status S] [--note N] [--worktree W]")
	}
	bugID := rest[0]
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
	st := newStore(proj)
	team, err := st.SetTask(bugID, opts)
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
}

func runDoneTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard done-task <bug_id>")
	}
	st := newStore(proj)
	team, err := st.MarkDone(rest[0])
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
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
	team, err := newStore(proj).Load()
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
}

func runBtw(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard btw <agent> <message>")
	}
	if err := appendBtw(logFile(proj), rest[0], rest[1]); err != nil {
		return err
	}
	// Sync agent-status.json so the TUI picks up the new BTW entry immediately.
	st := newStore(proj)
	team, err := st.Load()
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
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
	team, err := newStore(proj).Load()
	if err != nil {
		return err
	}
	return writeStatus(proj, team)
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
