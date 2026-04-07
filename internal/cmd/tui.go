package cmd

import "github.com/alastor0325/taskboard/internal/tui"

func runTUIImpl(proj string) error {
	return tui.Run(proj)
}
