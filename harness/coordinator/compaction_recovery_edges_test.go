package coordinator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// A manual control accepted after a regular dispatch must not change that
// request's retry kind or steal its pinned regular attempt identity.
func TestCompactionRecoveryRegularRetryPreservesNewManual(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, _ := recordedCompactionHistory(t)
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "work", "continue")})
		turn := session.Turn{ID: "regular-crash", PreviousTurnID: "B", Type: session.TurnRegular}
		if err := c.chargeModelRequest(t.Context(), turn); err != nil {
			t.Fatal(err)
		}
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "new-manual", inbox.Compact)})
		r, cancel := startRequestRun(t, c.dependencies.Sessions)
		assertCompactionTurn(t, r, 1, session.TurnRegular)
		records := requestRecords(t, c.dependencies.Sessions)
		if len(records) != 3 || records[2].Attempt != 2 || records[2].RequestID != records[0].RequestID || !r.current.state.manualCompaction || r.current.state.manualCompactionInput != "new-manual" {
			t.Fatalf("retry lost identity/manual: %+v", records)
		}
		response := textResponse("regular retry complete")
		response.Usage = llm.Usage{InputTokens: 49000, OutputTokens: 1000}
		r.respond(t, 0, response)
		assertCompactionTurn(t, r, 2, session.TurnCompaction)
		summaries := compactionRecoveryRecords(t, c.dependencies.Sessions)
		if len(summaries) != 1 || summaries[0].ManualInputID != "new-manual" || summaries[0].Request.Attempt != 1 {
			t.Fatalf("summary origin: %+v", summaries)
		}
		cancel()
		synctest.Wait()
		if err := <-r.done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}

type compactionRecoveryBuildFailure struct{ contextbuilder.Builder }

func (b compactionRecoveryBuildFailure) BuildCompaction() (contextbuilder.Result, error) {
	return contextbuilder.Result{}, errors.New("cannot reconstruct summary")
}

func TestCompactionRecoveryImpossibleRetryDoesNotFallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, _ := recordedCompactionHistory(t)
		compactionRecoveryPolicy(t, c, 3)
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "manual", inbox.Compact)})
		if err := c.chargeCompactionRequest(t.Context(), session.Turn{ID: "lost", Type: session.TurnCompaction}); err != nil {
			t.Fatal(err)
		}
		r := newStopTestRun(t, 0)
		r.current.dependencies.Sessions = c.dependencies.Sessions
		r.current.dependencies.ContextBuilder = compactionRecoveryBuildFailure{r.current.dependencies.ContextBuilder}
		r.start(t)
		if err := <-r.done; !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("error=%v", err)
		}
		records := compactionRecoveryRecords(t, c.dependencies.Sessions)
		if len(r.calls) != 0 || len(records) != 2 || records[1].Request.Action != "interrupted" || r.current.state.compactionFailures != 1 {
			t.Fatalf("unsafe fallback: calls=%d records=%+v", len(r.calls), records)
		}
	})
}

type compactionRecoveryStageFailure struct {
	sessionstore.Store
	mode   inbox.ControlMode
	action string
	err    error
	hits   *int
}

func (s compactionRecoveryStageFailure) AppendInput(ctx context.Context, id session.ID, input inbox.Input) error {
	if input.Kind == inbox.InputControl {
		m, err := input.DecodeControlMessage()
		if err != nil {
			return err
		}
		if m.Mode == s.mode {
			match := s.action == ""
			if r, ok := m.Parameters.(inbox.CompactionRequestRecord); ok {
				match = r.Request.Action == s.action
			}
			if match {
				*s.hits++
				return s.err
			}
		}
	}
	return s.Store.AppendInput(ctx, id, input)
}

// Each failure is injected at its actual coordinator lifecycle boundary, then
// the same durable history is reopened without the fault and with a larger
// configured limit. A failed control must never authorize dispatch or reset a
// budget already pinned in history.
func TestCompactionRecoveryControlStoreFaultStages(t *testing.T) {
	for _, stage := range []string{"policy", "started", "interrupted", "exhausted", "disabled"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c, _ := recordedCompactionHistory(t)
				limit := 3
				if stage == "exhausted" || stage == "disabled" {
					limit = 1
				}
				if stage != "policy" {
					compactionRecoveryPolicy(t, c, limit)
				}
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "pending", "preserve queued work")})
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "manual", inbox.Compact)})
				if stage == "interrupted" || stage == "exhausted" || stage == "disabled" {
					turn := session.Turn{ID: "lost-summary", PreviousTurnID: "B", Type: session.TurnCompaction}
					if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
						t.Fatal(err)
					}
					appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
				}
				durable := c.dependencies.Sessions
				mode := inbox.CompactionRequest
				action := stage
				if stage == "policy" {
					mode = inbox.SetCompactionPolicy
					action = ""
				}
				if stage == "disabled" {
					mode = inbox.CompactionDisabled
					action = ""
				}
				want := errors.New("injected " + stage + " persistence failure")
				hits := 0
				r, _ := startRequestRun(t, compactionRecoveryStageFailure{Store: durable, mode: mode, action: action, err: want, hits: &hits})
				if err := <-r.done; !errors.Is(err, want) {
					t.Fatalf("error=%v", err)
				}
				if hits != 1 || len(r.calls) != 0 {
					t.Fatalf("hits=%d dispatched=%d", hits, len(r.calls))
				}
				// Failed persistence leaves the live reducer equivalent to canonical replay.
				replay := newStopTestRun(t, 0).current
				replay.dependencies.Sessions = durable
				if err := replay.loadHistory(t.Context()); err != nil {
					t.Fatal(err)
				}
				if r.current.state.compactionFailures != replay.state.compactionFailures || r.current.state.compactionDisabled != replay.state.compactionDisabled || !reflect.DeepEqual(r.current.requests, replay.requests) {
					t.Fatal("failed control corrupted canonical accounting")
				}
				resumed := newStopTestRun(t, 0)
				resumed.current.dependencies.Sessions = durable
				resumed.current.dependencies.CompactionFailureLimit = 99
				resumed.start(t)
				if stage == "exhausted" || stage == "disabled" {
					assertCompactionTurn(t, resumed, 1, session.TurnRegular)
					if resumed.current.state.compactionFailures != 1 || !resumed.current.state.compactionDisabled || resumed.current.compactionFailureLimit() != 1 || resumed.current.pendingInputs() != 1 {
						t.Fatal("resume replenished budget or lost queued work")
					}
				} else {
					assertCompactionTurn(t, resumed, 1, session.TurnCompaction)
					records := compactionRecoveryRecords(t, durable)
					last := records[len(records)-1]
					attempt := 1
					if stage == "interrupted" {
						attempt = 2
					}
					expectedLimit := 3
					if stage == "policy" {
						expectedLimit = 99
					}
					if last.Request.Attempt != attempt || last.Request.Limit != expectedLimit || last.ManualInputID != "manual" {
						t.Fatalf("resume charge=%+v", last)
					}
				}
				resumed.input(t, stopInput(t, "hard-stop", inbox.StopHard))
				resumed.assertStopped(t)
			})
		})
	}
}
