package cmd

import "github.com/alastor0325/taskboard/internal/tui"

func runReviewServer(_ []string) error {
	return tui.ReviewServerRun()
}
