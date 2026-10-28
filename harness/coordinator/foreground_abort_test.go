package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func foregroundAbortInput(t *testing.T, id inbox.ID, target session.TurnID) inbox.Input {
	t.Helper()
	data, err := json.Marshal(inbox.ControlMessage{Mode: inbox.AbortForeground, Parameters: inbox.ForegroundAbort{Version: 1, TargetTurnID: string(target)}})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: data}
}
func TestForegroundAbortWaitsCallbacksDiscardsLateToolsAndKeepsSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		// This deliberately adversarial provider ignores cancellation until released.
		run.current.dependencies.LLM = &fakeAdapter{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
			call := stopTestCall{ctx: ctx, request: request, response: make(chan llm.Response)}
			run.calls = append(run.calls, call)
			return <-call.response, nil
		}}
		run.start(t)
		run.input(t, externalEvent(t, 0, "first", "first request"))
		target := run.current.state.currentTurnID
		control := foregroundAbortInput(t, "abort-first", target)
		run.input(t, control)
		if run.calls[0].ctx.Err() == nil || run.current.abort == nil {
			t.Fatal("abort did not cancel callback while retaining draining state")
		}
		if _, ok := run.current.state.abortResults[control.ID]; ok {
			t.Fatal("claimed completion before callback returned")
		}
		run.input(t, externalEvent(t, 0, "second", "new request queued during abort"))
		if len(run.calls) != 1 {
			t.Fatal("started new model while old callback was unjoined")
		}
		run.respond(t, 0, llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "must-not-dispatch", Name: "Bash", Arguments: `{"command":"false"}`}}}})
		synctest.Sleep(10 * slurpIdleTimeout)
		synctest.Wait()
		result := run.current.state.abortResults[control.ID]
		if result.Status != "completed" || run.current.abort != nil {
			t.Fatalf("abort outcome: %+v", result)
		}
		if len(run.current.state.toolCalls) != 0 || len(run.operations.adds) != 0 {
			t.Fatal("late canceled response tool was dispatched")
		}
		if len(run.calls) != 2 {
			t.Fatalf("queued new input did not start a fresh request: %d", len(run.calls))
		}
		// Same ID dedups; a new control against the old target is explicitly stale.
		run.input(t, control, foregroundAbortInput(t, "old-target-retry", target))
		if run.calls[1].ctx.Err() != nil || run.current.state.abortResults["old-target-retry"].Status != "stale" {
			t.Fatal("old control affected new turn")
		}
		run.respond(t, 1, textResponse("fresh answer"))
		run.assertRunning(t)
	})
}

func TestForegroundAbortRecordsIntentBeforeCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		run.input(t, externalEvent(t, 0, "first", "first"))
		target := run.current.state.currentTurnID
		recorded := false
		run.store.onAppendInput = func(value inbox.Input) {
			if value.ID == "abort" {
				recorded = true
				if run.calls[0].ctx.Err() != nil {
					t.Fatal("model canceled before intent persistence")
				}
			}
		}
		run.input(t, foregroundAbortInput(t, "abort", target))
		synctest.Sleep(10 * slurpIdleTimeout)
		synctest.Wait()
		if !recorded || run.current.state.abortResults["abort"].Status != "completed" {
			t.Fatal("intent/outcome missing")
		}
		run.assertRunning(t)
	})
}

func TestForegroundAbortRestoreUnfinishedIntentRefusesAutomaticWork(t *testing.T) {
	run := newStopTestRun(t, 0)
	control := foregroundAbortInput(t, "pending-abort", "turn-1")
	run.store.items = append(run.store.items, storedItem(3, sessionstore.ItemInput, control))
	if err := run.current.Run(t.Context()); !errors.Is(err, ErrLifecycleRecoveryRequired) {
		t.Fatalf("Run = %v", err)
	}
	if len(run.calls) != 0 || len(run.operations.adds) != 0 || len(run.operations.cancels) != 0 {
		t.Fatal("unfinished recovered lifecycle intent dispatched work")
	}
}

func TestForegroundAbortScopeRefusesUnknownAndPreservesExplicitBackground(t *testing.T) {
	for _, tc := range []struct {
		name    string
		plan    operation.RemoteJobPlan
		status  operation.Status
		allowed bool
	}{
		{"explicit-background", runPlan(t, "background"), operation.StatusCompleted, true},
		{"legacy", runPlan(t, ""), operation.StatusCompleted, false},
		{"task", runPlan(t, "task"), operation.StatusCompleted, false},
		{"still-running-background", runPlan(t, "background"), operation.StatusAwaiting, false},
		{"unsupported-background", runPlan(t, "background"), operation.StatusUnsupported, false},
		{"graph", operation.RemoteJobPlan{Type: "taskgraph", Version: 1, Data: []byte(`{}`)}, operation.StatusCompleted, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := operation.NewRemoteJobSpec(tc.plan)
			if err != nil {
				t.Fatal(err)
			}
			c := newStopTestRun(t, 0).current
			c.state.operations["remote"] = operation.Operation{ID: "remote", Type: spec.Type, Version: spec.Version, State: spec.State, Status: tc.status, MaxOutputLength: 128}
			targets, err := c.foregroundAbortTargets()
			if (err == nil) != tc.allowed || len(targets) != 0 {
				t.Fatalf("targets=%v err=%v", targets, err)
			}
		})
	}
}
func runPlan(t *testing.T, lifetime string) operation.RemoteJobPlan {
	t.Helper()
	data, err := json.Marshal(toolsubagent.RunPlan{Name: "child", Prompt: "work", Lifetime: lifetime})
	if err != nil {
		t.Fatal(err)
	}
	return operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: data}
}

