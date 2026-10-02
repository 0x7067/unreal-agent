package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestCoordinatorSettingsDoNotWakeIdleModelAndRestartActiveRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		limit := int64(2048)
		initialModel := llm.Model{ID: "initial", CompactionThreshold: 200_000, MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortLow}
		run.input(t, settingsInput(t, "initial", inbox.Settings{Model: initialModel.ID, CompactionThreshold: new(initialModel.CompactionThreshold), MaxOutputTokens: &limit, ReasoningEffort: initialModel.ReasoningEffort}))
		if run.requestCount() != 0 || run.current.pendingInputs() != 0 {
			t.Fatal("settings woke an idle model")
		}
		run.assertRunning(t)
		run.input(t, externalEvent(t, 0, "prompt", "hello"))
		if run.requestCount() != 1 || !reflect.DeepEqual(run.calls[0].request.Model, initialModel) {
			t.Fatal("next turn did not use settings")
		}
		run.input(t, settingsInput(t, "next", inbox.Settings{Model: "next", CompactionThreshold: new(int64(80_000)), ReasoningEffort: llm.ReasoningEffortHigh}))
		if run.requestCount() != 2 || !errors.Is(run.calls[0].ctx.Err(), context.Canceled) || run.calls[1].ctx.Err() != nil {
			t.Fatal("settings did not restart the active request")
		}
		nextModel := llm.Model{ID: "next", CompactionThreshold: 80_000, MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortHigh}
		want := run.calls[0].request
		want.Model = nextModel
		if !reflect.DeepEqual(run.calls[1].request, want) || !reflect.DeepEqual(run.calls[0].request.Model, initialModel) {
			t.Fatal("restart did not preserve context and apply updated settings")
		}
		nextLimit := int64(4096)
		run.input(t, settingsInput(t, "limit", inbox.Settings{MaxOutputTokens: &nextLimit}))
		if run.requestCount() != 3 || !errors.Is(run.calls[1].ctx.Err(), context.Canceled) || run.calls[2].ctx.Err() != nil {
			t.Fatal("output token limit did not restart the active request")
		}
		nextModel.MaxOutputTokens = &nextLimit
		want.Model = nextModel
		if !reflect.DeepEqual(run.calls[2].request, want) {
			t.Fatal("restart did not preserve context and apply the output token limit")
		}
		run.respond(t, 2, textResponse("Hello."))
		if run.requestCount() != 3 || run.current.pendingInputs() != 0 {
			t.Fatal("settings caused an extra turn after the response")
		}
		run.input(t, externalEvent(t, 1, "follow-up", "continue"))
		if run.requestCount() != 4 || !reflect.DeepEqual(run.calls[3].request.Model, nextModel) {
			t.Fatal("following turn did not use updated model settings")
		}
		run.respond(t, 3, textResponse("Done."))
		run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
		run.assertStopped(t)
	})
}

func TestCoordinatorSettingsRestartWithoutWaitingForCanceledResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.current.dependencies.LLM = &fakeAdapter{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
			call := stopTestCall{ctx: ctx, request: request, response: make(chan llm.Response)}
			run.calls = append(run.calls, call)
			return <-call.response, nil
		}}
		run.start(t)
		run.input(t, externalEvent(t, 0, "prompt", "hello"))
		run.input(t, settingsInput(t, "settings", inbox.Settings{ReasoningEffort: llm.ReasoningEffortHigh}))
		if run.requestCount() != 2 || !errors.Is(run.calls[0].ctx.Err(), context.Canceled) {
			t.Fatal("restart waited for the canceled request to return")
		}
		run.respond(t, 0, textResponse("Stale response."))
		if len(run.store.appendedResponses) != 0 || run.current.pendingInputs() != 1 || run.calls[1].ctx.Err() != nil {
			t.Fatal("canceled response affected the replacement request")
		}
		run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
		run.respond(t, 1, textResponse("Done."))
		run.assertStopped(t)
	})
}

