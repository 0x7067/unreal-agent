package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

var ErrLifecycleRecoveryRequired = sessionstore.ErrLifecycleRecoveryRequired

type foregroundAbortState struct {
	id         inbox.ID
	target     session.TurnID
	operations []operation.ID
}

// foregroundAbortTargets intentionally supports only classified root shell work,
// local model callbacks, and already-reported explicit background Agent branches.
// It refuses unknown/legacy remote ownership and all root TaskGraph histories;
// terminal remote status alone cannot classify a retained execution subtree.
func (c *coordinator) foregroundAbortTargets() ([]operation.ID, error) {
	if c.state.currentTurnType == session.TurnCompaction || c.state.manualCompaction {
		return nil, errors.New("compaction foreground ownership is not supported")
	}
	var targets []operation.ID
	for _, value := range c.state.operations {
		if value.Status == operation.StatusUnsupported {
			return nil, fmt.Errorf("operation %s has unresolved unsupported cleanup", value.ID)
		}
		if value.Type == operation.TypeRemoteJob {
			if policy, ok := c.dependencies.Tools.(RemoteLifecyclePolicy); ok {
				mode, handled, err := policy.ClassifyRemoteForeground(value, string(c.state.currentTurnID))
				if err != nil {
					return nil, err
				}
				if handled {
					switch mode {
					case "background":
						continue
					case "terminal":
						if !operationIsTerminal(value.Status) || policy.RemoteCleanupProof(value) != nil {
							return nil, errors.New("remote terminal cleanup unconfirmed")
						}
						continue
					case "foreground":
						targets = append(targets, value.ID)
						continue
					default:
						return nil, errors.New("invalid remote lifecycle classification")
					}
				}
			}
			state, err := operation.DecodeRemoteJobState(value)
			if err != nil {
				return nil, err
			}
			// Only an explicit background RunPlan proves the entire retained branch is
			// outside this foreground. Message/control plans cannot establish ownership.
			if state.Plan.Type != toolsubagent.RunPlanType || state.Plan.Version != toolsubagent.PlanVersion {
				return nil, errors.New("remote foreground ownership is not classified")
			}
			var plan toolsubagent.RunPlan
			if err := json.Unmarshal(state.Plan.Data, &plan, json.RejectUnknownMembers(true)); err != nil {
				return nil, err
			}
			if plan.Lifetime != "background" || plan.GraphID != "" || !operationIsTerminal(value.Status) {
				return nil, errors.New("only already-reported explicit background branches can be retained")
			}
		}
		if operationIsTerminal(value.Status) {
			continue
		}
		if value.Type != operation.TypeShell {
			return nil, fmt.Errorf("foreground operation %s/%d is not classified", value.Type, value.Version)
		}
		if _, err := operation.DecodeShellState(value); err != nil {
			return nil, err
		}
		targets = append(targets, value.ID)
	}
	for _, call := range c.state.toolCalls {
		if call.status == nil {
			return nil, errors.New("untranslated foreground calls cannot be aborted safely")
		}
	}
	if len(targets) > 256 {
		return nil, errors.New("foreground cleanup proof exceeds bounded receipt window")
	}
	if len(targets) != 0 {
		if _, ok := c.dependencies.Operations.(operation.LocalCleanupVerifier); !ok {
			return nil, errors.New("operation manager has no local cleanup proof capability")
		}
	}
	slices.Sort(targets)
	return targets, nil
}

func (c *coordinator) abortOutcome(ctx context.Context, id inbox.ID, target session.TurnID, status, message string) error {
	if len(message) > 4096 {
		message = message[:4096]
	}
	return c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.ForegroundAbortFinished, Parameters: inbox.ForegroundAbortResult{Version: 1, InputID: id, TargetTurnID: string(target), Status: status, Error: message}})
}

