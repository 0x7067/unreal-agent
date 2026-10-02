package coordinator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func newCompactionRun(t *testing.T) *stopTestRun {
	t.Helper()
	recorded, _ := recordedCompactionHistory(t)
	run := newStopTestRun(t, 0)
	run.current.dependencies.Sessions = recorded.dependencies.Sessions
	run.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
	return run
}

func assertCompactionTurn(t *testing.T, run *stopTestRun, count int, kind session.TurnType) {
	t.Helper()
	if len(run.calls) != count || run.current.state.currentTurnType != kind || run.current.cancelModel == nil {
		t.Fatalf("calls = %d, turn = %s, want %d/%s active", len(run.calls), run.current.state.currentTurnType, count, kind)
	}
	page, err := run.current.dependencies.Sessions.Items(t.Context(), "session-1", sessionstore.BeforeFirst, 100)
	if err != nil {
		t.Fatal(err)
	}
	var turn session.Turn
	for _, item := range page.Items {
		if item.Kind == sessionstore.ItemTurn {
			turn = item.Data.(session.Turn)
		}
	}
	if turn.ID != run.current.state.currentTurnID || turn.Type != kind {
		t.Fatalf("persisted turn = %+v", turn)
	}
}

func TestAutomaticCompactionContinuesOrdinaryWork(t *testing.T) {
	for _, startup := range []bool{false, true} {
		t.Run(map[bool]string{false: "input", true: "startup"}[startup], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newCompactionRun(t)
				input := externalEvent(t, 0, "pending", "Continue work")
				if startup {
					if err := run.current.dependencies.Sessions.AppendInput(t.Context(), "session-1", input); err != nil {
						t.Fatal(err)
					}
				}
				run.start(t)
				if !startup {
					run.input(t, input)
				}
				assertCompactionTurn(t, run, 1, session.TurnCompaction)
				if len(run.calls[0].request.Tools) != 0 || run.current.pendingInputs() != 1 {
					t.Fatal("summary delivered input or exposed tools")
				}
				run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
				summary := textResponse("Summary")
				summary.Stop = llm.StopComplete
				run.respond(t, 0, summary)
				assertCompactionTurn(t, run, 2, session.TurnRegular)
				if run.current.pendingInputs() != 1 {
					t.Fatal("summary answered pending input")
				}
				var summaryFound, inputFound bool
				for _, item := range run.calls[1].request.Input {
					if item.Type == llm.ItemMessage {
						text := item.Data.(llm.Message).Text
						summaryFound = summaryFound || text == "Summary"
						inputFound = inputFound || text == "Continue work"
					}
				}
				if !summaryFound || !inputFound {
					t.Fatalf("continuation = %+v", run.calls[1].request)
				}
				run.respond(t, 1, textResponse("Done"))
				run.assertStopped(t)
				if run.current.pendingInputs() != 0 || len(run.calls) != 2 {
					t.Fatal("ordinary input delivery or request count differs")
				}
			})
		})
	}
}

func TestAutomaticCompactionRejectedResponseRetries(t *testing.T) {
	for _, response := range []llm.Response{
		{Stop: llm.StopRefused}, {Stop: llm.StopMaxOutputTokens}, {Failure: &llm.Failure{Code: "failed"}},
	} {
		t.Run(string(response.Stop), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newCompactionRun(t)
				run.start(t)
				before := run.current.dependencies.ContextBuilder
				run.input(t, externalEvent(t, 0, "pending", "Continue"))
				for attempt := range 2 {
					run.respond(t, attempt, response)
					assertCompactionTurn(t, run, attempt+2, session.TurnCompaction)
					if before != run.current.dependencies.ContextBuilder || run.current.pendingInputs() != 1 {
						t.Fatal("rejected summary replaced context or delivered input")
					}
				}
				run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
				run.respond(t, 2, llm.Response{Stop: llm.StopComplete, Output: textResponse("Summary").Output})
				assertCompactionTurn(t, run, 4, session.TurnRegular)
				run.respond(t, 3, textResponse("Done"))
				run.assertStopped(t)
			})
		})
	}
}

