package cmd

import (
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
	_ = proj
	// TUI is implemented in internal/tui — placeholder until section 8 is built.
	return runTUIImpl(proj)
}

func runOpen(args []string) error {
	proj, rest := resolveProject(args)
	width := 35
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--width" && i+1 < len(rest) {
			var w int
			if _, err := parseWidth(rest[i+1], &w); err == nil {
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

func parseWidth(s string, out *int) (string, error) {
	var v int
	for _, c := range s {
		if c < '0' || c > '9' {
			return s, &parseError{}
		}
		v = v*10 + int(c-'0')
	}
	*out = v
	return s, nil
}

type parseError struct{}

func (e *parseError) Error() string { return "not a number" }
