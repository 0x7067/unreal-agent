package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"strings"
	"testing"
	"time"
)

func TestLateReplayCannotResurrectTerminalTool(t *testing.T) {
	m := toolsModel(t, defaultTheme, 54, 20, 0)
	m.registry = newUIRegistry(true, true)
	response := sessionstore.Item{Sequence: 1, Data: sessionstore.ModelResponse{TurnID: "turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: tool.BashName, Arguments: `{"command":"quiet-command"}`}}}}}}
	m.apply(response)
	op := workerResultOperation(t, tool.BashName, operation.StatusCompleted, t.TempDir(), "quiet-command")
	completed := sessionstore.Item{Sequence: 3, Data: sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Operations: []operation.Operation{op}}}
	m.apply(completed)
	op.Status = operation.StatusAwaiting
	late := sessionstore.Item{Sequence: 2, Data: sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Operations: []operation.Operation{op}}}
	m.apply(late)
	m.apply(response)
	m.apply(completed)
	if len(m.tools) != 1 || m.tools[0].status != "Completed" {
		t.Fatalf("resurrected: %+v", m.tools)
	}
	// An erroneously re-emitted running snapshot with a NEW envelope sequence
	// also cannot revive the same terminal operation/call identity.
	late.Sequence = 4
	m.apply(late)
	if m.tools[0].status != "Completed" {
		t.Fatal("new envelope resurrected terminal call")
	}
	m.toolsCollapsed = true
	if strings.Contains(ansi.Strip(m.View().Content), "Waiting for tools") {
		t.Fatal("historical terminal card caused waiting")
	}
}

func TestAgentCommandsAreExplicitAndBounded(t *testing.T) {
	for _, text := range []string{"/agents", "/agents list", "/agents inspect scope/child@1", "/agents stop scope/child@1", "/agents message scope/child@1 review feedback"} {
		command, err := parseAgentCommand(text)
		if err != nil || command.action == "" {
			t.Fatalf("%q: %+v %v", text, command, err)
		}
	}
	for _, text := range []string{"/agents stop", "/agents inspect child extra", "/agents message child", "/agents list extra", "/agents unknown", "/agents message child " + strings.Repeat("x", 16001)} {
		if _, err := parseAgentCommand(text); err == nil {
			t.Fatalf("accepted %q", text[:min(80, len(text))])
		}
	}
}

func TestExecutableSummariesOmitSecretsAndShellSyntax(t *testing.T) {
	for _, command := range []string{"curl --header Authorization:secret-value", "TOKEN=secret-value curl", "sh -c 'echo secret-value'", "$(secret-value)", "printf secret-value"} {
		got := safeExecutableSummary(command)
		if strings.Contains(got, "secret-value") || !strings.Contains(got, "hidden") {
			t.Fatalf("unsafe summary %q", got)
		}
	}
}

func TestActivitySeparatesTextFromDrainAndQuietFromStall(t *testing.T) {
	now := time.Unix(100, 0)
	child := subagent.ChildActivity{Name: "reviewer", AttemptedAt: now.Add(-34 * time.Minute), ProcessStartedAt: now.Add(-34 * time.Minute), ModelTextObserved: true, LastFrameAt: now.Add(-time.Minute), ObservedNonterminalOperations: 1, RelevantOperation: &subagent.OperationActivity{Type: operation.Type("shell"), Status: operation.StatusAwaiting, FirstObservedAt: now.Add(-34 * time.Minute)}}
	line := childActivityLine(child, now)
	for _, part := range []string{"execution still owned", "reviewer", "34m", "1 pending", "shell"} {
		if !strings.Contains(strings.ToLower(line), part) {
			t.Fatalf("missing %q in %s", part, line)
		}
	}
	if strings.Contains(line, "stall") || strings.Contains(line, "Idle") {
		t.Fatalf("false diagnosis: %s", line)
	}
	child.CancellationRequested = true
	if !strings.Contains(childActivityPhase(child), "waiting for owned execution drain") {
		t.Fatal("stop requested claimed drain")
	}
	child.ProcessWaitReturned = true
	if strings.Contains(childActivityPhase(child), "Execution drained") {
		t.Fatal("Wait alone claimed drain")
	}
	child.ReaderReturned = true
	child.OutputEOFObserved = true
	if !strings.Contains(childActivityPhase(child), "publication pending") {
		t.Fatal("process drain claimed publication")
	}
}

