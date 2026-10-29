package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func TestMain(m *testing.M) {
	testtmp.Resolve()
	if os.Getenv("UNREAL_TUI_DIAGNOSTICS_HELPER") == "1" && len(os.Args) > 1 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		identity := workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: "ui-integration"}
		switch os.Args[1] {
		case "worker-probe":
			if err := workerruntime.Probe(os.Stdout, identity); err != nil {
				os.Exit(30)
			}
			return
		case "worker":
			if err := workerruntime.Run(ctx, os.Stdin, os.Stdout, os.Stderr, os.Getenv, identity); err != nil && ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(31)
			}
			return
		case "agent-runner":
			os.Exit(agentrunner.RunMain(ctx, os.Args[2:], os.Getenv, os.Environ, os.Stdin, os.Stdout, os.Stderr, agentrunner.Config{Name: "ui-fixture", Providers: providers.Default(), ParseRequest: agentrunner.ParseRequest}))
		}
	}
	os.Exit(m.Run())
}

func TestAgentsUIRealDurableStopWithoutModelControlTurn(t *testing.T) {
	t.Setenv("UNREAL_TUI_DIAGNOSTICS_HELPER", "1")
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		var output []map[string]any
		if id <= 2 {
			name := "Agent"
			args := map[string]any{"name": "reviewer", "prompt": "quiet reviewer"}
			if id == 2 {
				name = "Bash"
				args = map[string]any{"command": "touch ui-shell-started; sleep 30"}
			}
			encoded, _ := json.Marshal(args)
			output = []map[string]any{{"id": fmt.Sprintf("function-%d", id), "type": "function_call", "call_id": fmt.Sprintf("call-%d", id), "name": name, "arguments": string(encoded), "status": "completed"}}
		} else {
			output = []map[string]any{{"id": fmt.Sprintf("message-%d", id), "type": "message", "role": "assistant", "status": "completed", "content": []map[string]any{{"type": "output_text", "text": "done"}}}}
		}
		body, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprintf("response-%d", id), "status": "completed", "output": output, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", body)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	events := newWorkerEvents(ctx)
	executable, _ := os.Executable()
	host, err := workerhost.Start(ctx, workerhost.Config{Command: []string{executable, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "ui-durable", Directory: filepath.Join(root, "sessions"), Workspace: root, Provider: "openai", BaseURL: server.URL, Model: llm.Model{ID: "fake"}, MaxAttempts: 1, Shell: "/bin/sh", Subagents: true}, OnItem: events.item, OnStatus: events.status, Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := host.Close(closeCtx); err != nil {
			t.Errorf("close host: %v", err)
		}
	}()
	m := toolsModel(t, defaultTheme, 58, 25, 0)
	m.ctx = ctx
	m.inputs = host
	m.registry = newUIRegistry(true, false)
	drain := func() {
		for {
			events.mu.Lock()
			if len(events.queue) == 0 {
				events.mu.Unlock()
				return
			}
			msg := events.queue[0]
			events.queue = events.queue[1:]
			events.mu.Unlock()
			m = submissionUpdate(m, msg)
		}
	}
	cmd := m.startSubmission("start reviewer")
	m = submissionUpdate(m, cmd())
	defer func() {
		if m.submissionQueue != nil {
			m.submissionQueue.Cancel()
			if err := m.submissionQueue.Wait(context.Background()); err != nil {
				t.Error(err)
			}
		}
	}()
	for {
		if _, err := os.Stat(filepath.Join(root, "ui-shell-started")); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("nested shell did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	for {
		cmd = m.startSubmission("/agents list")
		m = submissionUpdate(m, cmd())
		drain()
		if len(m.activity.Snapshot.Agents) > 0 && m.activity.Snapshot.Agents[0].RelevantOperation != nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("blocker not projected")
		case <-time.After(10 * time.Millisecond):
		}
	}
	agent := m.activity.Snapshot.Agents[0]
	if calls.Load() != 2 {
		t.Fatalf("inspection consulted model: %d", calls.Load())
	}
	m.toolsCollapsed = true
	if !strings.Contains(ansi.Strip(m.View().Content), "reviewer") {
		t.Fatalf("narrow/collapsed view hid blocker: %s", ansi.Strip(m.View().Content))
	}
	cmd = m.startSubmission("/agents inspect " + agentTarget(agent))
	m = submissionUpdate(m, cmd())
	cmd = m.startSubmission("/agents stop " + agentTarget(agent))
	m = submissionUpdate(m, cmd())
	var controlID inbox.ID
	for _, line := range m.lines {
		if line.label == "Agent control" {
			controlID = line.inputID
		}
	}
	if controlID == "" {
		t.Fatal("control had no stable admission identity")
	}
	for {
		drain()
		found := false
		for _, line := range m.lines {
			if line.label == "Agent control result" {
				found = true
				if !strings.Contains(line.text, "cancellation requested") && !strings.Contains(line.text, "drain confirmed") {
					t.Fatalf("unproven result: %s", line.text)
				}
			}
		}
		if found {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("no persisted structural control outcome (admission alone is insufficient)")
		case <-time.After(10 * time.Millisecond):
		}
	}
	closeCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if err := host.Close(closeCtx); err != nil {
		t.Fatal(err)
	}
	store, err := localfile.New(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := store.Resume(t.Context(), "ui-durable")
	if err != nil {
		t.Fatal(err)
	}

	// Resume intentionally returns unresolved operations only. Verify the real
	// persisted log's initializer -> terminal operation -> visible outcome barriers,
	// rather than incorrectly expecting drained historical controls in Resume.
	for _, op := range restored.Operations {
		if !terminalOperation(op.Status) {
			t.Fatalf("close left unresolved %s", op.ID)
		}
	}
	log, err := os.ReadFile(filepath.Join(root, "sessions", "ui-durable.session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(log))
	scanner.Buffer(make([]byte, 4096), workerproto.MaxFrame)
	var persistedInput, initialized, terminal, outcome bool
	var controlOperation operation.ID
	for scanner.Scan() {
		var record struct {
			Type string         `json:"type"`
			Data jsontext.Value `json:"data"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		switch record.Type {
		case "item":
			var data struct{ Item sessionstore.Item }
			if json.Unmarshal(record.Data, &data) != nil {
				continue
			}
			input, ok := data.Item.Data.(inbox.Input)
			if !ok {
				continue
			}
			if input.ID == controlID {
				persistedInput = true
			}
			if input.Kind == inbox.InputControl {
				if message, err := input.DecodeControlMessage(); err == nil && message.Mode == inbox.ToolInvocationFinished {
					result := message.Parameters.(inbox.ToolInvocationResult)
					if result.InputID == controlID {
						if !terminal {
							t.Fatal("visible outcome preceded persisted terminal operation")
						}
						outcome = true
					}
				}
			}
		case "control_operation_v1":
			var data struct {
				Version   int
				InputID   inbox.ID
				Operation operation.Operation
			}
			if err := json.Unmarshal(record.Data, &data); err != nil {
				t.Fatal(err)
			}
			if data.InputID != controlID {
				continue
			}
			if !persistedInput || data.Version != 1 {
				t.Fatal("initializer lacked prior durable admission or supported record version")
			}
			state, err := operation.DecodeRemoteJobState(data.Operation)
			if err != nil || state.Plan.Type != toolsubagent.ControlPlanType {
				t.Fatalf("bad durable control initializer: %v", err)
			}
			initialized = true
			controlOperation = data.Operation.ID
		case "operation":
			var data struct{ Operation operation.Operation }
			if json.Unmarshal(record.Data, &data) != nil {
				continue
			}
			if data.Operation.ID == controlOperation && data.Operation.Status == operation.StatusCompleted {
				terminal = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !persistedInput || !initialized || !terminal || !outcome {
		t.Fatalf("missing real persistence barriers: input=%t init=%t terminal=%t outcome=%t", persistedInput, initialized, terminal, outcome)
	}
}
