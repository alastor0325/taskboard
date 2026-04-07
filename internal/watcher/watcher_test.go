package watcher

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestGetWidthDefault(t *testing.T) {
	t.Setenv("TASKBOARD_WIDTH", "")
	if w := getWidth(); w != 35 {
		t.Errorf("default width: got %d, want 35", w)
	}
}

func TestGetWidthFromEnv(t *testing.T) {
	t.Setenv("TASKBOARD_WIDTH", "50")
	if w := getWidth(); w != 50 {
		t.Errorf("expected 50, got %d", w)
	}
}

func TestGetWidthBelowMinReturnsDefault(t *testing.T) {
	t.Setenv("TASKBOARD_WIDTH", "10")
	// Values outside [20, 70] are rejected; the function falls back to 35.
	if w := getWidth(); w != 35 {
		t.Errorf("value below 20 should fall back to 35, got %d", w)
	}
}

func TestGetWidthAboveMaxReturnsDefault(t *testing.T) {
	t.Setenv("TASKBOARD_WIDTH", "80")
	// Values outside [20, 70] are rejected; the function falls back to 35.
	if w := getWidth(); w != 35 {
		t.Errorf("value above 70 should fall back to 35, got %d", w)
	}
}

func TestGetWidthAtBoundaries(t *testing.T) {
	t.Setenv("TASKBOARD_WIDTH", "20")
	if w := getWidth(); w != 20 {
		t.Errorf("lower boundary: got %d, want 20", w)
	}
	t.Setenv("TASKBOARD_WIDTH", "70")
	if w := getWidth(); w != 70 {
		t.Errorf("upper boundary: got %d, want 70", w)
	}
}

func TestPIDFileContents(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "watcher.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("pid file contents not a number: %q", data)
	}
	if pid != os.Getpid() {
		t.Errorf("pid file: got %d, want %d", pid, os.Getpid())
	}
}

func TestCheckRelaunchMarkerNoMarker(t *testing.T) {
	dir := t.TempDir()
	safe := filepath.Base(dir)
	// Neither marker nor lock file exists — function should be a no-op.
	marker := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-tui-relaunch")
	os.Remove(marker) // ensure absent
	checkRelaunchMarker("test-proj", safe)
	// If we reach here without panic the test passes.
}

func TestCheckRelaunchMarkerWithLockAlreadyHeld(t *testing.T) {
	dir := t.TempDir()
	safe := filepath.Base(dir)

	marker := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-tui-relaunch")
	lockFile := marker + ".lock"

	// Create the marker so that, if the lock were acquired, it would be removed.
	if err := os.WriteFile(marker, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(marker) })

	// Hold the lock manually so checkRelaunchMarker cannot acquire it.
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("could not create lock file: %v", err)
	}
	defer f.Close()
	defer os.Remove(lockFile)

	checkRelaunchMarker("test-proj", safe)

	// Marker must still exist because the lock was already held.
	if _, err := os.Stat(marker); err != nil {
		t.Error("marker should NOT have been removed while lock was held")
	}
}

func TestCheckRelaunchMarkerRemovesMarkerWhenLockFree(t *testing.T) {
	dir := t.TempDir()
	safe := filepath.Base(dir)

	marker := filepath.Join(os.TempDir(), ".taskboard-"+safe+"-tui-relaunch")
	lockFile := marker + ".lock"

	// Ensure lock file is absent.
	os.Remove(lockFile)

	if err := os.WriteFile(marker, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(marker) })

	// checkRelaunchMarker will acquire the lock, see the marker, remove it, and
	// attempt to launch a TUI pane (which will silently fail in a test env).
	checkRelaunchMarker("test-proj", safe)

	if _, err := os.Stat(marker); err == nil {
		t.Error("marker should have been removed when lock was free")
	}
}
