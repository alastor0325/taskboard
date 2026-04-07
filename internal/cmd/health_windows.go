//go:build windows

package cmd

import "os/exec"

func psOutput() (string, error) {
	out, err := exec.Command("tasklist").Output()
	return string(out), err
}
