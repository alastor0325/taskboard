package store

// Team is the top-level structure of team.json.
type Team struct {
	Tasks               map[string]*Task               `json:"tasks"`
	InvestigationAgents map[string]*InvestigationAgent `json:"investigation_agents"`
	BuildAgents         map[string]*BuildAgent         `json:"build_agents"`
	TaskAgents          map[string]*TaskAgent          `json:"task_agents"`
	UtilityAgents       map[string]*UtilityAgent       `json:"utility_agents"`
}

type Task struct {
	Summary  string        `json:"summary,omitempty"`
	Status   string        `json:"status,omitempty"`
	Note     string        `json:"note,omitempty"`
	Worktree string        `json:"worktree,omitempty"`
	DoneAt   *float64      `json:"done_at,omitempty"`
	Agents   []AgentRecord `json:"agents,omitempty"`
}

type AgentRecord struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type InvestigationAgent struct {
	AgentID      string   `json:"agent_id,omitempty"`
	Status       string   `json:"status,omitempty"`
	BuildType    string   `json:"build_type,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	ClaimedFiles []string `json:"claimed_files,omitempty"`
	OutputFile   string   `json:"output_file,omitempty"`
}

type BuildAgent struct {
	AgentID      string   `json:"agent_id,omitempty"`
	Status       string   `json:"status,omitempty"`
	ObjDir       string   `json:"obj_dir,omitempty"`
	CurrentBug   *int64   `json:"current_bug,omitempty"`
	Queue        []int64  `json:"queue,omitempty"`
	ClaimedFiles []string `json:"claimed_files,omitempty"`
}

type TaskAgent struct {
	AgentID    string `json:"agent_id,omitempty"`
	Status     string `json:"status,omitempty"`
	AgentName  string `json:"agent_name,omitempty"`
	Goal       string `json:"goal,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
}

type UtilityAgent struct {
	AgentID    string `json:"agent_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Goal       string `json:"goal,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
}
