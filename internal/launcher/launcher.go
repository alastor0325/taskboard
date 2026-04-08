package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alastor0325/taskboard/internal/selfexec"
)

const tuiWindowName = "taskboard-tui"

// Open detects the active multiplexer and opens a new window for the TUI.
func Open(proj string, _ int) error {
	if os.Getenv("TMUX") != "" {
		return openTmux(proj)
	}
	if os.Getenv("ZELLIJ_SESSION_NAME") != "" {
		return openZellij(proj)
	}
	fmt.Printf("Run 'taskboard tui --project %s' in a new terminal pane.\n", proj)
	return nil
}

func openTmux(proj string) error {
	killTmuxTUIWindows()
	cmd := exec.Command(
		"tmux", "new-window",
		"-n", tuiWindowName,
		"--", selfexec.Path(), "tui", "--project", proj,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			fmt.Fprintf(os.Stderr, "tmux new-window: %s\n", msg)
		}
		fmt.Printf("Run 'taskboard tui --project %s' in a new terminal pane.\n", proj)
	}
	return nil
}

// killTmuxTUIWindows closes any existing taskboard-tui windows in the current session.
func killTmuxTUIWindows() {
	out, err := exec.Command("tmux", "list-windows",
		"-F", "#{window_id} #{window_name}").Output()
	if err != nil {
		return
	}
	for _, id := range tuiWindowsToKill(string(out)) {
		exec.Command("tmux", "kill-window", "-t", id).Run() //nolint:errcheck
	}
}

func tuiWindowsToKill(output string) []string {
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == tuiWindowName {
			ids = append(ids, fields[0])
		}
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
