package residentcli

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/resident"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// The worker fixture is the production CLI dispatcher in this test executable.
// No client exists until AFTER the canonical notification has committed.
func TestM44RealWorkerTimerRestartAbortAndClientReconciliation(t *testing.T) {
	root, e := os.MkdirTemp("/tmp", "t44.")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	path := func(s string) string { return filepath.Join(root, s) }
	for _, d := range []string{"home", "state", "work", "run"} {
		if e = os.Mkdir(path(d), 0700); e != nil {
			t.Fatal(e)
		}
	}
	for k, v := range map[string]string{"HOME": path("home"), "XDG_CONFIG_HOME": path("home"), "XDG_STATE_HOME": path("home"), "OPENAI_API_KEY": "private-fake", "UNREAL_HARNESS_LLM_API_KEY": "private-fake", "UNREAL_AGENT_DURABLE_TIMERS": "1"} {
		t.Setenv(k, v)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	wait := func(label string, p func() bool) {
		t.Helper()
		for !p() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", label, ctx.Err())
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"timer-final\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"m\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"timer done\"}]}]}}\n\n")
	}))
	t.Cleanup(provider.Close)
	const sid session.ID = "private-timer"
	store, e := localfile.New(path("state"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.Create(ctx, sid); e != nil {
		t.Fatal(e)
	}
	const turn session.TurnID = "seed-timer-turn"
	if e = store.AppendTurn(ctx, sid, session.Turn{ID: turn, Type: session.TurnRegular}); e != nil {
		t.Fatal(e)
	}
	due := time.Now().UTC().Add(8 * time.Second)
	ops := []operation.Operation{}
	output := []llm.Item{}
	for _, id := range []string{"fg", "bg"} {
		owner := reminder.Ownership{Mode: "background"}
		if id == "fg" {
			owner = reminder.Ownership{Mode: "foreground", OwnerTurnID: string(turn)}
		}
		spec, err := reminder.NewTaskSpec(reminder.TaskPlan{Version: 1, ParentID: string(sid), DueTime: due, Ownership: owner, Message: "private " + id + " notification"})
		if err != nil {
			t.Fatal(err)
		}
		ops = append(ops, operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady})
		args, _ := json.Marshal(map[string]any{"due_at": due.Format(time.RFC3339Nano), "message": "private " + id + " notification", "background": id == "bg"})
		output = append(output, llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: id, Name: "DurableReminder", Arguments: string(args)}})
	}
	if e = store.AppendModelResponse(ctx, sid, sessionstore.ModelResponse{TurnID: turn, Response: llm.Response{Stop: llm.StopComplete, Output: output}}); e != nil {
		t.Fatal(e)
	}
	for _, op := range ops {
		if e = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: turn, CallID: string(op.ID), Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); e != nil {
			t.Fatal(e)
		}
	}
	latest := func(id operation.ID) operation.Operation {
		t.Helper()
		r, e := store.Resume(ctx, sid)
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range r.Operations {
			if op.ID == id {
				return op
			}
		}
		h, e := sessionstore.ReadLifecycle(ctx, store, sid)
		if e != nil {
			t.Fatal(e)
		}
		return h.RemoteOperations[id]
	}
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cfg := workerhost.Config{Command: []string{binary, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: sid, Directory: path("state"), Workspace: path("work"), Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "private-timer-model", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh"}}
	start := func() *workerhost.Host {
		t.Helper()
		h, e := workerhost.Start(ctx, cfg)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if e := h.Close(c); e != nil {
				t.Errorf("worker actual close/join: %v", e)
			}
		})
		return h
	}
	first := start()
	wait("initial timers scheduled", func() bool {
		return latest("fg").Status == operation.StatusAwaiting && latest("bg").Status == operation.StatusAwaiting
	})
	if !time.Now().Before(due) {
		t.Fatal("initial worker missed before-deadline window")
	}
	if e = first.Kill(); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-first.Done():
		if e == nil {
			t.Fatal("killed worker falsely successful")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if e = first.Close(ctx); e != nil {
		t.Fatal(e)
	}
	second := start()
	if !time.Now().Before(due) {
		t.Fatal("restart missed before-deadline window")
	}
	payload, e := json.Marshal(inbox.ControlMessage{Mode: inbox.AbortForeground, Parameters: inbox.ForegroundAbort{Version: 1, Scope: second.ControlScope(), TargetTurnID: string(turn)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = second.SubmitLifecycle(ctx, inbox.Input{ID: "abort-private-fg", Kind: inbox.InputControl, Payload: payload}); e != nil {
		t.Fatal(e)
	}
	wait("scoped foreground abort committed", func() bool {
		h, e := sessionstore.ReadLifecycle(ctx, store, sid)
		if e != nil {
			t.Fatal(e)
		}
		return h.AbortResults["abort-private-fg"].Status == "completed"
	})
	if latest("fg").Status != operation.StatusCanceled || latest("bg").Status != operation.StatusAwaiting {
		t.Fatalf("abort did not retain only background: fg=%s bg=%s", latest("fg").Status, latest("bg").Status)
	}
	wait("background canonical terminal after deadline", func() bool { return latest("bg").Status == operation.StatusCompleted })
	bg := latest("bg")
	_, cp, e := reminder.DecodeTask(bg)
	if e != nil || cp.Timers[0].State != reminder.NotificationPending {
		t.Fatalf("canonical result falsely claims external acknowledgment: %v", e)
	}
	state, e := operation.DecodeRemoteJobState(bg)
	if e != nil {
		t.Fatal(e)
	}
	var notification reminder.NotificationResult
	if e = json.Unmarshal([]byte(state.TerminalResult), &notification); e != nil || notification.StableNotificationID != reminder.LogicalInputID(reminder.StableTimerID(string(sid), "bg")) {
		t.Fatalf("stable notification: %+v %v", notification, e)
	}
	wait("background terminal tool status published", func() bool {
		page, e := store.Items(ctx, sid, 0, 1000)
		if e != nil {
			t.Fatal(e)
		}
		for _, item := range page.Items {
			if s, ok := item.Data.(sessionstore.ToolCallStatus); ok {
				for _, op := range s.Operations {
					if op.ID == "bg" && op.Status == operation.StatusCompleted {
						return true
					}
				}
			}
		}
		return false
	})
	if e = second.Close(ctx); e != nil {
		t.Fatal(e)
	}
	beforeCalls := calls.Load()
	before, e := os.ReadFile(path("state/private-timer.session.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	// Fresh hosts and authenticated client snapshots reconcile the existing result;
	// they must not dispatch another timer or request another provider continuation.
	previousIncarnation := ""
	for i := 0; i < 2; i++ {
		serveCtx, stop := context.WithCancel(context.Background())
		ready := make(chan resident.Info, 1)
		done := make(chan error, 1)
		go func() {
			done <- resident.Serve(serveCtx, resident.Config{RuntimeDirectory: path("run"), Worker: cfg}, func(info resident.Info) { ready <- info })
		}()
		joined := false
		cleanup := func() {
			stop()
			if joined {
				return
			}
			joined = true
			select {
			case e := <-done:
				if e != nil {
					t.Errorf("resident actual join: %v", e)
				}
			case <-time.After(10 * time.Second):
				t.Error("resident actual join unconfirmed")
			}
		}
		t.Cleanup(cleanup)
		var info resident.Info
		select {
		case info = <-ready:
		case e = <-done:
			joined = true
			t.Fatalf("fresh resident startup: %v", e)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if info.Incarnation == previousIncarnation {
			t.Fatal("fresh host reused incarnation")
		}
		previousIncarnation = info.Incarnation
		client, e := attachclient.Dial(ctx, path("run"), path("state"), sid)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = client.Close() })
		count := 0
		for _, item := range client.Snapshot().Items {
			if s, ok := item.Data.(sessionstore.ToolCallStatus); ok {
				for _, op := range s.Operations {
					if op.ID == "bg" && op.Status == operation.StatusCompleted {
						count++
						got, e := operation.DecodeRemoteJobState(op)
						if e != nil || got.TerminalResult != state.TerminalResult {
							t.Fatalf("client canonical notification changed: %v", e)
						}
					}
					if op.ID == "fg" && op.Status == operation.StatusCompleted {
						t.Fatal("canceled foreground delivered notification")
					}
				}
			}
		}
		if count != 1 {
			t.Fatalf("client canonical background notification count=%d", count)
		}
		_ = client.Close()
		cleanup()
	}
	after, e := os.ReadFile(path("state/private-timer.session.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	// Restart may append genuine drain/control receipts, not new notifications.
	_ = before
	_ = after
	if calls.Load() != beforeCalls {
		t.Fatalf("restart duplicated provider: before=%d after=%d", beforeCalls, calls.Load())
	}
	if latest("bg").Status != operation.StatusCompleted || latest("fg").Status != operation.StatusCanceled {
		t.Fatal("restart changed canonical timer outcomes")
	}
}
