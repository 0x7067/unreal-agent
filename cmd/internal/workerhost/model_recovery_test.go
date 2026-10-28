package workerhost

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// All state, credentials and processes here belong to this test. The existing
// TestMain dispatches the actual runtime/provider, not the protocol-only helper.
func m23Config(t *testing.T, endpoint string) Config {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UA_HOST_TEST_REAL", "1")
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Config{Command: []string{exe, "-test.run=^$", "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: m21Session, Directory: root, Workspace: root, Provider: "openai", BaseURL: endpoint, MaxAttempts: 1, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, Shell: "/bin/sh", SystemPrompt: "Private model recovery fixture"}, Stderr: os.Stderr}
}

func m23Start(t *testing.T, cfg Config) *Host {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	t.Cleanup(cancel)
	h, err := Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.Close(ctx); err != nil {
			t.Logf("owned host Close: %v", err)
		}
	})
	return h
}

func m23Close(t *testing.T, h *Host, dir string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// After SIGKILL an unproven execution-drain error is truthful. Lease checks
	// below are made only after process/reader/owner barriers have completed.
	if err := h.Close(ctx); err != nil {
		t.Logf("Close (no invented drain proof): %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("owned Close did not join within budget: %v", ctx.Err())
	}
	select {
	case <-h.dispatcherDone:
	default:
		t.Fatal("owned dispatcher join unconfirmed")
	}
	h.mu.Lock()
	remaining := h.current
	h.mu.Unlock()
	if remaining != nil {
		t.Fatal("owned worker still published after Close")
	}
	m21AssertLeases(t, dir, false)
}

func m23KillJoin(t *testing.T, h *Host, dir string) {
	t.Helper()
	h.mu.Lock()
	p := h.current
	h.mu.Unlock()
	if p == nil {
		t.Fatal("owned worker disappeared before kill")
	}
	m21AssertLeases(t, dir, true)
	if err := p.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	// consume owns Wait; never call Cmd.Wait a second time. done follows the
	// actual child join, scanner drain, and owner callbacks.
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("owned worker join unconfirmed")
	}
	if p.command.ProcessState == nil {
		t.Fatal("owned killed process exit not observed")
	}
	status, ok := p.command.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("owned SIGKILL not observed: %v", p.command.ProcessState)
	}
	m23Close(t, h, dir)
}

func m23Wait(t *testing.T, dir string, predicate func([]sessionstore.Item) bool) []sessionstore.Item {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		items := m21ReadItems(t, dir)
		if predicate(items) {
			return items
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("canonical boundary not reached: %+v", items)
		}
	}
}

func m23Records(t *testing.T, items []sessionstore.Item) []inbox.ModelRequestRecord {
	t.Helper()
	var records []inbox.ModelRequestRecord
	for _, item := range items {
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
			c, err := input.DecodeControlMessage()
			if err != nil {
				t.Fatal(err)
			}
			if c.Mode == inbox.ModelRequest {
				records = append(records, c.Parameters.(inbox.ModelRequestRecord))
			}
		}
	}
	return records
}

func m23Submit(t *testing.T, h *Host, id string) {
	t.Helper()
	if err := h.Submit(t.Context(), inbox.Input{ID: inbox.ID(id), Kind: inbox.InputExternal, Payload: []byte(`"private external input"`)}); err != nil {
		t.Fatal(err)
	}
}