func TestAutomaticCompactionStopAndSteering(t *testing.T) {
	for _, mode := range []inbox.ControlMode{inbox.StopHard, "steer"} {
		t.Run(string(mode), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newCompactionRun(t)
				run.current.dependencies.LLM = &fakeAdapter{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
					call := stopTestCall{ctx: ctx, request: request, response: make(chan llm.Response)}
					run.calls = append(run.calls, call)
					return <-call.response, nil
				}}
				run.start(t)
				run.input(t, externalEvent(t, 0, "pending", "Continue"))
				old := run.current.dependencies.ContextBuilder
				if mode == "steer" {
					run.input(t, externalEvent(t, 0, "steer", "New direction"))
				} else {
					run.input(t, stopInput(t, "stop", mode))
				}
				if run.calls[0].ctx.Err() == nil {
					t.Fatal("compaction not canceled")
				}
				summary := textResponse("Late summary")
				summary.Stop = llm.StopComplete
				run.respond(t, 0, summary)
				if old != run.current.dependencies.ContextBuilder {
					t.Fatal("late summary replaced context")
				}
				if mode != inbox.StopHard {
					assertCompactionTurn(t, run, 2, session.TurnCompaction)
					run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
					run.respond(t, 1, llm.Response{Stop: llm.StopComplete, Output: textResponse("Summary").Output})
					assertCompactionTurn(t, run, 3, session.TurnRegular)
					run.respond(t, 2, textResponse("Done"))
				}
				run.assertStopped(t)
			})
		})
	}
}

func TestCompactionResponsePreservesPendingWork(t *testing.T) {
	for _, outcome := range []string{"complete", "refused", "no cutoff"} {
		t.Run(outcome, func(t *testing.T) {
			current, _ := recordedCompactionHistory(t)
			appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "pending", "Continue")})
			appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "compact", PreviousTurnID: "B", Type: session.TurnCompaction}})
			if outcome == "no cutoff" {
				current.dependencies.ContextBuilder = newStopTestRun(t, 0).current.dependencies.ContextBuilder
			}
			previous := current.dependencies.ContextBuilder
			response := llm.Response{Stop: llm.StopComplete}
			if outcome == "refused" {
				response.Stop = llm.StopRefused
			}
			err := current.processModelResponse(t.Context(), modelResponseResult{turnID: "compact", response: response})
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if current.pendingInputs() != 1 {
				t.Fatal("compaction response consumed pending input")
			}
			callModel, err := current.processEvents(t.Context())
			if err != nil || !callModel {
				t.Fatalf("pending work did not schedule continuation: callModel = %t, error = %v", callModel, err)
			}
			if outcome == "complete" {
				if previous == current.dependencies.ContextBuilder {
					t.Fatal("completed compaction did not reconstruct context")
				}
				rebuilt := current.dependencies.ContextBuilder
				if err := current.processModelResponse(t.Context(), modelResponseResult{turnID: "compact", response: response}); err == nil {
					t.Fatal("duplicate response was persisted")
				}
				if current.pendingInputs() != 1 || current.dependencies.ContextBuilder != rebuilt {
					t.Fatal("duplicate response changed pending input or context")
				}
			} else if previous != current.dependencies.ContextBuilder {
				t.Fatal("unsuccessful response changed context")
			}
		})
	}
}

