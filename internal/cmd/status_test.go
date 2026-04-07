package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alastor0325/taskboard/internal/types"
)

func tmpLogFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "log.json")
}

func TestAppendLog(t *testing.T) {
	lf := tmpLogFile(t)
	if err := appendLog(lf, "agent-1", "hello"); err != nil {
		t.Fatal(err)
	}
	entries, err := loadLog(lf)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Agent != "agent-1" || entries[0].Message != "hello" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestLogRolling(t *testing.T) {
	lf := tmpLogFile(t)
	for i := 0; i < maxLog+10; i++ {
		appendLog(lf, "a", "msg")
	}
	entries, _ := loadLog(lf)
	if len(entries) > maxLog {
		t.Errorf("log should be capped at %d, got %d", maxLog, len(entries))
	}
}

func TestLogResetMarker(t *testing.T) {
	lf := tmpLogFile(t)
	appendLog(lf, "a", "before reset")

	// Write log reset marker with current time
	proj := filepath.Base(filepath.Dir(lf))
	marker := filepath.Join(os.TempDir(), ".taskboard-"+proj+"-log-reset")
	os.WriteFile(marker, []byte{}, 0o644)
	defer os.Remove(marker)

	// loadLog should return nil because marker is fresh (< 10s ago)
	entries, err := loadLog(lf)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("fresh reset marker should clear log, got %d entries", len(entries))
	}
}

func TestBtwTTL(t *testing.T) {
	lf := tmpLogFile(t)
	btwPath := lf[:len(lf)-len(filepath.Ext(lf))] + "-btw.json"

	// Write old entry directly (timestamp in the past)
	old := types.BtwEntry{Time: float64(time.Now().Unix() - 200), Agent: "old-agent", Message: "old"}
	fresh := types.BtwEntry{Time: float64(time.Now().Unix()), Agent: "fresh-agent", Message: "new"}
	import_json_via_indirect_write(t, btwPath, []types.BtwEntry{old, fresh})

	active, err := loadBtw(lf)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active entry, got %d", len(active))
	}
	if active[0].Agent != "fresh-agent" {
		t.Errorf("got agent %q, want fresh-agent", active[0].Agent)
	}
}

func import_json_via_indirect_write(t *testing.T, path string, v any) {
	t.Helper()
	if err := writeJSON(path, v); err != nil {
		t.Fatal(err)
	}
}

func TestAppendBtwDeduplicatesAgent(t *testing.T) {
	lf := tmpLogFile(t)
	appendBtw(lf, "agent-1", "first message")
	appendBtw(lf, "agent-1", "second message")

	active, err := loadBtw(lf)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range active {
		if e.Agent == "agent-1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("agent should appear only once, found %d times", count)
	}
	if active[0].Message != "second message" {
		t.Errorf("last message should win, got %q", active[0].Message)
	}
}

func TestAppendBtwMaxEntries(t *testing.T) {
	lf := tmpLogFile(t)
	for i := 0; i < btwMaxEntries+10; i++ {
		appendBtw(lf, "agent-x", "msg")
		// Use different agent names to avoid deduplication
	}
	// Add many distinct agents
	lf2 := tmpLogFile(t)
	for i := 0; i < btwMaxEntries+5; i++ {
		appendBtw(lf2, "agent-"+string(rune('a'+i%26))+string(rune('0'+i/26)), "msg")
	}
	active, _ := loadBtw(lf2)
	if len(active) > btwMaxEntries {
		t.Errorf("btw should be capped at %d, got %d", btwMaxEntries, len(active))
	}
}