func TestM23OwnedInterruptedModelRecovery(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan int64, 3)
	joined := make(chan int64, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		defer func() { joined <- n }()
		_, _ = io.Copy(io.Discard, r.Body)
		if n <= 2 {
			w.Header().Set("Content-Type", "text/event-stream")
			// A complete-looking tool item, but no committed response.completed.
			// Streaming text must never become translator/operation authority.
			fmt.Fprint(w, "data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"partial\",\"type\":\"function_call\",\"call_id\":\"uncommitted\",\"name\":\"Bash\",\"arguments\":\"{\\\"command\\\":\\\"touch forbidden-effect\\\"}\",\"status\":\"completed\"}}\n\n")
			w.(http.Flusher).Flush()
			entered <- n
			<-r.Context().Done()
			return
		}
		completed(w, n, assistant(n, "fresh request completed"))
	}))
	t.Cleanup(server.Close)
	cfg := m23Config(t, server.URL)
	dir := cfg.Bootstrap.Directory
	h := m23Start(t, cfg)
	m23Submit(t, h, "m23-first")
	for attempt := int64(1); attempt <= 2; attempt++ {
		select {
		case n := <-entered:
			if n != attempt {
				t.Fatalf("HTTP attempt %d", n)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("request not observed")
		}
		before := m21ReadItems(t, dir)
		records := m23Records(t, before)
		last := records[len(records)-1]
		if last.Action != "started" || last.Attempt != int(attempt) || last.Limit != 2 || last.Version != 1 {
			t.Fatalf("charge before HTTP: %+v", last)
		}
		turnFound := false
		chargeIndex, turnIndex := -1, -1
		for index, item := range before {
			if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
				c, e := input.DecodeControlMessage()
				if e == nil && c.Mode == inbox.ModelRequest {
					r := c.Parameters.(inbox.ModelRequestRecord)
					if r.Action == "started" && r.TurnID == last.TurnID {
						chargeIndex = index
					}
				}
			}
			if turn, ok := item.Data.(session.Turn); ok && string(turn.ID) == last.TurnID {
				turnFound = true
				turnIndex = index
			}
			if item.Kind == sessionstore.ItemModelResponse || item.Kind == sessionstore.ItemToolCallStatus {
				t.Fatalf("uncommitted output became authority: %+v", item)
			}
		}
		if !turnFound || chargeIndex < 0 || chargeIndex >= turnIndex {
			t.Fatal("HTTP request without charged turn")
		}
		m23KillJoin(t, h, dir)
		select {
		case n := <-joined:
			if n != attempt {
				t.Fatal("wrong handler joined")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("HTTP handler not joined")
		}
		h = m23Start(t, cfg)
	}
	items := m23Wait(t, dir, func(items []sessionstore.Item) bool {
		rs := m23Records(t, items)
		return len(rs) > 0 && rs[len(rs)-1].Action == "exhausted"
	})
	rs := m23Records(t, items)
	if len(rs) != 5 {
		t.Fatalf("request lifecycle: %+v", rs)
	}
	want := []string{"started", "interrupted", "started", "interrupted", "exhausted"}
	for i, r := range rs {
		if r.Action != want[i] || r.RequestID != rs[0].RequestID {
			t.Fatalf("unstable lifecycle: %+v", rs)
		}
	}
	if rs[0].TurnID == rs[2].TurnID || rs[2].Attempt != 2 {
		t.Fatalf("retry reused turn/attempt: %+v", rs)
	}
	if calls.Load() != 2 {
		t.Fatalf("exhaustion dispatched third HTTP request: %d", calls.Load())
	}
	for _, item := range items {
		if item.Kind == sessionstore.ItemToolCallStatus || item.Kind == sessionstore.ItemModelResponse {
			t.Fatalf("invented result/usage/tool authority: %+v", item)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "forbidden-effect")); !os.IsNotExist(err) {
		t.Fatalf("uncommitted tool effect: %v", err)
	}
	// Exhaustion leaves the same runtime alive and ready for NEW external input.
	m23Submit(t, h, "m23-new")
	final := m23Wait(t, dir, func(items []sessionstore.Item) bool {
		for _, i := range items {
			if i.Kind == sessionstore.ItemModelResponse {
				return true
			}
		}
		return false
	})
	rs = m23Records(t, final)
	last := rs[len(rs)-1]
	if calls.Load() != 3 || last.Action != "started" || last.Attempt != 1 || last.RequestID == rs[0].RequestID {
		t.Fatalf("new logical request: calls=%d records=%+v", calls.Load(), rs)
	}
	m23Close(t, h, dir)
	select {
	case <-joined:
	case <-time.After(5 * time.Second):
		t.Fatal("completed HTTP handler not joined")
	}
}

func TestM23CommittedResponseReplay(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		completed(w, n, assistant(n, "committed answer"))
	}))
	t.Cleanup(server.Close)
	cfg := m23Config(t, server.URL)
	dir := cfg.Bootstrap.Directory
	h := m23Start(t, cfg)
	m23Submit(t, h, "m23-commit")
	m23Wait(t, dir, func(items []sessionstore.Item) bool {
		for _, i := range items {
			if i.Kind == sessionstore.ItemModelResponse {
				return true
			}
		}
		return false
	})
	m23KillJoin(t, h, dir)
	stable := m21ReadItems(t, dir)
	h = m23Start(t, cfg)
	scope := h.ControlScope()
	m23Close(t, h, dir)
	got := m21ReadItems(t, dir)
	if calls.Load() != 1 || len(got) != len(stable)+2 || !reflect.DeepEqual(stable, got[:len(stable)]) {
		t.Fatalf("response replay changed history or dispatched: calls=%d items=%d -> %d", calls.Load(), len(stable), len(got))
	}
	for i, mode := range []inbox.ControlMode{inbox.StopHard, inbox.ExecutionDrained} {
		input, ok := got[len(stable)+i].Data.(inbox.Input)
		if !ok {
			t.Fatal("non-control replay tail")
		}
		c, err := input.DecodeControlMessage()
		if err != nil || c.Mode != mode {
			t.Fatalf("unexpected replay tail: %+v %v", c, err)
		}
		if mode == inbox.ExecutionDrained && c.Parameters.(inbox.ExecutionDrainReceipt).Scope != scope {
			t.Fatal("foreign replay drain scope")
		}
	}
}
