package attachproto

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func interventionInput(t *testing.T, tool, arguments, scope string) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Scope: scope, Call: llm.ToolCall{Name: tool, CallID: "call", Arguments: arguments}}})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: "stable", Kind: inbox.InputControl, Payload: payload}
}

func TestInterventionStrictScopedBoundary(t *testing.T) {
	for _, tc := range []struct{ tool, arguments string }{
		{"AgentControl", `{"action":"list"}`},
		{"AgentControl", `{"action":"status","handle":"child","generation":1}`},
		{"AgentControl", `{"action":"stop","handle":"child","generation":1}`},
		{"AgentControl", `{"action":"message","handle":"child","generation":1,"message":"finish"}`},
		{"TaskGraph", `{"action":"status","name":"graph"}`},
		{"TaskGraph", `{"action":"cancel","name":"graph"}`},
		{"TaskGraph", `{"action":"accept","name":"graph","task_id":"task","generation":1,"evidence":"validated"}`},
		{"TaskGraph", `{"action":"reject","name":"graph","task_id":"task","generation":1,"evidence":"failed"}`},
	} {
		in := interventionInput(t, tc.tool, tc.arguments, "host/7")
		if err := ValidateIntervention(in, "host/7"); err != nil {
			t.Fatalf("%s %s: %v", tc.tool, tc.arguments, err)
		}
		for _, scope := range []string{"", "host/6", "other/7", " host/7"} {
			if ValidateIntervention(in, scope) == nil {
				t.Fatalf("accepted scope %q", scope)
			}
		}
	}
	valid := interventionInput(t, "AgentControl", `{"action":"list"}`, "host/7")
	for name, in := range map[string]inbox.Input{
		"blank ID":           {ID: " \t", Kind: valid.Kind, Payload: valid.Payload},
		"long ID":            {ID: inbox.ID(strings.Repeat("i", 129)), Kind: valid.Kind, Payload: valid.Payload},
		"external":           {ID: valid.ID, Kind: inbox.InputExternal, Payload: valid.Payload},
		"crash":              {ID: valid.ID, Kind: inbox.InputCrash, Payload: valid.Payload},
		"missing scope":      interventionInput(t, "AgentControl", `{"action":"list"}`, ""),
		"launch":             interventionInput(t, "AgentControl", `{"action":"launch","goal":"work"}`, "host/7"),
		"launch fields":      interventionInput(t, "AgentControl", `{"action":"list","goal":"work"}`, "host/7"),
		"retry":              interventionInput(t, "TaskGraph", `{"action":"retry","name":"graph"}`, "host/7"),
		"revise":             interventionInput(t, "TaskGraph", `{"action":"revise","name":"graph"}`, "host/7"),
		"missing generation": interventionInput(t, "AgentControl", `{"action":"stop","handle":"child"}`, "host/7"),
		"unknown tool":       interventionInput(t, "Bash", `{"command":"true"}`, "host/7"),
		"unknown envelope":   {ID: valid.ID, Kind: valid.Kind, Payload: []byte(strings.Replace(string(valid.Payload), `"Mode":`, `"Injected":true,"Mode":`, 1))},
		"unknown invocation": {ID: valid.ID, Kind: valid.Kind, Payload: []byte(strings.Replace(string(valid.Payload), `"Scope":`, `"Injected":true,"Scope":`, 1))},
		"oversize":           {ID: valid.ID, Kind: valid.Kind, Payload: []byte(strings.Repeat(" ", MaxPayload) + string(valid.Payload))},
	} {
		t.Run(name, func(t *testing.T) {
			if ValidateIntervention(in, "host/7") == nil {
				t.Fatal("accepted invalid intervention")
			}
		})
	}
	// The public bound applies to the complete payload, not just tool arguments.
	prefix := `{"Mode":"invoke_tool","Reason":"`
	suffix := `","Parameters":{"Scope":"host/7","Call":{"Name":"AgentControl","CallID":"call","Arguments":"{\"action\":\"list\"}"}}}`
	valid.ID = inbox.ID(strings.Repeat("i", 128))
	valid.Payload = []byte(prefix + strings.Repeat("r", MaxPayload-len(prefix)-len(suffix)) + suffix)
	if err := ValidateIntervention(valid, "host/7"); err != nil {
		t.Fatal(err)
	}
	frame, err := Encode(Request{Type: "intervention", Input: &valid})
	if err != nil || len(frame) > MaxFrame {
		t.Fatalf("frame bounds: %d %v", len(frame), err)
	}
	r, err := ReadRequest(Scanner(strings.NewReader(string(frame))))
	if err != nil || r.Type != "intervention" || ValidateIntervention(*r.Input, "host/7") != nil {
		t.Fatalf("protocol round trip: %+v %v", r, err)
	}
}
