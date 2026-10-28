package workerhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const m25Operation operation.ID = "m25-original-shell"

// Controlled fixture, not a runtime hook: the real manager executes Bash and
// emits all checkpoints. The initial ready intent precedes actual dispatch;
// SaveOperation commits the selected latest checkpoint, while the final
// ToolCallStatus is withheld. AwaitStopped precedes the producer marker.
func TestM25ShellProducer(t *testing.T) {
	cut := os.Getenv("UA_M25_CUT")
	if cut == "" {
		t.Skip("owned controlled producer only")
	}
	dir := os.Getenv("UA_M25_STATE")
	if err := os.MkdirAll(filepath.Join(dir, "captures"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []session.ID{session.ID("host-" + string(m21Session)), m21Session} {
		lease, err := localfile.AcquireLease(dir, id)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
	}
	journal, err := ingress.Open(filepath.Join(dir, "ingress"), m21Session)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	store, err := localfile.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), m21Session); err != nil {
		t.Fatal(err)
	}
	input := operation.ShellInput{Command: "printf 'sentinel\\n' >> sentinel; printf preserved-output", Shell: "/bin/bash", Directory: dir}
	if strings.Contains(cut, "v4") {
		input.Budget = executionbudget.Policy{Timeout: time.Minute}
	}
	limit := 4096
	if strings.Contains(cut, "tail") {
		limit = 32
		input.Command = "printf 'sentinel\\n' >> sentinel; i=0; while [ $i -lt 12 ]; do printf preserved-output; printf preserved-stderr >&2; i=$((i+1)); done"
	}
	if cut == "canceling" {
		input.Command += "; while [ ! -f release ]; do sleep 0.02; done"
	}
	spec, err := operation.NewShellSpec(input, filepath.Join(dir, "captures"), limit)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: m25Operation, Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
	if err = store.AppendTurn(t.Context(), m21Session, session.Turn{ID: "m25-turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"command": input.Command})
	if err = store.AppendModelResponse(t.Context(), m21Session, sessionstore.ModelResponse{TurnID: "m25-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "m25-call", Name: tool.BashName, Arguments: string(args)}}}}}); err != nil {
		t.Fatal(err)
	}
	persist := func(current operation.Operation) {
		t.Helper()
		if err := store.AppendToolCallStatus(t.Context(), m21Session, sessionstore.ToolCallStatus{TurnID: "m25-turn", CallID: "m25-call", Status: tool.CallStatus{WaitingFor: []operation.ID{current.ID}}, Operations: []operation.Operation{current}}); err != nil {
			t.Fatal(err)
		}
	}
	persist(op)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	manager := operation.NewLocalOperationManager(ctx)
	defer func() {
		cancel()
		join, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := manager.AwaitStopped(join); err != nil {
			t.Error(err)
		}
	}()
	if err = manager.Add(op); err != nil {
		t.Fatal(err)
	}
	selected := cut == "ready"
	target := operation.ShellPhaseReadOut
	if strings.Contains(cut, "outtail") {
		target = operation.ShellPhaseReadOutTail
	} else if strings.Contains(cut, "errtail") {
		target = operation.ShellPhaseReadErrTail
	} else if strings.Contains(cut, "read-err-") {
		target = operation.ShellPhaseReadErr
	}
	for {
		select {
		case update, open := <-manager.Updates():
			if !open {
				t.Fatal("manager stopped before terminal")
			}
			state, err := operation.DecodeShellState(update)
			if err != nil {
				t.Fatal(err)
			}
			if !selected && update.Status == operation.StatusAwaiting {
				match := strings.HasPrefix(cut, "read-") && state.Phase == target || cut == "preparation" && state.Phase == operation.ShellPhaseCreateOut || cut == "process" && state.Phase == operation.ShellPhaseProcess && state.ProcessGroupID > 1
				if match {
					if err := store.SaveOperation(t.Context(), m21Session, update); err != nil {
						t.Fatal(err)
					}
					selected = true
				}
			}
			if cut == "canceling" && !selected {
				if update.Status == operation.StatusCanceling {
					if err := store.SaveOperation(t.Context(), m21Session, update); err != nil {
						t.Fatal(err)
					}
					selected = true
				} else if update.Status == operation.StatusAwaiting && state.Phase == operation.ShellPhaseProcess && state.ProcessGroupID > 1 {
					for {
						b, e := os.ReadFile(filepath.Join(dir, "sentinel"))
						if e == nil && string(b) == "sentinel\n" {
							break
						}
						select {
						case <-time.After(time.Millisecond):
						case <-ctx.Done():
							t.Fatal("cancel effect not observed")
						}
					}
					// Coordinator cancellation persists its intent before asking
					// the manager to cancel; the manager may terminalize in one step.
					checkpoint := update
					checkpoint.Status = operation.StatusCanceling
					if err := store.SaveOperation(t.Context(), m21Session, checkpoint); err != nil {
						t.Fatal(err)
					}
					selected = true
					if err := manager.Cancel(op.ID, "fixture cancellation"); err != nil {
						t.Fatal(err)
					}
				}
			}
			if update.Status == operation.StatusCompleted || cut == "canceling" && update.Status == operation.StatusCanceled {
				if !selected || cut != "canceling" && (state.Result == nil || !strings.Contains(state.Result.Out, "preserved-output") || state.Result.ExitCode != 0) {
					t.Fatalf("actual execution not proven: selected=%v state=%+v", selected, state)
				}
				cancel()
				join, stop := context.WithTimeout(context.Background(), 3*time.Second)
				err := manager.AwaitStopped(join)
				stop()
				if err != nil {
					t.Fatal(err)
				}
				fmt.Println("M25-DRAINED-COMMITTED:" + cut)
				for {
					time.Sleep(time.Hour)
				}
			}
			if update.Status == operation.StatusFailed || update.Status == operation.StatusCanceled {
				t.Fatalf("original failed: %+v", state)
			}
		case <-ctx.Done():
			t.Fatal("original execution timed out")
		}
	}
}

func TestM25ControlledShellRecovery(t *testing.T) {
	for _, cut := range []string{"read-v3", "read-v4", "read-outtail-v3", "read-err-v3", "read-errtail-v4", "read-missing-v3", "ready", "preparation", "process", "canceling"} {
		t.Run(cut, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			var calls atomic.Int64
			var preserved, toolError atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				var request struct {
					Input []struct {
						Type   string `json:"type"`
						Output any    `json:"output"`
					} `json:"input"`
				}
				if err := json.Unmarshal(body, &request); err != nil {
					t.Error(err)
				}
				for _, item := range request.Input {
					if item.Type == "function_call_output" {
						encoded, _ := json.Marshal(item.Output)
						preserved.Store(strings.Contains(string(encoded), "preserved-output"))
						toolError.Store(strings.Contains(string(encoded), "Error:"))
					}
				}
				completed(w, n, assistant(n, "M25 recovered"))
			}))
			defer server.Close()
			cfg := m23Config(t, server.URL)
			dir := cfg.Bootstrap.Directory
			if err := os.MkdirAll(filepath.Join(dir, "ingress"), 0700); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(exe, "-test.run=^TestM25ShellProducer$", "-test.count=1")
			child.Env = append(os.Environ(), "UA_M25_CUT="+cut, "UA_M25_STATE="+dir)
			var stderr bytes.Buffer
			child.Stderr = &stderr
			stdout, err := child.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = child.Start(); err != nil {
				t.Fatal(err)
			}
			marker := make(chan bool, 1)
			readerDone := make(chan error, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				seen := false
				for scanner.Scan() {
					t.Logf("producer: %s", scanner.Text())
					if scanner.Text() == "M25-DRAINED-COMMITTED:"+cut {
						seen = true
						marker <- true
					}
				}
				if !seen {
					marker <- false
				}
				readerDone <- scanner.Err()
			}()
			joined := false
			defer func() {
				if !joined {
					_ = child.Process.Kill()
					<-readerDone
					_ = child.Wait()
				}
			}()
			select {
			case ok := <-marker:
				if !ok {
					t.Fatalf("producer failed before drained boundary: %s", stderr.String())
				}
			case <-ctx.Done():
				t.Fatal("producer marker timed out")
			}
			m21AssertLeases(t, dir, true)
			if err = child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			// Scanner must actually finish before Cmd.Wait closes its pipe.
			select {
			case err = <-readerDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("stdout reader join unconfirmed")
			}
			err = child.Wait()
			joined = true
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("producer Wait=%v", err)
			}
			status, ok := exit.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("not owned SIGKILL: %v", err)
			}
			m21AssertLeases(t, dir, false)
			sentinel := func() {
				t.Helper()
				data, err := os.ReadFile(filepath.Join(dir, "sentinel"))
				if err != nil || string(data) != "sentinel\n" {
					t.Fatalf("side effect must occur exactly once: %q %v", data, err)
				}
			}
			sentinel()
			_ = m21ReadItems(t, dir)
			store, err := localfile.New(dir)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := store.Resume(t.Context(), m21Session)
			if err != nil {
				t.Fatal(err)
			}
			if len(restored.Operations) != 1 {
				t.Fatalf("restored unfinished checkpoints: %+v", restored.Operations)
			}
			checkpoint := restored.Operations[0]

			state, err := operation.DecodeShellState(checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			supported := strings.HasPrefix(cut, "read-")
			if supported && (checkpoint.Status != operation.StatusAwaiting || state.Phase == "" || state.PendingExitCode == nil || *state.PendingExitCode != 0 || state.ProcessGroupID != 0 || state.Result != nil || state.TerminalError != "" || state.OutPath != filepath.Join(dir, "captures", string(m25Operation), "out") || state.ErrPath != filepath.Join(dir, "captures", string(m25Operation), "err")) {
				t.Fatalf("not a real recorded-exit checkpoint: %+v", state)
			}
			if strings.Contains(cut, "missing") {
				if err = os.Remove(state.OutPath); err != nil {
					t.Fatal(err)
				}
			}
			canonical := filepath.Join(dir, string(m21Session)+".session.jsonl")
			before, err := os.ReadFile(canonical)
			if err != nil {
				t.Fatal(err)
			}
			records := &recorder{changed: make(chan struct{}, 1)}
			cfg.OnItem, cfg.OnStatus = records.item, records.status
			h, err := Start(ctx, cfg)
			if !supported {
				if h != nil {
					_ = h.Close(ctx)
				}
				if err == nil || !strings.Contains(err.Error(), "prior execution uncertain") || !strings.Contains(err.Error(), "ownership reconciliation required") || calls.Load() != 0 {
					t.Fatalf("uncertain checkpoint admitted: err=%v calls=%d", err, calls.Load())
				}
				after, e := os.ReadFile(canonical)
				if e != nil || !bytes.Equal(before, after) {
					t.Fatalf("refusal changed canonical: %v", e)
				}
				sentinel()
				m21AssertLeases(t, dir, false)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				closeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_ = h.Close(closeCtx)
			}()
			waitResponses(t, records, 2)
			if err = h.Close(ctx); err != nil {
				t.Fatal(err)
			}
			terminal := 0
			for _, item := range m21ReadItems(t, dir) {
				if s, ok := item.Data.(sessionstore.ToolCallStatus); ok {
					for _, op := range s.Operations {
						if op.ID != m25Operation {
							t.Fatalf("operation identity changed: %s", op.ID)
						}
						if op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed {
							continue
						}
						terminal++
						final, e := operation.DecodeShellState(op)
						if e != nil {
							t.Fatal(e)
						}
						if strings.Contains(cut, "missing") {
							if op.Status != operation.StatusFailed || final.TerminalError == "" || final.Result != nil {
								t.Fatalf("missing capture invented result: %+v", final)
							}
						} else if op.Status != operation.StatusCompleted || final.Result == nil || !strings.Contains(final.Result.Out, "preserved-output") || final.Result.ExitCode != 0 {
							t.Fatalf("wrong recovered output: %+v", final)
						}
					}
				}
			}
			if terminal != 1 || calls.Load() != 1 {
				t.Fatalf("canonical completions/provider calls=%d/%d", terminal, calls.Load())
			}
			if strings.Contains(cut, "missing") {
				if !toolError.Load() || preserved.Load() {
					t.Fatal("provider did not receive safe missing-capture error")
				}
			} else if !preserved.Load() {
				t.Fatal("actual provider did not see preserved output")
			}
			sentinel()
			m21AssertLeases(t, dir, false)
			t.Logf("controlled %s: actual manager AwaitStopped, producer SIGKILL+scanner+Wait joined, same operation, one terminal/provider request, sentinel=1", cut)
		})
	}
}

