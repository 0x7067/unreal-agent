package workerruntime

import (
	"context"
	"encoding/json/v2"
	"errors"
	"sort"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/lifecycleproof"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

type runtimeCleanupEvidence struct {
	extensionsJoined, timersJoined      bool
	jobsJoined                          bool
	managerSeen, ownersSeen, modelsSeen bool
	managerErr, ownersErr, modelsErr    error
}

// Called only after the existing model seal/join + client.Close defer, the
// coordinator join, all local effect completions and strict owner joins, while
// observer and session lease remain held. No EOF/status substitution is allowed.
func finalizeRuntimeLifecycle(store sessionstore.Store, b workerproto.Bootstrap, proof *runtimeCleanupEvidence) error {
	if proof == nil || !proof.managerSeen || !proof.ownersSeen || !proof.modelsSeen {
		return errors.New("runtime cleanup proof unavailable")
	}
	if proof.modelsErr != nil {
		return errors.New("model cleanup proof unavailable")
	}
	ctx := context.Background()
	history, err := sessionstore.ReadLifecycle(ctx, store, b.SessionID)
	if err != nil {
		return err
	}
	// Structural control specs are persisted outside ToolCallStatus records. The
	// resume view retains terminal control operations missing ordinary tool history.
	resumed, err := store.Resume(ctx, b.SessionID)
	if err != nil {
		return err
	}
	if history.RemoteOperations == nil {
		history.RemoteOperations = map[operation.ID]operation.Operation{}
	}
	for _, op := range resumed.Operations {
		if op.Type == operation.TypeRemoteJob {
			history.RemoteOperations[op.ID] = op
		}
	}
	if reader, ok := store.(sessionstore.ControlOperationReader); ok {
		controls, err := reader.ControlOperations(ctx, b.SessionID)
		if err != nil {
			return err
		}
		for _, op := range controls {
			history.RemoteOperations[op.ID] = op
		}
	}
	// setupOperations with a nonempty ControlScope configures fresh proof on every
	// supervisor, including dynamic graph factories. ownedExecution.wait joins all
	// runtimes before snapshotting those supervisors, and joins the Hub workers.
	joined := map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{
		taskgraph.RunPlanType: taskgraph.PlanVersion, taskgraph.ControlPlanType: taskgraph.PlanVersion,
	}
	if b.DurableGraphs {
		joined[taskgraph.RunPlanType] = taskgraph.DurablePlanVersion
		joined[taskgraph.ControlPlanType] = taskgraph.DurablePlanVersion
	}
	if proof.extensionsJoined {
		joined[extension.PlanType] = extension.PlanVersion
	}
	if proof.timersJoined {
		joined[reminder.TaskType] = reminder.TaskVersion
	}
	if proof.jobsJoined {
		joined[durablejob.PlanType] = durablejob.Version
	}
	if b.Subagents {
		joined[toolsubagent.RunPlanType] = toolsubagent.PlanVersion
		joined[toolsubagent.MessagePlanType] = toolsubagent.PlanVersion
		joined[toolsubagent.ControlPlanType] = toolsubagent.PlanVersion
	}
	if err := lifecycleproof.Reconcile(proof.managerErr, proof.ownersErr, history.RemoteOperations, joined); err != nil {
		return err
	}
	var shutdowns []string
	for id, request := range history.Shutdowns {
		outcome, ok := history.ShutdownResults[id]
		if request.Scope == b.ControlScope && (!ok || outcome.Phase == "uncertain") {
			shutdowns = append(shutdowns, string(id))
		}
	}
	sort.Strings(shutdowns)
	appendControl := func(mode inbox.ControlMode, parameters any) error {
		payload, err := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: parameters})
		if err != nil {
			return err
		}
		return store.AppendInput(ctx, b.SessionID, inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload})
	}
	if len(shutdowns) > 0 {
		// An explicit whole-runtime shutdown supersedes pending selective aborts only
		// after independent evidence proves every known owner has joined.
		var aborts []string
		for id, request := range history.Aborts {
			result, ok := history.AbortResults[id]
			if (request.Scope == "" || request.Scope == b.ControlScope) && (!ok || result.Status == "uncertain") {
				aborts = append(aborts, string(id))
			}
		}
		sort.Strings(aborts)
		for _, id := range aborts {
			request := history.Aborts[inbox.ID(id)]
			if err := appendControl(inbox.ForegroundAbortFinished, inbox.ForegroundAbortResult{Version: 1, InputID: inbox.ID(id), TargetTurnID: request.TargetTurnID, Status: "completed", Error: "superseded by explicit session shutdown after all owned work drained"}); err != nil {
				return err
			}
		}
	}
	for _, id := range shutdowns {
		if err := appendControl(inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: inbox.ID(id), Scope: b.ControlScope, Phase: "work_drained"}); err != nil {
			return err
		}
	}
	return appendControl(inbox.ExecutionDrained, inbox.ExecutionDrainReceipt{Version: 1, Scope: b.ControlScope})
}
