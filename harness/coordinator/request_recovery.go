package coordinator

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// This is a bound on charged logical attempts, not HTTP transport retries or
// confirmed billable requests. A crash after charge but before dispatch still
// consumes an attempt. The provider may have billed an interrupted request.
const interruptedRequestLimit = 2

type requestAttempt struct {
	mode             inbox.ControlMode
	manualInputID    inbox.ID
	failuresBefore   int
	record           inbox.ModelRequestRecord
	turnSeen         bool
	completed        bool
	stoppedByControl bool
}

type requestLedger struct {
	inheritedTurn session.TurnID
	knownTurns    map[session.TurnID]bool
	latest        *requestAttempt
	byTurn        map[session.TurnID]*requestAttempt
	seenRequests  map[string]bool
}

func requestStateError(message string) error {
	return fmt.Errorf("model request recovery: %s: %w", message, operation.ErrUnsupported)
}

// observeRequestRecord is pure. A start precedes its turn and external dispatch;
// no optional turn field can be silently ignored by an old canonical reader.
func (c *coordinator) observeRequestRecord(r inbox.ModelRequestRecord) error {
	if r.TurnType != string(session.TurnRegular) || r.Limit != interruptedRequestLimit {
		return requestStateError("request type or retry policy is not supported")
	}
	return c.observeRequestAttempt(r, inbox.ModelRequest)
}

func (c *coordinator) observeRequestAttempt(r inbox.ModelRequestRecord, mode inbox.ControlMode) error {
	if err := r.Validate(); err != nil {
		return err
	}
	l := &c.requests
	if l.byTurn == nil {
		l.byTurn = make(map[session.TurnID]*requestAttempt)
		l.seenRequests = make(map[string]bool)
	}
	if l.seenRequests == nil {
		l.seenRequests = make(map[string]bool)
	}
	old := l.latest
	if r.Action == "started" {
		if r.Inputs != c.state.availableInputs || r.ExternalInputs != c.state.externalInputs {
			return requestStateError("start input cursors do not match history")
		}
		if l.knownTurns[session.TurnID(r.TurnID)] {
			return requestStateError("charged identity already belongs to history")
		}
		if _, exists := l.byTurn[session.TurnID(r.TurnID)]; exists {
			return requestStateError("turn identity reused")
		}
		retry := old != nil && old.record.Action == "interrupted" && old.record.RequestID == r.RequestID
		if retry {
			if old.mode != mode || old.completed || r.Attempt != old.record.Attempt+1 || r.Limit != old.record.Limit || r.TurnType != old.record.TurnType {
				return requestStateError("invalid retry identity or attempt")
			}
		} else {
			if old != nil && !old.completed && (old.record.Action == "started" || old.record.Action == "interrupted") {
				return requestStateError("new request overlaps an unsettled request")
			}
			if r.Attempt != 1 || l.seenRequests[r.RequestID] {
				return requestStateError("new request identity or attempt reused")
			}
		}
		a := &requestAttempt{record: r, mode: mode}
		l.latest = a
		l.byTurn[session.TurnID(r.TurnID)] = a
		l.seenRequests[r.RequestID] = true
		return nil
	}
	if old == nil || old.completed || old.mode != mode {
		return requestStateError("outcome has no unfinished attempt")
	}
	expected := old.record
	expected.Action = r.Action
	if r != expected {
		return requestStateError("outcome identity, bound or cursors changed")
	}
	switch r.Action {
	case "interrupted":
		if old.record.Action != "started" || old.stoppedByControl {
			return requestStateError("interruption does not match a live attempt")
		}
	case "exhausted":
		if old.record.Action != "interrupted" || r.Attempt != r.Limit {
			return requestStateError("premature retry exhaustion")
		}
	case "stopped", "superseded":
		if old.record.Action != "started" && old.record.Action != "interrupted" {
			return requestStateError("stop does not match a live attempt")
		}
	}
	old.record = r
	if r.Action == "exhausted" {
		if mode == inbox.ModelRequest {
			c.state.deliveredInputs = max(c.state.deliveredInputs, r.Inputs)
		}
		c.state.callModel = false
	}
	return nil
}

func (c *coordinator) observeRequestTurn(turn session.Turn) error {
	if c.requests.knownTurns == nil {
		c.requests.knownTurns = make(map[session.TurnID]bool)
	}
	c.requests.knownTurns[turn.ID] = true
	c.requests.inheritedTurn = ""
	a := c.requests.latest
	if a != nil && a.record.Action == "started" && !a.completed {
		if a.turnSeen || a.record.TurnID != string(turn.ID) || a.record.TurnType != string(turn.Type) {
			return requestStateError("charged attempt does not match its turn")
		}
		a.turnSeen = true
	} else if a != nil && turn.Type == session.TurnRegular {
		return requestStateError("regular turn has no preceding attempt charge")
	}
	return nil
}

func (c *coordinator) requestResponseInert(id session.TurnID) bool {
	a := c.requests.byTurn[id]
	return a != nil && (a.record.Action != "started" || a.stoppedByControl)
}

func (c *coordinator) recordRequestAction(ctx context.Context, action, reason string) error {
	a := c.requests.latest
	if a == nil || a.completed || (a.record.Action != "started" && a.record.Action != "interrupted") {
		return nil
	}
	r := a.record
	r.Action = action
	if a.mode == inbox.CompactionRequest {
		return c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.CompactionRequest, Reason: reason, Parameters: inbox.CompactionRequestRecord{Version: 1, Request: r, FailuresBefore: a.failuresBefore, ManualInputID: a.manualInputID}})
	}
	return c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.ModelRequest, Reason: reason, Parameters: r})
}

