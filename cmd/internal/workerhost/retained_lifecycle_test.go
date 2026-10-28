package workerhost

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

// Actual worker -> strict supervisor -> actual runner -> actual Bash. No
// fixture emits cleanup receipts or substitutes PID absence for owner joins.
func TestRetainedBackgroundSurvivesForegroundAbortThenStrictShutdown(t *testing.T) {
	var calls atomic.Int64
	rootStarted, rootCanceled, releaseHTTP := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		id := calls.Add(1)
		switch id {
		case 1:
			completed(w, id, toolCall(id, "Agent", map[string]any{"name": "retained", "prompt": "retained work", "lifetime": "background", "purpose": "test", "parent_task": "test", "owned_resources": []string{"retained-sentinel", "retained-started", "retained-probe", "retained-probed", "retained-release"}}))
		case 2:
			output := append(assistant(id, "Background branch acknowledged; retained work continues."), toolCall(id, "Bash", map[string]any{"command": "printf x >> retained-sentinel; touch retained-started; while [ ! -f retained-probe ]; do sleep .02; done; printf p >> retained-sentinel; touch retained-probed; while [ ! -f retained-release ]; do sleep .02; done; printf z >> retained-sentinel"})...)
			completed(w, id, output)
		case 3:
			close(rootStarted)
			select {
			case <-r.Context().Done():
				close(rootCanceled)
			case <-releaseHTTP:
			}
		default:
			completed(w, id, assistant(id, "owned work ended"))
		}
	}))
	t.Cleanup(server.Close)
	h, records, directory := realHost(t, server, true)
	// LIFO: release only this fixture's HTTP/file barriers before realHost.Close,
	// even if an assertion fails. The success path NEVER supplies shell-release.
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseHTTP) })
		_ = os.WriteFile(filepath.Join(directory, "retained-probe"), nil, 0600)
		_ = os.WriteFile(filepath.Join(directory, "retained-release"), nil, 0600)
	})
	h.mu.Lock()
	worker := h.current
	h.mu.Unlock()
	scope := h.ControlScope()
	send(t, h, "retained-start", "start retained background work")
	waitFile(t, filepath.Join(directory, "retained-started"))
	select {
	case <-rootStarted:
	case <-time.After(5 * time.Second):
		t.Fatalf("root did not receive background acknowledgment; calls=%d", calls.Load())
	}
	records.mu.Lock()
	var target session.TurnID
	for _, item := range records.items {
		if item.Kind == sessionstore.ItemTurn {
			target = item.Data.(session.Turn).ID
		}
	}
	records.mu.Unlock()
	if target == "" {
		t.Fatal("no canonical root turn")
	}
	activity, err := h.ActivitySnapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(activity.Snapshot.Agents) != 1 {
		t.Fatalf("missing real child: %+v", activity)
	}
	child := activity.Snapshot.Agents[0]
	if child.Name != "retained" || child.Lifetime != "background" || child.ProcessStartedAt.IsZero() || child.ProcessWaitReturned || child.ReaderReturned || child.OwnerTerminalEnqueued {
		t.Fatalf("child not actively retained: %+v", child)
	}
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	childHistory, err := sessionstore.ReadLifecycle(t.Context(), store, session.ID(child.SessionID))
	if err != nil || childHistory.LastDrainSequence != 0 {
		t.Fatalf("active child already has drain receipt: %+v %v", childHistory, err)
	}
	abort := lifecycleIntent(t, "retained-abort", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, Scope: scope, TargetTurnID: string(target)})
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	if err := h.SubmitLifecycle(ctx, abort); err != nil {
		t.Fatal(err)
	}
	retainedWaitAbort(t, ctx, records, abort.ID, string(target))
	select {
	case <-rootCanceled:
	case <-ctx.Done():
		t.Fatal("root provider callback did not cancel")
	}
	// Continued effects AFTER exact canonical abort completion are the positive
	// survival oracle, not a terminal remote ticket or an unobserved process PID.
	if err := os.WriteFile(filepath.Join(directory, "retained-probe"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitFile(t, filepath.Join(directory, "retained-probed"))
	retainedSentinel(t, directory, "xp")
	before, err := sessionstore.ReadLifecycle(ctx, store, h.config.Bootstrap.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if before.AbortResults[abort.ID].Status != "completed" || before.LastDrainSequence != 0 {
		t.Fatalf("foreground abort pretended global cleanup: %+v", before)
	}
	shutdown := lifecycleIntent(t, "retained-shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: scope})
	if err := h.SubmitLifecycle(ctx, shutdown); err != nil {
		t.Fatal(err)
	}
	if err := h.AwaitShutdown(ctx); err != nil {
		t.Fatalf("strict actual child/global receipt and joins rejected: %v", err)
	}
	// AwaitShutdown must join the exact process and private output decoder; its
	// fresh post-ready scope-bound receipt is mandatory in addition to disk history.
	select {
	case <-worker.done:
	default:
		t.Fatal("worker process/output owner not joined")
	}
	select {
	case <-worker.scanDone:
	default:
		t.Fatal("private output decoder not joined")
	}
	if !worker.cleanupProven.Load() || worker.result() != nil {
		t.Fatalf("missing fresh private global proof: %v", worker.result())
	}
	history, err := sessionstore.ReadLifecycle(ctx, store, h.config.Bootstrap.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if history.ShutdownResults[shutdown.ID].Phase != "completed" || history.ShutdownResults[shutdown.ID].Scope != scope || history.LastDrainSequence <= before.LastRemoteSequence {
		t.Fatalf("no exact final canonical shutdown: %+v", history)
	}
	if err := history.ValidateResume(); err != nil {
		t.Fatal(err)
	}
	childHistory, err = sessionstore.ReadLifecycle(ctx, store, session.ID(child.SessionID))
	if err != nil || childHistory.LastDrainSequence == 0 {
		t.Fatalf("actual strict child lacks fresh canonical receipt: %+v %v", childHistory, err)
	}
	retainedSentinel(t, directory, "xp") // no z: deliberate shutdown joined active Bash
	for _, id := range []session.ID{"host-real-session", "real-session", session.ID(child.SessionID)} {
		lease, err := localfile.AcquireLease(directory, id)
		if err != nil {
			t.Fatalf("owner %s not released after proven shutdown: %v", id, err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("actual retained Bash continued xp after canonical abort; strict child receipt=%d global receipt=%d, exact worker/output joins and host/worker/child leases released", childHistory.LastDrainSequence, history.LastDrainSequence)
}

func retainedSentinel(t *testing.T, directory, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, "retained-sentinel"))
	if err != nil || string(data) != want {
		t.Fatalf("retained effects=%q want %q: %v", data, want, err)
	}
}
func retainedWaitAbort(t *testing.T, ctx context.Context, r *recorder, id inbox.ID, target string) {
	t.Helper()
	for {
		r.mu.Lock()
		var result *inbox.ForegroundAbortResult
		for _, item := range r.items {
			if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
				control, err := input.DecodeControlMessage()
				if err == nil && control.Mode == inbox.ForegroundAbortFinished {
					value := control.Parameters.(inbox.ForegroundAbortResult)
					if value.InputID == id {
						copy := value
						result = &copy
					}
				}
			}
		}
		r.mu.Unlock()
		if result != nil {
			if result.Status != "completed" || result.TargetTurnID != target {
				t.Fatalf("exact abort failed: %+v", result)
			}
			return
		}
		select {
		case <-r.changed:
		case <-ctx.Done():
			t.Fatal("no canonical exact foreground abort completion")
		}
	}
}
