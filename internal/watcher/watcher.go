package watcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alastor0325/taskboard/internal/launcher"
	"github.com/alastor0325/taskboard/internal/project"
	"github.com/alastor0325/taskboard/internal/selfexec"
)

const pollInterval = time.Second

func Run(proj string) error {
	safe := project.Sanitize(proj)
	pidFile := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-watcher.pid")

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
			exec.Command(selfexec.Path(), "sync", "--project", proj).Run()
		}
		checkRelaunchMarker(proj, safe)
		time.Sleep(pollInterval)
	}
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
	launcher.Open(proj, getWidth())
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
