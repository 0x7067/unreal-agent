package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func manualCompactInput(t *testing.T, id inbox.ID) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.Compact})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
}

func TestCompactionBoundNoticeReplayAndManualRecovery(t *testing.T) {
	for _, response := range []llm.Response{
		{Stop: llm.StopComplete},
		{Stop: llm.StopComplete, Output: textResponse(" \n\t").Output},
		{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "no"}}}},
		{Stop: llm.StopRefused}, {Stop: llm.StopMaxOutputTokens},
		{Failure: &llm.Failure{Code: "server_error"}},
	} {
		t.Run(string(response.Stop), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newCompactionRun(t)
				run.start(t)
				original := run.current.dependencies.ContextBuilder
				run.input(t, externalEvent(t, 0, "pending", "continue"))
				for attempt := range 3 {
					assertCompactionTurn(t, run, attempt+1, session.TurnCompaction)
					run.respond(t, attempt, response)
				}
				assertCompactionTurn(t, run, 4, session.TurnRegular)
				if run.current.dependencies.ContextBuilder != original || !run.current.state.compactionDisabled || run.current.pendingInputs() != 1 {
					t.Fatal("bound lost context or pending work")
				}
				page, err := run.current.dependencies.Sessions.Items(t.Context(), "session-1", 0, 100)
				if err != nil {
					t.Fatal(err)
				}
				notices, policies := 0, 0
				for _, item := range page.Items {
					if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
						control, err := input.DecodeControlMessage()
						if err != nil {
							t.Fatal(err)
						}
						if control.Mode == inbox.CompactionDisabled {
							notices++
							if control.Reason == "" {
								t.Fatal("notice is empty")
							}
						}
						if control.Mode == inbox.SetCompactionPolicy {
							policies++
						}
					}
				}
				if notices != 1 || policies != 1 {
					t.Fatalf("notices=%d policies=%d", notices, policies)
				}
				replay := newStopTestRun(t, 0).current
				replay.dependencies.Sessions = run.current.dependencies.Sessions
				replay.dependencies.CompactionFailureLimit = 99
				replay.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
				if err := replay.loadHistory(t.Context()); err != nil {
					t.Fatal(err)
				}
				liveContext, _ := run.current.dependencies.ContextBuilder.Build()
				replayContext, _ := replay.dependencies.ContextBuilder.Build()
				if !reflect.DeepEqual(liveContext, replayContext) || !reflect.DeepEqual(run.current.state, replay.state) || replay.compactionFailureLimit() != 3 {
					t.Fatal("live/replay policy, context or state differs")
				}
				continuation := textResponse("done")
				continuation.Usage = llm.Usage{InputTokens: 40_000}
				run.respond(t, 3, continuation)
				run.input(t, manualCompactInput(t, "manual"))
				assertCompactionTurn(t, run, 5, session.TurnCompaction)
				run.respond(t, 4, llm.Response{Stop: llm.StopComplete, Output: textResponse("Recovered summary").Output})
				if run.current.state.compactionDisabled || run.current.state.compactionFailures != 0 || run.current.dependencies.ContextBuilder == original {
					t.Fatal("manual success did not re-enable compaction")
				}
				if len(run.calls) != 5 {
					t.Fatal("manual request caused unsolicited ordinary turn")
				}
				run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
				run.assertStopped(t)
			})
		})
	}
}

func TestCompactionConfiguredLimitAndDuplicateResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newCompactionRun(t)
		run.current.dependencies.CompactionFailureLimit = 1
		run.start(t)
		run.input(t, externalEvent(t, 0, "pending", "continue"))
		turn := run.current.state.currentTurnID
		run.respond(t, 0, llm.Response{Stop: llm.StopComplete})
		assertCompactionTurn(t, run, 2, session.TurnRegular)
		if !run.current.state.compactionDisabled || run.current.compactionFailureLimit() != 1 {
			t.Fatal("custom bound ignored")
		}
		before := run.current.state.compactionFailures
		if _, err := run.current.handleModelResponse(t.Context(), sessionstore.ModelResponse{TurnID: turn, Response: llm.Response{Stop: llm.StopComplete}}); err != nil {
			t.Fatal(err)
		}
		if run.current.state.compactionFailures != before {
			t.Fatal("duplicate counted twice")
		}
		run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		run.respond(t, 1, textResponse("done"))
		run.assertStopped(t)
	})
}

