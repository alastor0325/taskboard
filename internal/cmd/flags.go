package cmd

import (
	"os"

	"github.com/alastor0325/taskboard/internal/project"
	"github.com/alastor0325/taskboard/internal/store"
)

// resolveProject extracts --project from args and returns (projectName, remainingArgs).
func resolveProject(args []string) (string, []string) {
	var explicit string
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			explicit = args[i+1]
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	return project.Detect(explicit), rest
}

// newStore returns a TaskStore for the given project.
func newStore(proj string) *store.TaskStore {
	return store.New(project.TeamFile(proj))
}

// statusFile returns the agent-status.json path for the given project.
func statusFile(proj string) string {
	return project.StatusFile(proj)
}

// logFile returns the log file path for the given project.
func logFile(proj string) string {
	return os.ExpandEnv("$HOME/.firefox-manager/" + proj + "/log.json")
}
