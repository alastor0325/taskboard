package tui

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alastor0325/taskboard/internal/project"
	"github.com/alastor0325/taskboard/internal/reviewserver"
)

// Run starts the TUI for the given project.
func Run(proj string) error {
	statusFile := project.StatusFile(proj)
	heartbeatFile := project.WatcherHeartbeatFile(proj)
	ensureReviewServer()

	m := New(proj, statusFile, heartbeatFile)
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

func ensureReviewServer() {
	port := reviewserver.Port()
	conn, err := net.DialTimeout("tcp",
		"localhost:"+strconv.Itoa(port), 0)
	if err == nil {
		conn.Close()
		return
	}
	// Start review server in background.
	self, _ := os.Executable()
	if self == "" {
		self = "taskboard"
	}
	cmd := exec.Command(self, "review-server")
	cmd.Start()
}

// ReviewServerRun starts the standalone review server (called from cmd).
func ReviewServerRun() error {
	fmt.Printf("review server starting on port %d\n", reviewserver.Port())
	return reviewserver.Start()
}
