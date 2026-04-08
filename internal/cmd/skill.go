package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alastor0325/taskboard/internal/skilldata"
)

func installSkill() error {
	dest := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	return os.WriteFile(dest, skilldata.Taskboard, 0o644)
}

func runInstallSkill(args []string) error {
	if err := installSkill(); err != nil {
		return err
	}
	fmt.Println("skill installed →", filepath.Join(os.Getenv("HOME"), ".claude", "skills", "taskboard", "SKILL.md"))
	return nil
}
