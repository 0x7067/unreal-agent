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
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func requestTestStore(t *testing.T) *localfile.Store {
	t.Helper()
	s, err := localfile.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(t.Context(), "session-1"); err != nil {
		t.Fatal(err)
	}
	return s
}
func startRequestRun(t *testing.T, s sessionstore.Store) (*stopTestRun, context.CancelFunc) {
	t.Helper()
	r := newStopTestRun(t, 0)
	restoreTestRun(t, r, s)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go func() { r.done <- r.current.Run(ctx) }()
	synctest.Sleep(2 * slurpIdleTimeout)
	synctest.Wait()
	return r, cancel
}
func requestRecords(t *testing.T, s sessionstore.Store) []inbox.ModelRequestRecord {
	t.Helper()
	p, err := s.Items(t.Context(), "session-1", sessionstore.BeforeFirst, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []inbox.ModelRequestRecord
	for _, i := range p.Items {
		if v, ok := i.Data.(inbox.Input); ok && v.Kind == inbox.InputControl {
			m, e := v.DecodeControlMessage()
			if e != nil {
				t.Fatal(e)
			}
			if m.Mode == inbox.ModelRequest {
				out = append(out, m.Parameters.(inbox.ModelRequestRecord))
			}
		}
	}
	return out
}
func TestRequestRecoveryBoundSurvivesRepeatedRuntimeLoss(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := requestTestStore(t)
		if err := s.AppendInput(t.Context(), "session-1", externalEvent(t, 0, "original", "hello")); err != nil {
			t.Fatal(err)
		}
		first, cancel := startRequestRun(t, s)
		if len(first.calls) != 1 {
			t.Fatal("first request not dispatched")
		}
		cancel()
		synctest.Wait()
		if err := <-first.done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		second, cancel := startRequestRun(t, s)
		if len(second.calls) != 1 {
			t.Fatal("one bounded retry not dispatched")
		}
		cancel()
		synctest.Wait()
		if err := <-second.done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		third, _ := startRequestRun(t, s)
		if len(third.calls) != 0 || third.current.pendingInputs() != 0 {
			t.Fatal("exhausted request automatically retried")
		}
		records := requestRecords(t, s)
		if len(records) != 5 {
			t.Fatalf("records=%+v", records)
		}
		if records[0].Action != "started" || records[1].Action != "interrupted" || records[2].Action != "started" || records[3].Action != "interrupted" || records[4].Action != "exhausted" {
			t.Fatal(records)
		}
		if records[0].RequestID != records[2].RequestID || records[0].TurnID == records[2].TurnID || records[0].Attempt != 1 || records[2].Attempt != 2 {
			t.Fatal("retry identity/budget reset")
		}
		third.input(t, externalEvent(t, 1, "new", "new work"), stopInput(t, "idle", inbox.StopWhenIdle))
		if len(third.calls) != 1 {
			t.Fatal("new input stranded by exhaustion")
		}
		third.respond(t, 0, textResponse("done"))
		third.assertStopped(t)
		fourth, _ := startRequestRun(t, s)
		if len(fourth.calls) != 0 {
			t.Fatal("committed response requested again")
		}
		fourth.input(t, stopInput(t, "idle-again", inbox.StopWhenIdle))
		fourth.assertStopped(t)
		records = requestRecords(t, s)
		last := records[len(records)-1]
		if last.Action != "started" || last.Attempt != 1 || last.RequestID == records[0].RequestID {
			t.Fatal("new input did not get a distinct bounded request")
		}
	})
}
func TestRequestRecoveryChargeBeforeTurnGapStillConsumesAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := requestTestStore(t)
		if err := s.AppendInput(t.Context(), "session-1", externalEvent(t, 0, "original", "hello")); err != nil {
			t.Fatal(err)
		}
		if err := s.AppendInput(t.Context(), "session-1", fixtureRequestStart(t, "never-dispatched", 1, 1)); err != nil {
			t.Fatal(err)
		}
		r, _ := startRequestRun(t, s)
		if len(r.calls) != 1 {
			t.Fatal("charged gap not recovered")
		}
		records := requestRecords(t, s)
		if len(records) != 3 || records[2].Attempt != 2 || records[2].RequestID != "request-never-dispatched" {
			t.Fatal(records)
		}
		r.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		r.respond(t, 0, textResponse("done"))
		r.assertStopped(t)
	})
}
func TestRequestRecoveryLegacyUnansweredTurnRefusesWithoutMutation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := requestTestStore(t)
		if err := s.AppendInput(t.Context(), "session-1", externalEvent(t, 0, "original", "hello")); err != nil {
			t.Fatal(err)
		}
		if err := s.AppendTurn(t.Context(), "session-1", session.Turn{ID: "legacy", Type: session.TurnRegular}); err != nil {
			t.Fatal(err)
		}
		before, _ := s.Items(t.Context(), "session-1", 0, 100)
		r, _ := startRequestRun(t, s)
		if err := <-r.done; !errors.Is(err, operation.ErrUnsupported) {
			t.Fatal(err)
		}
		after, _ := s.Items(t.Context(), "session-1", 0, 100)
		if len(r.calls) != 0 || !reflect.DeepEqual(before, after) {
			t.Fatal("legacy refusal dispatched or changed history")
		}
	})
}
func TestRequestRecoveryIncompleteResponseNeverDispatchesTools(t *testing.T) {
	for _, stop := range []llm.StopReason{"", llm.StopInterrupted, llm.StopMaxOutputTokens, llm.StopRefused} {
		t.Run(string(stop), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newStopTestRun(t, 0)
				r.start(t)
				r.input(t, externalEvent(t, 0, "original", "hello"), stopInput(t, "idle", inbox.StopWhenIdle))
				response := llm.Response{Stop: stop, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "partial", Name: "ViewImage", Arguments: `{"path":"must-not-read"}`}}}}
				r.respond(t, 0, response)
				r.assertStopped(t)
				if len(r.operations.adds) != 0 || len(r.store.appendedStatuses) != 0 {
					t.Fatal("partial tool dispatched")
				}
				built, err := r.current.dependencies.ContextBuilder.Build()
				if err != nil {
					t.Fatal(err)
				}
				for _, i := range built.Request.Input {
					if i.Type == llm.ItemToolCall {
						t.Fatal("partial call entered future context")
					}
				}
			})
		})
	}
}
func TestRequestLedgerRejectsInvalidTransitions(t *testing.T) {
	base := inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: "request", TurnID: "turn", TurnType: "regular", Attempt: 1, Limit: 2}
	for _, which := range []string{"future", "cursor", "limit", "retry-skip", "changed-outcome", "premature-exhaustion", "reused-turn", "new-overlap", "type"} {
		t.Run(which, func(t *testing.T) {
			c := newStopTestRun(t, 0).current
			r := base
			switch which {
			case "future":
				r.Version = 2
			case "cursor":
				r.Inputs = 1
			case "limit":
				r.Limit = 3
			case "type":
				r.TurnType = "compaction"
			default:
				if err := c.observeRequestRecord(base); err != nil {
					t.Fatal(err)
				}
				switch which {
				case "retry-skip":
					r.Action = "interrupted"
					if err := c.observeRequestRecord(r); err != nil {
						t.Fatal(err)
					}
					r.Action = "started"
					r.TurnID = "next"
					r.Attempt = 1
				case "changed-outcome":
					r.Action = "interrupted"
					r.RequestID = "other"
				case "premature-exhaustion":
					r.Action = "exhausted"
				case "reused-turn":
					r.Action = "started"
				case "new-overlap":
					r.RequestID = "other"
					r.TurnID = "other"
				}
			}
			if err := c.observeRequestRecord(r); err == nil {
				t.Fatalf("accepted invalid %s transition", which)
			}
		})
	}
}