func TestCompactionNegativeLimitRejected(t *testing.T) {
	err := New(Dependencies{CompactionFailureLimit: -1}).Run(t.Context())
	if err == nil {
		t.Fatal("negative bound accepted")
	}
}

func TestManualCompactionWithoutHistoryDoesNotCallModel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		run.input(t, manualCompactInput(t, "manual"))
		if len(run.calls) != 0 {
			t.Fatal("manual compaction submitted without eligible history")
		}
		run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		run.assertStopped(t)
	})
}

func failedToolResponse() llm.Response {
	return llm.Response{ID: "failed", Failure: &llm.Failure{Code: "insufficient_quota", Message: "fixture failure"}, Usage: llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 123, OutputTokens: 4}}, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "unsafe", Name: tool.ViewImageName, Arguments: `{}`}}}}
}

func TestRegularFailureDurableTypedAndNoToolScheduling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		run.input(t, externalEvent(t, 0, "user", "hello"))
		response := failedToolResponse()
		run.respond(t, 0, response)
		var failed *ModelFailureError
		if err := <-run.done; !errors.As(err, &failed) || failed.Failure.Code != "insufficient_quota" {
			t.Fatalf("error=%v", err)
		}
		if len(run.store.appendedResponses) != 1 || !reflect.DeepEqual(run.store.appendedResponses[0].Response, response) {
			t.Fatal("response/usage not durable")
		}
		if len(run.store.appendedStatuses) != 0 || len(run.operations.adds) != 0 || len(run.current.state.toolCalls) != 0 {
			t.Fatal("failed calls scheduled")
		}
		assertNoToolCallsInContext(t, run.current)
	})
}

func assertNoToolCallsInContext(t *testing.T, current *coordinator) {
	t.Helper()
	built, err := current.dependencies.ContextBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range built.Request.Input {
		if item.Type == llm.ItemToolCall {
			t.Fatal("failed output is an orphan tool call in context")
		}
	}
}

func TestFailedReplayRequiresNewInputAndIgnoresLegacyOperations(t *testing.T) {
	for _, newInput := range []string{"none", "recorded", "queued", "during failed turn"} {
		t.Run(newInput, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 0)
				response := failedToolResponse()
				run.store.items = []sessionstore.Item{
					storedItem(1, sessionstore.ItemInput, externalEvent(t, 0, "user", "hello")),
					storedItem(2, sessionstore.ItemTurn, session.Turn{ID: "failed-turn", Type: session.TurnRegular}),
				}
				if newInput == "during failed turn" {
					run.store.items = append(run.store.items, storedItem(3, sessionstore.ItemInput, externalEvent(t, 0, "new", "continue")))
				}
				run.store.items = append(run.store.items, storedItem(sessionstore.Sequence(len(run.store.items)+1), sessionstore.ItemModelResponse, sessionstore.ModelResponse{TurnID: "failed-turn", Response: response}))
				// A legacy binary may have already scheduled this partial call. Do not
				// execute its recorded ready operation on a newer binary's resume.
				op := operation.Operation{ID: "unsafe-op", Status: operation.StatusReady}
				status := sessionstore.ToolCallStatus{TurnID: "failed-turn", CallID: "unsafe", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}
				run.store.items = append(run.store.items, storedItem(sessionstore.Sequence(len(run.store.items)+1), sessionstore.ItemToolCallStatus, status))
				run.current.dependencies.Restored.Operations = []operation.Operation{op}
				if newInput == "recorded" {
					run.store.items = append(run.store.items, storedItem(sessionstore.Sequence(len(run.store.items)+1), sessionstore.ItemInput, externalEvent(t, 0, "new", "continue")))
				}
				if newInput == "queued" {
					submitTestInput(t, run.inputs, externalEvent(t, 0, "new", "continue"))
				}
				run.start(t)
				synctest.Sleep(2 * slurpIdleTimeout)
				synctest.Wait()
				if newInput == "none" {
					var failed *ModelFailureError
					if err := <-run.done; !errors.As(err, &failed) {
						t.Fatalf("resume error=%v", err)
					}
					if len(run.calls) != 0 {
						t.Fatal("retried failure without new input")
					}
				} else {
					if len(run.calls) != 1 {
						t.Fatalf("new input did not start a turn: %d", len(run.calls))
					}
					run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
					run.respond(t, 0, textResponse("done"))
					run.assertStopped(t)
				}
				if len(run.operations.adds) != 0 || len(run.current.state.toolCalls) != 0 {
					t.Fatal("legacy failed calls scheduled on replay")
				}
				assertNoToolCallsInContext(t, run.current)
			})
		})
	}
}

