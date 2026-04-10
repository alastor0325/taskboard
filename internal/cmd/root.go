package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/alastor0325/taskboard/internal/project"
)

// Version is set at build time via -ldflags "-X github.com/alastor0325/taskboard/internal/cmd.Version=vX.Y.Z".
// Falls back to the module version embedded by go install.
var Version = "dev"

func resolvedVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}

func Execute() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	// Strip global flags that may appear before the subcommand.
	// resolveProject() handles these within each handler, so we just need
	// to find the first non-flag argument to use as the subcommand.
	allArgs := os.Args[1:]
	var sub string
	var args []string
	for i := 0; i < len(allArgs); i++ {
		a := allArgs[i]
		if (a == "--project" || a == "-project") && i+1 < len(allArgs) {
			// skip flag + value; they're handled by resolveProject in each cmd
			args = append(args, a, allArgs[i+1])
			i++
		} else if sub == "" {
			sub = a
		} else {
			args = append(args, a)
		}
	}
	if sub == "" {
		printUsage()
		return nil
	}

	switch sub {
	case "init":
		return runInit(args)
	case "sync":
		return runSync(args)
	case "set-task":
		return runSetTask(args)
	case "done-task":
		return runDoneTask(args)
	case "claim-task":
		return runClaimTask(args)
	case "who-owns":
		return runWhoOwns(args)
	case "file-conflicts":
		return runFileConflicts(args)
	case "log":
		return runLog(args)
	case "btw":
		return runBtw(args)
	case "event":
		return runEvent(args)
	case "notify":
		return runNotify(args)
	case "upgrade":
		return runUpgrade(args)
	case "install-skill":
		return runInstallSkill(args)
	case "detect":
		fmt.Println(project.Detect(""))
		return nil
	case "agent-health":
		return runAgentHealth(args)
	case "check-build-progress":
		return runCheckBuildProgress(args)
	case "watcher":
		return runWatcher(args)
	case "healthcheck":
		return runHealthcheck(args)
	case "tui":
		return runTUI(args)
	case "open":
		return runOpen(args)
	case "review-server":
		return runReviewServer(args)
	case "version", "--version", "-v":
		fmt.Println(resolvedVersion())
		return nil
	case "--help", "-h", "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q — run 'taskboard help'", sub)
	}
}

func printUsage() {
	fmt.Fprint(os.Stdout, `taskboard — task board CLI for multi-agent session management

Usage:
  taskboard <subcommand> [flags]

Subcommands:
  init                            start session, write log entry, sync
  sync                            re-read team.json, update agent-status.json
  set-task <bug_id> [flags]       partial update task (--summary, --status, --note, --worktree)
  done-task <bug_id>              mark task done
  claim-task <bug_id> <agent>     atomic claim (JSON output)
  who-owns <bug_id>               ownership query (JSON output)
  file-conflicts <bug_id>         file conflict check (JSON output)
  log <agent> <message>           append log entry
  btw <agent> <message>           volatile heartbeat (TTL 120s)
  event <type> <agent> <msg>      structured milestone + conditional Matrix
  notify <log|alert|done> <msg>   Matrix notification + log
  upgrade [version]               upgrade binary + skill (default: latest)
  install-skill                   install bundled skill to ~/.claude/skills/taskboard/
  detect                          print detected project name
  agent-health <file> [secs]      liveness check by output file mtime
  check-build-progress <dir> [m]  build stall detection
  tui                             launch TUI dashboard
  watcher                         start watcher daemon
  healthcheck                     run healthcheck pass
  open                            split pane + launch tui
  version                         print version

Global flags:
  --project <name>                override project detection
`)
}
