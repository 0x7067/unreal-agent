package taskgraph

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

const DurablePlanVersion operation.RemoteJobPlanVersion = 2

type DurableRunPlan struct {
	Version  int        `json:"version"` // 1
	ParentID session.ID `json:"parent_id"`
	Run      RunPlan    `json:"run"`
}
type DurableControlPlan struct {
	Version  int         `json:"version"` // 1
	ParentID session.ID  `json:"parent_id"`
	Scope    string      `json:"scope"`
	Control  ControlPlan `json:"control"`
}
type DurableExecution struct {
	Workspace     string `json:"workspace"`
	Shell         string `json:"shell"`
	BaseDirectory string `json:"base_directory"`
}
type DurableCheckpoint struct {
	Version   int              `json:"version"` // 2; the nested reducer checkpoint remains version 1.
	ParentID  session.ID       `json:"parent_id"`
	Scope     string           `json:"scope"`
	State     checkpoint       `json:"state"`
	Execution DurableExecution `json:"execution"`
}

func durableIdentity(s string) bool {
	return strings.TrimSpace(s) == s && s != "" && len(s) <= 256 && !strings.ContainsAny(s, "/\\") && s != "." && s != ".." && !strings.ContainsFunc(s, unicode.IsControl)
}
func strictDurable(data []byte, out any) error {
	return json.Unmarshal(data, out, json.RejectUnknownMembers(true))
}
func durableEnvelope(op operation.Operation, typ operation.RemoteJobPlanType) (operation.RemoteJobState, error) {
	if len(op.State) > 16<<20 {
		return operation.RemoteJobState{}, errors.New("durable graph state exceeds 16 MiB")
	}
	switch op.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled:
	default:
		return operation.RemoteJobState{}, errors.New("unknown durable operation status")
	}
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return state, err
	}
	if err = strictDurable(op.State, &state); err != nil {
		return state, err
	}
	if state.Plan.Type != typ || state.Plan.Version != DurablePlanVersion {
		return state, errors.New("unsupported durable graph plan type or version")
	}
	if state.ResultBytes < 0 || state.ErrorBytes < 0 {
		return state, errors.New("negative remote output metadata")
	}
	return state, nil
}
func durableCommands(tasks []Task) error {
	if len(tasks) > 128 {
		return errors.New("durable graph exceeds 128 tasks")
	}
	for _, t := range tasks {
		if strings.TrimSpace(t.Command) == "" || t.Prompt != "" {
			return fmt.Errorf("task %s: durable recovery supports command-only tasks", t.ID)
		}
	}
	return nil
}

// DecodeDurableRun validates an isolated, freshly decoded plan and optional
// checkpoint. It performs no I/O and does not authorize a particular parent.
func DecodeDurableRun(op operation.Operation) (DurableRunPlan, *DurableCheckpoint, error) {
	var p DurableRunPlan
	state, err := durableEnvelope(op, RunPlanType)
	if err != nil {
		return p, nil, err
	}
	if err = strictDurable(state.Plan.Data, &p); err != nil {
		return p, nil, err
	}
	if p.Version != 1 || !durableIdentity(string(p.ParentID)) {
		return p, nil, errors.New("invalid durable run version or parent")
	}
	if err = durableCommands(p.Run.Tasks); err != nil {
		return p, nil, err
	}
	if _, err = NewGraph(p.Run.Name, p.Run.Tasks, p.Run.Concurrency); err != nil {
		return p, nil, err
	}
	if len(state.Handle) == 0 {
		return p, nil, nil
	}
	var cp DurableCheckpoint
	if err = strictDurable(state.Handle, &cp); err != nil {
		return p, nil, err
	}
	if cp.Version != 2 || cp.ParentID != p.ParentID || !durableIdentity(cp.Scope) || cp.State.Version != 1 {
		return p, nil, errors.New("invalid durable checkpoint version, parent, or scope")
	}
	if !filepath.IsAbs(cp.Execution.Workspace) || !filepath.IsAbs(cp.Execution.BaseDirectory) || strings.TrimSpace(cp.Execution.Shell) == "" {
		return p, nil, errors.New("missing durable execution configuration")
	}
	if len(cp.State.Graph.Nodes) > 128 || len(cp.State.Attempts) > 128 {
		return p, nil, errors.New("durable graph exceeds 128 tasks")
	}
	if err = cp.State.Graph.Validate(); err != nil {
		return p, nil, err
	}
	if cp.State.Graph.Name != p.Run.Name {
		return p, nil, errors.New("durable checkpoint graph name mismatch")
	}
	for _, n := range cp.State.Graph.Nodes {
		if err = durableCommands([]Task{n.Task}); err != nil {
			return p, nil, err
		}
	}
	if err = validateAttempts(cp.State); err != nil {
		return p, nil, err
	}
	if op.Status == operation.StatusCompleted && !cp.State.Graph.AllAccepted() {
		return p, nil, errors.New("completed graph has unaccepted work")
	}
	// Validate even historical children, not only attempts in the current generation.
	for id, a := range cp.State.Attempts {
		if a == nil {
			return p, nil, fmt.Errorf("task %s: nil attempt", id)
		}
		if cp.State.Graph.Find(id) == nil {
			return p, nil, fmt.Errorf("unknown attempt task %s", id)
		}
		if a.ChildOperation.ID != graphChildID(op.ID, id, a.Generation) {
			return p, nil, fmt.Errorf("task %s: child identity does not bind owner/task/generation", id)
		}
		child, decodeErr := durableShell(a.ChildOperation)
		if decodeErr != nil {
			err = decodeErr
		} else if child.Input.Directory != cp.Execution.Workspace || child.Input.Shell != cp.Execution.Shell || child.BaseDirectory != cp.Execution.BaseDirectory {
			err = errors.New("child execution configuration mismatch")
		} else if n := cp.State.Graph.Find(id); n != nil && n.Generation == a.Generation && child.Input.Command != n.Task.Command {
			err = errors.New("child command/declaration mismatch")
		}
		if err != nil {
			return p, nil, fmt.Errorf("task %s: %w", id, err)
		}
	}
	return p, &cp, nil
}

