package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func compactionRecoveryRecords(t *testing.T, s sessionstore.Store) []inbox.CompactionRequestRecord {
	t.Helper()
	p, err := s.Items(t.Context(), "session-1", sessionstore.BeforeFirst, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []inbox.CompactionRequestRecord
	for _, item := range p.Items {
		if v, ok := item.Data.(inbox.Input); ok && v.Kind == inbox.InputControl {
			m, err := v.DecodeControlMessage()
			if err != nil {
				t.Fatal(err)
			}
			if m.Mode == inbox.CompactionRequest {
				out = append(out, m.Parameters.(inbox.CompactionRequestRecord))
			}
		}
	}
	return out
}

func compactionRecoveryPolicy(t *testing.T, c *coordinator, limit int) {
	t.Helper()
	if err := c.recordControl(t.Context(), inbox.ControlMessage{Mode: inbox.SetCompactionPolicy, Parameters: inbox.CompactionPolicy{FailureLimit: limit}}); err != nil {
		t.Fatal(err)
	}
}

// Both the charge/turn gap and a dispatched manual-only summary must recover
// without ordinary input or the transient manualCompaction boolean as a trigger.
func TestCompactionRecoveryManualOnlyCrashAndChargeGap(t *testing.T) {
	for _, turnWritten := range []bool{false, true} {
		t.Run(map[bool]string{false: "bare-charge", true: "turn-written"}[turnWritten], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c, dir := recordedCompactionHistory(t)
				compactionRecoveryPolicy(t, c, 3)
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "manual", inbox.Compact)})
				turn := session.Turn{ID: "charged-summary", PreviousTurnID: "B", Type: session.TurnCompaction}
				if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
					t.Fatal(err)
				}
				if turnWritten {
					appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
				}
				s, err := localfile.New(dir)
				if err != nil {
					t.Fatal(err)
				}
				r, cancel := startRequestRun(t, s)
				assertCompactionTurn(t, r, 1, session.TurnCompaction)
				if r.current.pendingInputs() != 0 {
					t.Fatal("manual-only recovery invented ordinary work")
				}
				records := compactionRecoveryRecords(t, s)
				if len(records) != 3 || records[1].Request.Action != "interrupted" || records[2].Request.Attempt != 2 || records[2].Request.RequestID != records[0].Request.RequestID || records[2].Request.TurnID == records[0].Request.TurnID || records[2].ManualInputID != "manual" || records[2].FailuresBefore != 0 || records[2].Request.Limit != 3 {
					t.Fatalf("recovery records = %+v", records)
				}
				if r.current.state.compactionFailures != 1 {
					t.Fatalf("failure count=%d", r.current.state.compactionFailures)
				}
				cancel()
				synctest.Wait()
				if err := <-r.done; !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestCompactionRecoveryRemainingBudgetAndQueuedInput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, dir := recordedCompactionHistory(t)
		compactionRecoveryPolicy(t, c, 3)
		// Committed unusable responses are counted by applyCompactionResponse,
		// whereas an uncommitted charged summary is counted by interruption.
		for i, id := range []session.TurnID{"failure-one", "failure-two"} {
			turn := session.Turn{ID: id, PreviousTurnID: c.state.currentTurnID, Type: session.TurnCompaction}
			if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
				t.Fatal(err)
			}
			appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
			appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: id, Response: llm.Response{Stop: llm.StopRefused}}})
			if c.state.compactionFailures != i+1 {
				t.Fatalf("committed summary counted %d times", c.state.compactionFailures)
			}
		}
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "queued", "Do not lose this")})
		turn := session.Turn{ID: "last-budget", PreviousTurnID: c.state.currentTurnID, Type: session.TurnCompaction}
		if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
			t.Fatal(err)
		}
		records := compactionRecoveryRecords(t, c.dependencies.Sessions)
		last := records[len(records)-1]
		if last.Request.Limit != 1 || last.FailuresBefore != 2 {
			t.Fatalf("remaining budget = %+v", last)
		}
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
		s, err := localfile.New(dir)
		if err != nil {
			t.Fatal(err)
		}
		r, _ := startRequestRun(t, s)
		assertCompactionTurn(t, r, 1, session.TurnRegular)
		if !r.current.state.compactionDisabled || r.current.state.compactionFailures != 3 || r.current.pendingInputs() != 1 {
			t.Fatalf("exhaustion: disabled=%v failures=%d pending=%d", r.current.state.compactionDisabled, r.current.state.compactionFailures, r.current.pendingInputs())
		}
		records = compactionRecoveryRecords(t, s)
		if len(records) != 5 || records[3].Request.Action != "interrupted" || records[4].Request.Action != "exhausted" {
			t.Fatalf("records=%+v", records)
		}
		found := false
		for _, item := range r.calls[0].request.Input {
			if item.Type == llm.ItemMessage && item.Data.(llm.Message).Text == "Do not lose this" {
				found = true
			}
		}
		if !found {
			t.Fatal("exhaustion omitted queued input from ordinary request")
		}
		r.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		r.respond(t, 0, textResponse("Done"))
		r.assertStopped(t)
	})
}

