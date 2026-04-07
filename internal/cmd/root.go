package cmd

import (
	"fmt"
	"os"
)

func Execute() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}
	sub := os.Args[1]
	args := os.Args[2:]

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
	case "notify":
		return runNotify(args)
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
  notify <log|alert|done> <msg>   Matrix notification + log
  agent-health <file> [secs]      liveness check by output file mtime
  check-build-progress <dir> [m]  build stall detection
  tui                             launch TUI dashboard
  watcher                         start watcher daemon
  healthcheck                     run healthcheck pass
  open                            split pane + launch tui

Global flags:
  --project <name>                override project detection
`)
}
