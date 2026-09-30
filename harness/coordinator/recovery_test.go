package coordinator

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestCoordinatorResumeProcessesQueuedHardStopBeforeModelRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.store.items = []sessionstore.Item{
			storedItem(1, sessionstore.ItemInput, externalEvent(t, 0, "pending", "hello")),
		}
		submitTestInput(t, run.inputs, stopInput(t, "stop", inbox.StopHard))
		run.start(t)
		if err := <-run.done; err != nil {
			t.Fatal(err)
		}
		if len(run.store.appendedTurns) != 0 {
			t.Fatal("resume started a model turn before accepting the queued hard stop")
		}
	})
}

func TestCoordinatorResumeProcessesQueuedSettingsBeforeModelRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.store.items = []sessionstore.Item{
			storedItem(1, sessionstore.ItemInput, externalEvent(t, 0, "pending", "hello")),
		}
		run.current.dependencies.ContextBuilder.SetModel(llm.Model{ID: "previous"})
		adapter := &fakeAdapter{respond: func(context.Context, llm.Request) (llm.Response, error) {
			return textResponse("Hello."), nil
		}}
		run.current.dependencies.LLM = adapter
		submitTestInput(t, run.inputs, settingsInput(t, "settings", inbox.Settings{Model: "next"}))
		submitTestInput(t, run.inputs, stopInput(t, "stop", inbox.StopWhenIdle))
		run.start(t)
		if err := <-run.done; err != nil {
			t.Fatal(err)
		}
		requests := adapter.requestSnapshot()
		if len(requests) != 1 {
			t.Fatalf("requests = %d, want one resumed request", len(requests))
		}
		if got := requests[0].Model.ID; got != "next" {
			t.Fatalf("resumed request model = %q, want the queued model %q", got, "next")
		}
	})
}

func TestCoordinatorResumeProcessesQueuedInputBeforeModelRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.store.items = []sessionstore.Item{
			storedItem(1, sessionstore.ItemInput, externalEvent(t, 0, "pending", "hello")),
		}
		adapter := &fakeAdapter{respond: func(context.Context, llm.Request) (llm.Response, error) {
			return textResponse("Hello."), nil
		}}
		run.current.dependencies.LLM = adapter
		submitTestInput(t, run.inputs, externalEvent(t, 1, "queued", "continue"))
		submitTestInput(t, run.inputs, stopInput(t, "stop", inbox.StopWhenIdle))
		run.start(t)
		if err := <-run.done; err != nil {
			t.Fatal(err)
		}
		requests := adapter.requestSnapshot()
		want := withPreamble(t,
			llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "hello"}},
			llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "continue"}},
		)
		if len(requests) != 1 {
			t.Fatalf("requests = %d, want one resumed request", len(requests))
		}
		if !reflect.DeepEqual(requests[0].Input, want) {
			t.Fatal("resumed request did not include both inputs")
		}
	})
}

func TestCoordinatorResumeProcessesQueuedCompletionBeforeModelRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 1)
		run.store.items = append(run.store.items,
			storedItem(4, sessionstore.ItemInput, externalEvent(t, 0, "pending", "check progress")),
		)
		adapter := &fakeAdapter{respond: func(context.Context, llm.Request) (llm.Response, error) {
			return textResponse("Done."), nil
		}}
		run.current.dependencies.LLM = adapter
		completed := run.store.resume.Operations[0]
		completed.Status = operation.StatusCompleted
		run.operations.updates <- completed
		submitTestInput(t, run.inputs, stopInput(t, "stop", inbox.StopWhenIdle))
		run.start(t)
		if err := <-run.done; err != nil {
			t.Fatal(err)
		}
		requests := adapter.requestSnapshot()
		if len(requests) != 1 {
			t.Fatalf("requests = %d, want one resumed request", len(requests))
		}
		assertStopResult(t, requests[0], "call-0", string(operation.StatusCompleted))
	})
}

func TestCoordinatorResumesUnavailableTool(t *testing.T) {
	for _, stage := range []string{"untranslated", "rejected", "accepted"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 1)
				wantError := `tool "ViewImage" is not available`
				switch stage {
				case "untranslated":
					run.store.items = run.store.items[:2]
				case "rejected":
					status := run.store.items[2].Data.(sessionstore.ToolCallStatus)
					status.Status = tool.CallStatus{Error: wantError}
					status.Operations = nil
					run.store.items[2].Data = status
				}
				store := persistTestRun(t, run)
				before, err := store.Items(t.Context(), "session-1", sessionstore.BeforeFirst, 100)
				if err != nil {
					t.Fatal(err)
				}
				restoreTestRun(t, run, store)
				run.current.dependencies.Tools = tool.NewRegistry(tool.StaticTranslators{})
				run.start(t)
				if stage != "accepted" {
					run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
					if len(run.calls) != 1 || len(run.operations.adds) != 0 {
						t.Fatal("unavailable tool did not produce one corrective request without dispatch")
					}
					assertStopResult(t, run.calls[0].request, "call-0", wantError)
					run.respond(t, 0, textResponse("Corrected."))
					run.assertStopped(t)

					resumed := newStopTestRun(t, 0)
					resumed.current.dependencies.Tools = tool.NewRegistry(tool.StaticTranslators{})
					restoreTestRun(t, resumed, store)
					resumed.start(t)
					resumed.input(t, stopInput(t, "stop-again", inbox.StopWhenIdle))
					resumed.assertStopped(t)
					if len(resumed.calls) != 0 || len(resumed.operations.adds) != 0 {
						t.Fatal("settled validation error started work on resume")
					}
					return
				}
				select {
				case err := <-run.done:
					if err == nil || !strings.Contains(err.Error(), `tool "ViewImage" required by recorded call "call-0" is not available`) {
						t.Fatalf("Run error = %v, want unavailable-tool error", err)
					}
				default:
					t.Fatal("Run did not abort")
				}
				if len(run.calls) != 0 || len(run.operations.adds) != 0 {
					t.Fatal("unresolvable tool started work")
				}
				after, err := store.Items(t.Context(), "session-1", sessionstore.BeforeFirst, 100)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, after) {
					t.Fatal("failed resume changed persisted history")
				}
			})
		})
	}
}