func TestCompactionRecoveryMetadataRejectedBeforeCommit(t *testing.T) {
	for _, which := range []string{"version", "forged-manual", "cursor", "failure-cursor", "limit", "retry-origin", "retry-limit", "retry-failures", "retry-cursor"} {
		t.Run(which, func(t *testing.T) {
			c, _ := recordedCompactionHistory(t)
			compactionRecoveryPolicy(t, c, 3)
			r := inbox.CompactionRequestRecord{Version: 1, Request: inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: "group", TurnID: "summary", TurnType: "compaction", Attempt: 1, Limit: 3, Inputs: c.state.availableInputs, ExternalInputs: c.state.externalInputs}}
			switch which {
			case "version":
				r.Version = 2
			case "forged-manual":
				r.ManualInputID = "forged"
			case "cursor":
				r.Request.Inputs++
			case "failure-cursor":
				r.FailuresBefore++
			case "limit":
				r.Request.Limit++
			default:
				if err := c.recordControl(t.Context(), inbox.ControlMessage{Mode: inbox.CompactionRequest, Parameters: r}); err != nil {
					t.Fatal(err)
				}
				if err := c.recordRequestAction(t.Context(), "interrupted", "crash"); err != nil {
					t.Fatal(err)
				}
				r.Request.Attempt = 2
				r.Request.TurnID = "retry"
				switch which {
				case "retry-origin":
					r.ManualInputID = "forged"
				case "retry-limit":
					r.Request.Limit = 2
				case "retry-failures":
					r.FailuresBefore = 1
				case "retry-cursor":
					r.Request.ExternalInputs++
				}
			}
			before, err := c.dependencies.Sessions.Items(t.Context(), "session-1", 0, 1000)
			if err != nil {
				t.Fatal(err)
			}
			state := c.state
			ledger := c.requests.clone()
			if c.requests.byTurn == nil {
				ledger.byTurn = nil
			}
			if err := c.recordControl(t.Context(), inbox.ControlMessage{Mode: inbox.CompactionRequest, Parameters: r}); err == nil {
				t.Fatalf("accepted %s", which)
			}
			after, err := c.dependencies.Sessions.Items(t.Context(), "session-1", 0, 1000)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(state, c.state) || !reflect.DeepEqual(ledger, c.requests) {
				t.Fatal("rejected metadata mutated canonical or live state")
			}
		})
	}
}

func TestCompactionRecoverySettlementAndNewManual(t *testing.T) {
	for _, action := range []string{"stopped", "superseded"} {
		t.Run(action, func(t *testing.T) {
			c, _ := recordedCompactionHistory(t)
			compactionRecoveryPolicy(t, c, 3)
			appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "old-manual", inbox.Compact)})
			if err := c.chargeCompactionRequest(t.Context(), session.Turn{ID: "old-turn", Type: session.TurnCompaction}); err != nil {
				t.Fatal(err)
			}
			old := c.requests.latest.record.RequestID
			if err := c.recordRequestAction(t.Context(), "interrupted", "crash"); err != nil {
				t.Fatal(err)
			}
			if c.state.compactionFailures != 1 {
				t.Fatal("interruption was not counted")
			}
			for _, id := range []inbox.ID{"new-manual", "latest-manual"} {
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, id, inbox.Compact)})
			}
			if err := c.recordRequestAction(t.Context(), action, "settle interrupted"); err != nil {
				t.Fatal(err)
			}
			if err := c.recordRequestAction(t.Context(), action, "duplicate settlement"); err != nil {
				t.Fatal(err)
			}
			c.consumeCompactionManualOrigin(c.requests.latest)
			if c.state.compactionFailures != 1 || c.state.manualCompactionInput != "latest-manual" {
				t.Fatal("settlement double-counted or consumed newer manual intent")
			}
			if err := c.chargeCompactionRequest(t.Context(), session.Turn{ID: "new-turn", Type: session.TurnCompaction}); err != nil {
				t.Fatal(err)
			}
			records := compactionRecoveryRecords(t, c.dependencies.Sessions)
			last := records[len(records)-1]
			if last.Request.RequestID == old || last.Request.Attempt != 1 || last.Request.Limit != 3 || last.FailuresBefore != 1 || last.ManualInputID != "latest-manual" {
				t.Fatalf("new manual did not get distinct full pinned budget: %+v", last)
			}
		})
	}
}