func TestRequestRecoverySemanticCorruptionRefusesBeforeMutation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := requestTestStore(t)
		if err := s.AppendInput(t.Context(), "session-1", externalEvent(t, 0, "original", "hello")); err != nil {
			t.Fatal(err)
		}
		if err := s.AppendInput(t.Context(), "session-1", fixtureRequestStart(t, "bad-cursor", 2, 1)); err != nil {
			t.Fatal(err)
		}
		before, _ := s.Items(t.Context(), "session-1", 0, 100)
		r, _ := startRequestRun(t, s)
		if err := <-r.done; !errors.Is(err, operation.ErrUnsupported) {
			t.Fatal(err)
		}
		after, _ := s.Items(t.Context(), "session-1", 0, 100)
		if len(r.calls) != 0 || !reflect.DeepEqual(before, after) {
			t.Fatal("corrupt policy changed history or dispatched")
		}
	})
}

func TestRequestRecoveryPreservesOnlyKnownSupersededUsage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := requestTestStore(t)
		r := newStopTestRun(t, 0)
		restoreTestRun(t, r, s)
		if err := r.current.handleInboxInput(t.Context(), externalEvent(t, 0, "original", "hello")); err != nil {
			t.Fatal(err)
		}
		if err := r.current.recordControl(t.Context(), inbox.ControlMessage{Mode: inbox.ModelRequest, Parameters: inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: "request", TurnID: "turn", TurnType: "regular", Attempt: 1, Limit: 2, Inputs: 1, ExternalInputs: 1}}); err != nil {
			t.Fatal(err)
		}
		turn := sessionstore.Item{Kind: sessionstore.ItemTurn, Data: session.Turn{ID: "turn", Type: session.TurnRegular}}
		if err := r.current.storeItemInSessionStore(t.Context(), turn); err != nil {
			t.Fatal(err)
		}
		if _, err := r.current.addItemToLocalState(turn); err != nil {
			t.Fatal(err)
		}
		if err := r.current.recordRequestAction(t.Context(), "superseded", "New user work replaced the request."); err != nil {
			t.Fatal(err)
		}
		usage := llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 11, OutputTokens: 3}}
		received := modelResponseResult{turnID: "turn", response: llm.Response{Stop: llm.StopComplete, Usage: usage, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "late", Name: "ViewImage", Arguments: `{"path":"must-not-read"}`}}}}}
		if err := r.current.preserveSupersededUsage(t.Context(), received); err != nil {
			t.Fatal(err)
		}
		if err := r.current.preserveSupersededUsage(t.Context(), received); err != nil {
			t.Fatal(err)
		}
		p, err := s.Items(t.Context(), "session-1", 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		var responses []sessionstore.ModelResponse
		for _, i := range p.Items {
			if v, ok := i.Data.(sessionstore.ModelResponse); ok {
				responses = append(responses, v)
			}
		}
		if len(responses) != 1 || !reflect.DeepEqual(responses[0].Response.Usage, usage) || len(responses[0].Response.Output) != 0 || responses[0].Response.Stop != llm.StopInterrupted {
			t.Fatalf("late usage = %+v", responses)
		}
		if len(r.current.state.toolCalls) != 0 || len(r.operations.adds) != 0 {
			t.Fatal("late tools revived")
		}
	})
}