func TestCoordinatorResumesUnansweredInput(t *testing.T) {
	for _, stage := range []string{"input", "turn", "response", "compaction", "compaction response"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 0)
				run.store.items = []sessionstore.Item{
					storedItem(1, sessionstore.ItemInput, externalEvent(t, 0, "user", "hello")),
				}
				if stage != "input" {
					turn := session.Turn{ID: "first", Type: session.TurnRegular}
					if stage == "compaction" || stage == "compaction response" {
						turn.Type = session.TurnCompaction
					}
					run.store.items = append(run.store.items, storedItem(2, sessionstore.ItemTurn, turn))
				}
				if stage == "response" || stage == "compaction response" {
					run.store.items = append(run.store.items, storedItem(3, sessionstore.ItemModelResponse,
						sessionstore.ModelResponse{TurnID: "first", Response: textResponse("Hello.")}))
				}
				run.start(t)
				run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
				if stage == "response" {
					if len(run.calls) != 0 {
						t.Fatal("answered input caused another request")
					}
				} else {
					if len(run.calls) != 1 {
						t.Fatalf("requests = %d, want resumed input delivery", len(run.calls))
					}
					if run.current.state.currentTurnType != session.TurnRegular || run.current.state.currentTurnID == "first" || run.current.pendingInputs() != 1 {
						t.Fatal("resume did not start ordinary delivery of pending input")
					}
					want := withPreamble(t, llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "hello"}})
					if !reflect.DeepEqual(run.calls[0].request.Input, want) {
						t.Fatal("resume changed pending input context")
					}
					run.respond(t, 0, textResponse("Hello."))
				}
				if run.current.pendingInputs() != 0 {
					t.Fatal("ordinary response did not deliver pending input")
				}
				run.assertStopped(t)
			})
		})
	}
}

func TestCoordinatorStopRejectsUnsupportedOperation(t *testing.T) {
	for _, mode := range []inbox.ControlMode{inbox.StopHard, inbox.StopWhenIdle} {
		t.Run(string(mode), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 1)
				run.current.dependencies.Operations = operation.NewLocalOperationManager(t.Context())
				submitTestInput(t, run.inputs, stopInput(t, "stop", mode))
				run.start(t)
				select {
				case err := <-run.done:
					if !errors.Is(err, operation.ErrUnsupported) {
						t.Fatalf("Run error = %v, want unsupported operation", err)
					}
				default:
					t.Fatal("Run is waiting for an unsupported operation")
				}
				if len(run.store.savedOperations) != 0 || len(run.calls) != 0 {
					t.Fatal("unsupported operation changed state or started a model request")
				}
			})
		})
	}
}

func TestCoordinatorResumesSavedTerminalOperation(t *testing.T) {
	for _, terminal := range []operation.Status{operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		t.Run(string(terminal), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 1)
				directory := t.TempDir()
				store, err := localfile.New(directory)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.Create(t.Context(), "session-1"); err != nil {
					t.Fatal(err)
				}
				if err := store.AppendTurn(t.Context(), "session-1", run.store.items[0].Data.(session.Turn)); err != nil {
					t.Fatal(err)
				}
				if err := store.AppendModelResponse(t.Context(), "session-1", run.store.items[1].Data.(sessionstore.ModelResponse)); err != nil {
					t.Fatal(err)
				}
				if err := store.AppendToolCallStatus(t.Context(), "session-1", run.store.items[2].Data.(sessionstore.ToolCallStatus)); err != nil {
					t.Fatal(err)
				}
				completed := run.store.resume.Operations[0]
				completed.Status = terminal
				if err := store.SaveOperation(t.Context(), "session-1", completed); err != nil {
					t.Fatal(err)
				}
				store, err = localfile.New(directory)
				if err != nil {
					t.Fatal(err)
				}
				restored, err := store.Resume(t.Context(), "session-1")
				if err != nil {
					t.Fatal(err)
				}
				run.current.dependencies.Sessions, run.current.dependencies.Restored = store, restored
				run.start(t)
				run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
				if len(run.operations.adds) != 0 {
					t.Fatal("saved terminal operation was dispatched again")
				}
				if len(run.calls) != 1 {
					t.Fatalf("requests = %d, want restored result delivery", len(run.calls))
				}
				assertStopResult(t, run.calls[0].request, "call-0", string(terminal))
				run.respond(t, 0, textResponse("Done."))
				run.assertStopped(t)

				resumed := newStopTestRun(t, 0)
				restored, err = store.Resume(t.Context(), "session-1")
				if err != nil {
					t.Fatal(err)
				}
				if len(restored.Operations) != 0 {
					t.Fatalf("settled operations retained for resume: %#v", restored.Operations)
				}
				resumed.current.dependencies.Sessions, resumed.current.dependencies.Restored = store, restored
				resumed.start(t)
				resumed.input(t, stopInput(t, "stop-again", inbox.StopWhenIdle))
				if len(resumed.calls) != 0 || len(resumed.operations.adds) != 0 {
					t.Fatal("delivered terminal result caused work on the next resume")
				}
				resumed.assertStopped(t)
			})
		})
	}
}
