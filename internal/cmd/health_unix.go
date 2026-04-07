//go:build !windows

package cmd

import "os/exec"

func psOutput() (string, error) {
	out, err := exec.Command("ps", "aux").Output()
	return string(out), err
}
