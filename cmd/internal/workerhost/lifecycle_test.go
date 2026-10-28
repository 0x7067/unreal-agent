package workerhost

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func lifecycleIntent(t *testing.T, id inbox.ID, mode inbox.ControlMode, parameters any) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: parameters})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
}

func TestDeliberateShutdownDrainsRealBashBeforeCanonicalCompletionAndLeaseRelease(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		if id == 1 {
			completed(w, id, toolCall(id, "Bash", map[string]any{"command": "printf x >> shutdown-sentinel; touch shutdown-started; sleep 30"}))
			return
		}
		completed(w, id, assistant(id, "done"))
	}))
	defer server.Close()
	h, _, directory := realHost(t, server, false)
	scope := h.ControlScope()
	wrong := lifecycleIntent(t, "wrong-scope", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: scope + "/old"})
	if err := h.SubmitLifecycle(t.Context(), wrong); err == nil || h.ShutdownSealed() {
		t.Fatalf("wrong scope mutated host: %v", err)
	}
	send(t, h, "start-bash", "start owned Bash")
	waitFile(t, filepath.Join(directory, "shutdown-started"))
	conflict := lifecycleIntent(t, "start-bash", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: scope})
	if err := h.SubmitLifecycle(t.Context(), conflict); err == nil || h.ShutdownSealed() {
		t.Fatalf("conflicting stable identity changed lifecycle: %v", err)
	}

	input := lifecycleIntent(t, "explicit-shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: scope})
	if err := h.SubmitLifecycle(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if err := h.SubmitLifecycle(t.Context(), input); err != nil {
		t.Fatalf("same stable control retry: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	if err := h.AwaitShutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.SubmitLifecycle(t.Context(), input); err != nil {
		t.Fatalf("completed stable control retry: %v", err)
	}
	if err := h.Submit(t.Context(), inbox.Input{ID: "too-late", Kind: inbox.InputExternal, Payload: []byte(`"must not run"`)}); err == nil {
		t.Fatal("sealed ingress reopened")
	}
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	history, err := sessionstore.ReadLifecycle(t.Context(), store, h.config.Bootstrap.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if history.ShutdownResults[input.ID].Phase != "completed" {
		t.Fatalf("missing final canonical completion: %+v", history)
	}
	if err := history.ValidateResume(); err != nil {
		t.Fatal(err)
	}
	lease, err := localfile.AcquireLease(directory, "host-real-session")
	if err != nil {
		t.Fatalf("proven shutdown retained parent authority: %v", err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	effect, err := os.ReadFile(filepath.Join(directory, "shutdown-sentinel"))
	if err != nil || string(effect) != "x" {
		t.Fatalf("effect repeated/lost: %q %v", effect, err)
	}
}

func TestShutdownWaitTimeoutRetainsAuthorityAfterWorkerLeaseRelease(t *testing.T) {
	blocked, release := make(chan struct{}), make(chan struct{})
	var blockedOnce, releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { completed(w, 1, assistant(1, "unused")) }))
	defer server.Close()
	h, _, directory := realHostWith(t, server, false, func(c *Config) {
		prior := c.OnItem
		c.OnItem = func(item sessionstore.Item) {
			if item.Kind == sessionstore.ItemInput {
				input := item.Data.(inbox.Input)
				if input.Kind == inbox.InputControl {
					control, err := input.DecodeControlMessage()
					if err == nil && control.Mode == inbox.ExecutionDrained {
						blockedOnce.Do(func() { close(blocked) })
						<-release
					}
				}
			}
			prior(item)
		}
	})
	// Register after realHost cleanup: even an assertion failure releases only this
	// owned publication barrier before Close joins the independently running drain.
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	input := lifecycleIntent(t, "wait-timeout", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: h.ControlScope()})
	if err := h.SubmitLifecycle(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("owned publication barrier not reached")
	}
	waitCtx, stopWait := context.WithTimeout(t.Context(), time.Millisecond)
	err := h.AwaitShutdown(waitCtx)
	stopWait()
	if !errors.Is(err, ErrShutdownUncertain) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout reported completion: %v", err)
	}
	closeCtx, stopClose := context.WithTimeout(t.Context(), time.Millisecond)
	err = h.Close(closeCtx)
	stopClose()
	if !errors.Is(err, ErrShutdownUncertain) {
		t.Fatalf("Close bypassed shutdown fence: %v", err)
	}
	// Independently observe the worker writer lease becoming available. Parent
	// authority must still reject takeover while the exact output callback is held.
	deadline := time.Now().Add(4 * time.Second)
	for {
		lease, leaseErr := localfile.AcquireLease(directory, "real-session")
		if leaseErr == nil {
			_ = lease.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker lease not released: %v", leaseErr)
		}
		time.Sleep(time.Millisecond)
	}
	if competing, startErr := Start(t.Context(), h.config); startErr == nil {
		_ = competing.Close(t.Context())
		t.Fatal("competing resident acquired authority")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runnerCtx, stopRunner := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopRunner()
	command := exec.CommandContext(runnerCtx, executable, "agent-runner", "-session-directory", directory, "-workspace", directory, `{"session_id":"real-session","prompt":"competing execution forbidden"}`)
	output, runErr := command.CombinedOutput()
	if runErr == nil || !strings.Contains(string(output), "authority") {
		t.Fatalf("actual standalone runner escaped authority fence: %v %s", runErr, output)
	}
	if err := h.SubmitLifecycle(t.Context(), input); err != nil {
		t.Fatalf("lost-ack retry while cleanup pending: %v", err)
	}
	select {
	case <-h.Done():
		t.Fatal("host terminated on wait timeout")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	finishCtx, stopFinish := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopFinish()
	if err := h.AwaitShutdown(finishCtx); err != nil {
		t.Fatal(err)
	}
}

func TestControlScopeRotationAndPreReadyReplayCannotProveCleanup(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("idle scope test invoked provider") }))
	defer provider.Close()
	first, _, directory := realHost(t, provider, false)
	originalScope := first.ControlScope()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	history, err := sessionstore.ReadLifecycle(ctx, store, "real-session")
	if err != nil || history.LastDrainSequence == 0 {
		t.Fatalf("first runtime emitted no owned receipt: %v", err)
	}
	config := first.config
	config.OnItem = nil
	config.OnStatus = nil
	// Deliberately reuse a private test prefix to make a historical receipt's
	// nonce match. Production resident incarnations are always freshly minted.
	replacement, err := Start(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		wait, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := replacement.Close(wait); err != nil {
			t.Error(err)
		}
	})
	replacement.mu.Lock()
	p := replacement.current
	replacement.mu.Unlock()
	if replacement.ControlScope() != originalScope || p.cleanupProven.Load() {
		t.Fatal("pre-ready historical replay satisfied current process cleanup")
	}
	if err := replacement.Reload(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if replacement.ControlScope() == originalScope {
		t.Fatal("worker generation retained old control scope")
	}
	replacement.mu.Lock()
	p = replacement.current
	replacement.mu.Unlock()
	if p.cleanupProven.Load() {
		t.Fatal("new generation inherited old process cleanup")
	}
	shutdown := lifecycleIntent(t, "rotated-stop", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: replacement.ControlScope()})
	if err := replacement.SubmitLifecycle(ctx, shutdown); err != nil {
		t.Fatal(err)
	}
	if err := replacement.AwaitShutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
