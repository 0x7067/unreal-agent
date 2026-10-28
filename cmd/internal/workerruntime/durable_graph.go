package workerruntime

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"path/filepath"
)

func graphOwnerScope(b workerproto.Bootstrap) string {
	h := sha256.Sum256([]byte(b.ControlScope))
	return fmt.Sprintf("graph-%x", h[:])
}

// The coordinator remains the sole writer. This decorator only notifies the
// graph actor after its exact SaveOperation has returned durable success.
type committedCheckpointStore struct {
	sessionstore.Store
	owners *ownedExecution
}

func (s committedCheckpointStore) SaveOperation(ctx context.Context, id session.ID, op operation.Operation) error {
	if err := s.Store.SaveOperation(ctx, id, op); err != nil {
		return err
	}
	if s.owners.timers != nil && op.Type == operation.TypeRemoteJob && op.Status == operation.StatusAwaiting {
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			return err
		}
		if state.Plan.Type == reminder.TaskType {
			if err := s.owners.timers.NotifyCheckpointCommitted(op.ID, op.State); err != nil {
				return err
			}
		}
	}
	for _, r := range s.owners.runtimes {
		_ = r.NotifyCheckpointCommitted(op)
	}
	return nil
}

// Preserve the store's optional structural-control commit boundary. Embedding
// the narrow Store interface alone would hide this capability from coordinator.
func (s committedCheckpointStore) InitializeControlOperation(ctx context.Context, id session.ID, input inbox.ID, op operation.Operation) error {
	initializer, ok := s.Store.(sessionstore.ControlOperationInitializer)
	if !ok {
		return errors.New("session store does not support structural control initialization")
	}
	return initializer.InitializeControlOperation(ctx, id, input, op)
}

var _ sessionstore.ControlOperationInitializer = committedCheckpointStore{}

func validateDurableOwnership(ctx context.Context, b workerproto.Bootstrap, h sessionstore.LifecycleHistory, latest []operation.Operation) error {
	if !b.DurableGraphs {
		return validateChildOwnership(ctx, b, h, latest)
	}
	if h.ValidateResume() == nil {
		return nil
	}
	allowed := map[operation.ID]bool{}
	ops := map[operation.ID]operation.Operation{}
	for id, op := range h.RetainedRemoteOperations {
		ops[id] = op
		allowed[id] = true
	}
	if err := h.ValidateResumeWithOwners(allowed); err != nil {
		return err
	}
	for _, op := range latest {
		if _, ok := ops[op.ID]; ok {
			ops[op.ID] = op
		}
	}
	children := map[operation.ID]operation.Operation{}
	for id, op := range ops {
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			return err
		}
		if state.Plan.Version == taskgraph.DurablePlanVersion && state.Plan.Type == taskgraph.RunPlanType {
			if err := validateGraphExecution(b, op); err != nil {
				return err
			}
			if err := taskgraph.ValidateDurableRecovery(op, b.SessionID); err != nil {
				return fmt.Errorf("graph owner %s requires reconciliation: %w", id, err)
			}
			continue
		}
		if state.Plan.Version == taskgraph.DurablePlanVersion && state.Plan.Type == taskgraph.ControlPlanType {
			p, err := taskgraph.DecodeDurableControl(op)
			if err != nil || p.ParentID != b.SessionID || op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed && op.Status != operation.StatusCanceled {
				return errors.New("unfinished/stale graph control requires explicit recovery")
			}
			continue
		}
		children[id] = op
	}
	if len(children) > 0 {
		subset := h
		subset.RetainedRemoteOperations = children
		if err := validateChildOwnership(ctx, b, subset, latest); err != nil {
			return err
		}
	}
	return h.ValidateResumeWithOwners(allowed)
}

func validateGraphExecution(b workerproto.Bootstrap, op operation.Operation) error {
	_, cp, err := taskgraph.DecodeDurableRun(op)
	if err != nil || cp == nil {
		return err
	}
	workspace, err := filepath.EvalSymlinks(b.Workspace)
	if err != nil {
		return err
	}
	shell := b.Shell
	if shell == "" {
		shell = "/bin/sh"
	}
	expected := taskgraph.DurableExecution{Workspace: workspace, Shell: shell, BaseDirectory: filepath.Join(b.Directory, "operations", string(b.SessionID))}
	if cp.Execution != expected {
		return errors.New("durable graph execution configuration conflict")
	}
	return nil
}
