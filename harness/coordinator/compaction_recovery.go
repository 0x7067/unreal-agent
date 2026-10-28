package coordinator

import (
	"context"
	"fmt"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

// Summary attempts share exclusion/turn/callback authority with regular requests,
// but have a distinct canonical schema, manual origin and failure-budget policy.
func (c *coordinator) observeCompactionRequest(value inbox.CompactionRequestRecord) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if c.state.compactionFailureLimit <= 0 {
		return requestStateError("compaction charge has no persisted failure policy")
	}
	r := value.Request
	old := c.requests.latest
	retry := r.Action == "started" && old != nil && !old.completed && old.record.Action == "interrupted" && old.record.RequestID == r.RequestID
	if r.Action == "started" {
		if retry {
			if old.mode != inbox.CompactionRequest || value.FailuresBefore != old.failuresBefore || value.ManualInputID != old.manualInputID || c.state.compactionFailures != old.failuresBefore+old.record.Attempt {
				return requestStateError("compaction retry changed its origin or failure budget")
			}
		} else {
			if value.FailuresBefore != c.state.compactionFailures {
				return requestStateError("compaction start failure cursor differs from history")
			}
			limit := c.compactionFailureLimit()
			if value.ManualInputID != "" {
				if !c.state.manualCompaction || c.state.manualCompactionInput != value.ManualInputID {
					return requestStateError("manual compaction has no matching pending owned input")
				}
			} else {
				if c.state.compactionDisabled || c.state.manualCompaction {
					return requestStateError("automatic compaction is disabled or misclassified")
				}
				limit -= c.state.compactionFailures
			}
			if limit <= 0 || limit > 100000 || r.Limit != limit {
				return requestStateError("compaction start replenishes or exceeds the failure budget")
			}
		}
	} else if old == nil || old.mode != inbox.CompactionRequest || old.manualInputID != value.ManualInputID || old.failuresBefore != value.FailuresBefore {
		return requestStateError("compaction outcome changed its origin or failure budget")
	}
	wasStarted := old != nil && !old.completed && old.record.Action == "started"
	if err := c.observeRequestAttempt(r, inbox.CompactionRequest); err != nil {
		return err
	}
	if r.Action == "started" {
		a := c.requests.latest
		a.manualInputID = value.ManualInputID
		a.failuresBefore = value.FailuresBefore
	} else if wasStarted && (r.Action == "interrupted" || r.Action == "stopped" || r.Action == "superseded") {
		c.state.compactionFailures++
	}
	return nil
}

func (c *coordinator) consumeCompactionManualOrigin(a *requestAttempt) {
	if a.mode == inbox.CompactionRequest && a.manualInputID != "" && c.state.manualCompactionInput == a.manualInputID {
		c.state.manualCompaction = false
		c.state.manualCompactionInput = ""
	}
}

func (c *coordinator) supersedeCompactionForNewManual(ctx context.Context) error {
	a := c.requests.latest
	if a == nil || a.completed || a.mode != inbox.CompactionRequest || a.record.Action != "interrupted" || !c.state.manualCompaction || c.state.manualCompactionInput == "" || c.state.manualCompactionInput == a.manualInputID {
		return nil
	}
	return c.recordRequestAction(ctx, "superseded", "A newer committed manual compaction input authorizes a distinct bounded request; the old attempt group is not retried.")
}

func (c *coordinator) finishCompactionExhaustion(ctx context.Context, a *requestAttempt) error {
	c.consumeCompactionManualOrigin(a)
	if err := c.disableUnusableCompaction(ctx); err != nil {
		return err
	}
	c.state.callModel = false // processEvents separately discovers preserved ordinary/new manual work.
	return nil
}

func (c *coordinator) chargeCompactionRequest(ctx context.Context, turn session.Turn) error {
	a := c.requests.latest
	r := inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: uuid.New().String(), TurnID: string(turn.ID), TurnType: string(session.TurnCompaction), Attempt: 1, Limit: c.compactionFailureLimit(), Inputs: c.state.availableInputs, ExternalInputs: c.state.externalInputs}
	value := inbox.CompactionRequestRecord{Version: 1, Request: r, FailuresBefore: c.state.compactionFailures}
	if a != nil && !a.completed && a.mode == inbox.CompactionRequest && a.record.Action == "interrupted" {
		value.Request.RequestID = a.record.RequestID
		value.Request.Attempt = a.record.Attempt + 1
		value.Request.Limit = a.record.Limit
		value.FailuresBefore = a.failuresBefore
		value.ManualInputID = a.manualInputID
	} else if c.state.manualCompaction {
		value.ManualInputID = c.state.manualCompactionInput
	} else {
		value.Request.Limit -= c.state.compactionFailures
	}
	if err := value.Validate(); err != nil {
		return fmt.Errorf("compaction charge: %w", err)
	}
	return c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.CompactionRequest, Parameters: value})
}