func TestGraphValidationDiagnosticIsNotTestExecutionOrRawCommand(t *testing.T) {
	plan, _ := json.Marshal(graph.RunPlan{Name: "build"})
	checkpoint := `{"version":2,"graph":{"name":"build","nodes":[{"task":{"id":"tests","command":"secret-command"},"generation":3,"status":"running"}]},"attempts":{"tests":{"generation":3,"phase":"input_validation","phase_started_at":"2026-10-27T15:00:00Z","command_dispatched":false}}}`
	state, _ := json.Marshal(operation.RemoteJobState{Plan: operation.RemoteJobPlan{Type: graph.RunPlanType, Version: graph.PlanVersion, Data: plan}, Handle: jsontext.Value(checkpoint)})
	got := toolDiagnostic(sessionstore.ToolCallStatus{Operations: []operation.Operation{{MaxOutputLength: 40000, Type: operation.TypeRemoteJob, Version: operation.VersionRemoteJob, Status: operation.StatusAwaiting, State: state}}})
	for _, part := range []string{"build/tests g3", "input validation", "command not dispatched"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %q: %s", part, got)
		}
	}
	if strings.Contains(got, "secret-command") || strings.Contains(got, "tests failed") {
		t.Fatalf("unsafe or false diagnostic: %s", got)
	}
}

func TestActivitySnapshotCannotFalseIdleOrResurrectAfterTerminal(t *testing.T) {
	for _, width := range []int{48, 160} {
		m := toolsModel(t, defaultTheme, width, 24, 0)
		m.workerGeneration = 2
		m.toolsCollapsed = true
		now := time.Now()
		live := subagent.AgentActivity{ChildActivity: subagent.ChildActivity{Name: "reviewer", Generation: 3, AttemptedAt: now.Add(-time.Minute), ProcessStartedAt: now.Add(-time.Minute), ReportAvailable: true, ObservedNonterminalOperations: 1, RelevantOperation: &subagent.OperationActivity{Type: operation.TypeShell, Status: operation.StatusAwaiting, ShellPhase: operation.ShellPhaseProcess, Executable: "find", FirstObservedAt: now.Add(-time.Minute)}}, Handle: "opaque", Phase: "report_available"}
		snapshot := workerhost.Activity{Generation: 2, Snapshot: subagent.FleetSnapshot{Version: 1, CapturedAt: now, Agents: []subagent.AgentActivity{live}}}
		m.applyActivity(activityFinished{activity: snapshot})
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "cleanup pending") || strings.Contains(view, "Idle") {
			t.Fatalf("false idle at width%d: %s", width, view)
		}
		m.applyActivity(activityFinished{err: fmt.Errorf("refresh unavailable")})
		if !strings.Contains(m.activitySummary(), "reviewer") {
			t.Fatal("query failure forgot live ownership")
		}
		done := snapshot
		done.Snapshot.Agents = append([]subagent.AgentActivity(nil), snapshot.Snapshot.Agents...)
		done.Snapshot.CapturedAt = now.Add(time.Second)
		done.Snapshot.Agents[0].OwnerTerminalEnqueued = true
		m.applyActivity(activityFinished{activity: done})
		if m.activitySummary() != "" {
			t.Fatal("terminal agent remained active")
		}
		m.applyActivity(activityFinished{activity: snapshot})
		if m.activitySummary() != "" {
			t.Fatal("late snapshot resurrected terminal agent")
		}
		snapshot.Snapshot.CapturedAt = now.Add(2 * time.Second)
		m.applyActivity(activityFinished{activity: snapshot})
		if m.activitySummary() != "" {
			t.Fatal("replayed live attempt resurrected despite new capture timestamp")
		}
		snapshot.Generation = 1
		m.applyActivity(activityFinished{activity: snapshot})
		if m.activity.Generation != 2 {
			t.Fatal("old worker snapshot replaced newer generation")
		}
	}
}

