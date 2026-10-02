package coordinator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func TestCompactionReconstructionMatchesLocalFileReplay(t *testing.T) {
	for _, stage := range []string{"turn", "persisted response", "applied response"} {
		t.Run(stage, func(t *testing.T) {
			current, directory := recordedCompactionHistory(t)
			appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "before", "Before compaction")})
			appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "compact", PreviousTurnID: "B", Type: session.TurnCompaction}})
			appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "during", "During compaction")})
			oldBuilder := current.dependencies.ContextBuilder
			before, err := oldBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			response := sessionstore.ModelResponse{TurnID: "compact", Response: textResponse("Summary")}
			response.Response.Stop = llm.StopComplete
			switch stage {
			case "persisted response":
				if err := current.dependencies.Sessions.AppendModelResponse(t.Context(), current.dependencies.SessionID, response); err != nil {
					t.Fatal(err)
				}
			case "applied response":
				if _, err := current.handleModelResponse(t.Context(), response); err != nil {
					t.Fatal(err)
				}
			}
			reopened, err := localfile.New(directory)
			if err != nil {
				t.Fatal(err)
			}
			replayed := newStopTestRun(t, 0).current
			replayed.dependencies.Sessions = reopened
			if err := replayed.loadHistory(t.Context()); err != nil {
				t.Fatal(err)
			}
			if stage == "persisted response" {
				if _, err := current.addItemToLocalState(sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: response}); err != nil {
					t.Fatal(err)
				}
			}
			live, err := current.dependencies.ContextBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			restored, err := replayed.dependencies.ContextBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(live, restored) || !reflect.DeepEqual(current.state, replayed.state) {
				t.Fatal("live reconstruction and replay differ")
			}
			if current.pendingInputs() != 2 || len(current.state.toolCalls) != 0 || len(current.state.operations) != 0 {
				t.Fatalf("compaction changed pending work: %+v", current.state)
			}
			if stage == "turn" {
				if !reflect.DeepEqual(live, before) {
					t.Fatal("unanswered compaction changed context")
				}
			} else {
				if current.dependencies.ContextBuilder == oldBuilder || reflect.DeepEqual(live, before) {
					t.Fatal("usable summary did not replace the builder")
				}
			}
			if stage == "applied response" {
				liveBuilder, replayedBuilder := current.dependencies.ContextBuilder, replayed.dependencies.ContextBuilder
				continuation := textResponse("Continue")
				continuation.Usage = llm.Usage{InputTokens: 29_000, OutputTokens: 1_000}
				for _, item := range []sessionstore.Item{
					{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "next", PreviousTurnID: "compact", Type: session.TurnRegular}},
					{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "next", Response: continuation}},
					{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "compact-again", PreviousTurnID: "next", Type: session.TurnCompaction}},
					{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "compact-again", Response: response.Response}},
				} {
					appendCompactionTestItem(t, current, item)
					if _, err := replayed.addItemToLocalState(item); err != nil {
						t.Fatal(err)
					}
				}
				if current.dependencies.ContextBuilder == liveBuilder || replayed.dependencies.ContextBuilder == replayedBuilder {
					t.Fatal("subsequent compaction did not replace both builders")
				}
				live, err = current.dependencies.ContextBuilder.Build()
				if err != nil {
					t.Fatal(err)
				}
				restored, err = replayed.dependencies.ContextBuilder.Build()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(live, restored) || !reflect.DeepEqual(current.state, replayed.state) {
					t.Fatal("new checkpoints and subsequent compaction differ after replay")
				}
			}
		})
	}
}