func TestCoordinatorModelSwitchDiscardsPreviousUsage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newCompactionRun(t)
		run.current.dependencies.ContextBuilder.SetModel(llm.Model{ID: "initial", CompactionThreshold: 100_000})
		run.current.dependencies.LLM = &fakeAdapter{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
			call := stopTestCall{ctx: ctx, request: request, response: make(chan llm.Response)}
			run.calls = append(run.calls, call)
			return <-call.response, nil
		}}
		run.start(t)
		run.input(t, externalEvent(t, 0, "prompt", "continue"))
		assertCompactionTurn(t, run, 1, session.TurnRegular)
		run.input(t, settingsInput(t, "settings", inbox.Settings{Model: "next", CompactionThreshold: new(int64(25_000))}))
		assertCompactionTurn(t, run, 2, session.TurnRegular)
		if !errors.Is(run.calls[0].ctx.Err(), context.Canceled) || run.calls[1].request.Model.ID != "next" {
			t.Fatal("model switch did not replace the active request")
		}
		stale := textResponse("Old model response")
		stale.Usage = llm.Usage{InputTokens: 90_000}
		run.respond(t, 0, stale)
		if run.current.dependencies.ContextBuilder.NeedsCompaction() {
			t.Fatal("old model usage survived the model switch")
		}
		replayed := newStopTestRun(t, 0).current
		replayed.dependencies.Sessions = run.current.dependencies.Sessions
		if err := replayed.loadHistory(t.Context()); err != nil {
			t.Fatal(err)
		}
		if replayed.dependencies.ContextBuilder.NeedsCompaction() {
			t.Fatal("replay restored checkpoints from the old model")
		}
		fresh := textResponse("New model response")
		fresh.Usage = llm.Usage{InputTokens: 30_000}
		run.respond(t, 1, fresh)
		if !run.current.dependencies.ContextBuilder.NeedsCompaction() {
			t.Fatal("new model response did not establish measured usage")
		}
		run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
		run.assertStopped(t)
	})
}

func TestCoordinatorSettingsPreserveToolGraceAndApplyToContinuation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newToolGraceTestRun(t)
		run.start(t)
		run.input(t, externalEvent(t, 0, "prompt", "run both tools"))
		run.respond(t, 0, toolGraceResponse("A", "B"))
		updateToolGraceCall(t, run, "A", operation.StatusCompleted)
		grace := run.current.state.grace
		run.input(t, settingsInput(t, "settings", inbox.Settings{ReasoningEffort: llm.ReasoningEffortMax}))
		if run.requestCount() != 1 || len(run.current.state.graceToolCalls) != 1 || run.current.state.grace != grace || len(run.operations.cancels) != 0 {
			t.Fatal("settings disturbed pending operations or ended their grace period")
		}
		updateToolGraceCall(t, run, "B", operation.StatusCompleted)
		if run.requestCount() != 2 || run.calls[1].request.Model.ReasoningEffort != llm.ReasoningEffortMax {
			t.Fatal("automatic continuation did not use updated settings")
		}
		assertStopResult(t, run.calls[1].request, "A", string(operation.StatusCompleted))
		assertStopResult(t, run.calls[1].request, "B", string(operation.StatusCompleted))
	})
}

func TestCoordinatorSettingsFollowInboxOrderAndDeduplicate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newHeartbeatTestRun(t, 0)
		run.start(t)
		limit := int64(123)
		first := settingsInput(t, "first", inbox.Settings{Model: "initial", MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortLow})
		last := settingsInput(t, "last", inbox.Settings{Model: "next", ReasoningEffort: llm.ReasoningEffortHigh})
		repeated := settingsInput(t, "repeated", inbox.Settings{Model: "next"})
		run.input(t, first, last, first, repeated)
		if !reflect.DeepEqual(run.recordedInputs, []inbox.Input{first, last, repeated}) {
			t.Fatalf("recorded settings = %#v", run.recordedInputs)
		}
		run.input(t, externalEvent(t, 0, "prompt", "hello"))
		if !reflect.DeepEqual(run.calls[0].request.Model, llm.Model{ID: "next", MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortHigh}) {
			t.Fatal("duplicate rolled back the latest settings")
		}
		run.input(t, first, last, repeated)
		if run.requestCount() != 1 || run.calls[0].ctx.Err() != nil {
			t.Fatal("duplicate settings restarted the active request")
		}
		run.input(t,
			settingsInput(t, "new-first", inbox.Settings{Model: "final", ReasoningEffort: llm.ReasoningEffortLow}),
			settingsInput(t, "new-last", inbox.Settings{ReasoningEffort: llm.ReasoningEffortMax}),
		)
		if run.requestCount() != 2 || !reflect.DeepEqual(run.calls[1].request.Model, llm.Model{ID: "final", MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortMax}) {
			t.Fatal("restart did not apply batched settings in inbox order")
		}
	})
}