func TestAgentStructuralInputHasStableIdentityAndHonestAdmission(t *testing.T) {
	m := toolsModel(t, defaultTheme, 100, 30, 0)
	var admitted inbox.Input
	m.inputs = submissionWriter(func(_ context.Context, input inbox.Input) error { admitted = input; return nil })
	cmd := m.startSubmission("/agents stop opaque@7")
	if cmd == nil {
		t.Fatal("missing durable admission command")
	}
	m = submissionUpdate(m, cmd())
	defer func() { m.submissionQueue.Cancel(); m.submissionQueue.Wait(context.Background()) }()
	if err := admitted.Validate(); err != nil {
		t.Fatal(err)
	}
	control, err := admitted.DecodeControlMessage()
	if err != nil {
		t.Fatal(err)
	}
	invocation := control.Parameters.(inbox.ToolInvocation)
	if control.Mode != inbox.InvokeTool || invocation.Call.CallID != string(admitted.ID) || invocation.Call.Name != tool.AgentControlName {
		t.Fatalf("invalid structural identity: %+v", admitted)
	}
	var plan struct {
		Action     string `json:"action"`
		Handle     string `json:"handle"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal([]byte(invocation.Call.Arguments), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Action != "stop" || plan.Handle != "opaque" || plan.Generation != 7 {
		t.Fatalf("bad plan %+v", plan)
	}
	if !strings.Contains(m.lines[len(m.lines)-1].text, "not consumption or drain") {
		t.Fatal("admission implied completion")
	}
	result, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ToolInvocationFinished, Parameters: inbox.ToolInvocationResult{InputID: admitted.ID, CallID: invocation.Call.CallID, Status: "failed", Error: "unknown, stale-generation or cross-session agent handle"}})
	m.apply(sessionstore.Item{Sequence: 1, Data: inbox.Input{ID: "outcome", Kind: inbox.InputControl, Payload: result}})
	if !strings.Contains(m.lines[len(m.lines)-1].text, "stale-generation") {
		t.Fatal("terminal control failure not surfaced")
	}
}

func TestNestedToolSummariesShowArguments(t *testing.T) {
	for _, tc := range []struct{ name, args string }{{"Agent", `{"name":"reviewer","prompt":"secret-value"}`}, {"SendMessage", `{"to":"reviewer","message":"secret-value"}`}, {"AgentControl", `{"action":"message","handle":"opaque","message":"secret-value"}`}, {"TaskGraph", `{"action":"start","name":"validation","tasks":[{"command":"secret-value"}]}`}} {
		if got := toolSummary(tc.name, tc.args); got != tc.args {
			t.Fatalf("%s summary = %q, want %q", tc.name, got, tc.args)
		}
	}
}

func TestStructuralToolCardShowsArguments(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	call := llm.ToolCall{CallID: "control", Name: "AgentControl", Arguments: `{"action":"message","handle":"reviewer","message":"check the sidebar"}`}
	m.structuralCalls = map[string]llm.ToolCall{call.CallID: call}
	m.apply(sessionstore.Item{Data: sessionstore.ToolCallStatus{TurnID: "turn", CallID: call.CallID}})
	if len(m.tools) != 1 || m.tools[0].summary != call.Arguments {
		t.Fatalf("structural tool arguments not shown: %+v", m.tools)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), `"action":"message"`) {
		t.Fatal("structural tool arguments missing from sidebar")
	}
}
