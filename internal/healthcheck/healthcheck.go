package healthcheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alastor0325/taskboard/internal/project"
	"github.com/alastor0325/taskboard/internal/store"
)

const defaultStaleAgent = 30 * time.Minute

func Run(proj string) error {
	safe := project.Sanitize(proj)
	teamFile := project.TeamFile(proj)
	st := store.New(teamFile)

	team, err := st.Load()
	if err != nil {
		return fmt.Errorf("load team: %w", err)
	}

	// Check investigation agents.
	for id, agent := range team.InvestigationAgents {
		if agent.Status != "running" {
			continue
		}
		if agent.OutputFile != "" && isStale(agent.OutputFile) {
			if err := st.MarkAgentDead("investigation_agents", id); err != nil {
				fmt.Fprintf(os.Stderr, "mark dead %s: %v\n", id, err)
			}
		}
	}

	// Check task agents.
	for id, agent := range team.TaskAgents {
		if agent.Status != "running" {
			continue
		}
		if agent.OutputFile != "" && isStale(agent.OutputFile) {
			if err := st.MarkAgentDead("task_agents", id); err != nil {
				fmt.Fprintf(os.Stderr, "mark dead %s: %v\n", id, err)
			}
		}
	}

	// Restart watcher if stale.
	pidFile := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-watcher.pid")
	if isWatcherDead(pidFile) {
		restartWatcher(proj)
	}

	// Sync status.
	exec.Command("taskboard", "sync", "--project", proj).Run()
	return nil
}

func isStale(outputFile string) bool {
	info, err := os.Stat(outputFile)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > defaultStaleAgent
}

func isWatcherDead(pidFile string) bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return true
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	// On Unix, FindProcess always succeeds; send signal 0 to check liveness.
	return !isProcessAlive(proc)
}

func restartWatcher(proj string) {
	cmd := exec.Command("taskboard", "watcher", "--project", proj)
	cmd.Start()
}
