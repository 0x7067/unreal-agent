package inbox

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestStructuralControlEnvelopeValidation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       ControlMode
		parameters any
		valid      bool
	}{
		{"list", InvokeTool, ToolInvocation{Call: llm.ToolCall{CallID: "id", Name: "AgentControl", Arguments: `{"action":"list"}`}}, true},
		{"arbitrary tool", InvokeTool, ToolInvocation{Call: llm.ToolCall{CallID: "id", Name: "Bash", Arguments: `{}`}}, false},
		{"missing identity", InvokeTool, ToolInvocation{Call: llm.ToolCall{Name: "AgentControl", Arguments: `{}`}}, false},
		{"malformed arguments", InvokeTool, ToolInvocation{Call: llm.ToolCall{CallID: "id", Name: "AgentControl", Arguments: `{`}}, false},
		{"oversize arguments", InvokeTool, ToolInvocation{Call: llm.ToolCall{CallID: "id", Name: "AgentControl", Arguments: `"` + strings.Repeat("x", 64<<10) + `"`}}, false},
		{"completed outcome", ToolInvocationFinished, ToolInvocationResult{InputID: "input", CallID: "call", Status: "completed"}, true},
		{"canceled outcome", ToolInvocationFinished, ToolInvocationResult{InputID: "input", CallID: "call", Status: "canceled"}, true},
		{"pending is not outcome", ToolInvocationFinished, ToolInvocationResult{InputID: "input", CallID: "call", Status: "awaiting"}, false},
		{"outcome missing input", ToolInvocationFinished, ToolInvocationResult{CallID: "call", Status: "completed"}, false},
		{"oversize outcome", ToolInvocationFinished, ToolInvocationResult{InputID: "input", CallID: "call", Status: "completed", Text: strings.Repeat("x", 4097)}, false},
		{"old mode cannot carry invocation", StopHard, ToolInvocation{Call: llm.ToolCall{CallID: "id", Name: "AgentControl", Arguments: `{}`}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(ControlMessage{Mode: tc.mode, Parameters: tc.parameters})
			if err != nil {
				t.Fatal(err)
			}
			_, err = (Input{Kind: InputControl, Payload: payload}).DecodeControlMessage()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t: %v", tc.valid, err)
			}
		})
	}
}