func DecodeDurableControl(op operation.Operation) (DurableControlPlan, error) {
	var p DurableControlPlan
	state, err := durableEnvelope(op, ControlPlanType)
	if err != nil {
		return p, err
	}
	if err = strictDurable(state.Plan.Data, &p); err != nil {
		return p, err
	}
	if p.Version != 1 || !durableIdentity(string(p.ParentID)) || !durableIdentity(p.Scope) || strings.TrimSpace(p.Control.Name) == "" {
		return p, errors.New("invalid durable control identity or version")
	}
	c := p.Control
	switch c.Action {
	case "revise":
		if len(c.Tasks) == 0 || c.TaskID != "" || c.Generation != 0 || c.Evidence != "" {
			return p, errors.New("invalid revise syntax")
		}
		if err = durableCommands(c.Tasks); err != nil {
			return p, err
		}
		tasks := make([]Task, len(c.Tasks))
		for i, t := range c.Tasks {
			seen := map[string]bool{}
			for _, dep := range t.DependsOn {
				if strings.TrimSpace(dep) == "" || dep == t.ID || seen[dep] {
					return p, errors.New("invalid revision dependency")
				}
				seen[dep] = true
			}
			tasks[i] = t
			tasks[i].DependsOn = nil
		}
		// Existing graph dependencies and claims are checked by the runtime, not here.
		for _, t := range tasks {
			if _, err = NewGraph(c.Name, []Task{t}, 0); err != nil {
				return p, err
			}
		}
		ids := map[string]bool{}
		for _, t := range tasks {
			if ids[t.ID] {
				return p, errors.New("duplicate revision task")
			}
			ids[t.ID] = true
		}
	case "accept", "reject":
		if strings.TrimSpace(c.TaskID) == "" || c.Generation == 0 || strings.TrimSpace(c.Evidence) == "" || len(c.Tasks) != 0 {
			return p, errors.New("invalid acceptance syntax")
		}
	case "invalidate", "retry":
		if strings.TrimSpace(c.TaskID) == "" || c.Generation != 0 || c.Evidence != "" || len(c.Tasks) != 0 {
			return p, errors.New("invalid task control syntax")
		}
	case "cancel", "status":
		if c.Generation != 0 || c.Evidence != "" || len(c.Tasks) != 0 || (c.Action == "status" && c.TaskID != "") {
			return p, errors.New("invalid graph control syntax")
		}
	default:
		return p, errors.New("unknown durable control action")
	}
	return p, nil
}

func durableShell(op operation.Operation) (operation.ShellState, error) {
	s, err := operation.DecodeShellState(op)
	if err != nil {
		return s, err
	}
	if err = strictDurable(op.State, &s); err != nil {
		return s, err
	}
	if _, err = operation.NewShell(op); err != nil {
		return s, err
	}
	if !durableIdentity(string(op.ID)) {
		return s, errors.New("invalid shell child identity")
	}
	return s, nil
}
func durableTerminalShell(op operation.Operation, s operation.ShellState) bool {
	if s.Phase != "" || s.ProcessGroupID != 0 || s.PendingExitCode != nil || s.Result == nil || s.Result.OutSize < 0 || s.Result.ErrSize < 0 {
		return false
	}
	switch op.Status {
	case operation.StatusCompleted:
		return s.TerminalError == ""
	case operation.StatusFailed, operation.StatusCanceled:
		return strings.TrimSpace(s.TerminalError) != ""
	default:
		return false
	}
}

// ValidateDurableRecovery is a fail-closed admission check, not a process probe
// or permission to replay a dispatched command. Only a terminal recorded child
// or the explicit store-ack prelaunch boundary supplies recovery evidence.
func ValidateDurableRecovery(op operation.Operation, expectedParent session.ID) error {
	p, cp, err := DecodeDurableRun(op)
	if err != nil {
		return err
	}
	if !durableIdentity(string(expectedParent)) || p.ParentID != expectedParent {
		return errors.New("durable recovery parent mismatch")
	}
	if cp == nil {
		if op.Status == operation.StatusReady || op.Status == operation.StatusAwaiting {
			return nil
		}
		return errors.New("durable recovery lacks checkpoint proof")
	}
	for _, n := range cp.State.Graph.Nodes {
		a := cp.State.Attempts[n.Task.ID]
		if a == nil {
			continue
		} // Graph/attempt validation already requires all active/candidate attempts.
		s, err := durableShell(a.ChildOperation)
		if err != nil {
			return err
		}
		if a.Generation == n.Generation && a.Phase == "input_validation" && !a.CommandDispatched && a.ChildOperation.Status == operation.StatusReady && initialShellState(s) {
			continue
		}
		if !durableTerminalShell(a.ChildOperation, s) {
			return fmt.Errorf("task %s: dispatched shell outcome is not proven terminal", n.Task.ID)
		}
		if a.Generation == n.Generation && (n.Status == Accepted || n.Status == Completed) && (a.ChildOperation.Status != operation.StatusCompleted || s.Result.ExitCode != 0) {
			return fmt.Errorf("task %s: candidate child did not succeed", n.Task.ID)
		}
	}
	return nil
}
