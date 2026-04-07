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

// killTmuxTUIPanes kills all tmux panes whose command is running "taskboard tui".
func killTmuxTUIPanes() {
	// List all panes with format: session:window.pane pane_id current_command
	out, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{pane_id} #{pane_current_command}").Output()
	if err != nil {
		return
	}
	myPane := os.Getenv("TMUX_PANE")
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		paneID, cmd := fields[0], fields[1]
		if paneID == myPane {
			continue
		}
		if cmd == "taskboard" {
			exec.Command("tmux", "kill-pane", "-t", paneID).Run() //nolint:errcheck
		}
	}
}

func openZellij(proj string) error {
	return exec.Command(
		"zellij", "action", "new-pane",
		"--direction", "right",
		"--", selfexec.Path(), "tui", "--project", proj,
	).Run()
}
