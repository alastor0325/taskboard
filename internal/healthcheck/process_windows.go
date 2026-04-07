//go:build windows

package healthcheck

import "os"

func isProcessAlive(proc *os.Process) bool {
	// On Windows, use a best-effort check.
	return proc != nil
}
