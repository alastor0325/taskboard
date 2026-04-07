package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alastor0325/taskboard/internal/store"
	"github.com/alastor0325/taskboard/internal/types"
)

const maxLog = 100
const btwTTL = 120.0
const btwMaxEntries = 50

func loadLog(logPath string) ([]types.LogEntry, error) {
	data, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Check log reset marker.
	resetMarker := filepath.Join(os.TempDir(), ".taskboard-"+filepath.Base(filepath.Dir(logPath))+"-log-reset")
	if info, err := os.Stat(resetMarker); err == nil {
		if time.Since(info.ModTime()) < 10*time.Second {
			return nil, nil
		}
	}
	var entries []types.LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func loadBtw(logPath string) ([]types.BtwEntry, error) {
	btwPath := logPath[:len(logPath)-len(filepath.Ext(logPath))] + "-btw.json"
	data, err := os.ReadFile(btwPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []types.BtwEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	now := float64(time.Now().Unix())
	var active []types.BtwEntry
	for _, e := range entries {
		if now-e.Time < btwTTL {
			active = append(active, e)
		}
	}
	return active, nil
}

func appendLog(logPath, agent, message string) error {
	entries, err := loadLog(logPath)
	if err != nil {
		entries = nil
	}
	entries = append(entries, types.LogEntry{
		Time:    float64(time.Now().Unix()),
		Agent:   agent,
		Message: message,
	})
	if len(entries) > maxLog {
		entries = entries[len(entries)-maxLog:]
	}
	return writeJSON(logPath, entries)
}

func appendBtw(logPath, agent, message string) error {
	btwPath := logPath[:len(logPath)-len(filepath.Ext(logPath))] + "-btw.json"
	data, _ := os.ReadFile(btwPath)
	var entries []types.BtwEntry
	json.Unmarshal(data, &entries)

	now := float64(time.Now().Unix())
	var filtered []types.BtwEntry
	for _, e := range entries {
		if e.Agent != agent && now-e.Time < btwTTL {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, types.BtwEntry{Time: now, Agent: agent, Message: message})
	if len(filtered) > btwMaxEntries {
		filtered = filtered[len(filtered)-btwMaxEntries:]
	}
	return writeJSON(btwPath, filtered)
}

func writeStatus(proj string, st *store.TaskStore) error {
	team, err := st.Load()
	if err != nil {
		return err
	}
	logPath := logFile(proj)
	logEntries, _ := loadLog(logPath)
	btwEntries, _ := loadBtw(logPath)

	status := types.AgentStatus{
		Project:     proj,
		UpdatedAt:   float64(time.Now().Unix()),
		Tasks:       team.Tasks,
		BuildAgents: team.BuildAgents,
		Log:         logEntries,
		Btw:         btwEntries,
	}
	return writeJSON(statusFile(proj), status)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func writeLogResetMarker(proj string) {
	marker := filepath.Join(os.TempDir(), ".taskboard-"+proj+"-log-reset")
	os.WriteFile(marker, []byte{}, 0o644)
}

// printJSON prints v as indented JSON to stdout.
func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
