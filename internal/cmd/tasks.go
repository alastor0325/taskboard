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
	if err := appendLog(logFile(proj), "manager", "session started"); err != nil {
		return err
	}
	return writeStatus(proj, st)
}

func runSync(args []string) error {
	proj, _ := resolveProject(args)
	return writeStatus(proj, newStore(proj))
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
	if err := st.SetTask(bugID, opts); err != nil {
		return err
	}
	return writeStatus(proj, st)
}

func runDoneTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 1 {
		return fmt.Errorf("usage: taskboard done-task <bug_id>")
	}
	bugID := rest[0]
	st := newStore(proj)
	if err := st.MarkDone(bugID); err != nil {
		return err
	}
	return writeStatus(proj, st)
}

func runClaimTask(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard claim-task <bug_id> <agent>")
	}
	bugID, agent := rest[0], rest[1]
	st := newStore(proj)
	claimed, owner, err := st.ClaimTask(bugID, agent)
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
	bugID := rest[0]
	owner, err := newStore(proj).WhoOwns(bugID)
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
	bugID := rest[0]
	conflicts, err := newStore(proj).FileConflicts(bugID)
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
	agent, message := rest[0], rest[1]
	if err := appendLog(logFile(proj), agent, message); err != nil {
		return err
	}
	return writeStatus(proj, newStore(proj))
}

func runBtw(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard btw <agent> <message>")
	}
	agent, message := rest[0], rest[1]
	return appendBtw(logFile(proj), agent, message)
}

func runNotify(args []string) error {
	proj, rest := resolveProject(args)
	if len(rest) < 2 {
		return fmt.Errorf("usage: taskboard notify <log|alert|done> <message>")
	}
	level, message := rest[0], rest[1]
	_ = level // matrix-cli routing handled externally
	if err := appendLog(logFile(proj), "notify", message); err != nil {
		return err
	}
	// Attempt matrix-cli notification; failure is non-fatal.
	matrixArgs := []string{"notify", level, message}
	_ = matrixArgs
	// matrix-cli is a separate tool; invoke it if available.
	notifyMatrix(level, message)
	return writeStatus(proj, newStore(proj))
}

func notifyMatrix(level, message string) {
	// Best-effort: if matrix-cli is not installed, silently skip.
	exec.Command("matrix-cli", "notify", level, message).Run()
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
