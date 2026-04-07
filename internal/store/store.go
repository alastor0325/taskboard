package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DONE_TASK_TTL is how long a done task stays in team.json before cleanup.
const DONE_TASK_TTL = 300 * time.Second

// validTransitions defines allowed task status transitions.
var validTransitions = map[string]map[string]bool{
	"":        {"running": true},
	"idle":    {"running": true, "done": true},
	"waiting": {"running": true, "idle": true, "done": true},
	"running": {"idle": true, "waiting": true, "done": true},
	"done":    {},
}

// TaskStore is the sole owner of all team.json mutations.
type TaskStore struct {
	teamFile string
}

func New(teamFile string) *TaskStore {
	return &TaskStore{teamFile: teamFile}
}

func (s *TaskStore) Load() (*Team, error) {
	data, err := os.ReadFile(s.teamFile)
	if os.IsNotExist(err) {
		return emptyTeam(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read team file: %w", err)
	}
	var team Team
	if err := json.Unmarshal(data, &team); err != nil {
		return nil, fmt.Errorf("parse team file: %w", err)
	}
	// Ensure all 5 top-level keys are initialised.
	if team.Tasks == nil {
		team.Tasks = map[string]*Task{}
	}
	if team.InvestigationAgents == nil {
		team.InvestigationAgents = map[string]*InvestigationAgent{}
	}
	if team.BuildAgents == nil {
		team.BuildAgents = map[string]*BuildAgent{}
	}
	if team.TaskAgents == nil {
		team.TaskAgents = map[string]*TaskAgent{}
	}
	if team.UtilityAgents == nil {
		team.UtilityAgents = map[string]*UtilityAgent{}
	}
	return &team, nil
}

func (s *TaskStore) Save(team *Team) error {
	if err := os.MkdirAll(filepath.Dir(s.teamFile), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := json.MarshalIndent(team, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal team: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.teamFile), ".team-*.json")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, s.teamFile); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

type SetTaskOpts struct {
	Summary  *string
	Status   *string
	Note     *string
	Worktree *string
}

func (s *TaskStore) SetTask(bugID string, opts SetTaskOpts) (*Team, error) {
	team, err := s.Load()
	if err != nil {
		return nil, err
	}
	task, ok := team.Tasks[bugID]
	if !ok {
		task = &Task{}
		team.Tasks[bugID] = task
	}
	if opts.Summary != nil {
		task.Summary = *opts.Summary
	}
	if opts.Status != nil {
		newStatus := *opts.Status
		allowed, exists := validTransitions[task.Status]
		if !exists {
			allowed = validTransitions[""]
		}
		if task.Status != newStatus {
			if !allowed[newStatus] {
				return nil, fmt.Errorf("invalid transition %q → %q for task %s", task.Status, newStatus, bugID)
			}
			task.Status = newStatus
			s.syncAgentStatus(team, bugID, newStatus)
		}
	}
	if opts.Note != nil {
		task.Note = *opts.Note
	}
	if opts.Worktree != nil {
		task.Worktree = *opts.Worktree
	}
	return team, s.Save(team)
}

func (s *TaskStore) MarkDone(bugID string) (*Team, error) {
	team, err := s.Load()
	if err != nil {
		return nil, err
	}
	task, ok := team.Tasks[bugID]
	if !ok {
		task = &Task{}
		team.Tasks[bugID] = task
	}
	task.Status = "done"
	now := float64(time.Now().Unix())
	task.DoneAt = &now
	s.syncAgentStatus(team, bugID, "done")
	return team, s.Save(team)
}

func (s *TaskStore) MarkAgentDead(section, name string) error {
	team, err := s.Load()
	if err != nil {
		return err
	}
	note := fmt.Sprintf("agent %s/%s died", section, name)
	switch section {
	case "investigation_agents":
		if a, ok := team.InvestigationAgents[name]; ok {
			a.Status = "dead"
		}
		if task, ok := team.Tasks[name]; ok {
			task.Status = "failed"
			task.Note = note
		}
	case "build_agents":
		if a, ok := team.BuildAgents[name]; ok {
			a.Status = "dead"
			if a.CurrentBug != nil {
				bugID := fmt.Sprintf("%d", *a.CurrentBug)
				if task, ok := team.Tasks[bugID]; ok {
					task.Status = "failed"
					task.Note = note
				}
			}
		}
	case "task_agents":
		if a, ok := team.TaskAgents[name]; ok {
			a.Status = "dead"
		}
		if task, ok := team.Tasks[name]; ok {
			task.Status = "failed"
			task.Note = note
		}
	case "utility_agents":
		if a, ok := team.UtilityAgents[name]; ok {
			a.Status = "dead"
		}
	}
	return s.Save(team)
}

func (s *TaskStore) CleanupDone(ttl time.Duration) (bool, error) {
	team, err := s.Load()
	if err != nil {
		return false, err
	}
	now := float64(time.Now().Unix())
	modified := false
	for id, task := range team.Tasks {
		if task.Status == "done" {
			if task.DoneAt == nil {
				t := now
				task.DoneAt = &t
				modified = true
			} else if now-*task.DoneAt > ttl.Seconds() {
				delete(team.Tasks, id)
				modified = true
			}
		}
	}
	if modified {
		return true, s.Save(team)
	}
	return false, nil
}

// ClaimTask atomically claims a bug for an agent.
// Returns (true, "") if claimed, (false, ownerName) if already owned.
func (s *TaskStore) ClaimTask(bugID, agentName string) (bool, string, error) {
	team, err := s.Load()
	if err != nil {
		return false, "", err
	}
	// Check all bug-centric sections for an existing claim.
	if a, ok := team.InvestigationAgents[bugID]; ok && a.Status == "running" {
		return false, a.AgentID, nil
	}
	if a, ok := team.TaskAgents[bugID]; ok && a.Status == "running" {
		return false, a.AgentID, nil
	}
	// Record the claim in investigation_agents.
	team.InvestigationAgents[bugID] = &InvestigationAgent{
		AgentID: agentName,
		Status:  "running",
	}
	return true, "", s.Save(team)
}

// WhoOwns returns the agent name currently owning the bug, or "".
func (s *TaskStore) WhoOwns(bugID string) (string, error) {
	team, err := s.Load()
	if err != nil {
		return "", err
	}
	if a, ok := team.InvestigationAgents[bugID]; ok && a.Status == "running" {
		return a.AgentID, nil
	}
	if a, ok := team.TaskAgents[bugID]; ok && a.Status == "running" {
		return a.AgentID, nil
	}
	return "", nil
}

// FileConflicts returns agents that share claimed files with the given bug.
func (s *TaskStore) FileConflicts(bugID string) ([]FileConflict, error) {
	team, err := s.Load()
	if err != nil {
		return nil, err
	}
	agent, ok := team.InvestigationAgents[bugID]
	if !ok || len(agent.ClaimedFiles) == 0 {
		return nil, nil
	}
	myFiles := map[string]bool{}
	for _, f := range agent.ClaimedFiles {
		myFiles[f] = true
	}
	var conflicts []FileConflict
	for id, a := range team.InvestigationAgents {
		if id == bugID {
			continue
		}
		var shared []string
		for _, f := range a.ClaimedFiles {
			if myFiles[f] {
				shared = append(shared, f)
			}
		}
		if len(shared) > 0 {
			conflicts = append(conflicts, FileConflict{Agent: a.AgentID, Files: shared})
		}
	}
	for _, a := range team.BuildAgents {
		var shared []string
		for _, f := range a.ClaimedFiles {
			if myFiles[f] {
				shared = append(shared, f)
			}
		}
		if len(shared) > 0 {
			conflicts = append(conflicts, FileConflict{Agent: a.AgentID, Files: shared})
		}
	}
	return conflicts, nil
}

type FileConflict struct {
	Agent string   `json:"agent"`
	Files []string `json:"files"`
}

// syncAgentStatus updates the matching agent entry status when a task status changes.
func (s *TaskStore) syncAgentStatus(team *Team, bugID, status string) {
	if a, ok := team.InvestigationAgents[bugID]; ok {
		a.Status = status
	}
	if a, ok := team.TaskAgents[bugID]; ok {
		a.Status = status
	}
}

func emptyTeam() *Team {
	return &Team{
		Tasks:               map[string]*Task{},
		InvestigationAgents: map[string]*InvestigationAgent{},
		BuildAgents:         map[string]*BuildAgent{},
		TaskAgents:          map[string]*TaskAgent{},
		UtilityAgents:       map[string]*UtilityAgent{},
	}
}
