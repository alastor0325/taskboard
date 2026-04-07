package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alastor0325/taskboard/internal/selfexec"
)

// Open detects the active multiplexer and splits a pane for the TUI.
func Open(proj string, widthPercent int) error {
	if os.Getenv("TMUX") != "" {
		return openTmux(proj, widthPercent)
	}
	if os.Getenv("ZELLIJ_SESSION_NAME") != "" {
		return openZellij(proj)
	}
	fmt.Printf("Run 'taskboard tui --project %s' in a new terminal pane.\n", proj)
	return nil
}

func openTmux(proj string, widthPercent int) error {
	// Kill any existing taskboard tui panes before opening a new one.
	killTmuxTUIPanes()
	return exec.Command(
		"tmux", "split-window", "-h",
		"-p", strconv.Itoa(widthPercent),
		selfexec.Path(), "tui", "--project", proj,
	).Run()
}

// killTmuxTUIPanes kills all tmux panes whose command is "taskboard", skipping the caller's own pane.
func killTmuxTUIPanes() {
	out, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{pane_id} #{pane_current_command}").Output()
	if err != nil {
		return
	}
	myPane := os.Getenv("TMUX_PANE")
	for _, paneID := range tuiPanesToKill(string(out), myPane) {
		exec.Command("tmux", "kill-pane", "-t", paneID).Run() //nolint:errcheck
	}
}

// tuiPanesToKill parses tmux list-panes output and returns IDs of panes running
// "taskboard" that are not the caller's own pane (myPane).
func tuiPanesToKill(output, myPane string) []string {
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		paneID, cmd := fields[0], fields[1]
		if paneID == myPane || cmd != "taskboard" {
			continue
		}
		ids = append(ids, paneID)
	}
	return ids
}

func openZellij(proj string) error {
	return exec.Command(
		"zellij", "action", "new-pane",
		"--direction", "right",
		"--", selfexec.Path(), "tui", "--project", proj,
	).Run()
}
