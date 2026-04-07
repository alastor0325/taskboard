//go:build !windows

package healthcheck

import (
	"os"
	"syscall"
)

func isProcessAlive(proc *os.Process) bool {
	return proc.Signal(syscall.Signal(0)) == nil
}
