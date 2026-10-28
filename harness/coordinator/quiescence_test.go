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
)

func assertQuiesced(t *testing.T, run *stopTestRun) {
	t.Helper()
	if err := run.current.TryQuiesce(t.Context()); err != nil {
		t.Fatalf("TryQuiesce = %v", err)
	}
	if err := <-run.done; err != ErrQuiesced {
		t.Fatalf("Run = %v, want ErrQuiesced", err)
	}
}

func TestTryQuiesceIdleRetiresWithFrozenInbox(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		assertQuiesced(t, run)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- run.inputs.Submit(ctx, externalEvent(t, 0, "late", "hello")) }()
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("retired inbox admitted input: %v", err)
		default:
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked Submit = %v", err)
		}
		if err := run.current.TryQuiesce(ctx); err != nil {
			t.Fatalf("retirement masked by late cancellation: %v", err)
		}
		if len(run.calls) != 0 || len(run.operations.cancels) != 0 {
			t.Fatal("idle quiescence started or canceled work")
		}
	})
}

func TestTryQuiesceBusyModelAndCompactionLeaveWorkUntouched(t *testing.T) {
	for _, compact := range []bool{false, true} {
		t.Run(map[bool]string{false: "model", true: "compaction"}[compact], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 0)
				requestStarted := make(chan struct{}, 4)
				adapter := run.current.dependencies.LLM.(*fakeAdapter)
				respond := adapter.respond
				adapter.respond = func(ctx context.Context, request llm.Request) (llm.Response, error) {
					requestStarted <- struct{}{}
					return respond(ctx, request)
				}
				if compact {
					// Keep this fence test independent of the shared disk-backed
					// compaction fixture: all persistence and scheduling here use
					// the same in-memory store as the ordinary-model subtest.
					run.store.items = nil
					previous := session.TurnID("")
					for index, tokens := range []int64{30_000, 40_000} {
						id := string(rune('A' + index))
						turn := session.Turn{ID: session.TurnID(id), PreviousTurnID: previous, Type: session.TurnRegular}
						response := textResponse(id + " output")
						response.Usage = llm.Usage{InputTokens: tokens - 1_000, OutputTokens: 1_000}
						for _, item := range []sessionstore.Item{
							{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, inbox.ID(id), id+" input")},
							{Kind: sessionstore.ItemTurn, Data: turn},
							{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: turn.ID, Response: response}},
						} {
							item.Sequence = sessionstore.Sequence(len(run.store.items) + 1)
							run.store.items = append(run.store.items, item)
						}
						previous = turn.ID
					}
					// Both threshold and the 20k retained-token budget must be
					// exceeded; history length alone does not trigger compaction.
					run.current.dependencies.ContextBuilder.SetModel(llm.Model{CompactionThreshold: 25_000})
				}
				run.start(t)
				if compact {
					if _, err := run.current.dependencies.ContextBuilder.BuildCompaction(); err != nil {
						t.Fatalf("fixture has no eligible compaction history: %v", err)
					}
				}
				run.input(t, externalEvent(t, 0, "work", "hello"))
				<-requestStarted
				synctest.Wait()
				wantType := session.TurnRegular
				if compact {
					wantType = session.TurnCompaction
				}
				if len(run.calls) != 1 || run.current.state.currentTurnType != wantType {
					t.Fatalf("calls = %d, turn = %s, want one active %s request", len(run.calls), run.current.state.currentTurnType, wantType)
				}
				if err := run.current.TryQuiesce(t.Context()); !errors.Is(err, ErrBusy) {
					t.Fatalf("active TryQuiesce = %v", err)
				}
				if run.calls[0].ctx.Err() != nil || len(run.operations.cancels) != 0 {
					t.Fatal("busy request canceled live work")
				}
				if compact {
					summary := textResponse("Summary")
					summary.Stop = llm.StopComplete
					run.respond(t, 0, summary)
					// Block on actual next-request admission so virtual-time slurp timers
					// can advance, rather than assuming two timeout durations suffice.
					<-requestStarted
					synctest.Wait()
					if len(run.calls) != 2 || run.current.state.currentTurnType != session.TurnRegular {
						select {
						case err := <-run.done:
							t.Fatalf("after summary Run exited: %v", err)
						default:
						}
						t.Fatalf("after summary: calls = %d, turn = %s, pending=%d model=%v failures=%d callModel=%v want ordinary continuation", len(run.calls), run.current.state.currentTurnType, run.current.pendingInputs(), run.current.cancelModel != nil, run.current.state.compactionFailures, run.current.state.callModel)
					}
					run.respond(t, 1, textResponse("Done"))
				} else {
					run.respond(t, 0, textResponse("Done"))
				}
				assertQuiesced(t, run)
			})
		})
	}
}

