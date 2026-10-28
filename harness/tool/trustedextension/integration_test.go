package trustedextension_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/trustedextension"
)

type captureSubmission struct{ op operation.Operation }

func (c *captureSubmission) CurrentTurnID() string { return "turn" }

func (c *captureSubmission) Submit(s operation.Spec) operation.ID {
	c.op = operation.Operation{ID: "echo-" + operation.ID(fmt.Sprint(c.op.ID)), Type: s.Type, Version: s.Version, MaxOutputLength: s.MaxOutputLength, State: bytes.Clone(s.State), Status: operation.StatusReady}
	return c.op.ID
}

// This integration deliberately uses the production translator, canonical local
// store and operation-manager actor, not a fake dispatch or in-memory journal.
func TestM41CanonicalCommitBeforePinnedGenerationDispatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := localfile.New(root)
	if err != nil {
		t.Fatal(err)
	}
	const sid session.ID = "extension-canonical"
	if _, err = store.Create(ctx, sid); err != nil {
		t.Fatal(err)
	}
	turn := session.Turn{ID: "turn", Type: session.TurnRegular}
	if err = store.AppendTurn(ctx, sid, turn); err != nil {
		t.Fatal(err)
	}
	r := extension.NewRegistry()
	a := extension.Descriptor{Name: extension.Name, Generation: 1, SchemaVersion: 1, ImplementationVersion: 1, Implementation: "A"}
	b := a
	b.Generation = 2
	b.Implementation = "B"
	if err = r.Install(a); err != nil {
		t.Fatal(err)
	}
	if err = r.Select(1); err != nil {
		t.Fatal(err)
	}
	translated := trustedextension.New(r)
	entered := make(chan uint64, 2)
	gate := make(chan struct{})
	proofErr := make(chan error, 2)
	var executions atomic.Int32
	h, err := extension.NewHandler(extension.Config{Registry: r, BeforeExecute: func(ctx context.Context, d extension.Descriptor) error {
		executions.Add(1)
		// Reopen the real canonical file after dispatch. Both the status and initial
		// operation must have survived the same AppendToolCallStatus transaction.
		disk, e := localfile.New(root)
		if e == nil {
			var page sessionstore.Page
			page, e = disk.Items(ctx, sid, sessionstore.BeforeFirst, 100)
			if e == nil {
				found := false
				for _, it := range page.Items {
					if status, ok := it.Data.(sessionstore.ToolCallStatus); ok {
						for _, op := range status.Operations {
							_, p, de := extension.Decode(op)
							if de == nil && p.Generation == d.Generation && len(status.Status.WaitingFor) == 1 && status.Status.WaitingFor[0] == op.ID {
								found = true
							}
						}
					}
				}
				if !found {
					e = fmt.Errorf("generation %d executed without atomic committed status+operation", d.Generation)
				}
			}
		}
		proofErr <- e
		entered <- d.Generation
		if d.Generation == 1 {
			select {
			case <-gate:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return e
	}})
	if err != nil {
		t.Fatal(err)
	}
	managerCtx, stopManager := context.WithCancel(ctx)
	manager := operation.NewLocalOperationManager(managerCtx, h)
	t.Cleanup(func() {
		_ = h.Close()
		stopManager()
		joinCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if e := manager.AwaitStopped(joinCtx); e != nil && !errors.Is(e, operation.ErrCleanupUncertain) {
			t.Error(e)
		}
		if e := h.AwaitStopped(joinCtx); e != nil {
			t.Error(e)
		}
	})
	call := func(id, input string) (tool.CallStatus, operation.Operation) {
		t.Helper()
		call := llm.ToolCall{CallID: id, Name: "TrustedEcho", Arguments: fmt.Sprintf(`{"input":%q}`, input)}
		if id == "a" {
			second := llm.ToolCall{CallID: "b", Name: "TrustedEcho", Arguments: `{"input":"new-input"}`}
			if e := store.AppendModelResponse(ctx, sid, sessionstore.ModelResponse{TurnID: turn.ID, Response: llm.Response{ID: "response", Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: call}, {Type: llm.ItemToolCall, Data: second}}}}); e != nil {
				t.Fatal(e)
			}
		}
		before, e := os.ReadFile(filepath.Join(root, string(sid)+".session.jsonl"))
		if e != nil {
			t.Fatal(e)
		}
		capture := &captureSubmission{op: operation.Operation{ID: operation.ID(id)}}
		status := translated.Translate(capture, call)
		if status.Error != "" || len(status.WaitingFor) != 1 {
			t.Fatalf("translate: %+v", status)
		}
		after, e := os.ReadFile(filepath.Join(root, string(sid)+".session.jsonl"))
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("translator performed canonical I/O")
		}
		return status, capture.op
	}
	statusA, opA := call("a", "old-input")
	if executions.Load() != 0 {
		t.Fatal("translation executed before commit")
	}
	resumed, e := store.Resume(ctx, sid)
	if e != nil {
		t.Fatal(e)
	}
	if len(resumed.Operations) != 0 {
		t.Fatal("uncommitted operation escaped translation")
	}
	if e = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: turn.ID, CallID: "a", Status: statusA, Operations: []operation.Operation{opA}}); e != nil {
		t.Fatal(e)
	}
	if executions.Load() != 0 {
		t.Fatal("commit itself dispatched execution")
	}
	if e = manager.Add(opA); e != nil {
		t.Fatal(e)
	}
	select {
	case g := <-entered:
		if g != 1 {
			t.Fatal(g)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if e = <-proofErr; e != nil {
		t.Fatal(e)
	}
	if e = r.Install(b); e != nil {
		t.Fatal(e)
	}
	if e = r.Select(2); e != nil {
		t.Fatal(e)
	}
	statusB, opB := call("b", "new-input")
	_, planB, e := extension.Decode(opB)
	if e != nil || planB.Generation != 2 {
		t.Fatalf("new selection not pinned: %+v %v", planB, e)
	}
	if executions.Load() != 1 {
		t.Fatal("new translation dispatched before commit")
	}
	if e = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: turn.ID, CallID: "b", Status: statusB, Operations: []operation.Operation{opB}}); e != nil {
		t.Fatal(e)
	}
	if e = manager.Add(opB); e != nil {
		t.Fatal(e)
	}
	select {
	case g := <-entered:
		if g != 2 {
			t.Fatal(g)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if e = <-proofErr; e != nil {
		t.Fatal(e)
	}
	terminal := func(id operation.ID, want string) operation.Operation {
		t.Helper()
		for {
			select {
			case op := <-manager.Updates():
				if op.ID != id {
					if op.Status == operation.StatusCompleted {
						t.Fatalf("other call completed before release: %s", op.ID)
					}
					continue
				}
				if op.Status != operation.StatusCompleted {
					continue
				}
				s, _, e := extension.Decode(op)
				if e != nil || s.TerminalResult != want {
					t.Fatalf("terminal %+v %v", s, e)
				}
				if e = store.SaveOperation(ctx, sid, op); e != nil {
					t.Fatal(e)
				}
				return op
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	_ = terminal(opB.ID, "B:new-input")
	if e = r.Remove(1); e == nil {
		t.Fatal("removed pinned running A")
	}
	close(gate)
	doneA := terminal(opA.ID, "A:old-input")
	// Historical projection does not consult the selected implementation or even
	// require any registry. It returns the stored output byte-for-byte.
	result, e := trustedextension.New(nil).TranslateResult("a", statusA, []operation.Operation{doneA})
	if e != nil {
		t.Fatal(e)
	}
	out := result.ToLLMResult()
	if len(out.Output) != 1 || out.Output[0].Value != "A:old-input" {
		t.Fatalf("historical result rewritten: %+v", out)
	}
}
