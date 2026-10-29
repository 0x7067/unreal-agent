package durablechild_test

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablechild"
	"github.com/unreallabsai/unreal-agent/cmd/internal/residentcli"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func TestMain(m *testing.M) {
	testtmp.Resolve()
	args := os.Args[1:]
	if len(args) == 2 && args[0] == "m26-launch-owner" {
		runLaunchFixture(args[1])
		return
	}
	if residentcli.Handles(args) {
		os.Exit(residentcli.Main(args))
	}
	os.Exit(m.Run())
}

// The boundary is constructed with canonical persistence APIs, not a claim of
// intercepting a runtime-internal crash window. Both the independently owned
// child and every parent replacement execute the real resident runtime.
func TestIndependentChildSurvivesParentSIGKILL(t *testing.T) {
	for _, cut := range []string{"awaiting_child", "child_answer_committed", "parent_operation_saved", "child_interrupted"} {
		t.Run(cut, func(t *testing.T) { independentChildRecovery(t, cut) })
	}
}
func independentChildRecovery(t *testing.T, cut string) {
	ctx, cancel := context.WithTimeout(t.Context(), 95*time.Second)
	defer cancel()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	runtimeDir, err := testtmp.MkdirTemp("u26-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(runtimeDir)
	if err := os.Chmod(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const parentID session.ID = "u26-parent"
	const opID operation.ID = "u26-stable-run"
	const prompt = "u26 independent child prompt"
	const answer = "u26 independent child answer"
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var enteredOnce, releaseOnce, joinedOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var childCalls, parentCalls atomic.Int64
	var oldCapability atomic.Value
	childStopped := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		encoded, _ := json.Marshal(body)
		text := "parent final"
		if strings.Contains(string(encoded), prompt) && !strings.Contains(string(encoded), "function_call_output") {
			childCalls.Add(1)
			enteredOnce.Do(func() { close(entered) })
			defer joinedOnce.Do(func() { close(joined) })
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			text = answer
		}
		if text == "parent final" {
			if !strings.Contains(string(encoded), answer) {
				http.Error(w, "missing original child answer", 500)
				return
			}
			n := parentCalls.Add(1)
			if n == 1 {
				arguments, _ := json.Marshal(map[string]any{"action": "stop", "handle": oldCapability.Load().(string), "generation": 1})
				completedCall(w, "AgentControl", string(arguments))
				return
			}
		}
		completed(w, text)
	}))
	defer server.Close()
	defer unblock()
	cfg := durablechild.Config{Directory: root, RuntimeDirectory: runtimeDir, Workspace: root, Provider: "openai", BaseURL: server.URL, Model: "fake", Command: []string{exe}}
	req := durablechild.Request{ParentID: parentID, OperationID: opID, Name: "independent", Prompt: prompt, Lifetime: "background"}
	store, err := localfile.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, parentID); err != nil {
		t.Fatal(err)
	}
	plan := toolsubagent.DurableRunPlan{Version: 1, ParentID: parentID, Run: toolsubagent.RunPlan{Name: req.Name, Prompt: prompt, Lifetime: req.Lifetime}}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.DurablePlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{MaxOutputLength: spec.MaxOutputLength, ID: opID, Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), Status: operation.StatusReady}
	if err := store.AppendTurn(ctx, parentID, session.Turn{ID: "u26-turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendModelResponse(ctx, parentID, sessionstore.ModelResponse{TurnID: "u26-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "u26-call", Name: "Agent", Arguments: `{"name":"independent","prompt":"u26 independent child prompt","lifetime":"background"}`}}}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendToolCallStatus(ctx, parentID, sessionstore.ToolCallStatus{TurnID: "u26-turn", CallID: "u26-call", Status: tool.CallStatus{WaitingFor: []operation.ID{opID}}, Operations: []operation.Operation{op}}); err != nil {
		t.Fatal(err)
	}
	owner, err := durablechild.Open(ctx, cfg, req, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		unblock()
		cleanup, done := context.WithTimeout(context.Background(), 12*time.Second)
		defer done()
		// Shutdown is only admission; WaitLaunch is the actual owned Cmd.Wait join.
		fresh, err := (*durablechild.Connection)(nil), error(nil)
		if !childStopped {
			fresh, err = durablechild.Open(cleanup, cfg, req, false)
		}
		if err != nil {
			t.Errorf("cleanup owner reconnect: %v", err)
		} else if fresh != nil {
			defer fresh.Close()
		}
		if err == nil && fresh != nil {
			if err := fresh.Shutdown(cleanup); err != nil {
				t.Errorf("child shutdown admission: %v", err)
			}
		}
		if err := owner.WaitLaunch(cleanup); err != nil {
			t.Errorf("child launch join: %v", err)
		}
		if err := owner.Close(); err != nil {
			t.Errorf("child reader join: %v", err)
		}
	}()
	link := owner.Link()
	if err := owner.SubmitInitial(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	var hosts []*workerhost.Host
	defer func() {
		for _, h := range hosts {
			closeHost(t, h)
		}
	}()
	start := func() *workerhost.Host {
		h, err := workerhost.Start(ctx, workerhost.Config{Command: []string{exe, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: parentID, Directory: root, Workspace: root, Provider: "openai", BaseURL: server.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh", DurableSubagents: true, ChildRuntimeDirectory: runtimeDir, DurableChildCommand: []string{exe}}, Stderr: os.Stderr, ShutdownGrace: 2 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		hosts = append(hosts, h)
		return h
	}
	h1 := start()
	first := awaitHandle(t, ctx, store, parentID, opID, "")
	if first.Version != 1 || first.Link != link || first.Handle == "" {
		t.Fatalf("first linkage: %+v", first)
	}
	oldCapability.Store(first.Handle)
	// This first adapter explicitly refuses foreground abort of a pending
	// independent branch rather than canceling it or claiming cleanup.
	abort, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.AbortForeground, Parameters: inbox.ForegroundAbort{Version: 1, Scope: h1.ControlScope(), TargetTurnID: "u26-turn"}})
	if err := h1.SubmitLifecycle(ctx, inbox.Input{ID: "u26-foreground-abort", Kind: inbox.InputControl, Payload: abort}); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, ctx, func() bool {
		for _, i := range readItems(t, ctx, store, parentID) {
			if in, ok := i.Data.(inbox.Input); ok && in.Kind == inbox.InputControl {
				control, e := in.DecodeControlMessage()
				if e == nil && control.Mode == inbox.ForegroundAbortFinished {
					r := control.Parameters.(inbox.ForegroundAbortResult)
					if r.InputID == "u26-foreground-abort" {
						if r.Status != "refused" {
							t.Fatalf("pending child abort mislabeled: %+v", r)
						}
						return true
					}
				}
			}
		}
		return false
	})
	select {
	case <-joined:
		t.Fatal("foreground abort affected independent child")
	default:
	}
	// Kill targets only the current process group owned by this Host; no PID lookup.
	if err := h1.Kill(); err != nil {
		t.Fatal(err)
	}
	closeHost(t, h1)
	select {
	case <-joined:
		t.Fatal("independent child HTTP ended when parent was killed")
	default:
	}
	if cut == "child_answer_committed" || cut == "parent_operation_saved" {
		// Original parent is actually dead/joined while the child commits its
		// canonical final answer. No hook into the old parent's polling loop.
		unblock()
		waitUntil(t, ctx, func() bool {
			a, found, err := owner.Answer(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return found && a.Text == answer
		})
		if cut == "parent_operation_saved" {
			// Controlled durable boundary: operation head committed, final canonical
			// ToolCallStatus not committed. SaveOperation is the actual head API.
			restored, err := store.Resume(ctx, parentID)
			if err != nil {
				t.Fatal(err)
			}
			for _, old := range restored.Operations {
				if old.ID == opID {
					remote, err := operation.DecodeRemoteJobState(old)
					if err != nil {
						t.Fatal(err)
					}
					a, _, _ := owner.Answer(ctx)
					first.Phase = "reported_owner_retained"
					first.AnswerSequence = a.Sequence
					first.AnswerTurnID = a.TurnID
					remote.Handle, _ = json.Marshal(first)
					remote.TerminalResult = answer
					step, err := operation.UpdateRemoteJob(old, remote, operation.StatusCompleted)
					if err != nil {
						t.Fatal(err)
					}
					if err := store.SaveOperation(ctx, parentID, *step.Operation); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	h2 := start()
	var second durablechild.Handle
	if cut != "parent_operation_saved" {
		second = awaitHandle(t, ctx, store, parentID, opID, first.Handle)
		if second.Link != link || second.Handle == first.Handle || second.Generation == 0 || second.ResultDeliveryID != first.ResultDeliveryID {
			t.Fatalf("replacement identity: %+v", second)
		}
	}
	if cut == "child_interrupted" {
		args, _ := json.Marshal(map[string]any{"action": "stop", "handle": second.Handle, "generation": second.Generation})
		payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Call: llm.ToolCall{CallID: "u26-stop-child", Name: "AgentControl", Arguments: string(args)}}})
		if err := h2.Submit(ctx, inbox.Input{ID: "u26-stop-child-input", Kind: inbox.InputControl, Payload: payload}); err != nil {
			t.Fatal(err)
		}
		if err := owner.WaitLaunch(ctx); err != nil {
			t.Fatal(err)
		}
		childStopped = true
		waitUntil(t, ctx, func() bool {
			activity, e := h2.ActivitySnapshot(ctx)
			if e != nil {
				t.Fatal(e)
			}
			for _, a := range activity.Snapshot.Agents {
				if a.OwnerOperationID == string(opID) {
					return a.Phase == "uncertain"
				}
			}
			return false
		})
		if _, found, e := owner.Answer(ctx); e != nil || found {
			t.Fatalf("interrupted child invented answer: %v %v", found, e)
		}
		if childCalls.Load() != 1 || parentCalls.Load() != 0 {
			t.Fatal("interruption retried child or reported invented result")
		}
		closeHost(t, h2)
		before, e := os.ReadFile(filepath.Join(root, string(parentID)+".session.jsonl"))
		if e != nil {
			t.Fatal(e)
		}
		rejected, e := workerhost.Start(ctx, workerhost.Config{Command: []string{exe, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: parentID, Directory: root, Workspace: root, Provider: "openai", BaseURL: server.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh", DurableSubagents: true, ChildRuntimeDirectory: runtimeDir, DurableChildCommand: []string{exe}}, Stderr: os.Stderr})
		if e == nil {
			closeHost(t, rejected)
			t.Fatal("unknown/interrupted ownership automatically resumed")
		}
		after, e := os.ReadFile(filepath.Join(root, string(parentID)+".session.jsonl"))
		if e != nil || string(before) != string(after) {
			t.Fatal("rejected restart changed parent canonical history")
		}
		return
	}
	if childCalls.Load() != 1 {
		t.Fatalf("child redispatched: %d", childCalls.Load())
	}
	unblock()
	waitUntil(t, ctx, func() bool {
		a, found, err := owner.Answer(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return found && a.Text == answer
	})
	select {
	case <-joined:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waitUntil(t, ctx, func() bool { return parentCalls.Load() == 2 })
	expectedStatuses := 1
	if n := finalResults(t, ctx, store, parentID, opID); n != expectedStatuses {
		t.Fatalf("canonical final statuses: got %d want %d", n, expectedStatuses)
	}
	closeHost(t, h2)
	h3 := start()
	// A real round trip proves the third runtime is alive after replay, rather
	// than treating Start's process exit/status as recovery evidence.
	waitUntil(t, ctx, func() bool {
		activity, err := h3.ActivitySnapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range activity.Snapshot.Agents {
			if a.OwnerOperationID == string(opID) {
				return a.Handle != "" && a.Handle != first.Handle && a.Phase == "reported_owner_retained"
			}
		}
		return false
	})
	if n := finalResults(t, ctx, store, parentID, opID); n != expectedStatuses || parentCalls.Load() != 2 {
		t.Fatalf("parent final tool results after second recovery: %d", n)
	}
	childStore, err := localfile.New(link.Directory)
	if err != nil {
		t.Fatal(err)
	}
	inputs := 0
	for _, item := range readItems(t, ctx, childStore, link.ChildID) {
		if in, ok := item.Data.(inbox.Input); ok && string(in.ID) == link.DeliveryID {
			inputs++
		}
	}
	if inputs != 1 || childCalls.Load() != 1 || owner.Link() != link {
		t.Fatalf("child identity/input/request dedup: inputs=%d requests=%d link=%+v", inputs, childCalls.Load(), owner.Link())
	}
	closeHost(t, h3)
	history, err := sessionstore.ReadLifecycle(ctx, store, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if history.ValidateResume() == nil || history.LastDrainSequence > 0 {
		t.Fatalf("independent retention became false global drain: %+v", history)
	}
}

func completed(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	body := map[string]any{"type": "response.completed", "response": map[string]any{"id": "u26-response", "status": "completed", "output": []map[string]any{{"id": "u26-message", "type": "message", "role": "assistant", "status": "completed", "content": []map[string]any{{"type": "output_text", "text": text}}}}, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}}}
	data, _ := json.Marshal(body)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}
func waitUntil(t *testing.T, ctx context.Context, f func() bool) {
	t.Helper()
	for {
		if f() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
func readItems(t *testing.T, ctx context.Context, s *localfile.Store, id session.ID) []sessionstore.Item {
	t.Helper()
	var items []sessionstore.Item
	var after sessionstore.Sequence
	for i := 0; i < 32; i++ {
		p, err := s.Items(ctx, id, after, 128)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, p.Items...)
		if !p.More {
			return items
		}
		after = p.NextAfter
	}
	t.Fatal("history exceeds bounded test scan")
	return nil
}
func awaitHandle(t *testing.T, ctx context.Context, s *localfile.Store, id session.ID, opID operation.ID, old string) durablechild.Handle {
	t.Helper()
	var got durablechild.Handle
	waitUntil(t, ctx, func() bool {
		state, err := s.Resume(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		ops := append([]operation.Operation(nil), state.Operations...)
		for _, item := range readItems(t, ctx, s, id) {
			if st, ok := item.Data.(sessionstore.ToolCallStatus); ok {
				ops = append(ops, st.Operations...)
			}
		}
		for _, op := range ops {
			if op.ID != opID {
				continue
			}
			remote, err := operation.DecodeRemoteJobState(op)
			if err != nil {
				t.Fatal(err)
			}
			if len(remote.Handle) == 0 {
				continue
			}
			if err := json.Unmarshal(remote.Handle, &got); err != nil {
				t.Fatal(err)
			}
			// Publishing a capability precedes transport attachment. A stop
			// needs the actually connected binding, not the connecting record.
			if got.Handle != "" && got.Handle != old && (got.Phase == "awaiting_answer" || got.Phase == "reported_owner_retained") {
				return true
			}
		}
		return false
	})
	return got
}
func finalResults(t *testing.T, ctx context.Context, s *localfile.Store, id session.ID, opID operation.ID) int {
	t.Helper()
	n := 0
	for _, item := range readItems(t, ctx, s, id) {
		if status, ok := item.Data.(sessionstore.ToolCallStatus); ok && status.CallID == "u26-call" {
			for _, op := range status.Operations {
				if op.ID == opID && op.Status == operation.StatusCompleted {
					n++
				}
			}
		}
	}
	return n
}
func closeHost(t *testing.T, h *workerhost.Host) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Errorf("parent host close did not prove cleanup: %v", err)
	}
}

// Private process fixture: retain a real launch reader until the exact owned
// launcher is SIGKILLed. The child host must be outside this process group.
func runLaunchFixture(path string) {
	var f launchFixture
	mustFixture(json.UnmarshalRead(mustOpenFixture(path), &f, json.RejectUnknownMembers(true)))
	c, err := durablechild.Open(context.Background(), f.Config, f.Request, true)
	mustFixture(err)
	mustFixture(c.SubmitInitial(context.Background()))
	data, err := json.Marshal(struct {
		Link    durablechild.Link
		HostPID int
	}{c.Link(), c.ClientSnapshot().Hello.HostPID})
	mustFixture(err)
	mustFixture(os.WriteFile(f.Ready, data, 0600))
	select {}
}

type launchFixture struct {
	Config  durablechild.Config
	Request durablechild.Request
	Ready   string
}

func mustFixture(err error) {
	if err != nil {
		panic(err)
	}
}
func mustOpenFixture(path string) *os.File { f, err := os.Open(path); mustFixture(err); return f }

func completedCall(w http.ResponseWriter, name, arguments string) {
	w.Header().Set("Content-Type", "text/event-stream")
	body := map[string]any{"type": "response.completed", "response": map[string]any{"id": "u26-stale-control-response", "status": "completed", "output": []map[string]any{{"id": "u26-stale-control", "type": "function_call", "call_id": "u26-stale-control", "name": name, "arguments": arguments, "status": "completed"}}}}
	data, _ := json.Marshal(body)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}
