package selfexec

import "os"

// Path returns the absolute path to the running binary.
func Path() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return "taskboard"
}