// A committed stop is authoritative even if a crash interrupts the small gap
// before its redundant typed request settlement is appended.
func TestRequestRecoveryHardStopAfterInterruptionCannotRetry(t *testing.T) {
	for _, stage := range []string{"started", "interrupted"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := requestTestStore(t)
				if err := s.AppendInput(t.Context(), "session-1", externalEvent(t, 0, "original", "hello")); err != nil {
					t.Fatal(err)
				}
				start := fixtureRequestStart(t, "turn", 1, 1)
				if err := s.AppendInput(t.Context(), "session-1", start); err != nil {
					t.Fatal(err)
				}
				if err := s.AppendTurn(t.Context(), "session-1", session.Turn{ID: "turn", Type: session.TurnRegular}); err != nil {
					t.Fatal(err)
				}
				if stage == "interrupted" {
					m, _ := start.DecodeControlMessage()
					r := m.Parameters.(inbox.ModelRequestRecord)
					r.Action = "interrupted"
					payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.ModelRequest, Reason: "owned crash boundary", Parameters: r})
					if err != nil {
						t.Fatal(err)
					}
					if err := s.AppendInput(t.Context(), "session-1", inbox.Input{ID: "interrupted", Kind: inbox.InputControl, Payload: payload}); err != nil {
						t.Fatal(err)
					}
				}
				if err := s.AppendInput(t.Context(), "session-1", stopInput(t, "committed-stop", inbox.StopHard)); err != nil {
					t.Fatal(err)
				}
				r, _ := startRequestRun(t, s)
				if len(r.calls) != 0 || r.current.pendingInputs() != 0 {
					t.Fatal("committed stop retried or resurrected original inputs")
				}
				records := requestRecords(t, s)
				if records[len(records)-1].Action != "stopped" {
					t.Fatal(records)
				}
				r.input(t, stopInput(t, "finish-idle", inbox.StopWhenIdle))
				r.assertStopped(t)
			})
		})
	}
}
