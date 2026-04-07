package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const defaultStaleSecs = 1800.0

func agentHealth(args []string) error {
	outputFile := args[0]
	staleThreshold := defaultStaleSecs
	if len(args) >= 2 {
		if v, err := strconv.ParseFloat(args[1], 64); err == nil {
			staleThreshold = v
		}
	}

	info, err := os.Stat(outputFile)
	if os.IsNotExist(err) {
		return printJSON(map[string]any{
			"status":      "dead",
			"exists":      false,
			"age_seconds": nil,
		})
	}
	if err != nil {
		return err
	}

	age := time.Since(info.ModTime()).Seconds()
	status := "alive"
	if age > staleThreshold {
		status = "stale"
	}
	return printJSON(map[string]any{
		"status":      status,
		"exists":      true,
		"age_seconds": age,
	})
}

func checkBuildProgress(args []string) error {
	objDir := args[0]
	staleMinutes := 30.0
	if len(args) >= 2 {
		if v, err := strconv.ParseFloat(args[1], 64); err == nil {
			staleMinutes = v
		}
	}
	staleThreshold := staleMinutes * 60

	hasCompiler := compilerRunning()
	artifact, age, err := newestArtifact(objDir)
	if err != nil || artifact == "" {
		return printJSON(map[string]any{
			"status":                    "no_artifacts",
			"has_compiler":              hasCompiler,
			"last_artifact_age_seconds": nil,
			"last_artifact":             nil,
		})
	}

	status := "active"
	if !hasCompiler && age > staleThreshold {
		status = "stalled"
	}
	return printJSON(map[string]any{
		"status":                    status,
		"has_compiler":              hasCompiler,
		"last_artifact_age_seconds": age,
		"last_artifact":             artifact,
	})
}

func compilerRunning() bool {
	// Check for common compiler processes via /proc (Linux) or ps (macOS).
	compilers := []string{"ninja", "rustc", "cargo-build-script", "cl.exe", "cc", "c++"}
	out, err := runPS()
	if err != nil {
		return false
	}
	for _, c := range compilers {
		if contains(out, c) {
			return true
		}
	}
	return false
}

func runPS() (string, error) {
	// Implemented in health_unix.go / health_windows.go
	return psOutput()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func newestArtifact(objDir string) (path string, age float64, err error) {
	var newest time.Time
	var newestPath string

	extensions := map[string]bool{".o": true, ".obj": true, ".rlib": true}
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, e := range entries {
			full := dir + "/" + e.Name()
			if e.IsDir() {
				walk(full)
				continue
			}
			ext := ""
			if i := len(e.Name()) - 1; i >= 0 {
				for j := i; j >= 0; j-- {
					if e.Name()[j] == '.' {
						ext = e.Name()[j:]
						break
					}
				}
			}
			if !extensions[ext] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(newest) {
				newest = info.ModTime()
				newestPath = full
			}
		}
		return nil
	}
	if err := walk(objDir); err != nil {
		return "", 0, fmt.Errorf("walk: %w", err)
	}
	if newestPath == "" {
		return "", 0, nil
	}
	return newestPath, time.Since(newest).Seconds(), nil
}