func TestTryQuiesceBusyShellAndGraphLeaveOperationsUntouched(t *testing.T) {
	for _, kind := range []operation.Type{operation.TypeShell, operation.Type("task_graph")} {
		t.Run(string(kind), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				run := newStopTestRun(t, 1)
				value := run.store.resume.Operations[0]
				value.Type = kind
				run.store.resume.Operations[0] = value
				status := run.store.items[2].Data.(sessionstore.ToolCallStatus)
				status.Operations[0] = value
				run.store.items[2].Data = status
				run.start(t)
				if err := run.current.TryQuiesce(t.Context()); !errors.Is(err, ErrBusy) {
					t.Fatalf("active operation TryQuiesce = %v", err)
				}
				if len(run.operations.cancels) != 0 || len(run.calls) != 0 {
					t.Fatal("busy request changed running work")
				}
				run.update(t, 0, operation.StatusCompleted)
				run.respond(t, 0, textResponse("Done"))
				assertQuiesced(t, run)
			})
		})
	}
}

func TestQuiesceFenceDrainsAcceptedFIFOAndThawsIfItCreatesWork(t *testing.T) {
	run := newStopTestRun(t, 0)
	for _, id := range []inbox.ID{"first", "second"} {
		submitTestInput(t, run.inputs, externalEvent(t, 0, id, string(id)))
	}
	request := quiesceRequest{ctx: t.Context(), result: make(chan error, 1)}
	retired, fatal := run.current.handleQuiesce(t.Context(), request)
	if retired || fatal != nil || !errors.Is(<-request.result, ErrBusy) {
		t.Fatalf("queued input incorrectly retired: retired=%t fatal=%v", retired, fatal)
	}
	if len(run.store.appendedInputs) != 2 || run.store.appendedInputs[0].ID != "first" || run.store.appendedInputs[1].ID != "second" || !run.current.state.callModel {
		t.Fatal("drained input was not processed in FIFO order")
	}
	// A thawed inbox accepts another input even though no Output receiver runs.
	submitTestInput(t, run.inputs, externalEvent(t, 0, "third", "third"))
}

func TestQuiesceRejectsPendingSchedulingAndFailedState(t *testing.T) {
	for _, kind := range []string{"callModel", "compaction", "failure", "operation", "toolCall"} {
		t.Run(kind, func(t *testing.T) {
			run := newStopTestRun(t, 0)
			switch kind {
			case "callModel":
				run.current.state.callModel = true
			case "compaction":
				run.current.state.manualCompaction = true
			case "failure":
				run.current.state.lastFailure = &ModelFailureError{}
			case "operation":
				run.current.state.operations["pending"] = operation.Operation{ID: "pending", Status: operation.StatusReady}
			case "toolCall":
				run.current.state.toolCalls[toolCallKey{callID: "pending"}] = toolCallState{}
			}
			request := quiesceRequest{ctx: t.Context(), result: make(chan error, 1)}
			retired, fatal := run.current.handleQuiesce(t.Context(), request)
			if retired || fatal != nil || !errors.Is(<-request.result, ErrBusy) {
				t.Fatalf("pending state incorrectly retired: retired=%t fatal=%v", retired, fatal)
			}
			submitTestInput(t, run.inputs, externalEvent(t, 0, "still-live", "hello"))
		})
	}
}