func TestCoordinatorSettingsPreservePendingStop(t *testing.T) {
	for _, mode := range []inbox.ControlMode{inbox.StopHard, inbox.StopWhenIdle} {
		t.Run(string(mode), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 1)
				run.start(t)
				run.input(t, stopInput(t, "stop", mode), settingsInput(t, "settings", inbox.Settings{ReasoningEffort: llm.ReasoningEffortHigh}))
				run.assertRunning(t)
				if run.current.stop.request.Mode != mode || run.requestCount() != 0 {
					t.Fatal("settings changed the pending stop or started a turn")
				}
				if mode == inbox.StopHard {
					if len(run.operations.cancels) != 1 {
						t.Fatal("settings prevented operation cancellation")
					}
					run.update(t, 0, operation.StatusCanceled)
					if run.requestCount() != 0 {
						t.Fatal("hard stop started a final turn")
					}
				} else {
					if len(run.operations.cancels) != 0 {
						t.Fatal("settings canceled pending work")
					}
					run.update(t, 0, operation.StatusCompleted)
					if run.requestCount() != 1 || run.calls[0].request.Model.ReasoningEffort != llm.ReasoningEffortHigh {
						t.Fatal("final turn did not use updated settings")
					}
					run.respond(t, 0, textResponse("Done."))
				}
				run.assertStopped(t)
			})
		})
	}
}

func TestCoordinatorSettingsRequirePersistence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		want := errors.New("settings storage failed")
		run.store.appendInputErr = want
		run.start(t)
		run.input(t, settingsInput(t, "settings", inbox.Settings{ReasoningEffort: llm.ReasoningEffortHigh}))
		if err := <-run.done; !errors.Is(err, want) {
			t.Fatalf("Run error = %v, want %v", err, want)
		}
		if run.requestCount() != 0 {
			t.Fatal("model ran after settings persistence failed")
		}
	})
}

func TestCoordinatorSettingsReplayOnResumeAndFork(t *testing.T) {
	for _, mode := range []string{"resume", "fork"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				parent := newStopTestRun(t, 0)
				store := persistTestRun(t, parent)
				limit := int64(123)
				for _, input := range []inbox.Input{
					settingsInput(t, "first", inbox.Settings{Model: "initial-model", CompactionThreshold: new(int64(200_000)), MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortLow}),
					settingsInput(t, "effort", inbox.Settings{ReasoningEffort: llm.ReasoningEffortHigh}),
					settingsInput(t, "model", inbox.Settings{Model: "saved-model"}),
					settingsInput(t, "threshold", inbox.Settings{CompactionThreshold: new(int64(80_000))}),
				} {
					if err := store.AppendInput(t.Context(), "session-1", input); err != nil {
						t.Fatal(err)
					}
				}
				turn := session.Turn{ID: "parent-turn", PreviousTurnID: "turn-1", Type: session.TurnRegular}
				if err := store.AppendTurn(t.Context(), "session-1", turn); err != nil {
					t.Fatal(err)
				}
				if err := store.AppendModelResponse(t.Context(), "session-1", sessionstore.ModelResponse{TurnID: turn.ID, Response: textResponse("Done.")}); err != nil {
					t.Fatal(err)
				}
				id := session.ID("session-1")
				if mode == "fork" {
					id = "child"
					if _, err := store.Fork(t.Context(), id, "session-1", turn.ID); err != nil {
						t.Fatal(err)
					}
				}
				restored, err := store.Resume(t.Context(), id)
				if err != nil {
					t.Fatal(err)
				}
				run := newStopTestRun(t, 0)
				run.current.dependencies.SessionID = id
				run.current.dependencies.Sessions = store
				run.current.dependencies.Restored = restored
				run.current.dependencies.ContextBuilder.SetModel(llm.Model{ID: "model", CompactionThreshold: 250_000, ReasoningEffort: llm.ReasoningEffortMedium})
				run.start(t)
				if run.requestCount() != 0 {
					t.Fatal("replayed settings started a turn")
				}
				run.input(t, externalEvent(t, 0, "prompt", "continue"))
				if !reflect.DeepEqual(run.calls[0].request.Model, llm.Model{ID: "saved-model", CompactionThreshold: 80_000, MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortHigh}) {
					t.Fatal("latest recorded settings did not override initial configuration")
				}
			})
		})
	}
}

func settingsInput(t *testing.T, id inbox.ID, settings inbox.Settings) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: settings})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
}
