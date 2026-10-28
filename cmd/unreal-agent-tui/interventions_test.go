package main

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type graphControlWriter struct {
	scope    string
	normal   []inbox.Input
	controls []inbox.Input
}

func (w *graphControlWriter) Submit(_ context.Context, input inbox.Input) error {
	w.normal = append(w.normal, input)
	return nil
}
func (w *graphControlWriter) ControlScope() string { return w.scope }
func (w *graphControlWriter) SubmitIntervention(_ context.Context, input inbox.Input) error {
	w.controls = append(w.controls, input)
	return inbox.ValidateIntervention(input, w.scope)
}

func TestGraphCommandsOnlyExistingOwnerInterventions(t *testing.T) {
	for _, text := range []string{"/graph status checks", "/graph cancel checks", "/graph accept checks test 7 verified output and tests", "/graph reject checks test 7 validation failed"} {
		args, err := parseGraphCommand(text)
		if err != nil || args.Name != "checks" {
			t.Fatalf("%q: %+v %v", text, args, err)
		}
	}
	for _, text := range []string{"/graph", "/graph start checks", "/graph retry checks test", "/graph revise checks", "/graph accept checks test 0 evidence", "/graph accept checks test -1 evidence", "/graph accept checks test 1", "/graph status checks extra", "/graph cancel checks extra", "/graph accept checks test 1 " + strings.Repeat("x", 16000)} {
		if _, err := parseGraphCommand(text); err == nil {
			t.Fatalf("accepted %q", text[:min(len(text), 100)])
		}
	}
}

func TestGraphAndAgentInterventionsUseCapturedGenerationLane(t *testing.T) {
	for _, command := range []string{"/graph status checks", "/graph accept checks test 7 verified tests", "/graph cancel checks", "/agents stop opaque@7", "/agents message opaque@7 please finish"} {
		t.Run(command, func(t *testing.T) {
			m := toolsModel(t, defaultTheme, 100, 30, 0)
			w := &graphControlWriter{scope: "host/3"}
			m.inputs = w
			cmd := m.startSubmission(command)
			if cmd == nil {
				t.Fatal("control not submitted")
			}
			m = submissionUpdate(m, cmd())
			defer func() { m.submissionQueue.Cancel(); m.submissionQueue.Wait(context.Background()) }()
			if len(w.normal) != 0 || len(w.controls) != 1 {
				t.Fatalf("normal=%d controls=%d", len(w.normal), len(w.controls))
			}
			input := w.controls[0]
			control, err := input.DecodeControlMessage()
			if err != nil {
				t.Fatal(err)
			}
			invocation := control.Parameters.(inbox.ToolInvocation)
			if invocation.Scope != "host/3" || invocation.Call.CallID != string(input.ID) {
				t.Fatalf("scope/identity lost: %+v", invocation)
			}
			if strings.HasPrefix(command, "/graph") && invocation.Call.Name != tool.TaskGraphName {
				t.Fatalf("wrong tool %s", invocation.Call.Name)
			}
			if err := inbox.ValidateIntervention(input, "host/4"); err == nil {
				t.Fatal("old generation retargeted")
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(invocation.Call.Arguments), &args); err != nil {
				t.Fatal(err)
			}
			if args["action"] == "accept" && args["evidence"] != "verified tests" {
				t.Fatalf("evidence lost: %+v", args)
			}
		})
	}
}

func TestGraphControlRequiresLiveScopeAndNeverBecomesPrompt(t *testing.T) {
	m := toolsModel(t, defaultTheme, 100, 30, 0)
	w := &graphControlWriter{}
	m.inputs = w
	if cmd := m.startSubmission("/graph accept checks test 1 checked"); cmd != nil {
		t.Fatal("control without scope was scheduled")
	}
	if len(w.normal) != 0 || len(w.controls) != 0 || len(m.pendingSubmissions) != 0 {
		t.Fatal("invalid control became input")
	}
}