func TestTryQuiesceBeforeRunAndCanceledRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- run.current.TryQuiesce(ctx) }()
		synctest.Wait()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("before-Run cancellation = %v", err)
		}
		go func() { done <- run.current.TryQuiesce(t.Context()) }()
		synctest.Wait()
		run.start(t)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if err := <-run.done; err != ErrQuiesced {
			t.Fatalf("Run = %v", err)
		}
	})
}

func TestTryQuiesceCanceledLiveRequestDoesNotRetire(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := run.current.TryQuiesce(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled request = %v", err)
		}
		run.assertRunning(t)
		run.input(t, externalEvent(t, 0, "live", "hello"))
		run.respond(t, 0, textResponse("Done"))
		assertQuiesced(t, run)
	})
}

func TestTryQuiesceAfterNormalAndFailedExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
		run.assertStopped(t)
		if err := run.current.TryQuiesce(t.Context()); !errors.Is(err, ErrStopped) {
			t.Fatalf("after ordinary stop = %v", err)
		}
	})
	t.Run("restored-model-failure", func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.store.items = []sessionstore.Item{
			storedItem(1, sessionstore.ItemTurn, session.Turn{ID: "failed", Type: session.TurnRegular}),
			storedItem(2, sessionstore.ItemModelResponse, sessionstore.ModelResponse{
				TurnID: "failed", Response: llm.Response{Failure: &llm.Failure{Code: "failed", Message: "failure"}},
			}),
		}
		var failure *ModelFailureError
		if err := run.current.Run(t.Context()); !errors.As(err, &failure) {
			t.Fatalf("restored Run = %v", err)
		}
		err := run.current.TryQuiesce(t.Context())
		if !errors.Is(err, ErrStopped) || !errors.As(err, &failure) || errors.Is(err, ErrQuiesced) {
			t.Fatalf("after failed restore = %v", err)
		}
	})
}

func TestTryQuiesceRacingSubmitEitherDrainsOrBlocksProducer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.start(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		submitted := make(chan error, 1)
		go func() { submitted <- run.inputs.Submit(ctx, externalEvent(t, 0, "race", "hello")) }()
		err := run.current.TryQuiesce(t.Context())
		if err == nil {
			if result := <-run.done; result != ErrQuiesced {
				t.Fatalf("Run = %v", result)
			}
			synctest.Wait()
			select {
			case result := <-submitted:
				t.Fatalf("retired after accepting work: %v", result)
			default:
			}
			cancel()
			if result := <-submitted; !errors.Is(result, context.Canceled) {
				t.Fatalf("fenced Submit = %v", result)
			}
			return
		}
		if !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
		if result := <-submitted; result != nil {
			t.Fatal(result)
		}
		synctest.Wait()
		synctest.Sleep(2 * slurpIdleTimeout)
		synctest.Wait()
		run.respond(t, 0, textResponse("Done"))

		// Canonical inputs include one internal charged-request control now.
		// Validate that control causally rather than counting/filtering it away.
		var external []inbox.Input
		charges := 0
		for _, input := range run.store.appendedInputs {
			if input.Kind == inbox.InputExternal {
				external = append(external, input)
				continue
			}
			control, err := input.DecodeControlMessage()
			if err != nil || control.Mode != inbox.ModelRequest {
				t.Fatalf("unexpected internal input: %+v %v", control, err)
			}
			r := control.Parameters.(inbox.ModelRequestRecord)
			if r.Action != "started" || r.Attempt != 1 || r.Limit != 2 || r.Inputs != 1 || r.ExternalInputs != 1 || r.TurnType != "regular" || r.TurnID != string(run.current.state.currentTurnID) {
				t.Fatalf("invalid racing request charge: %+v", r)
			}
			charges++
		}
		if !reflect.DeepEqual(external, []inbox.Input{externalEvent(t, 0, "race", "hello")}) || charges != 1 {
			t.Fatalf("racing input lost/duplicated or charge missing: external=%+v charges=%d", external, charges)
		}

		assertQuiesced(t, run)
	})
}