func TestCompactionCrashContinuation(t *testing.T) {
	for _, stage := range []string{"turn", "persisted response", "reconstructed"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				recorded, directory := recordedCompactionHistory(t)
				appendCompactionTestItem(t, recorded, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "pending", "Continue")})
				appendCompactionTestItem(t, recorded, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "compact", PreviousTurnID: "B", Type: session.TurnCompaction}})
				summary := sessionstore.ModelResponse{TurnID: "compact", Response: llm.Response{Stop: llm.StopComplete, Output: textResponse("Summary").Output}}
				if stage == "persisted response" {
					if err := recorded.dependencies.Sessions.AppendModelResponse(t.Context(), "session-1", summary); err != nil {
						t.Fatal(err)
					}
				}
				if stage == "reconstructed" {
					if _, err := recorded.handleModelResponse(t.Context(), summary); err != nil {
						t.Fatal(err)
					}
				}
				reopened, err := localfile.New(directory)
				if err != nil {
					t.Fatal(err)
				}
				run := newStopTestRun(t, 0)
				run.current.dependencies.Sessions = reopened
				run.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
				run.start(t)
				ordinary := 0
				if stage == "turn" {
					assertCompactionTurn(t, run, 1, session.TurnCompaction)
					run.respond(t, 0, summary.Response)
					recorded.applyCompactionResponse(summary.Response)
					ordinary = 1
				}
				assertCompactionTurn(t, run, ordinary+1, session.TurnRegular)
				if stage == "persisted response" {
					if _, err := recorded.addItemToLocalState(sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: summary}); err != nil {
						t.Fatal(err)
					}
				}
				recorded.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
				expected, err := recorded.dependencies.ContextBuilder.Build()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(expected.Request, run.calls[ordinary].request) {
					t.Fatal("resumed request differs from uninterrupted context")
				}
				run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
				run.respond(t, ordinary, textResponse("Done"))
				run.assertStopped(t)
			})
		})
	}
}

func TestCompactionKeepsOperationUpdatesLive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 1)
		recorded, _ := recordedCompactionHistory(t)
		for _, item := range run.store.items {
			if item.Kind == sessionstore.ItemTurn {
				turn := item.Data.(session.Turn)
				turn.PreviousTurnID = "B"
				item.Data = turn
			}
			if item.Kind == sessionstore.ItemModelResponse {
				response := item.Data.(sessionstore.ModelResponse)
				response.Response.Usage.InputTokens = 40_000
				item.Data = response
			}
			appendCompactionTestItem(t, recorded, item)
		}
		run.current.dependencies.Sessions = recorded.dependencies.Sessions
		run.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
		run.start(t)
		run.input(t, externalEvent(t, 0, "pending", "Continue"))
		assertCompactionTurn(t, run, 1, session.TurnCompaction)
		run.update(t, 0, operation.StatusCompleted)
		if len(run.calls) != 1 || run.calls[0].ctx.Err() != nil {
			t.Fatal("operation update interrupted compaction")
		}
		run.respond(t, 0, llm.Response{Stop: llm.StopComplete, Output: textResponse("Summary").Output})
		assertCompactionTurn(t, run, 2, session.TurnRegular)
		assertStopResult(t, run.calls[1].request, "call-0", string(operation.StatusCompleted))
		run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		run.respond(t, 1, textResponse("Done"))
		run.assertStopped(t)
	})
}

func TestAutomaticCompactionWithoutCutoffUsesOrdinaryRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorded, _ := recordedCompactionHistory(t)
		appendCompactionTestItem(t, recorded, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "small", PreviousTurnID: "B", Type: session.TurnRegular}})
		appendCompactionTestItem(t, recorded, sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "small", Response: llm.Response{Usage: llm.Usage{InputTokens: 10_000}}}})
		run := newStopTestRun(t, 0)
		run.current.dependencies.Sessions = recorded.dependencies.Sessions
		run.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 10_000})
		run.start(t)
		if !run.current.dependencies.ContextBuilder.NeedsCompaction() {
			t.Fatal("expected pressure despite missing cutoff")
		}
		run.input(t, externalEvent(t, 0, "pending", "Continue"))
		assertCompactionTurn(t, run, 1, session.TurnRegular)
		run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		run.respond(t, 0, textResponse("Done"))
		run.assertStopped(t)
	})
}

func TestCompactionTurnPersistenceFailureDoesNotSubmit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newCompactionRun(t)
		want := errors.New("turn write failed")
		run.current.dependencies.Sessions = compactionTurnFailureStore{Store: run.current.dependencies.Sessions, err: want}
		run.start(t)
		run.input(t, externalEvent(t, 0, "pending", "Continue"))
		if err := <-run.done; !errors.Is(err, want) {
			t.Fatalf("Run error = %v", err)
		}
		if len(run.calls) != 0 || run.current.cancelModel != nil {
			t.Fatal("failed turn persistence submitted a request")
		}
	})
}

type compactionTurnFailureStore struct {
	sessionstore.Store
	err error
}

func (store compactionTurnFailureStore) AppendTurn(context.Context, session.ID, session.Turn) error {
	return store.err
}
