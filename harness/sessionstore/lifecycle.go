package sessionstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

var ErrLifecycleRecoveryRequired = errors.New("unfinished lifecycle intent requires explicit recovery; automatic execution refused")

// LifecycleHistory is pure, ordered metadata for trusted canonical controls.
// It is not effect cleanup proof: only the implementation which joins owners may
// append receipts. Pending intents fence restart even if an old worker died and
// all operation statuses happen to be terminal.
type LifecycleHistory struct {
	RemoteOperations           map[operation.ID]operation.Operation
	RetainedRemoteOperations   map[operation.ID]operation.Operation
	UnclassifiedRemoteSequence Sequence
	LastRemoteSequence         Sequence
	LastDrainSequence          Sequence
	Aborts                     map[inbox.ID]inbox.ForegroundAbort
	AbortResults               map[inbox.ID]inbox.ForegroundAbortResult
	Shutdowns                  map[inbox.ID]inbox.ShutdownSessionRequest
	ShutdownResults            map[inbox.ID]inbox.SessionShutdownResult
}

func (h *LifecycleHistory) Observe(input inbox.Input) error {
	if input.Kind != inbox.InputControl {
		return nil
	}
	request, err := input.DecodeControlMessage()
	if err != nil {
		return err
	}
	switch request.Mode {
	case inbox.AbortForeground:
		v := request.Parameters.(inbox.ForegroundAbort)
		if h.Aborts == nil {
			h.Aborts = map[inbox.ID]inbox.ForegroundAbort{}
		}
		if p, ok := h.Aborts[input.ID]; ok && p != v {
			return fmt.Errorf("conflicting abort identity %s", input.ID)
		}
		h.Aborts[input.ID] = v
	case inbox.ForegroundAbortFinished:
		v := request.Parameters.(inbox.ForegroundAbortResult)
		p, ok := h.Aborts[v.InputID]
		if !ok || p.TargetTurnID != v.TargetTurnID {
			return errors.New("abort outcome has no matching intent/target")
		}
		if h.AbortResults == nil {
			h.AbortResults = map[inbox.ID]inbox.ForegroundAbortResult{}
		}
		if p, ok := h.AbortResults[v.InputID]; ok && p.Status != "uncertain" && p != v {
			return errors.New("conflicting final abort outcome")
		}
		h.AbortResults[v.InputID] = v
	case inbox.ShutdownSession:
		v := request.Parameters.(inbox.ShutdownSessionRequest)
		if h.Shutdowns == nil {
			h.Shutdowns = map[inbox.ID]inbox.ShutdownSessionRequest{}
		}
		if p, ok := h.Shutdowns[input.ID]; ok && p != v {
			return errors.New("conflicting shutdown identity")
		}
		h.Shutdowns[input.ID] = v
	case inbox.SessionShutdownFinished:
		v := request.Parameters.(inbox.SessionShutdownResult)
		p, ok := h.Shutdowns[v.InputID]
		if !ok || p.Scope != v.Scope {
			return errors.New("shutdown outcome has no matching intent/scope")
		}
		if h.ShutdownResults == nil {
			h.ShutdownResults = map[inbox.ID]inbox.SessionShutdownResult{}
		}
		if p, ok := h.ShutdownResults[v.InputID]; ok && (p.Phase == "completed" || p.Phase == "refused") && p != v {
			return errors.New("conflicting final shutdown outcome")
		}
		h.ShutdownResults[v.InputID] = v
	}
	return nil
}
func (h *LifecycleHistory) ValidateResume() error {
	if h.LastRemoteSequence > h.LastDrainSequence {
		return fmt.Errorf("%w: retained remote ownership has no later execution-drain receipt", ErrLifecycleRecoveryRequired)
	}
	return h.validatePendingLifecycle()
}

// ValidateResumeWithOwners grants caller-reconciled per-owner permission, not
// effect cleanup proof. The default ValidateResume remains unchanged/fail-closed.
func (h *LifecycleHistory) ValidateResumeWithOwners(classified map[operation.ID]bool) error {
	if h.LastRemoteSequence > h.LastDrainSequence {
		if h.UnclassifiedRemoteSequence > h.LastDrainSequence || len(h.RetainedRemoteOperations) == 0 {
			return fmt.Errorf("%w: unclassified retained ownership", ErrLifecycleRecoveryRequired)
		}
		for id := range h.RetainedRemoteOperations {
			if !classified[id] {
				return fmt.Errorf("%w: retained owner %s", ErrLifecycleRecoveryRequired, id)
			}
		}
	}
	return h.validatePendingLifecycle()
}
func (h *LifecycleHistory) validatePendingLifecycle() error {
	for id := range h.Aborts {
		v, ok := h.AbortResults[id]
		if !ok || v.Status == "uncertain" {
			return fmt.Errorf("%w: abort %s", ErrLifecycleRecoveryRequired, id)
		}
	}
	for id := range h.Shutdowns {
		v, ok := h.ShutdownResults[id]
		if !ok || v.Phase != "completed" && v.Phase != "refused" {
			return fmt.Errorf("%w: shutdown %s", ErrLifecycleRecoveryRequired, id)
		}
	}
	return nil
}

type lifecycleItemsReader interface {
	Items(context.Context, session.ID, Sequence, int) (Page, error)
}

// ReadLifecycle must be called under the session writer lease on startup, before
// provider/tool initialization. Read-only diagnostics may also use it with a
// bounded context; no receipt is invented from EOF or operation terminal flags.
func ReadLifecycle(ctx context.Context, store lifecycleItemsReader, id session.ID) (LifecycleHistory, error) {
	var history LifecycleHistory
	after := BeforeFirst
	for {
		page, err := store.Items(ctx, id, after, 512)
		if err != nil {
			return history, err
		}
		for _, item := range page.Items {
			if item.Kind == ItemToolCallStatus {
				status, ok := item.Data.(ToolCallStatus)
				if !ok {
					return history, errors.New("invalid typed canonical tool status")
				}
				for _, op := range status.Operations {
					if op.Type == operation.TypeRemoteJob {
						if history.RemoteOperations == nil {
							history.RemoteOperations = make(map[operation.ID]operation.Operation)
						}
						history.RemoteOperations[op.ID] = op
						if history.RetainedRemoteOperations == nil {
							history.RetainedRemoteOperations = make(map[operation.ID]operation.Operation)
						}
						history.RetainedRemoteOperations[op.ID] = op
						history.LastRemoteSequence = max(history.LastRemoteSequence, item.Sequence)
					}
				}
			}
			if item.Kind == ItemFork {
				history = LifecycleHistory{}
				continue
			}
			if item.Kind == ItemInput {
				input, ok := item.Data.(inbox.Input)
				if !ok {
					return history, errors.New("invalid typed canonical input")
				}
				if err := history.Observe(input); err != nil {
					return history, err
				}
				if input.Kind == inbox.InputControl {
					control, err := input.DecodeControlMessage()
					if err != nil {
						return history, err
					}
					switch control.Mode {
					case inbox.ExecutionDrained:
						history.LastDrainSequence = item.Sequence
						clear(history.RetainedRemoteOperations)
					case inbox.InvokeTool:
						history.UnclassifiedRemoteSequence = item.Sequence
						history.LastRemoteSequence = max(history.LastRemoteSequence, item.Sequence)
					}
				}
			}
		}
		if !page.More {
			return history, nil
		}
		if page.NextAfter <= after {
			return history, errors.New("non-advancing lifecycle history page")
		}
		after = page.NextAfter
	}
}
