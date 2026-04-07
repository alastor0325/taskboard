package types

import "github.com/alastor0325/taskboard/internal/store"

type LogEntry struct {
	Time    float64 `json:"time"`
	Agent   string  `json:"agent"`
	Message string  `json:"message"`
}

type BtwEntry struct {
	Time    float64 `json:"time"`
	Agent   string  `json:"agent"`
	Message string  `json:"message"`
}

type AgentStatus struct {
	Project     string                       `json:"project"`
	UpdatedAt   float64                      `json:"updated_at"`
	Tasks       map[string]*store.Task       `json:"tasks"`
	BuildAgents map[string]*store.BuildAgent `json:"build_agents"`
	Log         []LogEntry                   `json:"log"`
	Btw         []BtwEntry                   `json:"btw"`
}
