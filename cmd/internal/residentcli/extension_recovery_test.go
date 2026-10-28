package residentcli

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// The child is this test binary's TestMain production dispatcher, not an
// imitation handler: bootstrap, recovery fences, coordinator and provider all run
// in a separately waited OS worker. Every canonical fixture is private-owned.
func TestM42RealWorkerPinnedExtensionRecovery(t *testing.T) {
	for _, tc := range []struct {
		name               string
		completed, missing bool
		fixture            string
	}{
		{name: "pending_A_with_B_selected"},
		{name: "pending_A_missing_refused", missing: true},
		{name: "completed_A_missing_is_historical", completed: true, missing: true},
		{name: "future_plan_refused", fixture: "future-plan-v99"},
		{name: "future_schema_refused", fixture: "future-schema-v99"},
		{name: "future_adapter_refused", fixture: "future-adapter-v99"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			state, work, home := filepath.Join(root, "state"), filepath.Join(root, "work"), filepath.Join(root, "home")
			for _, dir := range []string{state, work, home} {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", home)
			t.Setenv("XDG_STATE_HOME", home)
			t.Setenv("OPENAI_API_KEY", "fake-private-fixture")
			t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-private-fixture")
			t.Setenv("UNREAL_AGENT_JOB_CONFIG", "")
			t.Setenv("UNREAL_AGENT_DURABLE_TIMERS", "")
			a := extension.Descriptor{Name: extension.Name, Generation: 1, SchemaVersion: 1, ImplementationVersion: 1, Implementation: "A"}
			b := a
			b.Generation = 2
			b.Implementation = "B"
			known := []extension.Descriptor{a, b}
			if tc.missing {
				known = []extension.Descriptor{b}
			}
			config, err := json.Marshal(struct {
				Version  int
				Known    []extension.Descriptor
				Selected uint64
			}{1, known, 2})
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(root, "extensions.json")
			if err = os.WriteFile(configPath, config, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("UNREAL_AGENT_EXTENSION_CONFIG", configPath)
			const sid session.ID = "private-extension-recovery"
			store, err := localfile.New(state)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.Create(ctx, sid); err != nil {
				t.Fatal(err)
			}
			user, _ := json.Marshal("run trusted echo")
			if err = store.AppendInput(ctx, sid, inbox.Input{ID: "original-extension-input", Kind: inbox.InputExternal, Payload: user}); err != nil {
				t.Fatal(err)
			}
			turn := session.Turn{ID: "original-extension-turn", Type: session.TurnRegular}
			if err = store.AppendTurn(ctx, sid, turn); err != nil {
				t.Fatal(err)
			}
			call := llm.ToolCall{CallID: "echo-call", Name: "TrustedEcho", Arguments: `{"input":"seed"}`}
			if err = store.AppendModelResponse(ctx, sid, sessionstore.ModelResponse{TurnID: turn.ID, Response: llm.Response{ID: "seed-response", Stop: llm.StopComplete, Output: []llm.Item{{ProviderID: "echo-call", Type: llm.ItemToolCall, Data: call}}}}); err != nil {
				t.Fatal(err)
			}

			fixture := "pending-a-v1"
			if tc.fixture != "" {
				fixture = tc.fixture
			}
			data, err := os.ReadFile(filepath.Join("testdata", "m4-extension", fixture+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var op operation.Operation
			if err = json.Unmarshal(data, &op, json.RejectUnknownMembers(true)); err != nil {
				t.Fatal(err)
			}
			if err = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: turn.ID, CallID: call.CallID, Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
				t.Fatal(err)
			}
			if tc.completed {
				data, err = os.ReadFile(filepath.Join("testdata", "m4-extension", "completed-a-v1.json"))
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(data, &op, json.RejectUnknownMembers(true)); err != nil {
					t.Fatal(err)
				}
				if err = store.SaveOperation(ctx, sid, op); err != nil {
					t.Fatal(err)
				}
				if err = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: turn.ID, CallID: call.CallID, Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
					t.Fatal(err)
				}
			}
			canonical := filepath.Join(state, string(sid)+".session.jsonl")
			before, err := os.ReadFile(canonical)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int64
			requests := make(chan []byte, 8)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				n := calls.Add(1)
				select {
				case requests <- body:
				default:
				}
				data, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprintf("extension-answer-%d", n), "status": "completed", "output": []any{map[string]any{"type": "message", "id": "answer", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "extension recovery complete"}}}}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}})
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			}))
			defer provider.Close()
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			var stderr lockedOutput
			host, err := workerhost.Start(ctx, workerhost.Config{Command: []string{binary, "worker"}, Stderr: &stderr, Bootstrap: workerproto.Bootstrap{SessionID: sid, Directory: state, Workspace: work, Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "private-model", CompactionThreshold: 64000}, MaxAttempts: 2, Shell: "/bin/sh"}})
			if tc.missing && !tc.completed || tc.fixture != "" {
				if host != nil {
					closeCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
					defer stop()
					_ = host.Close(closeCtx)
					t.Fatal("missing pinned A admitted a worker")
				}
				if err == nil || !(strings.Contains(err.Error()+stderr.String(), "generation unavailable") || strings.Contains(err.Error()+stderr.String(), "unsupported")) {
					t.Fatalf("missing A refusal not visible: %v %s", err, stderr.String())
				}
				if calls.Load() != 0 {
					t.Fatalf("provider contacted before recovery refusal: %d", calls.Load())
				}
				after, e := os.ReadFile(canonical)
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("recovery refusal mutated canonical history")
				}
				return
			}
			if err != nil {
				t.Fatalf("worker recovery: %v %s", err, stderr.String())
			}
			t.Cleanup(func() {
				closeCtx, stop := context.WithTimeout(context.Background(), 8*time.Second)
				defer stop()
				if err := host.Close(closeCtx); err != nil {
					t.Errorf("worker process join: %v %s", err, stderr.String())
				}
			})
			// Completed history may be idle rather than awaiting an automatic model
			// continuation. A fresh input forces actual provider context projection.
			if tc.completed {
				payload, _ := json.Marshal("report historical echo")
				if err = host.Submit(ctx, inbox.Input{ID: "historical-output-query", Kind: inbox.InputExternal, Payload: payload}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case body := <-requests:
				if !bytes.Contains(body, []byte("A:seed")) || bytes.Contains(body, []byte("B:seed")) {
					t.Fatalf("provider did not receive exact pinned/historical A result: %s", body)
				}
			case <-ctx.Done():
				t.Fatalf("no recovered provider continuation: %v %s", ctx.Err(), stderr.String())
			}
			// Wait for a durable final response before graceful Close, not just a server
			// request or a terminal handler claim.
			for {
				data, e := os.ReadFile(canonical)
				if e != nil {
					t.Fatal(e)
				}
				if bytes.Contains(data, []byte("extension recovery complete")) {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatalf("final model response not committed: %v %s", ctx.Err(), stderr.String())
				case <-time.After(5 * time.Millisecond):
				}
			}
			history, e := sessionstore.ReadLifecycle(ctx, store, sid)
			if e != nil {
				t.Fatal(e)
			}
			final, ok := history.RemoteOperations[op.ID]
			if !ok || final.Status != operation.StatusCompleted {
				t.Fatalf("original operation not completed: %+v", final)
			}
			s, _, e := extension.Decode(final)
			if e != nil || s.TerminalResult != "A:seed" {
				t.Fatalf("pinned result replaced by selection B: %+v %v", s, e)
			}
		})
	}
}