func TestCompactionResponseStatusOnLiveAndReplay(t *testing.T) {
	for _, test := range []struct {
		name    string
		stop    llm.StopReason
		failure *llm.Failure
		output  []llm.Item
	}{
		{name: "truncated summary", stop: llm.StopMaxOutputTokens, output: textResponse("Partial summary").Output},
		{name: "refusal", stop: llm.StopRefused, output: textResponse("Cannot summarize").Output},
		{name: "failure", failure: &llm.Failure{Code: "server_error", Message: "failed"}, output: textResponse("Summary").Output},
		{name: "empty output", stop: llm.StopComplete},
	} {
		t.Run(test.name, func(t *testing.T) {
			current, directory := recordedCompactionHistory(t)
			appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "compact", PreviousTurnID: "B", Type: session.TurnCompaction}})
			oldBuilder := current.dependencies.ContextBuilder
			before, err := oldBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			response := sessionstore.ModelResponse{TurnID: "compact", Response: llm.Response{Stop: test.stop, Failure: test.failure, Output: test.output}}
			if _, err := current.handleModelResponse(t.Context(), response); err != nil {
				t.Fatal(err)
			}
			live, err := current.dependencies.ContextBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			if test.stop != llm.StopComplete {
				if current.dependencies.ContextBuilder != oldBuilder || !reflect.DeepEqual(live, before) {
					t.Fatal("unsuccessful compaction changed live context")
				}
			} else if current.dependencies.ContextBuilder == oldBuilder {
				t.Fatal("completed response did not replace context")
			}
			reopened, err := localfile.New(directory)
			if err != nil {
				t.Fatal(err)
			}
			page, err := reopened.Items(t.Context(), "session-1", 0, 100)
			if err != nil {
				t.Fatal(err)
			}
			last := page.Items[len(page.Items)-1]
			if last.Kind != sessionstore.ItemModelResponse || !reflect.DeepEqual(last.Data, response) {
				t.Fatal("response was not preserved in history")
			}
			replayed := newStopTestRun(t, 0).current
			replayed.dependencies.Sessions = reopened
			if err := replayed.loadHistory(t.Context()); err != nil {
				t.Fatal(err)
			}
			restored, err := replayed.dependencies.ContextBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restored, live) || !reflect.DeepEqual(current.state, replayed.state) {
				t.Fatal("live and replayed context or state differ")
			}
		})
	}
}

func TestCompactionPersistenceFailureStopsAndRetriesOnResume(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorded, directory := recordedCompactionHistory(t)
		run := newStopTestRun(t, 0)
		want := errors.New("response write failed")
		run.current.dependencies.Sessions = compactionFailureStore{Store: recorded.dependencies.Sessions, err: want}
		run.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
		run.start(t)
		run.input(t, externalEvent(t, 0, "pending", "Continue work"))
		assertCompactionTurn(t, run, 1, session.TurnCompaction)
		request := run.calls[0].request
		response := textResponse("Summary")
		response.Stop = llm.StopComplete
		run.respond(t, 0, response)
		select {
		case err := <-run.done:
			if !errors.Is(err, want) {
				t.Fatalf("Run error = %v, want %v", err, want)
			}
		default:
			t.Fatal("Run continued after failed summary persistence")
		}
		if len(run.calls) != 1 {
			t.Fatal("failed persistence started another model request")
		}
		store, err := localfile.New(directory)
		if err != nil {
			t.Fatal(err)
		}
		page, err := store.Items(t.Context(), "session-1", sessionstore.BeforeFirst, 100)
		if err != nil {
			t.Fatal(err)
		}
		if page.Items[len(page.Items)-1].Kind != sessionstore.ItemTurn {
			t.Fatal("failed response was persisted")
		}
		resumed := newStopTestRun(t, 0)
		resumed.current.dependencies.Sessions = store
		resumed.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
		resumed.start(t)
		assertCompactionTurn(t, resumed, 1, session.TurnCompaction)
		if !reflect.DeepEqual(resumed.calls[0].request, request) {
			t.Fatal("resumed compaction request differs from the failed attempt")
		}
		if resumed.current.pendingInputs() != 1 {
			t.Fatal("pending input was lost during recovery")
		}
		resumed.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
		resumed.respond(t, 0, response)
		assertCompactionTurn(t, resumed, 2, session.TurnRegular)
		resumed.respond(t, 1, textResponse("Done"))
		resumed.assertStopped(t)
	})
}

func recordedCompactionHistory(t *testing.T) (*coordinator, string) {
	t.Helper()
	directory := t.TempDir()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), "session-1"); err != nil {
		t.Fatal(err)
	}
	current := newStopTestRun(t, 0).current
	current.dependencies.Sessions = store
	previous := session.TurnID("")
	for index, total := range []int64{30_000, 40_000} {
		label := string(rune('A' + index))
		appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, inbox.ID(label), label+" input")})
		turn := session.Turn{ID: session.TurnID(label), PreviousTurnID: previous, Type: session.TurnRegular}
		appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
		response := textResponse(label + " output")
		response.Usage = llm.Usage{InputTokens: total - 1_000, OutputTokens: 1_000}
		appendCompactionTestItem(t, current, sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: turn.ID, Response: response}})
		previous = turn.ID
	}
	return current, directory
}

func appendCompactionTestItem(t *testing.T, current *coordinator, item sessionstore.Item) {
	t.Helper()
	if _, err := current.addItemToLocalState(item); err != nil {
		t.Fatal(err)
	}
	if err := current.storeItemInSessionStore(t.Context(), item); err != nil {
		t.Fatal(err)
	}
}

type compactionFailureStore struct {
	sessionstore.Store
	err error
}

func (store compactionFailureStore) AppendModelResponse(context.Context, session.ID, sessionstore.ModelResponse) error {
	return store.err
}
