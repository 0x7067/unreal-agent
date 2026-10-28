// Package lifecycleproof reconciles manager uncertainty with independently
// established owner joins. It performs no joins, I/O, or operation translation.
package lifecycleproof

import (
	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Reconcile can discharge only a direct, nonempty CleanupUncertainError containing
// exclusively unproven remote IDs. Unknown outcomes and unsupported operations
// remain uncertain regardless of terminal status or forwarding-worker joins.
//
// The caller MUST populate joinedHandlers only after actual graph/supervisor/hub
// joins succeeded and every child supplied fresh required cleanup receipts. An
// empty map grants no authority. Entries attest exact plan type/version pairs,
// not arbitrary registered handlers; only current managed kinds are accepted.
// canonical must contain the original canonical operations, not reconstructed
// or status-derived substitutes. Inputs must not be mutated during this call.
//
// Wrappers and joined manager errors are conservatively retained whole: examining
// an errors.As match alone could discard independent failure evidence. No input
// maps, operation states, error structures, or evidence slices are modified.
func Reconcile(managerErr, ownersErr error, canonical map[operation.ID]operation.Operation, joinedHandlers map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion) error {
	if ownersErr == nil {
		if evidence, ok := managerErr.(*operation.CleanupUncertainError); ok && proven(evidence, canonical, joinedHandlers) {
			return nil
		}
		if managerErr == nil {
			return nil
		}
		return &uncertain{causes: []error{managerErr}}
	}
	if managerErr == nil {
		return &uncertain{causes: []error{ownersErr}}
	}
	return &uncertain{causes: []error{managerErr, ownersErr}}
}

// Error deliberately does not render causes: IDs and underlying diagnostics can
// be arbitrarily large. Unwrap preserves all original structured evidence.
type uncertain struct{ causes []error }

func (*uncertain) Error() string     { return "lifecycle cleanup proof is incomplete" }
func (e *uncertain) Unwrap() []error { return append([]error(nil), e.causes...) }

func proven(e *operation.CleanupUncertainError, canonical map[operation.ID]operation.Operation, joined map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion) bool {
	if e == nil || len(e.UnprovenRemoteIDs) == 0 || len(e.UnknownRemoteIDs) != 0 || len(e.UnsupportedOperationIDs) != 0 {
		return false
	}
	for _, id := range e.UnprovenRemoteIDs {
		op, ok := canonical[id]
		if !ok || id == "" || op.ID != id || op.Type != operation.TypeRemoteJob || op.Status == operation.StatusUnsupported {
			return false
		}
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			return false
		}
		var version operation.RemoteJobPlanVersion
		switch state.Plan.Type {
		case extension.PlanType:
			if op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed && op.Status != operation.StatusCanceled {
				return false
			}
			if extension.ValidateRecovery(op, nil) != nil {
				return false
			}
			version = extension.PlanVersion
		case reminder.TaskType:
			if !reminder.TaskTerminalProof(op) {
				return false
			}
			version = reminder.TaskVersion
		case durablejob.PlanType:
			if durablejob.TerminalSelfProof(op) != nil {
				return false
			}
			version = durablejob.Version
		case toolsubagent.RunPlanType, toolsubagent.MessagePlanType, toolsubagent.ControlPlanType:
			version = toolsubagent.PlanVersion
		case taskgraph.RunPlanType, taskgraph.ControlPlanType:
			version = taskgraph.PlanVersion
			if state.Plan.Version == taskgraph.DurablePlanVersion {
				if state.Plan.Type == taskgraph.RunPlanType {
					if _, _, err := taskgraph.DecodeDurableRun(op); err != nil {
						return false
					}
				} else {
					if _, err := taskgraph.DecodeDurableControl(op); err != nil {
						return false
					}
				}
				version = taskgraph.DurablePlanVersion
			}
		default:
			return false
		}
		got, ok := joined[state.Plan.Type]
		if !ok || got != version || state.Plan.Version != version {
			return false
		}
	}
	return true
}
