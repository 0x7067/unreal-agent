package workerruntime

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestActivityQueryResponsiveWithoutModelOrBlockedChildIO(t *testing.T) {
	b := bootstrap(t)
	// The child reports a final answer independently of process/output drain,
	// then remains quiet without reading its input. No broad scan is executed.
	report, _ := json.Marshal(sessionstore.Item{Sequence: 1, Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "child", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "review report ready"}}}}}})
	shellState, _ := json.Marshal(operation.ShellState{Input: operation.ShellInput{Command: "find / --secret=never-expose-this", Shell: "/bin/sh", Directory: b.Workspace}, Phase: operation.ShellPhaseProcess})
	blocker, _ := json.Marshal(sessionstore.Item{Sequence: 2, Kind: sessionstore.ItemToolCallStatus, Data: sessionstore.ToolCallStatus{TurnID: "child", CallID: "scan", Operations: []operation.Operation{{ID: "scan-operation", Type: operation.TypeShell, Version: operation.VersionShell, MaxOutputLength: 40000, Status: operation.StatusAwaiting, State: shellState}}}})
	fixture := filepath.Join(b.Workspace, "blocked-child")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s' '%s'\nexec sleep 120\n", report, blocker)
	if err := os.WriteFile(fixture, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	b.SubagentCommand = []string{fixture}
	client := newFakeClient()
	w := startWorker(t, b, client, nil)
	w.event(t, "ready")
	payload, _ := json.Marshal("start reviewer")
	w.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "review", Kind: inbox.InputExternal, Payload: payload}})
	select {
	case <-client.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("root model did not start")
	}
	client.responses <- llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "review-call", Name: "Agent", Arguments: `{"name":"reviewer","prompt":"review"}`}}}}
	deadline := time.Now().Add(5 * time.Second)
	for {
		w.send(t, workerproto.Request{Type: "activity_snapshot", ID: "inspect", Protocol: workerproto.Version})
		snapshot := w.event(t, "activity_snapshot")
		if snapshot.Activity == nil || snapshot.Activity.Version != 1 || snapshot.Resources == nil {
			t.Fatalf("missing authoritative bounded snapshot: %+v", snapshot)
		}
		if len(snapshot.Activity.Agents) > 0 && snapshot.Activity.Agents[0].ReportAvailable && snapshot.Activity.Agents[0].RelevantOperation != nil {
			agent := snapshot.Activity.Agents[0]
			if agent.OwnerTerminalEnqueued || agent.ProcessWaitReturned || agent.ReaderReturned {
				t.Fatalf("report falsely implied drain: %+v", agent)
			}
			if agent.RelevantOperation.Executable != "find" {
				t.Fatalf("missing safe executable: %+v", agent.RelevantOperation)
			}
			encoded, _ := json.Marshal(snapshot.Activity)
			if strings.Contains(string(encoded), "never-expose-this") {
				t.Fatal("snapshot leaked raw command secret")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("report + blocker not observed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Request loop still responds while parent model is unavailable/blocked.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	sent := make(chan error, 1)
	go func() {
		sent <- workerproto.Write(w.input, workerproto.Request{Type: "activity_snapshot", ID: "responsive"})
	}()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("snapshot request blocked behind child I/O")
	}
	if e := w.event(t, "activity_snapshot"); e.ID != "responsive" {
		t.Fatalf("wrong reply %+v", e)
	}
	w.send(t, workerproto.Request{Type: "stop"})
	w.finish(t)
}

func TestEffectiveChildBudgetEnvironmentHasNoDuplicateKeys(t *testing.T) {
	result := selectedEnvironment([]string{"UNREAL_HARNESS_OPERATION_TIMEOUT=9h", "OTHER=keep", "UNREAL_HARNESS_OPERATION_TIMEOUT=8h", "UNREAL_HARNESS_MAX_OPERATIONS=99"}, "UNREAL_HARNESS_OPERATION_TIMEOUT=0s", "UNREAL_HARNESS_MAX_OPERATIONS=0")
	if strings.Join(result, ";") != "OTHER=keep;UNREAL_HARNESS_OPERATION_TIMEOUT=0s;UNREAL_HARNESS_MAX_OPERATIONS=0" {
		t.Fatalf("ambiguous inherited policy: %v", result)
	}
}