// A terminal operation update alone cannot settle abort: only the concrete
// manager's live cleanup receipt can authorize it.
func TestForegroundAbortTerminalStatusRequiresIndependentCleanupReceipt(t *testing.T) {
	run := newStopTestRun(t, 0)
	spec, err := operation.NewShellSpec(operation.ShellInput{Shell: "/bin/sh", Command: "true"}, t.TempDir(), 128)
	if err != nil {
		t.Fatal(err)
	}
	value := operation.Operation{ID: "shell", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusCanceled, MaxOutputLength: 128}
	run.current.state.operations[value.ID] = value
	run.current.state.currentTurnID = "turn-1"
	run.current.abort = &foregroundAbortState{id: "abort", target: "turn-1", operations: []operation.ID{value.ID}}
	run.current.state.abortRequests = map[inbox.ID]inbox.ForegroundAbort{"abort": {Version: 1, TargetTurnID: "turn-1"}}
	done, err := run.current.advanceForegroundAbort(t.Context())
	if err != nil || done {
		t.Fatalf("terminal status with no verifier authorized cleanup: done=%v err=%v", done, err)
	}
	if _, ok := run.current.state.abortResults["abort"]; ok {
		t.Fatal("unproven terminal result published completed outcome")
	}
}

func TestForegroundAbortOutcomeMustMatchIntentAndCannotRewriteFinal(t *testing.T) {
	c := newStopTestRun(t, 0).current
	request := foregroundAbortInput(t, "abort", "turn-1")
	if _, err := c.addItemToLocalState(sessionstore.Item{Kind: sessionstore.ItemInput, Data: request}); err != nil {
		t.Fatal(err)
	}
	outcome := func(target, status string) error {
		payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.ForegroundAbortFinished, Parameters: inbox.ForegroundAbortResult{Version: 1, InputID: "abort", TargetTurnID: target, Status: status}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.addItemToLocalState(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{ID: inbox.ID("result-" + status), Kind: inbox.InputControl, Payload: payload}})
		return err
	}
	if err := outcome("wrong-turn", "completed"); err == nil {
		t.Fatal("accepted mismatched target")
	}
	if err := outcome("turn-1", "uncertain"); err != nil {
		t.Fatal(err)
	}
	if err := outcome("turn-1", "completed"); err != nil {
		t.Fatal(err)
	}
	if err := outcome("turn-1", "refused"); err == nil {
		t.Fatal("rewrote final outcome")
	}
}

func TestForegroundAbortConsumesToolContinuationsNotQueuedExternalInputs(t *testing.T) {
	c := newStopTestRun(t, 0).current
	c.state.currentTurnInputs = 5
	c.state.currentTurnExternalInputs = 2
	// Three old tool completions/heartbeats and two new external inputs arrived
	// in either order. Scalar availability must not drop the new external work.
	c.state.availableInputs = 10
	c.state.externalInputs = 4
	c.state.deliveredInputs = 5
	c.consumeAbortedTurn()
	if c.pendingInputs() != 2 || c.state.deliveredInputs != 8 {
		t.Fatalf("pending=%d delivered=%d", c.pendingInputs(), c.state.deliveredInputs)
	}
	c.state.availableInputs = 8
	c.state.externalInputs = 2
	c.state.deliveredInputs = 5
	c.consumeAbortedTurn()
	if c.pendingInputs() != 0 {
		t.Fatal("old canceled tool results restarted model automatically")
	}
}

func TestForegroundAbortJoinsSupersededModelCallbacks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.current.dependencies.LLM = &fakeAdapter{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
			call := stopTestCall{ctx: ctx, request: request, response: make(chan llm.Response)}
			run.calls = append(run.calls, call)
			return <-call.response, nil
		}}
		run.start(t)
		run.input(t, externalEvent(t, 0, "one", "one"))
		run.input(t, externalEvent(t, 0, "two", "two"))
		if len(run.calls) != 2 {
			t.Fatal("fixture did not supersede model request")
		}
		target := run.current.state.currentTurnID
		run.input(t, foregroundAbortInput(t, "abort", target))
		run.respond(t, 1, textResponse("late second"))
		synctest.Sleep(10 * slurpIdleTimeout)
		synctest.Wait()
		if _, ok := run.current.state.abortResults["abort"]; ok || run.current.abort == nil {
			t.Fatal("completion ignored still-unjoined superseded callback")
		}
		run.respond(t, 0, textResponse("late first"))
		synctest.Sleep(10 * slurpIdleTimeout)
		synctest.Wait()
		if run.current.state.abortResults["abort"].Status != "completed" || len(run.calls) != 2 {
			t.Fatal("superseded callback did not settle without retry")
		}
		run.assertRunning(t)
	})
}