// A still-live executor is deliberately owned outside the replacement runtime.
// This controlled fixture tests indistinguishability of a stale ready intent,
// not an uninstrumented runtime-crash hook. No persisted numeric PGID is used
// for cleanup: only the original manager capability can cancel/join this work.
func TestM25LiveOldExecutorRemainsUncertain(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); completed(w, 1, assistant(1, "forbidden")) }))
	defer server.Close()
	cfg := m23Config(t, server.URL)
	dir := cfg.Bootstrap.Directory
	if err := os.MkdirAll(filepath.Join(dir, "captures"), 0700); err != nil {
		t.Fatal(err)
	}
	store, err := localfile.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), m21Session); err != nil {
		t.Fatal(err)
	}
	input := operation.ShellInput{Command: "printf 'sentinel\\n' >> sentinel; printf preserved-output; while [ ! -f release ]; do sleep 0.02; done", Shell: "/bin/sh", Directory: dir}
	spec, err := operation.NewShellSpec(input, filepath.Join(dir, "captures"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: m25Operation, Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
	if err = store.AppendTurn(t.Context(), m21Session, session.Turn{ID: "live-old", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if err = store.AppendModelResponse(t.Context(), m21Session, sessionstore.ModelResponse{TurnID: "live-old", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "live-call", Name: tool.BashName, Arguments: `{"command":"owned fixture"}`}}}}}); err != nil {
		t.Fatal(err)
	}
	if err = store.AppendToolCallStatus(t.Context(), m21Session, sessionstore.ToolCallStatus{TurnID: "live-old", CallID: "live-call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
		t.Fatal(err)
	}
	oldCtx, stopOld := context.WithCancel(t.Context())
	manager := operation.NewLocalOperationManager(oldCtx)
	readerDone := make(chan struct{})
	terminal := make(chan operation.Operation, 1)
	go func() {
		defer close(readerDone)
		for u := range manager.Updates() {
			if u.Status == operation.StatusCompleted || u.Status == operation.StatusCanceled || u.Status == operation.StatusFailed {
				terminal <- u
			}
		}
	}()
	defer func() {
		stopOld()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := manager.AwaitStopped(ctx); err != nil {
			t.Error(err)
		}
		select {
		case <-readerDone:
		case <-ctx.Done():
			t.Error("original manager reader join missing")
		}
	}()
	if err = manager.Add(op); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		data, e := os.ReadFile(filepath.Join(dir, "sentinel"))
		if e == nil && string(data) == "sentinel\n" {
			break
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			t.Fatal("original sentinel not observed")
		}
	}
	canonical := filepath.Join(dir, string(m21Session)+".session.jsonl")
	before, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		h, e := Start(ctx, cfg)
		if h != nil {
			_ = h.Close(ctx)
		}
		if e == nil || !strings.Contains(e.Error(), "prior execution uncertain") || !strings.Contains(e.Error(), "ownership reconciliation required") {
			t.Fatalf("uncertain live executor admitted: %v", e)
		}
		m21AssertLeases(t, dir, false)
	}
	if calls.Load() != 0 {
		t.Fatal("uncertain execution reached provider")
	}
	after, err := os.ReadFile(canonical)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed canonical history")
	}
	select {
	case u := <-terminal:
		t.Fatalf("old work incorrectly considered stopped: %+v", u)
	default:
	}
	if err = manager.LocalCleanupProof([]operation.ID{op.ID}); !errors.Is(err, operation.ErrLocalCleanupUnproven) {
		t.Fatalf("live executor cleanup laundered: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "sentinel"))
	if err != nil || string(data) != "sentinel\n" {
		t.Fatalf("replay changed sentinel: %q %v", data, err)
	}
	// Signal only through the original manager capability, then demand its actual
	// cancellation/completion and local drain receipt. No PID lookup or kill(2).
	if err = manager.Cancel(op.ID, "owned fixture cleanup"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-terminal:
	case <-ctx.Done():
		t.Fatal("original work terminal event missing")
	}
	stopOld()
	if err = manager.AwaitStopped(ctx); err != nil {
		t.Fatal(err)
	}
	if err = manager.LocalCleanupProof([]operation.ID{op.ID}); err != nil {
		t.Fatalf("original owned cleanup not proven: %v", err)
	}
}
