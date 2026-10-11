package taskgraph

import "github.com/unreallabsai/unreal-agent/harness/operation"

const (
	RunPlanType     operation.RemoteJobPlanType    = "taskgraph.run"
	ControlPlanType operation.RemoteJobPlanType    = "taskgraph.control"
	PlanVersion     operation.RemoteJobPlanVersion = 1
)

// RunPlan is immutable; the graph's mutable state lives in its remote-job handle.
type RunPlan struct {
	Name        string `json:"name"`
	Tasks       []Task `json:"tasks"`
	Concurrency int    `json:"concurrency,omitempty"`
}

// ControlPlan applies one atomic action to an existing named graph.
type ControlPlan struct {
	Name       string `json:"name"`
	Action     string `json:"action"`
	Tasks      []Task `json:"tasks,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	Generation uint64 `json:"generation,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
}
