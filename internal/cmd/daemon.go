package cmd

import (
	"strconv"

	"github.com/alastor0325/taskboard/internal/healthcheck"
	"github.com/alastor0325/taskboard/internal/launcher"
	"github.com/alastor0325/taskboard/internal/watcher"
)

func runWatcher(args []string) error {
	proj, _ := resolveProject(args)
	return watcher.Run(proj)
}

func runHealthcheck(args []string) error {
	proj, _ := resolveProject(args)
	return healthcheck.Run(proj)
}

func runTUI(args []string) error {
	proj, _ := resolveProject(args)
	return runTUIImpl(proj)
}

func runOpen(args []string) error {
	proj, rest := resolveProject(args)
	width := 35
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--width" && i+1 < len(rest) {
			if w, err := strconv.Atoi(rest[i+1]); err == nil {
				if w < 20 {
					w = 20
				}
				if w > 70 {
					w = 70
				}
				width = w
			}
			i++
		}
	}
	return launcher.Open(proj, width)
}
