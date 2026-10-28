package agentrunner

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestRecoveryGateStandaloneRefusesShellBeforeProviders(t *testing.T) {
	directory, workspace := t.TempDir(), t.TempDir()
	sid := session.ID("standalone-unresolved")
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), sid); err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewShellSpec(operation.ShellInput{Command: "printf x >> forbidden-replay", Directory: workspace, Shell: "/bin/sh"}, filepath.Join(directory, "operations"), 1000)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "same-shell-intent", Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, State: spec.State.Clone(), MaxOutputLength: spec.MaxOutputLength}
	if err := store.AppendTurn(t.Context(), sid, session.Turn{ID: "original-turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendModelResponse(t.Context(), sid, sessionstore.ModelResponse{TurnID: "original-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "original-call", Name: tool.BashName, Arguments: `{"command":"printf x >> forbidden-replay"}`}}}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendToolCallStatus(t.Context(), sid, sessionstore.ToolCallStatus{TurnID: "original-turn", CallID: "original-call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, string(sid)+".session.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config := testConfig(nil)
	constructed := false
	for i := range config.Providers {
		config.Providers[i].NewClient = func(string, string, int, func(string) string) (providers.Client, error) {
			constructed = true
			return nil, errors.New("provider must not initialize")
		}
	}
	request, err := json.Marshal(Request{SessionID: new(string(sid)), Messages: []RequestMessage{{Role: "user", Content: "continue"}}})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = Run(t.Context(), []string{"-workspace", workspace, "-session-directory", directory, string(request)}, func(key string) string {
		if key == providers.APIKeyEnvironment {
			return "fake"
		}
		return ""
	}, func() []string { return nil }, strings.NewReader(""), &output, io.Discard, config)
	if !errors.Is(err, operation.ErrUnsupported) || !strings.Contains(err.Error(), `type="shell"`) || constructed {
		t.Fatalf("unsafe standalone recovery/provider=%v: %v", constructed, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || output.Len() != 0 {
		t.Fatalf("refusal mutated canonical history/output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "forbidden-replay")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsafe effect: %v", err)
	}
	for _, id := range []session.ID{sid, session.ID("host-" + string(sid))} {
		lease, err := localfile.AcquireLease(directory, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
