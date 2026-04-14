package launcher

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alastor0325/taskboard/internal/selfexec"
)

// tmuxAvailable is a variable so tests can override it without shelling out.
var tmuxAvailable = func() bool {
	return exec.Command("tmux", "ls").Run() == nil
}

// Open detects the active multiplexer and splits a pane for the TUI.
func Open(proj string, widthPercent int) error {
	if tmuxAvailable() {
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
	bin := selfexec.Path()
	tmuxArgs := []string{
		"split-window", "-h",
		"-l", strconv.Itoa(widthPercent) + "%",
		bin, "tui", "--project", proj,
	}
	cmd := exec.Command("tmux", tmuxArgs...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmtCmdErr("tmux "+strings.Join(tmuxArgs, " "), err, buf.String())
	}
	return nil
}

func openZellij(proj string) error {
	bin := selfexec.Path()
	zellijArgs := []string{
		"action", "new-pane",
		"--direction", "right",
		"--", bin, "tui", "--project", proj,
	}
	cmd := exec.Command("zellij", zellijArgs...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmtCmdErr("zellij "+strings.Join(zellijArgs, " "), err, buf.String())
	}
	return nil
}

// fmtCmdErr wraps an exec error, appending stderr output when present.
func fmtCmdErr(cmdName string, err error, stderr string) error {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%s: %w: %s", cmdName, err, msg)
	}
	return fmt.Errorf("%s: %w", cmdName, err)
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
