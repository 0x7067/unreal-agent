package workerruntime

import (
	"context"
	"errors"
	"fmt"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

type workloadConfiguration struct {
	jobs       *trustedJobs
	extensions *configuredExtensions
	timers     bool
}

// validateAuxiliary returns handled only for exact explicitly configured types.
// No execution or persistence occurs in any recovery/ownership classifier.
func validateAuxiliary(op operation.Operation, cfg workloadConfiguration, parent session.ID) (bool, error) {
	if op.Type != operation.TypeRemoteJob {
		return false, nil
	}
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return false, fmt.Errorf("unsupported or invalid remote envelope: %w", err)
	}
	switch state.Plan.Type {
	case extension.PlanType:
		if cfg.extensions == nil {
			return true, errors.New("trusted extension configuration unavailable")
		}
		return true, extension.ValidateRecovery(op, cfg.extensions.registry)
	case reminder.TaskType:
		if !cfg.timers {
			return true, errors.New("durable timer capability unavailable")
		}
		p, _, err := reminder.DecodeTask(op)
		if err != nil {
			return true, err
		}
		if p.ParentID != string(parent) {
			return true, errors.New("foreign timer parent")
		}
		if terminalOperation(op) {
			if !reminder.TaskTerminalProof(op) {
				return true, errors.New("timer terminal checkpoint invalid")
			}
			return true, nil
		}
		return true, reminder.ValidateTaskRecovery(op)
	}
	return false, nil
}
func validateWorkloadRecovery(ops []operation.Operation, cfg workloadConfiguration, parent session.ID) error {
	var other []operation.Operation
	for _, op := range ops {
		if terminalOperation(op) {
			other = append(other, op)
			continue
		}
		handled, err := validateAuxiliary(op, cfg, parent)
		if err != nil {
			return fmt.Errorf("workload recovery refused %s: %w", op.ID, err)
		}
		if !handled {
			other = append(other, op)
		}
	}
	return validateWorkerRecovery(other, cfg.jobs, parent)
}
func validateWorkloadOwnership(ctx context.Context, b workerproto.Bootstrap, h sessionstore.LifecycleHistory, latest []operation.Operation, cfg workloadConfiguration) error {
	allowed := map[operation.ID]bool{}
	remaining := map[operation.ID]operation.Operation{}
	newest := map[operation.ID]operation.Operation{}
	for id, op := range h.RemoteOperations {
		newest[id] = op
	}
	for _, op := range latest {
		newest[op.ID] = op
	}
	for id, op := range h.RetainedRemoteOperations {
		if current, ok := newest[id]; ok {
			op = current
		}
		handled, err := validateAuxiliary(op, cfg, b.SessionID)
		if err != nil {
			return fmt.Errorf("workload owner %s requires reconciliation: %w", id, err)
		}
		if handled {
			allowed[id] = true
		} else {
			remaining[id] = op
		}
	}
	if len(allowed) == 0 {
		return validateConfiguredOwnership(ctx, b, h, latest, cfg.jobs)
	}
	if len(remaining) > 0 {
		subset := h
		subset.RetainedRemoteOperations = remaining
		if err := validateConfiguredOwnership(ctx, b, subset, latest, cfg.jobs); err != nil {
			return err
		}
		for id := range remaining {
			allowed[id] = true
		}
	}
	return h.ValidateResumeWithOwners(allowed)
}

// validateNativeBeforeConfiguration is a staging fence, not recovery authority.
// Exact M4 envelopes are checked AGAIN against configured authority and canonical
// ownership below; all other unfinished native operations retain their original
// refusal before even trusted startup environment lookup.
func validateNativeBeforeConfiguration(ops []operation.Operation, parent session.ID) error {
	native := make([]operation.Operation, 0, len(ops))
	for _, op := range ops {
		if op.Type == operation.TypeRemoteJob {
			state, err := operation.DecodeRemoteJobState(op)
			if err != nil {
				return fmt.Errorf("unsupported or invalid remote envelope: %w", err)
			}
			switch state.Plan.Type {
			case extension.PlanType, reminder.TaskType, durablejob.PlanType:
				if state.Plan.Version != 1 {
					return fmt.Errorf("unsupported trusted workload plan version %d", state.Plan.Version)
				}
				continue
			}
		}
		native = append(native, op)
	}
	return validateWorkerRecovery(native, nil, parent)
}
