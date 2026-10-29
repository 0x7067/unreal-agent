package resident

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func residentLifecycleFixture(t *testing.T, provider *httptest.Server, enableRuntime ...bool) (string, string, string, Info, <-chan error) {
	t.Helper()
	root, err := testtmp.MkdirTemp("ulc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	runtime, state := filepath.Join(root, "run"), filepath.Join(root, "state")
	for _, dir := range []string{runtime, state} {
		if err = os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	ready, done := make(chan Info, 1), make(chan error, 1)
	go func() {
		done <- Serve(ctx, Config{RuntimeReplacement: len(enableRuntime) > 0 && enableRuntime[0], RuntimeDirectory: runtime, Worker: workerhost.Config{Command: []string{executable, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "lifecycle", Directory: state, Workspace: root, Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh"}}}, func(i Info) { ready <- i })
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("resident join: %v", err)
			}
		case <-time.After(8 * time.Second):
			t.Error("resident join unconfirmed")
		}
	})
	select {
	case info := <-ready:
		return root, runtime, state, info, done
	case err := <-done:
		t.Fatalf("startup: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("startup timeout")
	}
	panic("unreachable")
}
func residentControl(t *testing.T, id inbox.ID, mode inbox.ControlMode, parameters any) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: parameters})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
}
func lifecycleHistoryWait(t *testing.T, state string, predicate func(sessionstore.LifecycleHistory) bool) sessionstore.LifecycleHistory {
	t.Helper()
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		history, err := sessionstore.ReadLifecycle(t.Context(), store, "lifecycle")
		if err != nil {
			t.Fatal(err)
		}
		if predicate(history) {
			return history
		}
		if time.Now().After(deadline) {
			t.Fatalf("canonical lifecycle outcome not observed: %+v", history)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func lifecycleDial(t *testing.T, ctx context.Context, runtime, state string) *attachclient.Client {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		c, err := attachclient.Dial(ctx, runtime, state, "lifecycle")
		if err == nil {
			return c
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func latestLifecycleTurn(t *testing.T, c *attachclient.Client) string {
	t.Helper()
	var target string
	for _, item := range c.Snapshot().Items {
		if item.Kind == sessionstore.ItemTurn {
			target = string(item.Data.(session.Turn).ID)
		}
	}
	if target == "" {
		t.Fatal("no canonical foreground target")
	}
	return target
}
func lifecycleScope(c *attachclient.Client) string {
	snapshot := c.Snapshot()
	return fmt.Sprintf("%s/%d", snapshot.Hello.Incarnation, snapshot.Header.Status.Generation)
}

func TestLifecycleSDKDetachAbortAndDeliberateShutdownRealProcesses(t *testing.T) {
	var calls atomic.Int64
	modelStarted, modelCanceled := make(chan struct{}), make(chan struct{})
	var startedOnce, canceledOnce sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		n := calls.Add(1)
		if n == 1 {
			startedOnce.Do(func() { close(modelStarted) })
			<-r.Context().Done()
			canceledOnce.Do(func() { close(modelCanceled) })
			return
		}
		args, _ := json.Marshal(map[string]any{"command": "printf x >> public-effect; touch public-started; while [ ! -f public-release ]; do sleep .02; done"})
		var output []map[string]any
		if n == 2 {
			output = []map[string]any{{"id": "function", "type": "function_call", "call_id": "bash", "name": "Bash", "arguments": string(args), "status": "completed"}}
		} else {
			output = []map[string]any{{"id": "answer", "type": "message", "role": "assistant", "status": "completed", "content": []map[string]any{{"type": "output_text", "text": "done"}}}}
		}
		data, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprint(n), "status": "completed", "output": output}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", data)
	}))
	t.Cleanup(provider.Close)
	root, runtime, state, info, done := residentLifecycleFixture(t, provider)
	// Always unblock only this owned Bash before resident cleanup on failure.
	t.Cleanup(func() { _ = os.WriteFile(filepath.Join(root, "public-release"), nil, 0600) })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	first := lifecycleDial(t, ctx, runtime, state)
	firstReader := faultReader(t, first)
	if err := first.Submit(ctx, inbox.Input{ID: "model-input", Kind: inbox.InputExternal, Payload: []byte(`"model work"`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-modelStarted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	_ = first.Close()
	<-firstReader
	second := lifecycleDial(t, ctx, runtime, state)
	secondReader := faultReader(t, second)
	if second.Snapshot().Hello.Incarnation != info.Incarnation || second.Snapshot().Hello.HostPID != info.HostPID {
		t.Fatal("detach replaced host")
	}
	select {
	case <-modelCanceled:
		t.Fatal("detach canceled live model callback")
	default:
	}
	abort := residentControl(t, "abort-model", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, Scope: lifecycleScope(second), TargetTurnID: latestLifecycleTurn(t, second)})
	if err := second.SubmitLifecycle(ctx, abort); err != nil {
		t.Fatal(err)
	}
	lifecycleHistoryWait(t, state, func(h sessionstore.LifecycleHistory) bool { return h.AbortResults[abort.ID].Status == "completed" })
	select {
	case <-modelCanceled:
	case <-ctx.Done():
		t.Fatal("explicit abort did not cancel owned model")
	}
	if err := second.Submit(ctx, inbox.Input{ID: "bash-input", Kind: inbox.InputExternal, Payload: []byte(`"Bash work"`)}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "public-started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Bash not started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	_ = second.Close()
	<-secondReader
	third := lifecycleDial(t, ctx, runtime, state)
	thirdReader := faultReader(t, third)
	bashAbort := residentControl(t, "abort-bash", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, Scope: lifecycleScope(third), TargetTurnID: latestLifecycleTurn(t, third)})
	if err := third.SubmitLifecycle(ctx, bashAbort); err != nil {
		t.Fatal(err)
	}
	lifecycleHistoryWait(t, state, func(h sessionstore.LifecycleHistory) bool { return h.AbortResults[bashAbort.ID].Status == "completed" })
	if calls.Load() != 2 {
		t.Fatalf("old foreground continuation resumed: calls=%d", calls.Load())
	}
	shutdown := residentControl(t, "shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: lifecycleScope(third)})
	if err := third.SubmitLifecycle(ctx, shutdown); err != nil && !errors.Is(err, attachclient.ErrUncertain) {
		t.Fatal(err)
	}
	history := lifecycleHistoryWait(t, state, func(h sessionstore.LifecycleHistory) bool { return h.ShutdownResults[shutdown.ID].Phase == "completed" })
	if err := history.ValidateResume(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("host did not join")
	}
	_ = third.Close()
	<-thirdReader
	effect, err := os.ReadFile(filepath.Join(root, "public-effect"))
	if err != nil || string(effect) != "x" {
		t.Fatalf("Bash effect=%q %v", effect, err)
	}
	// Endpoint loss is not the oracle: query the committed canonical outcome with
	// the shared versioned read-only codec after the independent host join.
	file, err := os.Open(filepath.Join(state, "lifecycle.session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	offline, err := localfile.ReadLifecycle(ctx, file, "lifecycle")
	if err != nil || offline.ShutdownResults[shutdown.ID].Phase != "completed" {
		t.Fatalf("read-only reconciliation: %v", err)
	}
}

func TestLifecycleWireRejectsResultReceiptAndForeignScopeInjection(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	t.Cleanup(provider.Close)
	_, runtime, state, _, _ := residentLifecycleFixture(t, provider)
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Items(t.Context(), "lifecycle", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, attack := range []string{"receipt", "result", "stop", "foreign-scope", "external"} {
		t.Run(attack, func(t *testing.T) {
			conn, err := attachproto.Dial(t.Context(), runtime, state, "lifecycle")
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			if err = attachproto.Write(conn, attachproto.Request{Type: "attach", ID: "hello", Protocol: 1, SessionID: "lifecycle"}); err != nil {
				t.Fatal(err)
			}
			scanner := attachproto.Scanner(conn)
			hello, err := attachproto.ReadEvent(scanner)
			if err != nil {
				t.Fatal(err)
			}
			var generation uint64
			for {
				event, err := attachproto.ReadEvent(scanner)
				if err != nil {
					t.Fatal(err)
				}
				if event.Status != nil {
					generation = event.Status.Generation
				}
				if event.Type == "snapshot_end" {
					break
				}
			}
			scope := fmt.Sprintf("%s/%d", hello.Incarnation, generation)
			var input inbox.Input
			switch attack {
			case "receipt":
				input = residentControl(t, "injection", inbox.ExecutionDrained, inbox.ExecutionDrainReceipt{Version: 1, Scope: scope})
			case "result":
				input = residentControl(t, "injection", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "none", Scope: scope, Phase: "completed"})
			case "stop":
				input = residentControl(t, "injection", inbox.StopHard, nil)
			case "foreign-scope":
				input = residentControl(t, "injection", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: scope + "/old"})
			case "external":
				input = inbox.Input{ID: "injection", Kind: inbox.InputExternal, Payload: []byte(`"not control"`)}
			}
			if err = attachproto.Write(conn, attachproto.Request{Type: "lifecycle", ID: "attack", Protocol: 1, SessionID: "lifecycle", Connection: hello.Connection, Input: &input}); err != nil {
				t.Fatal(err)
			}
			_, err = attachproto.ReadEvent(scanner)
			var timeout net.Error
			if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
				t.Fatalf("attack not actually disconnected: %v", err)
			}
			if !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "closed") {
				t.Fatalf("unexpected attack result: %v", err)
			}
		})
	}
	after, err := store.Items(t.Context(), "lifecycle", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != len(before.Items) || calls.Load() != 0 {
		t.Fatal("rejected injection mutated execution")
	}
}

func TestLifecycleRealLostAckRetryCannotAbortNewTurn(t *testing.T) {
	var calls atomic.Int64
	started := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	canceled := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		n := calls.Add(1)
		if n > 2 {
			t.Errorf("unexpected model continuation %d", n)
			return
		}
		close(started[n-1])
		<-r.Context().Done()
		close(canceled[n-1])
	}))
	t.Cleanup(provider.Close)
	root, backend, state, info, hostDone := residentLifecycleFixture(t, provider)
	ctx, cancel := context.WithTimeout(t.Context(), 18*time.Second)
	defer cancel()
	seed := lifecycleDial(t, ctx, backend, state)
	seedReader := faultReader(t, seed)
	if err := seed.Submit(ctx, inbox.Input{ID: "seed", Kind: inbox.InputExternal, Payload: []byte(`"first model"`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started[0]:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	_ = seed.Close()
	<-seedReader
	public := filepath.Join(root, "p")
	if err := os.Mkdir(public, 0700); err != nil {
		t.Fatal(err)
	}
	proxyDone := faultProxy(t, ctx, public, backend, state, "lifecycle", "lost-abort", "persisted_before_ack")
	first := lifecycleDial(t, ctx, public, state)
	firstReader := faultReader(t, first)
	original := residentControl(t, "lost-abort", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, Scope: lifecycleScope(first), TargetTurnID: latestLifecycleTurn(t, first)})
	if err := first.SubmitLifecycle(ctx, original); !errors.Is(err, attachclient.ErrUncertain) {
		t.Fatalf("lost acknowledgement not uncertain: %v", err)
	}
	<-firstReader
	select {
	case err := <-proxyDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("causal proxy did not join")
	}
	lifecycleHistoryWait(t, state, func(h sessionstore.LifecycleHistory) bool { return h.AbortResults[original.ID].Status == "completed" })
	next := lifecycleDial(t, ctx, backend, state)
	nextReader := faultReader(t, next)
	if next.Snapshot().Hello.Incarnation != info.Incarnation {
		t.Fatal("lost ack replaced host")
	}
	if err := next.Submit(ctx, inbox.Input{ID: "new-turn", Kind: inbox.InputExternal, Payload: []byte(`"second model"`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started[1]:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := next.SubmitLifecycle(ctx, original); err != nil {
		t.Fatalf("original stable control retry: %v", err)
	}
	select {
	case <-canceled[1]:
		t.Fatal("old stable control aborted new turn")
	default:
	}
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Items(ctx, "lifecycle", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range page.Items {
		if in, ok := item.Data.(inbox.Input); ok && in.ID == original.ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("canonical stable control appears %d times", count)
	}
	shutdown := residentControl(t, "finish-lost-ack", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: lifecycleScope(next)})
	if err := next.SubmitLifecycle(ctx, shutdown); err != nil && !errors.Is(err, attachclient.ErrUncertain) {
		t.Fatal(err)
	}
	lifecycleHistoryWait(t, state, func(h sessionstore.LifecycleHistory) bool { return h.ShutdownResults[shutdown.ID].Phase == "completed" })
	select {
	case err := <-hostDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("host join unconfirmed")
	}
	_ = next.Close()
	<-nextReader
	if calls.Load() != 2 {
		t.Fatalf("duplicate model work: %d", calls.Load())
	}
}
