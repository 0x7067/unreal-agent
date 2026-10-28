package localfile

import (
	"encoding/json/v2"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"testing"
	"time"
)

func TestGraphControlInitializationRequiresExactAdmittedSpecification(t *testing.T) {
	for _, durable := range []bool{false, true} {
		for _, mismatch := range []bool{false, true} {
			head := newSessionHead("s", time.Time{})
			admitted := graph.ControlPlan{Action: "accept", Name: "g", TaskID: "a", Generation: 1, Evidence: "fresh"}
			arguments, _ := json.Marshal(admitted)
			payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Scope: "host/1", Call: llm.ToolCall{Name: "TaskGraph", CallID: "accept", Arguments: string(arguments)}}})
			input := inbox.Input{ID: "input", Kind: inbox.InputControl, Payload: payload}
			if _, err := head.appendInput(input, time.Time{}); err != nil {
				t.Fatal(err)
			}
			specPlan := admitted
			if mismatch {
				specPlan.Generation = 2
			}
			data, _ := json.Marshal(specPlan)
			version := graph.PlanVersion
			if durable {
				version = graph.DurablePlanVersion
				data, _ = json.Marshal(graph.DurableControlPlan{Version: 1, ParentID: session.ID("s"), Scope: "graph-owner", Control: specPlan})
			}
			spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: graph.ControlPlanType, Version: version, Data: data})
			if err != nil {
				t.Fatal(err)
			}
			op := operation.Operation{ID: controlOperationID(input.ID), Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
			err = head.initializeControlOperation(input.ID, op)
			if (err != nil) != mismatch {
				t.Fatalf("durable=%v mismatch=%v err=%v", durable, mismatch, err)
			}
		}
	}
}
