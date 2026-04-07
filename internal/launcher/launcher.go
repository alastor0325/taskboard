package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

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
	return exec.Command(
		"tmux", "split-window", "-h",
		"-p", strconv.Itoa(widthPercent),
		selfexec.Path(), "tui", "--project", proj,
	).Run()
}

func openZellij(proj string) error {
	return exec.Command(
		"zellij", "action", "new-pane",
		"--direction", "right",
		"--", selfexec.Path(), "tui", "--project", proj,
	).Run()
}