func TestQuiesceSeesQueuedOperationUpdateAtFence(t *testing.T) {
	run := newStopTestRun(t, 0)
	run.operations.updates <- operation.Operation{ID: "racing", Status: operation.StatusReady}
	request := quiesceRequest{ctx: t.Context(), result: make(chan error, 1)}
	retired, fatal := run.current.handleQuiesce(t.Context(), request)
	if retired || fatal != nil || !errors.Is(<-request.result, ErrBusy) {
		t.Fatalf("queued operation incorrectly retired: retired=%t fatal=%v", retired, fatal)
	}
	if len(run.store.savedOperations) != 1 || len(run.operations.cancels) != 0 {
		t.Fatal("queued operation was lost or canceled")
	}
	submitTestInput(t, run.inputs, externalEvent(t, 0, "live", "hello"))
}

func TestQuiesceCancellationAfterFreezeProcessesFIFOAndThaws(t *testing.T) {
	run := newStopTestRun(t, 0)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// A stop-when-idle input can be persisted without starting any work. Cancel
	// the request during persistence, after the inbox fence has committed.
	run.store.onAppendInput = func(inbox.Input) { cancel() }
	submitTestInput(t, run.inputs, stopInput(t, "queued", inbox.StopWhenIdle))
	request := quiesceRequest{ctx: ctx, result: make(chan error, 1)}
	retired, fatal := run.current.handleQuiesce(t.Context(), request)
	if retired || fatal != nil || !errors.Is(<-request.result, context.Canceled) {
		t.Fatalf("canceled fenced request: retired=%t fatal=%v", retired, fatal)
	}
	if len(run.store.appendedInputs) != 1 {
		t.Fatal("cancellation abandoned drained input")
	}
	submitTestInput(t, run.inputs, externalEvent(t, 0, "live", "hello"))
}

func TestQuiescePersistenceFailureThawsAndFailsLoop(t *testing.T) {
	run := newStopTestRun(t, 0)
	diskErr := errors.New("disk unavailable")
	run.store.appendInputErr = diskErr
	submitTestInput(t, run.inputs, externalEvent(t, 0, "queued", "hello"))
	request := quiesceRequest{ctx: t.Context(), result: make(chan error, 1)}
	retired, fatal := run.current.handleQuiesce(t.Context(), request)
	if retired || !errors.Is(fatal, diskErr) || !errors.Is(<-request.result, diskErr) {
		t.Fatalf("persistence failure: retired=%t fatal=%v", retired, fatal)
	}
	submitTestInput(t, run.inputs, externalEvent(t, 0, "live", "hello"))
}

func TestTryQuiesceProcessesSlurpedInputsBeforeStartupRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		submitTestInput(t, run.inputs, externalEvent(t, 0, "queued", "hello"))
		result := make(chan error, 1)
		go func() { result <- run.current.TryQuiesce(t.Context()) }()
		synctest.Wait()
		run.start(t)
		if err := <-result; !errors.Is(err, ErrBusy) {
			t.Fatalf("startup request over queued input = %v", err)
		}
		if len(run.calls) != 1 || run.calls[0].ctx.Err() != nil || len(run.store.appendedInputs) != 2 {
			t.Fatal("startup request lost or canceled slurped work")
		}
		assertFixtureRequestStart(t, run.store.appendedInputs[1], run.store.appendedTurns[0].ID, 1, 1)
		run.respond(t, 0, textResponse("Done"))
		assertQuiesced(t, run)
	})
}
