// Package taskgraph translates TaskGraph calls into inert durable remote plans.
package taskgraph

import (
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type arguments struct {
	Action      string       `json:"action"`
	Name        string       `json:"name"`
	Tasks       []graph.Task `json:"tasks,omitempty"`
	Concurrency int          `json:"concurrency,omitempty"`
	TaskID      string       `json:"task_id,omitempty"`
	Generation  uint64       `json:"generation,omitempty"`
	Evidence    string       `json:"evidence,omitempty"`
}

type translator struct{}

func New() tool.Translator { return translator{} }

func (translator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	var args arguments
	if err := json.Unmarshal([]byte(call.Arguments), &args, json.RejectUnknownMembers(true)); err != nil {
		return tool.ErrorStatus(fmt.Sprintf("decode TaskGraph arguments: %v", err), 0)
	}
	if err := validate(args); err != nil {
		return tool.ErrorStatus(err.Error(), 0)
	}
	var plan any
	planType := graph.ControlPlanType
	if args.Action == "start" {
		planType = graph.RunPlanType
		plan = graph.RunPlan{Name: args.Name, Tasks: args.Tasks, Concurrency: args.Concurrency}
	} else {
		plan = graph.ControlPlan{Name: args.Name, Action: args.Action, Tasks: args.Tasks, TaskID: args.TaskID, Generation: args.Generation, Evidence: args.Evidence}
	}
	data, err := json.Marshal(plan, json.FormatNilSliceAsNull(true))
	if err != nil {
		return tool.ErrorStatus(fmt.Sprintf("encode TaskGraph plan: %v", err), 0)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: planType, Version: graph.PlanVersion, Data: data})
	if err != nil {
		return tool.ErrorStatus(fmt.Sprintf("build TaskGraph operation: %v", err), 0)
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(spec)}}
}

func validate(args arguments) error {
	if strings.TrimSpace(args.Name) == "" {
		return fmt.Errorf("TaskGraph requires a name")
	}
	switch args.Action {
	case "start":
		if args.TaskID != "" || args.Generation != 0 || args.Evidence != "" {
			return fmt.Errorf("start does not take task_id, generation, or evidence")
		}
		_, err := graph.NewGraph(args.Name, args.Tasks, args.Concurrency)
		return err
	case "revise":
		if len(args.Tasks) == 0 {
			return fmt.Errorf("revise requires tasks")
		}
		// Existing dependencies cannot be resolved in a pure translator. Check the
		// declarations locally; the runtime validates the entire revised DAG atomically.
		tasks := append([]graph.Task(nil), args.Tasks...)
		for i := range tasks {
			seen := map[string]bool{}
			for _, dep := range tasks[i].DependsOn {
				if strings.TrimSpace(dep) == "" || dep == tasks[i].ID || seen[dep] {
					return fmt.Errorf("task %s has invalid dependency %q", tasks[i].ID, dep)
				}
				seen[dep] = true
			}
			tasks[i].DependsOn = nil
		}
		if _, err := graph.NewGraph(args.Name, tasks, 0); err != nil {
			return err
		}
	case "accept", "reject":
		if strings.TrimSpace(args.TaskID) == "" || args.Generation == 0 || strings.TrimSpace(args.Evidence) == "" {
			return fmt.Errorf("%s requires task_id, current generation, and evidence", args.Action)
		}
	case "invalidate", "retry":
		if strings.TrimSpace(args.TaskID) == "" {
			return fmt.Errorf("%s requires task_id", args.Action)
		}
	case "cancel", "status":
	default:
		return fmt.Errorf("unknown TaskGraph action %q", args.Action)
	}
	if args.Concurrency != 0 {
		return fmt.Errorf("concurrency is only valid for start")
	}
	if args.Action != "revise" && args.Tasks != nil {
		return fmt.Errorf("tasks are only valid for start or revise")
	}
	if args.Action != "accept" && args.Action != "reject" && (args.Generation != 0 || args.Evidence != "") {
		return fmt.Errorf("generation and evidence are only valid for accept or reject")
	}
	if (args.Action == "status" || args.Action == "revise") && args.TaskID != "" {
		return fmt.Errorf("%s does not take task_id", args.Action)
	}
	return nil
}

func (translator) TranslateResult(callID string, status tool.CallStatus, operations []operation.Operation) (tool.Result, error) {
	if status.Error != "" {
		if len(operations) != 0 {
			return nil, fmt.Errorf("TaskGraph call %q has both validation error and operations", callID)
		}
		return Result{CallID: callID, Error: status.Error}, nil
	}
	if len(operations) != 1 {
		return nil, fmt.Errorf("TaskGraph call %q has %d operations, want 1", callID, len(operations))
	}
	current := operations[0]
	state, err := operation.DecodeRemoteJobState(current)
	if err != nil {
		return nil, err
	}
	if (state.Plan.Type != graph.RunPlanType && state.Plan.Type != graph.ControlPlanType) || state.Plan.Version != graph.PlanVersion {
		return nil, fmt.Errorf("TaskGraph call %q has unsupported remote plan %s/%d", callID, state.Plan.Type, state.Plan.Version)
	}
	switch current.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling:
		return Result{CallID: callID, Running: true, Text: "Task graph operation is running. Progress and completed candidates arrive in your inbox. Use TaskGraph status to inspect tasks and their current generation; accept or reject candidates with current evidence while independent tasks continue. A completed candidate is not accepted automatically."}, nil
	case operation.StatusCompleted:
		return Result{CallID: callID, Text: state.TerminalResult}, nil
	case operation.StatusFailed, operation.StatusCanceled:
		if state.TerminalError == "" {
			state.TerminalError = "TaskGraph operation " + string(current.Status)
		}
		return Result{CallID: callID, Text: state.TerminalResult, Error: state.TerminalError}, nil
	default:
		return nil, fmt.Errorf("TaskGraph operation has invalid status %q", current.Status)
	}
}

type Result struct {
	CallID  string
	Running bool
	Text    string
	Error   string
}

func (result Result) ToLLMResult() llm.ToolResult {
	parts := []string{}
	if result.Text != "" {
		parts = append(parts, result.Text)
	}
	if result.Error != "" {
		parts = append(parts, "Error: "+result.Error)
	}
	if len(parts) == 0 {
		parts = append(parts, "(no output)")
	}
	return llm.ToolResult{CallID: result.CallID, Running: result.Running, Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: strings.Join(parts, "\n")}}}
}
