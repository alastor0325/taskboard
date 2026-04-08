package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alastor0325/taskboard/internal/skilldata"
)

func installSkill() (updated bool, err error) {
	dest := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, fmt.Errorf("create skill dir: %w", err)
	}
	existing, _ := os.ReadFile(dest)
	if bytes.Equal(existing, skilldata.Taskboard) {
		return false, nil
	}
	return true, os.WriteFile(dest, skilldata.Taskboard, 0o644)
}

func runInstallSkill(args []string) error {
	dest := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "SKILL.md")
	updated, err := installSkill()
	if err != nil {
		return err
	}
	if updated {
		fmt.Println("skill updated →", dest, "(restart Claude session to pick up changes)")
	} else {
		fmt.Println("skill already up to date →", dest)
	}
	return nil
}
