package taskgraph

import (
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func TestGraphChildCarriesImmutableAttemptMetadata(t *testing.T) {
	s := &runtimeState{runtime: &Runtime{config: Config{}}}
	g := &graphExecution{checkpoint: checkpoint{Graph: Graph{Name: "build"}}, owner: operation.Operation{ID: "graph-operation"}}
	n := &Node{Task: Task{ID: "review", Description: "review changed paths", Prompt: "inspect candidate", Writes: []string{"report.txt"}}, Generation: 3}
	child, err := s.newChild(g, n)
	if err != nil {
		t.Fatal(err)
	}
	state, err := operation.DecodeRemoteJobState(child)
	if err != nil {
		t.Fatal(err)
	}
	var plan toolsubagent.RunPlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.GraphID != "build" || plan.TaskID != "review" || plan.TaskGeneration != 3 || plan.Lifetime != "task" || plan.ParentTask != "build/review" || plan.Purpose != "review changed paths" || !reflect.DeepEqual(plan.OwnedResources, []string{"report.txt"}) {
		t.Fatalf("metadata=%+v", plan)
	}
	n.Generation++
	n.Task.Writes[0] = "other.txt"
	if plan.TaskGeneration != 3 || plan.OwnedResources[0] != "report.txt" {
		t.Fatal("metadata changed with live task")
	}
	replacement, err := s.newChild(g, n)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.ID == child.ID {
		t.Fatal("replacement reused attempt identity")
	}
}

func TestGraphChildMetadataPreservesExplicitReadOnlyResources(t *testing.T) {
	s := &runtimeState{runtime: &Runtime{}}
	g := &graphExecution{checkpoint: checkpoint{Graph: Graph{Name: "review"}}, owner: operation.Operation{ID: "owner"}}
	child, err := s.newChild(g, &Node{Task: Task{ID: "readonly", Prompt: "review only", Writes: []string{}}, Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	state, err := operation.DecodeRemoteJobState(child)
	if err != nil {
		t.Fatal(err)
	}
	var plan toolsubagent.RunPlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.OwnedResources == nil || len(plan.OwnedResources) != 0 {
		t.Fatalf("explicit no owned resources lost: %+v", plan)
	}
}
