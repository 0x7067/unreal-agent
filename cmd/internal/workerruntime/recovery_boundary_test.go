package workerruntime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// An intentionally lossy persistence driver models the crash window between
// committed operation intent and a later checkpoint/result commit. It runs real
// local shell primitives, but drops their updates instead of claiming that a
// terminal manager update is already a durable terminal operation. M0.2 covers
// actual abrupt process death; this test controls the durable boundary exactly.
func TestResumeRefusesShellIntentAcrossUncommittedExecution(t *testing.T) {
	for _, version := range []operation.Version{operation.VersionShell, operation.VersionShellBudget} {
		for _, boundary := range []string{"intent_only", "dispatched_before_effect", "effect_before_result_commit"} {
			t.Run(fmt.Sprintf("v%d/%s", version, boundary), func(t *testing.T) {
				b := bootstrap(t)
				lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
				store, err := localfile.New(b.Directory)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.Create(t.Context(), b.SessionID); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(b.Directory, "operations"), 0o700); err != nil {
					t.Fatal(err)
				}
				input := operation.ShellInput{Command: "printf x >> recovery-effect", Shell: "/bin/sh", Directory: b.Workspace}
				if boundary == "dispatched_before_effect" {
					input.Command = "while [ ! -e allow-effect ]; do sleep .01; done; printf x >> recovery-effect"
				}
				if version == operation.VersionShellBudget {
					input.Budget = executionbudget.Policy{WarnAfter: time.Second, Timeout: 10 * time.Second}
				}
				spec, err := operation.NewShellSpec(input, filepath.Join(b.Directory, "operations"), operation.DefaultMaxOutputLength)
				if err != nil {
					t.Fatal(err)
				}
				if spec.Version != version {
					t.Fatalf("shell spec version = %d, want %d", spec.Version, version)
				}
				op := operation.Operation{ID: "boundary-shell", Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
				if err := store.AppendTurn(t.Context(), b.SessionID, session.Turn{ID: "turn", Type: session.TurnRegular}); err != nil {
					t.Fatal(err)
				}
				arguments, err := json.Marshal(map[string]string{"command": input.Command, "directory": input.Directory})
				if err != nil {
					t.Fatal(err)
				}
				if err := store.AppendModelResponse(t.Context(), b.SessionID, sessionstore.ModelResponse{TurnID: "turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "Bash", Arguments: string(arguments)}}}}}); err != nil {
					t.Fatal(err)
				}
				if err := store.AppendToolCallStatus(t.Context(), b.SessionID, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
					t.Fatal(err)
				}

				if boundary != "intent_only" {
					runUncommittedBoundaryShell(t, op, boundary == "dispatched_before_effect")
				}
				wantEffect := ""
				if boundary == "effect_before_result_commit" {
					wantEffect = "x"
				}
				checkEffect := func() {
					t.Helper()
					data, err := os.ReadFile(filepath.Join(b.Workspace, "recovery-effect"))
					if wantEffect == "" {
						if !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("unexpected external effect: %q %v", data, err)
						}
					} else if err != nil || string(data) != wantEffect {
						t.Fatalf("effect = %q %v, want %q", data, err, wantEffect)
					}
				}
				checkEffect()
				if err := lease.Close(); err != nil {
					t.Fatal(err)
				}
				// Reopen from disk: a live manager's state is not recovery evidence.
				store, err = localfile.New(b.Directory)
				if err != nil {
					t.Fatal(err)
				}
				restored, err := store.Resume(t.Context(), b.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				if len(restored.Operations) != 1 || restored.Operations[0].Status != operation.StatusReady || !bytes.Equal(restored.Operations[0].State, op.State) {
					t.Fatalf("dropped updates changed durable intent: %+v", restored.Operations)
				}
				path := filepath.Join(b.Directory, string(b.SessionID)+".session.jsonl")
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var inputFrames, output bytes.Buffer
				if err := workerproto.Write(&inputFrames, workerproto.Request{Type: "start", Protocol: workerproto.Version, Bootstrap: &b}); err != nil {
					t.Fatal(err)
				}
				constructed, environmentRead := false, false
				available := []providers.Provider{{Name: "fake", NewClient: func(_, _ string, _ int, _ func(string) string) (providers.Client, error) {
					constructed = true
					return nil, errors.New("provider must not be constructed")
				}}}
				err = run(t.Context(), &inputFrames, &output, io.Discard, func(string) string { environmentRead = true; return "" }, testIdentity, available)
				if err == nil || !errors.Is(err, operation.ErrUnsupported) || !strings.Contains(err.Error(), `type="shell"`) || !strings.Contains(err.Error(), `status="ready"`) || constructed || environmentRead {
					t.Fatalf("unsafe resume = %v, provider=%t environment=%t", err, constructed, environmentRead)
				}
				scanner := workerproto.Scanner(&output)
				for scanner.Scan() {
					var event workerproto.Event
					if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
						t.Fatal(err)
					}
					if event.Type != "hello" && event.Type != "error" {
						t.Fatalf("unsafe resume emitted execution/history frame: %s", event.Type)
					}
				}
				if err := scanner.Err(); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("refused resume mutated session: %v", err)
				}
				checkEffect()
				lease, err = localfile.AcquireLease(b.Directory, b.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				if err := lease.Close(); err != nil {
					t.Fatal(err)
				}
				t.Logf("v%d %s: same durable ready intent, effect=%q; resume refused without execution or mutation", version, boundary, wantEffect)
			})
		}
	}
}

func runUncommittedBoundaryShell(t *testing.T, op operation.Operation, cancelBeforeEffect bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	manager := operation.NewLocalOperationManager(ctx)
	defer func() {
		cancel()
		deadline := time.NewTimer(10 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case _, open := <-manager.Updates():
				if !open {
					return
				}
			case <-deadline.C:
				t.Fatal("uncommitted shell driver did not drain; refusing to release ownership or attempt recovery")
			}
		}
	}()
	if err := manager.Add(op); err != nil {
		t.Fatal(err)
	}
	canceled := false
	for {
		select {
		case update, open := <-manager.Updates():
			if !open {
				t.Fatal("shell driver stopped before terminal update")
			}
			state, err := operation.DecodeShellState(update)
			if err != nil {
				t.Fatal(err)
			}
			if cancelBeforeEffect && !canceled && update.Status == operation.StatusAwaiting && state.ProcessGroupID != 0 {
				canceled = true
				if err := manager.Cancel(op.ID, "boundary interruption before effect"); err != nil {
					t.Fatal(err)
				}
			}
			if update.Status == operation.StatusCompleted || update.Status == operation.StatusCanceled || update.Status == operation.StatusFailed {
				want := operation.StatusCompleted
				if cancelBeforeEffect {
					want = operation.StatusCanceled
				}
				if update.Status != want || cancelBeforeEffect && !canceled {
					t.Fatalf("shell terminal update = %+v, want %s", update, want)
				}
				if !cancelBeforeEffect && (state.Result == nil || state.Result.ExitCode != 0) {
					t.Fatalf("effect finished without a successful result: %+v", state.Result)
				}
				// Do not SaveOperation: this is the uncommitted checkpoint/result.
				cancel()
				return
			}
		case <-ctx.Done():
			t.Fatal("shell boundary driver timed out:", ctx.Err())
		}
	}
}
