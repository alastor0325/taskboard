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
	"github.com/alastor0325/taskboard/internal/selfexec"
	"github.com/alastor0325/taskboard/internal/store"
)

const defaultStaleAgent = 30 * time.Minute

func Run(proj string) error {
	safe := project.Sanitize(proj)
	st := store.New(project.TeamFile(proj))

	team, err := st.Load()
	if err != nil {
		return fmt.Errorf("load team: %w", err)
	}

	for id, agent := range team.InvestigationAgents {
		if agent.Status == "running" && agent.OutputFile != "" && isStale(agent.OutputFile) {
			if err := st.MarkAgentDead("investigation_agents", id); err != nil {
				fmt.Fprintf(os.Stderr, "mark dead %s: %v\n", id, err)
			}
		}
	}

	for id, agent := range team.TaskAgents {
		if agent.Status == "running" && agent.OutputFile != "" && isStale(agent.OutputFile) {
			if err := st.MarkAgentDead("task_agents", id); err != nil {
				fmt.Fprintf(os.Stderr, "mark dead %s: %v\n", id, err)
			}
		}
	}

	pidFile := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-watcher.pid")
	if isWatcherDead(pidFile) {
		if err := exec.Command(selfexec.Path(), "watcher", "--project", proj).Start(); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck: respawn watcher: %v\n", err)
		}
	}

	exec.Command(selfexec.Path(), "sync", "--project", proj).Run()
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
	return !isProcessAlive(proc)
}
