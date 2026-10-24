//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package taskgraph

import (
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"context"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

type artifactAgent struct {
	updates chan operation.Operation
	full    string
}

func (h *artifactAgent) RemoteJobPlanType() operation.RemoteJobPlanType {
	return toolsubagent.RunPlanType
}
func (h *artifactAgent) RemoteJobPlanVersion() operation.RemoteJobPlanVersion {
	return toolsubagent.PlanVersion
}
func (h *artifactAgent) RemoteJobUpdates() <-chan operation.Operation { return h.updates }
func (h *artifactAgent) CancelRemoteJob(operation.ID, string) error   { return nil }
func (h *artifactAgent) AddRemoteJob(op operation.Operation) error {
	s, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		return e
	}
	s.TerminalResult = h.full
	step, e := operation.UpdateRemoteJob(op, s, operation.StatusCompleted)
	if e != nil {
		return e
	}
	h.updates <- *step.Operation
	return nil
}

func TestNestedGraphAgentArtifactDefaults(t *testing.T) {
	base := t.TempDir()
	full := strings.Repeat("nested agent full result\n", 4000)
	r, e := New(t.Context(), Config{Workspace: t.TempDir(), BaseDirectory: base, NewAgentHandlers: func(context.Context) ([]operation.RemoteJobHandler, error) {
		return []operation.RemoteJobHandler{&artifactAgent{updates: make(chan operation.Operation, 1), full: full}}, nil
	}})
	if e != nil {
		t.Fatal(e)
	}
	m := operation.NewLocalOperationManagerWithOptions(t.Context(), operation.LocalOperationManagerOptions{OutputDirectory: base}, r.Handlers()...)
	op := runtimeOperation(t, "graph-owner", RunPlanType, RunPlan{Name: "artifacts", Tasks: []Task{{ID: "agent", Prompt: "produce candidate", Reads: []string{}, Writes: []string{}, Acceptance: "parent checks candidate"}}})
	if e = m.Add(op); e != nil {
		t.Fatal(e)
	}
	op = runtimeWait(t, m, func(op operation.Operation) bool {
		g := runtimeGraph(t, op)
		return g.Find("agent").Status == Completed
	})
	remote, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		t.Fatal(e)
	}
	var cp checkpoint
	// Decode the actual nested child checkpoint and recover its full result.
	if e = json.Unmarshal(remote.Handle, &cp); e != nil {
		t.Fatal(e)
	}
	child := cp.Attempts["agent"].ChildOperation
	childState, e := operation.DecodeRemoteJobState(child)
	if e != nil {
		t.Fatal(e)
	}
	if childState.OutputDirectory != base || childState.ResultPath == "" || !strings.Contains(childState.TerminalResult, childState.ResultPath) {
		t.Fatalf("nested manager not configured: %#v", childState)
	}
	b, e := os.ReadFile(childState.ResultPath)
	if e != nil || string(b) != full {
		t.Fatalf("nested recovery failed: %v", e)
	}
}
