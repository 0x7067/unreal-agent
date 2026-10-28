package coordinator

import (
	"context"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"testing"
	"testing/synctest"
)

func TestIdleStopDrainsModelAndCompactionWithoutCancellation(t *testing.T) {
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
				run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
				run.assertRunning(t)
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
				run.assertStopped(t)
			})
		})
	}
}

func TestIdleStopDrainsShellAndGraphWithoutCancellation(t *testing.T) {
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
				run.input(t, stopInput(t, "stop", inbox.StopWhenIdle))
				run.assertRunning(t)
				if len(run.operations.cancels) != 0 || len(run.calls) != 0 {
					t.Fatal("busy request changed running work")
				}
				run.update(t, 0, operation.StatusCompleted)
				run.respond(t, 0, textResponse("Done"))
				run.assertStopped(t)
			})
		})
	}
}

// The producer relinquishes ownership after Submit acknowledges, in the exact
// window between event slurp and the stop decision. The accepted child input
// must still run; an owner snapshot alone would lose it.
func TestIdleStopConsumesProducerAdmissionAfterSlurp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		started := make(chan struct{}, 1)
		adapter := run.current.dependencies.LLM.(*fakeAdapter)
		respond := adapter.respond
		adapter.respond = func(ctx context.Context, request llm.Request) (llm.Response, error) {
			started <- struct{}{}
			return respond(ctx, request)
		}
		queued := false
		run.current.dependencies.PendingWork = func() bool {
			if !queued {
				queued = true
				if err := run.inputs.Submit(t.Context(), externalEvent(t, 0, "late-child", "required child completion")); err != nil {
					t.Fatal(err)
				}
			}
			return false
		}
		submitTestInput(t, run.inputs, stopInput(t, "stop", inbox.StopWhenIdle))
		run.start(t)
		run.assertRunning(t)
		<-started
		synctest.Wait()
		if len(run.calls) != 1 {
			t.Fatalf("queued producer input hidden at idle: calls=%d", len(run.calls))
		}
		run.respond(t, 0, textResponse("child consumed"))
		run.assertStopped(t)
	})
}
