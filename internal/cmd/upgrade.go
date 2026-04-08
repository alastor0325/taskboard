package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

const moduleURL = "github.com/alastor0325/taskboard/cmd/taskboard"

func runUpgrade(args []string) error {
	version := "latest"
	if len(args) > 0 && args[0] != "" {
		version = args[0]
	}

	pkg := moduleURL + "@" + version
	fmt.Printf("Installing %s...\n", pkg)

	cmd := exec.Command("go", "install", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install: %w", err)
	}

	fmt.Println("Updating skill...")
	updated, err := installSkill()
	if err != nil {
		return fmt.Errorf("install skill: %w", err)
	}
	if updated {
		fmt.Println("✓ Skill updated — restart Claude session to pick up changes.")
	} else {
		fmt.Println("✓ Skill already up to date.")
	}
	fmt.Println("Done.")
	return nil
}