func TestFailurePersistenceErrorTakesPrecedence(t *testing.T) {
	current, _ := recordedCompactionHistory(t)
	appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "regular", PreviousTurnID: "B", Type: session.TurnRegular}})
	want := errors.New("disk failed")
	current.dependencies.Sessions = compactionFailureStore{Store: current.dependencies.Sessions, err: want}
	_, err := current.handleModelResponse(context.Background(), sessionstore.ModelResponse{TurnID: "regular", Response: failedToolResponse()})
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestCompactionControlPersistenceFailuresRecoverWithoutExtraSummary(t *testing.T) {
	for _, mode := range []inbox.ControlMode{inbox.SetCompactionPolicy, inbox.CompactionDisabled} {
		t.Run(string(mode), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newCompactionRun(t)
				durable := run.current.dependencies.Sessions
				want := errors.New("control persistence failed")
				run.current.dependencies.Sessions = compactionControlFailureStore{Store: durable, mode: mode, err: want}
				run.start(t)
				run.input(t, externalEvent(t, 0, "pending", "continue"))
				if mode == inbox.CompactionDisabled {
					for attempt := range 3 {
						run.respond(t, attempt, llm.Response{Stop: llm.StopComplete})
					}
				}
				if err := <-run.done; !errors.Is(err, want) {
					t.Fatalf("error=%v", err)
				}
				wantCalls := 0
				if mode == inbox.CompactionDisabled {
					wantCalls = 3
				}
				if len(run.calls) != wantCalls {
					t.Fatalf("started work after failed control persistence: %d", len(run.calls))
				}
				resumed := newStopTestRun(t, 0)
				resumed.current.dependencies.Sessions = durable
				resumed.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
				resumed.current.dependencies.CompactionFailureLimit = 99
				resumed.start(t)
				if mode == inbox.CompactionDisabled {
					assertCompactionTurn(t, resumed, 1, session.TurnRegular)
					if resumed.current.compactionFailureLimit() != 3 || !resumed.current.state.compactionDisabled {
						t.Fatal("restored response did not disable compaction at recorded bound")
					}
				} else {
					assertCompactionTurn(t, resumed, 1, session.TurnCompaction)
					resumed.respond(t, 0, llm.Response{Stop: llm.StopComplete, Output: textResponse("Summary").Output})
					assertCompactionTurn(t, resumed, 2, session.TurnRegular)
				}
				resumed.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
				resumed.respond(t, len(resumed.calls)-1, textResponse("done"))
				resumed.assertStopped(t)
			})
		})
	}
}

type compactionControlFailureStore struct {
	sessionstore.Store
	mode inbox.ControlMode
	err  error
}

func (store compactionControlFailureStore) AppendInput(ctx context.Context, id session.ID, input inbox.Input) error {
	if input.Kind == inbox.InputControl {
		request, err := input.DecodeControlMessage()
		if err != nil {
			return err
		}
		if request.Mode == store.mode {
			return store.err
		}
	}
	return store.Store.AppendInput(ctx, id, input)
}