// Recover only declared requests. Completed legacy history remains readable;
// a bare legacy turn cannot establish a retry budget. Compaction requires its
// separate typed policy; uncharged legacy turns remain refused.
func (c *coordinator) prepareRequestRecovery(ctx context.Context) error {
	if c.requests.inheritedTurn != "" && c.requests.inheritedTurn == c.state.currentTurnID {
		return nil
	}
	a := c.requests.latest
	if a != nil && !a.completed && (a.record.Action == "started" || a.record.Action == "interrupted") {
		if a.stoppedByControl {
			if err := c.recordRequestAction(ctx, "stopped", "Stopped request is not automatically retried."); err != nil {
				return err
			}
		}
		if a.record.Action == "started" {
			if err := c.recordRequestAction(ctx, "interrupted", "Runtime ended before a response was committed. Provider outcome and billing may be unknown; partial tools are not execution authority."); err != nil {
				return err
			}
		}
	}
	if err := c.supersedeCompactionForNewManual(ctx); err != nil {
		return err
	}
	a = c.requests.latest
	if a != nil && !a.completed {
		if a.record.Action == "stopped" {
			c.consumeCompactionManualOrigin(a)
			c.state.deliveredInputs = max(c.state.deliveredInputs, a.record.Inputs)
			c.state.callModel = false
		}
		if a.record.Action == "superseded" {
			c.consumeCompactionManualOrigin(a)
			c.state.callModel = a.mode != inbox.CompactionRequest || c.pendingInputs() > 0 || c.state.manualCompaction
		}
	}
	if a != nil && !a.completed && a.mode == inbox.CompactionRequest && a.record.Action == "exhausted" {
		return c.finishCompactionExhaustion(ctx, a)
	}
	if a != nil && !a.completed && a.record.Action == "interrupted" {
		if a.record.Attempt == a.record.Limit {
			if a.mode == inbox.CompactionRequest {
				if err := c.recordRequestAction(ctx, "exhausted", "Compaction attempts exhausted. Original conversation and queued input preserved; no summary invented."); err != nil {
					return err
				}
				return c.finishCompactionExhaustion(ctx, a)
			}
			return c.recordRequestAction(ctx, "exhausted", fmt.Sprintf("Model request %s exhausted its %d charged attempts. No automatic retry; submit new input to continue. Interrupted provider billing may be duplicated.", a.record.RequestID, a.record.Limit))
		}
		c.state.callModel = true
	}
	id := c.state.currentTurnID
	if id != "" {
		if _, responded := c.state.respondedTurns[id]; !responded {
			if c.requests.byTurn[id] != nil {
				return nil
			}
			for _, r := range c.state.abortResults {
				if r.TargetTurnID == string(id) && r.Status == "completed" {
					return nil
				}
			}
			return requestStateError("legacy or compaction turn has no supported interrupted-request policy")
		}
	}
	return nil
}

func (c *coordinator) chargeModelRequest(ctx context.Context, turn session.Turn) error {
	if turn.Type == session.TurnCompaction {
		return c.chargeCompactionRequest(ctx, turn)
	}
	if turn.Type != session.TurnRegular {
		return requestStateError("unknown request kind")
	}
	a := c.requests.latest
	r := inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: uuid.New().String(), TurnID: string(turn.ID), TurnType: string(turn.Type), Attempt: 1, Limit: interruptedRequestLimit, Inputs: c.state.availableInputs, ExternalInputs: c.state.externalInputs}
	if a != nil && !a.completed && a.record.Action == "interrupted" {
		r.RequestID = a.record.RequestID
		r.Attempt = a.record.Attempt + 1
		r.Limit = a.record.Limit
	} else if err := c.recordRequestAction(ctx, "superseded", "Request superseded by newer work; no automatic retry of its partial output."); err != nil {
		return err
	}
	// recordControl commits before applying the pure transition and before any
	// Respond call. Replay independently validates the recorded transition.
	return c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.ModelRequest, Parameters: r})
}

func (c *coordinator) preserveSupersededUsage(ctx context.Context, result modelResponseResult) error {
	if _, seen := c.state.respondedTurns[result.turnID]; seen {
		return nil
	}
	if c.requests.byTurn[result.turnID] == nil || reflect.DeepEqual(result.response.Usage, llm.Usage{}) {
		return nil
	}
	// The stopped/interrupted ledger makes this response inert during replay.
	// Discard late output, retain only actually returned usage, and mark the
	// local interruption explicitly. Never estimate missing provider usage.
	item := sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: result.turnID, Response: llm.Response{Stop: llm.StopInterrupted, Usage: result.response.Usage}}}
	if err := c.storeItemInSessionStore(ctx, item); err != nil {
		return err
	}
	_, err := c.addItemToLocalState(item)
	return err
}

func (c *coordinator) recordTransportUsage(ctx context.Context, result modelResponseResult) error {
	if err := c.recordRequestAction(ctx, "interrupted", "Provider transport ended before completion; retry remains bounded by the committed attempt ledger."); err != nil {
		return err
	}
	if !reflect.DeepEqual(result.response.Usage, llm.Usage{}) {
		return c.preserveSupersededUsage(ctx, result)
	}
	return nil
}

// cloneRequestLedger isolates precommit validation from live cursor/attempt state.
func (l requestLedger) clone() requestLedger {
	out := requestLedger{inheritedTurn: l.inheritedTurn, knownTurns: maps.Clone(l.knownTurns), seenRequests: maps.Clone(l.seenRequests), byTurn: make(map[session.TurnID]*requestAttempt, len(l.byTurn))}
	for id, a := range l.byTurn {
		copy := *a
		out.byTurn[id] = &copy
		if a == l.latest {
			out.latest = &copy
		}
	}
	return out
}