// The request input is durable before this function can request cancellation.
func (c *coordinator) beginForegroundAbort(ctx context.Context, id inbox.ID, request inbox.ForegroundAbort) error {
	target := session.TurnID(request.TargetTurnID)
	if request.Scope != "" && request.Scope != c.dependencies.ControlScope {
		return c.abortOutcome(ctx, id, target, "refused", "runtime ownership scope changed; no work canceled")
	}
	if result, ok := c.state.abortResults[id]; ok && result.Status != "uncertain" {
		return nil
	}
	if c.abort != nil {
		if c.abort.id == id {
			return nil
		}
		return c.abortOutcome(ctx, id, target, "refused", "another foreground abort is draining")
	}
	if target != c.state.currentTurnID {
		return c.abortOutcome(ctx, id, target, "stale", "target is not the current turn; no work canceled")
	}
	if c.stop.request.Mode != "" {
		return c.abortOutcome(ctx, id, target, "refused", "session retirement already requested")
	}
	targets, err := c.foregroundAbortTargets()
	if err != nil {
		return c.abortOutcome(ctx, id, target, "refused", err.Error())
	}
	if err := c.recordRequestAction(ctx, "stopped", "Explicit foreground abort; no automatic retry of aborted request."); err != nil {
		return err
	}
	c.abort = &foregroundAbortState{id: id, target: target, operations: targets}
	c.interruptModel()
	var cancelErr error
	for _, op := range targets {
		if err := c.dependencies.Operations.Cancel(op, "explicit foreground abort"); err != nil {
			cancelErr = errors.Join(cancelErr, fmt.Errorf("%s: %w", op, err))
		}
	}
	if cancelErr != nil {
		// Keep the runtime and background owners alive; only actual completion
		// observations can settle a partially failed cancellation request.
		return c.abortOutcome(ctx, id, target, "uncertain", fmt.Sprintf("cancellation request failed: %v; cleanup unconfirmed", cancelErr))
	}
	return nil
}

func (c *coordinator) discardAbortedResponse(ctx context.Context, result modelResponseResult) error {
	if result.turnID != c.abort.target {
		return nil
	}
	if _, exists := c.state.respondedTurns[result.turnID]; exists {
		return nil
	}
	// A callback returning after cancellation may still contain full or partial
	// tool calls. Never feed that output to translators. Preserve known usage only.
	response := sessionstore.ModelResponse{TurnID: result.turnID, Response: llm.Response{Stop: llm.StopInterrupted, Usage: result.response.Usage, Failure: &llm.Failure{Code: "foreground_aborted", Message: "explicit foreground abort; response output discarded; remote provider billing may remain unknown"}}}
	item, err := c.addItemToLocalState(sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: response})
	if err != nil {
		return err
	}
	return c.storeItemInSessionStore(ctx, item)
}

func (c *coordinator) consumeAbortedTurn() {
	// availableInputs also counts tool results and heartbeats. Suppress those
	// old-task continuations, while preserving every external input accepted
	// after this turn began, irrespective of their interleaving with results.
	newExternal := max(0, c.state.externalInputs-c.state.currentTurnExternalInputs)
	c.state.deliveredInputs = max(c.state.deliveredInputs, c.state.availableInputs-newExternal)
	c.state.callModel = false
	c.state.lastFailure = nil
	c.clearToolGrace()
}

func (c *coordinator) advanceForegroundAbort(ctx context.Context) (bool, error) {
	a := c.abort
	if len(c.modelCalls) != 0 {
		return false, nil
	}
	for _, id := range a.operations {
		value, ok := c.state.operations[id]
		if !ok || !operationIsTerminal(value.Status) || value.Status == operation.StatusUnsupported {
			return false, nil
		}
	}
	if len(a.operations) != 0 {
		if c.abortCleanupProof(a.operations) != nil {
			return false, nil
		}
	}
	// All scoped local callback/effect completions have been observed. Background
	// owner contexts were never canceled. Keep the coordinator alive for new input.
	if err := c.abortOutcome(ctx, a.id, a.target, "completed", ""); err != nil {
		return false, err
	}
	c.abort = nil
	return true, nil
}
