package watcher

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

const pollInterval = time.Second

func Run(proj string) error {
	safe := project.Sanitize(proj)
	pidFile := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-watcher.pid")

	// Write PID file.
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	defer os.Remove(pidFile)

	teamFile := project.TeamFile(proj)
	var lastMtime time.Time

	for {
		info, err := os.Stat(teamFile)
		if err == nil && info.ModTime().After(lastMtime) {
			lastMtime = info.ModTime()
			syncStatus(proj)
		}
		checkRelaunchMarker(proj, safe)
		time.Sleep(pollInterval)
	}
}

func syncStatus(proj string) {
	st := store.New(project.TeamFile(proj))
	_ = st // writeStatus called via CLI to keep logic in one place
	exec.Command("taskboard", "sync", "--project", proj).Run()
}

func checkRelaunchMarker(proj, safe string) {
	marker := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-tui-relaunch")
	lockFile := marker + ".lock"

	// O_EXCL lock to prevent two watchers opening a pane simultaneously.
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	f.Close()
	defer os.Remove(lockFile)

	if _, err := os.Stat(marker); err != nil {
		return
	}
	os.Remove(marker)
	launchTUIPane(proj)
}

func launchTUIPane(proj string) {
	tmux := os.Getenv("TMUX")
	if tmux != "" {
		width := getWidth()
		exec.Command("tmux", "split-window", "-h", "-p", strconv.Itoa(width),
			"taskboard", "tui", "--project", proj).Run()
		return
	}
	if os.Getenv("ZELLIJ_SESSION_NAME") != "" {
		exec.Command("zellij", "action", "new-pane", "--direction", "right",
			"--", "taskboard", "tui", "--project", proj).Run()
		return
	}
}

func getWidth() int {
	if v := os.Getenv("TASKBOARD_WIDTH"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			if n >= 20 && n <= 70 {
				return n
			}
		}
	}
	return 35
}