func TestDuplicateSuccessfulResponseDoesNotReviveCompletedTool(t *testing.T) {
	run := newStopTestRun(t, 1)
	if err := run.current.restore(t.Context()); err != nil {
		t.Fatal(err)
	}
	completed := run.current.dependencies.Restored.Operations[0]
	completed.Status = operation.StatusCompleted
	run.current.addOperationToLocalState(completed)
	if _, err := run.current.reconcileToolCalls(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(run.current.state.toolCalls) != 0 {
		t.Fatal("tool did not complete")
	}
	before, _ := run.current.dependencies.ContextBuilder.Build()
	pending := run.current.pendingInputs()
	response := run.store.items[1].Data.(sessionstore.ModelResponse)
	if _, err := run.current.handleModelResponse(t.Context(), response); err != nil {
		t.Fatal(err)
	}
	after, _ := run.current.dependencies.ContextBuilder.Build()
	if len(run.current.state.toolCalls) != 0 || run.current.pendingInputs() != pending || !reflect.DeepEqual(before, after) {
		t.Fatal("duplicate revived completed work or changed context")
	}
	if err := run.current.dispatchOperationsToManager(); err != nil {
		t.Fatal(err)
	}
	if len(run.operations.adds) != 0 {
		t.Fatal("completed operation was dispatched again")
	}
}

func TestRefusalWithFailureLiveReplayAndFollowup(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("partial_call=%t", mixed), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Matches MessagesAPI's refusal conversion, including stop_details.
				response := llm.Response{ID: "refused", Stop: llm.StopRefused, Failure: &llm.Failure{Code: "policy_violation", Message: "Provider explanation"}, Output: textResponse("I cannot help with that.").Output, Usage: llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 123, OutputTokens: 4}}}
				if mixed {
					response.Output = append(response.Output, failedToolResponse().Output...)
				}
				live := newStopTestRun(t, 0)
				history := append([]sessionstore.Item(nil), live.store.items...)
				record := func(kind sessionstore.ItemKind, value any) {
					history = append(history, storedItem(sessionstore.Sequence(len(history)+1), kind, value))
				}
				live.store.onAppendInput = func(value inbox.Input) { record(sessionstore.ItemInput, value) }
				live.store.onAppendTurn = func(value session.Turn) { record(sessionstore.ItemTurn, value) }
				live.store.onAppendModelResponse = func(value sessionstore.ModelResponse) { record(sessionstore.ItemModelResponse, value) }
				live.start(t)
				live.input(t, externalEvent(t, 0, "user", "hello"))
				live.respond(t, 0, response)
				live.assertRunning(t)
				if len(live.calls) != 1 || len(live.store.appendedResponses) != 1 || !reflect.DeepEqual(live.store.appendedResponses[0].Response, response) {
					t.Fatal("refusal caused unsolicited retry or lost durable response/usage")
				}
				if live.current.state.lastFailure != nil || len(live.store.appendedStatuses) != 0 || len(live.operations.adds) != 0 || len(live.current.state.toolCalls) != 0 {
					t.Fatal("refusal terminated turn or scheduled failed partial calls")
				}
				assertRefusalHistory(t, live.current)
				liveContext, _ := live.current.dependencies.ContextBuilder.Build()

				replay := newStopTestRun(t, 0)
				replay.store.items = append([]sessionstore.Item(nil), history...)
				if mixed {
					// A legacy session may have scheduled the failed partial call.
					op := operation.Operation{ID: "unsafe-op", Status: operation.StatusReady}
					status := sessionstore.ToolCallStatus{TurnID: live.store.appendedResponses[0].TurnID, CallID: "unsafe", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}
					replay.store.items = append(replay.store.items, storedItem(sessionstore.Sequence(len(replay.store.items)+1), sessionstore.ItemToolCallStatus, status))
					replay.current.dependencies.Restored.Operations = []operation.Operation{op}
				}
				replay.start(t)
				replay.assertRunning(t)
				assertRefusalHistory(t, replay.current)
				replayContext, _ := replay.current.dependencies.ContextBuilder.Build()
				if !reflect.DeepEqual(liveContext, replayContext) || replay.current.state.lastFailure != nil || len(replay.calls) != 0 || len(replay.operations.adds) != 0 || len(replay.current.state.toolCalls) != 0 {
					t.Fatal("replay changed refusal history or revived partial calls")
				}
				for _, run := range []*stopTestRun{live, replay} {
					before := len(run.calls)
					run.input(t, externalEvent(t, 0, "followup", "Then help with something safe."))
					if len(run.calls) != before+1 {
						t.Fatal("ordinary follow-up did not start a turn")
					}
					assertRefusalItems(t, run.calls[before].request.Input)
					run.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
					run.respond(t, before, textResponse("Safe answer."))
					run.assertStopped(t)
				}
			})
		})
	}
}

func assertRefusalHistory(t *testing.T, current *coordinator) {
	t.Helper()
	built, err := current.dependencies.ContextBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	assertRefusalItems(t, built.Request.Input)
}

func assertRefusalItems(t *testing.T, items []llm.Item) {
	t.Helper()
	count := 0
	for _, item := range items {
		if item.Type == llm.ItemToolCall || item.Type == llm.ItemToolResult {
			t.Fatal("failed partial tool call/result entered history")
		}
		if message, ok := item.Data.(llm.Message); ok && message.Role == llm.RoleAssistant && message.Text == "I cannot help with that." {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("refusal messages = %d, want 1", count)
	}
}