func TestCompactionRecoveryUnknownMetadataDoesNotAppend(t *testing.T) {
	c, _ := recordedCompactionHistory(t)
	compactionRecoveryPolicy(t, c, 3)
	before, err := c.dependencies.Sessions.Items(t.Context(), "session-1", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"Mode":"compaction_request","Parameters":{"Version":1,"Request":{"Version":1,"Action":"started","RequestID":"group","TurnID":"summary","TurnType":"compaction","Attempt":1,"Limit":3,"Inputs":2,"ExternalInputs":2},"FailuresBefore":0,"UnknownAuthority":true}}`)
	// Confirm the malformed envelope itself is valid JSON, not a syntax-error test.
	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatal(err)
	}
	if err := c.handleInboxInput(t.Context(), inbox.Input{ID: "unknown", Kind: inbox.InputControl, Payload: payload}); err == nil {
		t.Fatal("accepted unknown causal metadata")
	}
	after, err := c.dependencies.Sessions.Items(t.Context(), "session-1", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unknown metadata was committed")
	}
}

func TestCompactionRecoveryNewManualSupersedesInterruptedGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, dir := recordedCompactionHistory(t)
		compactionRecoveryPolicy(t, c, 3)
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "old-manual", inbox.Compact)})
		turn := session.Turn{ID: "old-summary", PreviousTurnID: "B", Type: session.TurnCompaction}
		if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
			t.Fatal(err)
		}
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
		if err := c.recordRequestAction(t.Context(), "interrupted", "crash"); err != nil {
			t.Fatal(err)
		}
		for _, id := range []inbox.ID{"new-manual", "latest-manual"} {
			appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, id, inbox.Compact)})
		}
		s, err := localfile.New(dir)
		if err != nil {
			t.Fatal(err)
		}
		r, cancel := startRequestRun(t, s)
		assertCompactionTurn(t, r, 1, session.TurnCompaction)
		records := compactionRecoveryRecords(t, s)
		if len(records) != 4 || records[2].Request.Action != "superseded" || records[3].Request.RequestID == records[0].Request.RequestID || records[3].ManualInputID != "latest-manual" || records[3].Request.Attempt != 1 || records[3].Request.Limit != 3 || records[3].FailuresBefore != 1 || r.current.state.compactionFailures != 1 {
			t.Fatalf("new manual recovery records=%+v failures=%d", records, r.current.state.compactionFailures)
		}
		cancel()
		synctest.Wait()
		if err := <-r.done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}

func TestCompactionRecoveryHardStopPreventsRetry(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "started", true: "interrupted"}[interrupted], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c, dir := recordedCompactionHistory(t)
				compactionRecoveryPolicy(t, c, 3)
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "pending", "old work")})
				turn := session.Turn{ID: "old-summary", PreviousTurnID: "B", Type: session.TurnCompaction}
				if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
					t.Fatal(err)
				}
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
				if interrupted {
					if err := c.recordRequestAction(t.Context(), "interrupted", "crash"); err != nil {
						t.Fatal(err)
					}
				}
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "hard-stop", inbox.StopHard)})
				s, err := localfile.New(dir)
				if err != nil {
					t.Fatal(err)
				}
				r, cancel := startRequestRun(t, s)
				if len(r.calls) != 0 || r.current.pendingInputs() != 0 || r.current.state.compactionFailures != 1 {
					t.Fatalf("stopped recovery calls=%d pending=%d failures=%d", len(r.calls), r.current.pendingInputs(), r.current.state.compactionFailures)
				}
				records := compactionRecoveryRecords(t, s)
				if records[len(records)-1].Request.Action != "stopped" {
					t.Fatalf("records=%+v", records)
				}
				cancel()
				synctest.Wait()
				select {
				case err := <-r.done:
					if err != nil && !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				default:
					t.Fatal("run did not drain")
				}
			})
		})
	}
}

func TestCompactionRecoveryConsumedAndInheritedManualOriginsRefused(t *testing.T) {
	for _, boundary := range []string{"consumed", "fork"} {
		t.Run(boundary, func(t *testing.T) {
			c, _ := recordedCompactionHistory(t)
			compactionRecoveryPolicy(t, c, 3)
			appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: stopInput(t, "parent-manual", inbox.Compact)})
			if boundary == "consumed" {
				turn := session.Turn{ID: "completed-summary", PreviousTurnID: "B", Type: session.TurnCompaction}
				if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
					t.Fatal(err)
				}
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
				appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: turn.ID, Response: textResponse("Usable committed summary")}})
			} else {
				if _, err := c.addItemToLocalState(sessionstore.Item{Kind: sessionstore.ItemFork, Data: sessionstore.Fork{ParentID: "parent", PreviousTurnID: "B"}}); err != nil {
					t.Fatal(err)
				}
			}
			before := compactionRecoveryRecords(t, c.dependencies.Sessions)
			record := inbox.CompactionRequestRecord{Version: 1, Request: inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: "fresh-forged", TurnID: "forged-turn", TurnType: "compaction", Attempt: 1, Limit: 3, Inputs: c.state.availableInputs, ExternalInputs: c.state.externalInputs}, FailuresBefore: c.state.compactionFailures, ManualInputID: "parent-manual"}
			if err := c.recordControl(t.Context(), inbox.ControlMessage{Mode: inbox.CompactionRequest, Parameters: record}); err == nil {
				t.Fatal("old consumed/inherited origin authorized a fresh group")
			}
			after := compactionRecoveryRecords(t, c.dependencies.Sessions)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("refusal appended a charge")
			}
		})
	}
}

// Controlled canonical producer: AppendModelResponse commits directly, without
// applying it to the old in-memory builder. This is not a hook claiming the
// live implementation orders adoption after storage (it currently does not).
func TestCompactionRecoveryCommittedCanonicalSummaryBeforeFixtureAdoption(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, _ := recordedCompactionHistory(t)
		compactionRecoveryPolicy(t, c, 3)
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemInput, Data: externalEvent(t, 0, "queued", "pending after summary")})
		turn := session.Turn{ID: "committed-gap", PreviousTurnID: "B", Type: session.TurnCompaction}
		if err := c.chargeCompactionRequest(t.Context(), turn); err != nil {
			t.Fatal(err)
		}
		appendCompactionTestItem(t, c, sessionstore.Item{Kind: sessionstore.ItemTurn, Data: turn})
		response := textResponse("Canonical summary before fixture adoption")
		if err := c.dependencies.Sessions.AppendModelResponse(t.Context(), "session-1", sessionstore.ModelResponse{TurnID: turn.ID, Response: response}); err != nil {
			t.Fatal(err)
		}
		before, err := c.dependencies.ContextBuilder.Build()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(before.Request)
		if strings.Contains(string(raw), "Canonical summary before fixture adoption") {
			t.Fatal("fixture accidentally adopted before loss")
		}
		r, cancel := startRequestRun(t, c.dependencies.Sessions)
		defer cancel()
		assertCompactionTurn(t, r, 1, session.TurnRegular)
		raw, _ = json.Marshal(r.calls[0].request)
		if strings.Count(string(raw), "Canonical summary before fixture adoption") != 1 || !strings.Contains(string(raw), "pending after summary") || len(compactionRecoveryRecords(t, c.dependencies.Sessions)) != 1 {
			t.Fatalf("committed summary did not replay exactly once: %s", raw)
		}
		r.input(t, stopInput(t, "idle", inbox.StopWhenIdle))
		r.respond(t, 0, textResponse("done"))
		r.assertStopped(t)
	})
}
