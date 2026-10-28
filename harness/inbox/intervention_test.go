package inbox

import (
	"encoding/json/v2"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"testing"
)

func TestScopedInterventionsCannotLaunchOrCrossGeneration(t *testing.T) {
	for _, tc := range []struct {
		name, tool, args, scope string
		valid                   bool
	}{
		{"accept", "TaskGraph", `{"action":"accept","name":"g","task_id":"a","generation":2,"evidence":"fresh"}`, "host/1", true},
		{"status", "TaskGraph", `{"action":"status","name":"g"}`, "host/1", true},
		{"cancel", "TaskGraph", `{"action":"cancel","name":"g"}`, "host/1", true},
		{"launch", "TaskGraph", `{"action":"start","name":"g"}`, "host/1", false},
		{"retry", "TaskGraph", `{"action":"retry","name":"g","task_id":"a"}`, "host/1", false},
		{"revision", "TaskGraph", `{"action":"accept","name":"g","task_id":"a","generation":2,"evidence":"fresh","tasks":[]}`, "host/1", false},
		{"missing_generation", "TaskGraph", `{"action":"accept","name":"g","task_id":"a","evidence":"fresh"}`, "host/1", false},
		{"stale_scope", "TaskGraph", `{"action":"status","name":"g"}`, "host/0", false},
		{"no_scope", "TaskGraph", `{"action":"status","name":"g"}`, "", false},
		{"agent_message", "AgentControl", `{"action":"message","handle":"opaque","generation":1,"message":"finish"}`, "host/1", true},
		{"agent_missing_generation", "AgentControl", `{"action":"stop","handle":"opaque"}`, "host/1", false},
		{"ordinary_tool", "Bash", `{"command":"true"}`, "host/1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(ControlMessage{Mode: InvokeTool, Parameters: ToolInvocation{Scope: tc.scope, Call: llm.ToolCall{Name: tc.tool, CallID: "call", Arguments: tc.args}}})
			err := ValidateIntervention(Input{ID: "id", Kind: InputControl, Payload: payload}, "host/1")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}
