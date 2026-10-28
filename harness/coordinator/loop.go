package coordinator

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const historyPageSize = 256

const (
	slurpIdleTimeout = time.Millisecond
	slurpMaxItems    = 100
)

const (
	toolCallRunGracePeriod        = time.Second
	toolCallCompletionGracePeriod = time.Second
)

type coordinator struct {
	dependencies Dependencies
	state        loopState
	stop         stopState
	cancelModel  context.CancelFunc
	modelCalls   map[session.TurnID]struct{} // callback ownership, including superseded requests
	abort        *foregroundAbortState
	requests     requestLedger
}

type stopState struct {
	request               inbox.ControlMessage
	cancellationRequested bool // Once set, no new nonterminal operations may enter coordinator state.
}

type loopState struct {
	lifecycle         sessionstore.LifecycleHistory
	abortRequests     map[inbox.ID]inbox.ForegroundAbort
	abortResults      map[inbox.ID]inbox.ForegroundAbortResult
	controlInputs     map[inbox.ID]inbox.ToolInvocation
	controlCompleted  map[inbox.ID]bool
	controlOperations map[operation.ID]structuralControl

	failedTurns               map[session.TurnID]struct{}
	rejectedOperations        map[operation.ID]struct{}
	respondedTurns            map[session.TurnID]struct{}
	externalInputs            int
	currentTurnExternalInputs int
	lastFailure               *ModelFailureError
	compactionFailures        int
	compactionFailureLimit    int
	compactionDisabled        bool
	manualCompaction          bool
	manualCompactionInput     inbox.ID
	currentTurnID             session.TurnID
	currentTurnType           session.TurnType
	toolCalls                 map[toolCallKey]toolCallState
	operations                map[operation.ID]operation.Operation
	availableInputs           int
	deliveredInputs           int
	currentTurnInputs         int
	callModel                 bool
	grace                     <-chan time.Time
	graceToolCalls            map[toolCallKey]struct{}
}

type toolCallState struct {
	toolCall   llm.ToolCall
	status     *tool.CallStatus
	operations map[operation.ID]struct{}
}

type toolCallKey struct {
	turnID session.TurnID
	callID string
}

type toolCallContext struct {
	turnID     session.TurnID
	now        time.Time
	operations []operation.Operation
}

type modelResponseResult struct {
	turnID   session.TurnID
	response llm.Response
	err      error
}

var _ Coordinator = (*coordinator)(nil)

func newLoopState() loopState {
	return loopState{
		controlInputs:      make(map[inbox.ID]inbox.ToolInvocation),
		controlCompleted:   make(map[inbox.ID]bool),
		controlOperations:  make(map[operation.ID]structuralControl),
		respondedTurns:     make(map[session.TurnID]struct{}),
		failedTurns:        make(map[session.TurnID]struct{}),
		rejectedOperations: make(map[operation.ID]struct{}),
		toolCalls:          make(map[toolCallKey]toolCallState),
		operations:         make(map[operation.ID]operation.Operation),
		graceToolCalls:     make(map[toolCallKey]struct{}),
	}
}

func (current *coordinator) Run(ctx context.Context) error {
	if current.dependencies.CompactionFailureLimit < 0 {
		return fmt.Errorf("compaction failure limit must not be negative")
	}
	if current.dependencies.ToolHeartbeatInterval < 0 {
		return fmt.Errorf("tool heartbeat interval must not be negative")
	}
	if err := current.restore(ctx); err != nil {
		return err
	}
	// Recovery of an admitted but unfinished lifecycle intent is a separate policy.
	// Never silently redispatch it or restart its interrupted model/tools.
	if err := current.state.lifecycle.ValidateResume(); err != nil {
		return err
	}

	if err := current.prepareRequestRecovery(ctx); err != nil {
		return err
	}

	// Accept queued new user input before surfacing a restored failure. Do not
	// schedule or dispatch any recorded tool work merely to retry a failed turn.
	if current.state.lastFailure != nil {
		inputs, err := slurpChannel(ctx, current.dependencies.Inbox.Output())
		if err != nil {
			return err
		}
		if err := current.processInputs(ctx, inputs); err != nil {
			return err
		}
		if current.state.lastFailure != nil {
			return current.state.lastFailure
		}
	}

	modelContext, cancelModels := context.WithCancel(ctx)
	defer cancelModels()
	defer current.interruptModel()
	modelResponses := make(chan modelResponseResult)

	inboxOutput := current.dependencies.Inbox.Output()
	operationUpdates := current.dependencies.Operations.Updates()
	statuses, err := current.scheduleToolCalls(ctx)
	if err != nil {
		return err
	}
	if err := current.dispatchOperationsToManager(); err != nil {
		return err
	}
	current.state.callModel = current.state.callModel || toolCallStatusesRequireModelResponse(statuses)

	var heartbeat <-chan time.Time
	for {
		callModel, err := current.processEvents(ctx)
		if err != nil {
			return err
		}
		if current.stop.request.Mode == inbox.StopHard {
			stopped, err := current.handleStop()
			if err != nil {
				return err
			}
			if stopped {
				return ctx.Err()
			}
		} else if current.abort != nil {
			done, err := current.advanceForegroundAbort(ctx)
			if err != nil {
				return err
			}
			if done {
				continue
			}
		} else if callModel {
			if err := current.requestModelResponse(modelContext, modelResponses); err != nil {
				return err
			}
			current.clearToolGrace()
		}
		if current.stop.request.Mode == inbox.StopWhenIdle && current.isIdle() &&
			!current.state.callModel && !current.state.manualCompaction && current.state.grace == nil &&
			len(current.state.graceToolCalls) == 0 &&
			(current.dependencies.PendingWork == nil || !current.dependencies.PendingWork()) {
			// A producer may have completed admission after processEvents slurped
			// input but before releasing its owner. Check the accepted queue AFTER
			// owner state, and slurp again rather than sleeping over queued work.
			if current.dependencies.Inbox.HasPending() {
				continue
			}
			return ctx.Err()
		}

		if !current.isWaitingForOnlyToolCalls() {
			heartbeat = nil
		} else if heartbeat == nil && current.dependencies.ToolHeartbeatInterval > 0 {
			heartbeat = time.After(current.dependencies.ToolHeartbeatInterval)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()

		case received, open := <-inboxOutput:
			if !open {
				return closedInputError(ctx, "inbox output")
			}
			if err := current.processInputs(ctx, []inbox.Input{received}); err != nil {
				return err
			}

		case received, open := <-operationUpdates:
			if !open {
				return closedInputError(ctx, "operation updates")
			}
			if err := current.processOperations(ctx, []operation.Operation{received}); err != nil {
				return err
			}

		case <-current.dependencies.WorkChanged:
			// Recheck authoritative owners and consume their queued completion input.

		case <-heartbeat:
			if err := current.postHeartbeat(ctx); err != nil {
				return err
			}

		case <-current.state.grace:
			current.clearToolGrace()

		case received := <-modelResponses:
			delete(current.modelCalls, received.turnID)
			if current.abort != nil {
				if err := current.discardAbortedResponse(ctx, received); err != nil {
					return err
				}
				continue
			}
			if current.cancelModel == nil || received.turnID != current.state.currentTurnID {
				if err := current.preserveSupersededUsage(ctx, received); err != nil {
					return err
				}
				continue
			}
			if err := current.processModelResponse(ctx, received); err != nil {
				return err
			}
		}
	}
}

func (current *coordinator) processEvents(ctx context.Context) (bool, error) {
	inputs, err := slurpChannel(ctx, current.dependencies.Inbox.Output())
	if err != nil {
		return false, fmt.Errorf("slurp inbox: %w", err)
	}
	if err := current.processInputs(ctx, inputs); err != nil {
		return false, err
	}
	updates, err := slurpChannel(ctx, current.dependencies.Operations.Updates())
	if err != nil {
		return false, fmt.Errorf("slurp operation updates: %w", err)
	}
	if err := current.processOperations(ctx, updates); err != nil {
		return false, err
	}

	completed, err := current.reconcileToolCalls(ctx)
	if err != nil {
		return false, err
	}
	if len(completed) > 0 {
		hasPendingSiblings := false
		for _, status := range completed {
			for key := range current.state.toolCalls {
				if key.turnID == status.TurnID {
					current.state.graceToolCalls[key] = struct{}{}
					hasPendingSiblings = true
				}
			}
		}
		if hasPendingSiblings {
			current.state.grace = time.After(toolCallCompletionGracePeriod)
		}
	}
	if current.state.manualCompaction && current.cancelModel == nil {
		if _, err := current.dependencies.ContextBuilder.BuildCompaction(); err != nil {
			current.state.manualCompaction = false
			current.state.manualCompactionInput = ""
		}
	}
	return (current.state.manualCompaction && current.cancelModel == nil) || current.state.callModel || (current.pendingInputs() > 0 && current.cancelModel == nil && len(current.state.graceToolCalls) == 0), nil
}

func (current *coordinator) processInputs(ctx context.Context, inputs []inbox.Input) error {
	return current.handleInboxInputs(ctx, inputs)
}

func (current *coordinator) processOperations(ctx context.Context, updates []operation.Operation) error {
	for _, update := range updates {
		if err := current.handleOperationUpdate(ctx, update); err != nil {
			return err
		}
	}
	return nil
}

func (current *coordinator) processModelResponse(ctx context.Context, modelResponse modelResponseResult) error {
	if _, duplicate := current.state.respondedTurns[modelResponse.turnID]; duplicate {
		return nil
	}
	current.interruptModel()
	if modelResponse.err != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := current.recordTransportUsage(ctx, modelResponse); err != nil {
			return err
		}
		return fmt.Errorf("call model for turn %q: %w", modelResponse.turnID, modelResponse.err)
	}
	statuses, err := current.handleModelResponse(ctx, sessionstore.ModelResponse{
		TurnID:   modelResponse.turnID,
		Response: modelResponse.response,
	})
	if err != nil {
		return err
	}
	for _, status := range statuses {
		for _, value := range status.Operations {
			if err := current.dispatchOperationToManager(value); err != nil {
				return err
			}
		}
	}
	if !current.state.callModel && len(statuses) > 0 {
		for _, status := range statuses {
			current.state.graceToolCalls[toolCallKey{turnID: status.TurnID, callID: status.CallID}] = struct{}{}
		}
		current.state.grace = time.After(toolCallRunGracePeriod)
	}
	return nil
}

func (current *coordinator) clearToolGrace() {
	clear(current.state.graceToolCalls)
	current.state.grace = nil
}

func (current *coordinator) handleStop() (bool, error) {
	if !current.stop.cancellationRequested {
		current.interruptModel()
		if err := current.cancelOperations(); err != nil {
			return false, err
		}
		current.stop.cancellationRequested = true
	}
	return !current.hasPendingOperations(), nil
}

func (current *coordinator) isIdle() bool {
	return current.abort == nil && len(current.modelCalls) == 0 && current.cancelModel == nil && current.pendingInputs() == 0 &&
		len(current.state.toolCalls) == 0 && !current.hasPendingOperations()
}

func (current *coordinator) isWaitingForOnlyToolCalls() bool {
	return current.abort == nil && current.cancelModel == nil && current.stop.request.Mode != inbox.StopHard &&
		current.pendingInputs() == 0 && len(current.state.toolCalls) != 0
}

func (current *coordinator) postHeartbeat(ctx context.Context) error {
	type pendingCall struct {
		CallID string
		Name   string
	}
	calls := make([]pendingCall, 0, len(current.state.toolCalls))
	for _, call := range current.state.toolCalls {
		calls = append(calls, pendingCall{CallID: call.toolCall.CallID, Name: call.toolCall.Name})
	}
	slices.SortFunc(calls, func(a, b pendingCall) int {
		return cmp.Compare(a.CallID, b.CallID)
	})
	runningCalls, err := json.Marshal(calls)
	if err != nil {
		return fmt.Errorf("encode heartbeat tool calls: %w", err)
	}
	reason := fmt.Sprintf("Heartbeat: waited %g seconds for tool calls.\nPending (execution phase unconfirmed): %s", current.dependencies.ToolHeartbeatInterval.Seconds(), runningCalls)
	if current.dependencies.ActivitySummary != nil {
		summary := current.dependencies.ActivitySummary()
		if len(summary) > 4096 {
			summary = summary[:4096]
		}
		reason += "\nActivity observations: " + summary
	}
	payload, err := json.Marshal(inbox.ControlMessage{
		Mode:   inbox.Heartbeat,
		Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("encode heartbeat: %w", err)
	}
	return current.dependencies.Inbox.Submit(ctx, inbox.Input{
		ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload,
	})
}

func (current *coordinator) interruptModel() {
	if current.cancelModel != nil {
		current.cancelModel()
		current.cancelModel = nil
	}
}

func (current *coordinator) acceptStop(request inbox.ControlMessage) {
	if current.stop.request.Mode == inbox.StopHard {
		return
	}
	current.stop.request = request
}

func (current *coordinator) pendingInputs() int {
	return current.state.availableInputs - current.state.deliveredInputs
}

func (current *coordinator) hasPendingOperations() bool {
	for _, value := range current.state.operations {
		if !operationIsTerminal(value.Status) {
			return true
		}
	}
	return false
}

func (current *coordinator) cancelOperations() error {
	var result error
	for id, value := range current.state.operations {
		if operationIsTerminal(value.Status) {
			continue
		}
		if err := current.dependencies.Operations.Cancel(id, current.stop.request.Reason); err != nil {
			result = errors.Join(result, fmt.Errorf("cancel operation %q: %w", id, err))
		}
	}
	return result
}

func (current *coordinator) buildModelRequest() (llm.Request, session.TurnType, error) {
	if a := current.requests.latest; a != nil && !a.completed && a.record.Action == "interrupted" {
		if a.mode == inbox.CompactionRequest {
			built, err := current.dependencies.ContextBuilder.BuildCompaction()
			if err != nil {
				return llm.Request{}, "", requestStateError("cannot rebuild the charged compaction intent")
			}
			return built.Request, session.TurnCompaction, nil
		}
		built, err := current.dependencies.ContextBuilder.Build()
		if err != nil {
			return llm.Request{}, "", fmt.Errorf("rebuild regular request: %w", err)
		}
		return built.Request, session.TurnRegular, nil
	}

	if current.state.manualCompaction || (!current.state.compactionDisabled && current.dependencies.ContextBuilder.NeedsCompaction()) {
		if built, err := current.dependencies.ContextBuilder.BuildCompaction(); err == nil {
			return built.Request, session.TurnCompaction, nil
		}
	}
	built, err := current.dependencies.ContextBuilder.Build()
	if err != nil {
		return llm.Request{}, "", fmt.Errorf("build model request: %w", err)
	}
	return built.Request, session.TurnRegular, nil
}

func (current *coordinator) requestModelResponse(
	ctx context.Context,
	results chan<- modelResponseResult,
) error {
	current.interruptModel()
	if a := current.requests.latest; a != nil && !a.completed && a.record.Action == "started" {
		if err := current.recordRequestAction(ctx, "superseded", "New work superseded the previous request; late output is not authority."); err != nil {
			return err
		}
	}
	if err := current.supersedeCompactionForNewManual(ctx); err != nil {
		return err
	}
	if err := current.disableUnusableCompaction(ctx); err != nil {
		return err
	}
	request, turnType, err := current.buildModelRequest()
	if err != nil {
		return err
	}
	if turnType == session.TurnCompaction && current.compactionFailureLimit() > 100000 {
		return requestStateError("compaction policy exceeds supported attempt bound")
	}
	if turnType == session.TurnCompaction && current.state.compactionFailureLimit == 0 {
		if err := current.recordControl(ctx, inbox.ControlMessage{
			Mode:       inbox.SetCompactionPolicy,
			Parameters: inbox.CompactionPolicy{FailureLimit: current.compactionFailureLimit()},
		}); err != nil {
			return err
		}
	}
	// The committed turn consumes only its applicable manual trigger. A retry
	// retains its recorded origin rather than losing a manual-only request.
	turn := session.Turn{
		ID:             session.TurnID(uuid.New().String()),
		PreviousTurnID: current.state.currentTurnID,
		Type:           turnType,
	}
	if err := current.chargeModelRequest(ctx, turn); err != nil {
		return err
	}
	item, err := current.addItemToLocalState(sessionstore.Item{
		Kind: sessionstore.ItemTurn,
		Data: turn,
	})
	if err != nil {
		return err
	}
	if err := current.storeItemInSessionStore(ctx, item); err != nil {
		return err
	}

	requestContext, cancel := context.WithCancel(ctx)
	current.cancelModel = cancel
	current.state.callModel = false
	if current.modelCalls == nil {
		current.modelCalls = make(map[session.TurnID]struct{})
	}
	current.modelCalls[turn.ID] = struct{}{}
	go func() {
		response, err := current.dependencies.LLM.Respond(requestContext, request, llm.RequestOptions{
			CacheKey: string(current.dependencies.SessionID),
		})
		select {
		case results <- modelResponseResult{
			turnID:   turn.ID,
			response: response,
			err:      err,
		}:
		case <-ctx.Done():
		}
	}()
	return nil
}

func (current *coordinator) handleInboxInput(ctx context.Context, input inbox.Input) error {
	item, err := current.addItemToLocalState(sessionstore.Item{
		Kind: sessionstore.ItemInput,
		Data: input,
	})
	if err != nil {
		return err
	}
	if err := current.storeItemInSessionStore(ctx, item); err != nil {
		return err
	}
	switch input.Kind {
	case inbox.InputExternal:
		current.state.callModel = true
	case inbox.InputControl:
		request, err := input.DecodeControlMessage()
		if err != nil {
			return err
		}
		switch request.Mode {
		case inbox.ShutdownSession:
			value := request.Parameters.(inbox.ShutdownSessionRequest)
			if value.Scope != current.dependencies.ControlScope {
				return current.recordControl(ctx, inbox.ControlMessage{Mode: inbox.SessionShutdownFinished, Parameters: inbox.SessionShutdownResult{Version: 1, InputID: input.ID, Scope: value.Scope, Phase: "refused", Error: "runtime ownership scope changed; no work canceled"}})
			}
			if err := current.recordRequestAction(ctx, "stopped", "Explicit session shutdown; no request retry."); err != nil {
				return err
			}
			current.acceptStop(inbox.ControlMessage{Mode: inbox.StopHard, Reason: "explicit scoped session shutdown"})
		case inbox.SessionShutdownFinished, inbox.ExecutionDrained:
			return nil
		case inbox.AbortForeground:
			return current.beginForegroundAbort(ctx, input.ID, request.Parameters.(inbox.ForegroundAbort))
		case inbox.ForegroundAbortFinished:
			return nil
		case inbox.InvokeTool:
			return current.admitStructuralControl(ctx, input.ID, true)
		case inbox.ToolInvocationFinished:
			return nil
		case inbox.Compact:
			// Do not interrupt an active request; run the manual request next.
			return nil
		case inbox.UpdateSettings:
			if current.cancelModel != nil {
				current.state.callModel = true
			}
			return nil
		case inbox.StopHard, inbox.StopWhenIdle:
			if request.Mode == inbox.StopHard {
				if err := current.recordRequestAction(ctx, "stopped", "Explicit hard stop; no request retry."); err != nil {
					return err
				}
			}
			current.acceptStop(request)
		}
	}
	current.clearToolGrace()
	return nil
}

func (current *coordinator) handleInboxInputs(ctx context.Context, inputs []inbox.Input) error {
	for _, input := range inputs {
		if err := current.handleInboxInput(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

func slurpChannel[T any](
	ctx context.Context,
	output <-chan T,
) ([]T, error) {
	var inputs []T
	idle := time.NewTimer(slurpIdleTimeout)
	defer idle.Stop()
	for len(inputs) < slurpMaxItems {
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case input, open := <-output:
			if !open {
				return inputs, nil
			}
			inputs = append(inputs, input)
			idle.Reset(slurpIdleTimeout)
		case <-idle.C:
			return inputs, nil
		}
	}
	return inputs, nil
}

func (current *coordinator) handleModelResponse(
	ctx context.Context,
	response sessionstore.ModelResponse,
) ([]sessionstore.ToolCallStatus, error) {
	if _, duplicate := current.state.respondedTurns[response.TurnID]; duplicate {
		return nil, nil
	}
	item, err := current.addItemToLocalState(sessionstore.Item{
		Kind: sessionstore.ItemModelResponse,
		Data: response,
	})
	if err != nil {
		return nil, err
	}
	if err := current.storeItemInSessionStore(ctx, item); err != nil {
		return nil, err
	}
	if current.state.currentTurnType != session.TurnCompaction && terminalModelFailure(response.Response) {
		return nil, &ModelFailureError{TurnID: response.TurnID, Failure: *response.Response.Failure}
	}
	if err := current.disableUnusableCompaction(ctx); err != nil {
		return nil, err
	}
	statuses, err := current.scheduleToolCalls(ctx)
	if err != nil {
		return nil, err
	}
	current.state.callModel = toolCallStatusesRequireModelResponse(statuses)
	return statuses, nil
}

func (current *coordinator) handleOperationUpdate(
	ctx context.Context,
	update operation.Operation,
) error {
	update = current.addOperationToLocalState(update)
	if err := current.storeOperationInSessionStore(ctx, update); err != nil {
		return err
	}
	return current.finishStructuralControl(ctx, update)
}

func (current *coordinator) restore(ctx context.Context) error {
	if err := current.loadHistory(ctx); err != nil {
		return err
	}
	for _, value := range current.dependencies.Restored.Operations {
		if _, rejected := current.state.rejectedOperations[value.ID]; rejected {
			continue
		}
		current.addOperationToLocalState(value)
	}
	for id := range current.state.controlInputs {
		if err := current.admitStructuralControl(ctx, id, false); err != nil {
			return err
		}
	}
	return nil
}

func (current *coordinator) loadHistory(ctx context.Context) error {
	after := sessionstore.BeforeFirst
	for {
		page, err := current.dependencies.Sessions.Items(
			ctx,
			current.dependencies.SessionID,
			after,
			historyPageSize,
		)
		if err != nil {
			return fmt.Errorf("load session history after %d: %w", after, err)
		}
		for _, item := range page.Items {
			if err := current.restoreItem(item); err != nil {
				return fmt.Errorf("load session item %d: %w", item.Sequence, err)
			}
		}
		if !page.More {
			return nil
		}
		if page.NextAfter <= after {
			return fmt.Errorf("load session history did not advance after %d", after)
		}
		after = page.NextAfter
	}
}

func (current *coordinator) restoreItem(item sessionstore.Item) error {
	status, ok := item.Data.(sessionstore.ToolCallStatus)
	if item.Kind == sessionstore.ItemToolCallStatus && ok && toolCallRequiresTranslator(status) {
		call, exists := current.state.toolCalls[toolCallKey{turnID: status.TurnID, callID: status.CallID}]
		if exists {
			if _, available := current.dependencies.Tools.Resolve(call.toolCall.Name); !available {
				return fmt.Errorf("tool %q required by recorded call %q is not available", call.toolCall.Name, status.CallID)
			}
		}
	}
	_, err := current.addItemToLocalState(item)
	return err
}

func toolCallRequiresTranslator(status sessionstore.ToolCallStatus) bool {
	return status.Status.Error == "" || len(status.Status.WaitingFor) != 0 || len(status.Operations) != 0
}

func (current *coordinator) addItemToLocalState(
	item sessionstore.Item,
) (sessionstore.Item, error) {
	switch item.Kind {
	case sessionstore.ItemFork:
		if _, ok := item.Data.(sessionstore.Fork); !ok {
			return sessionstore.Item{}, fmt.Errorf(
				"fork data is %T, want sessionstore.Fork",
				item.Data,
			)
		}
		// A fork reads parent history, but does not acquire its request authority.
		current.requests = requestLedger{inheritedTurn: current.state.currentTurnID}
		current.state.manualCompaction = false
		current.state.manualCompactionInput = ""
		// FIXME: Forks leave inherited calls without results and retain pending-input accounting.
		clear(current.state.toolCalls)
		clear(current.state.operations)
		current.state.lifecycle = sessionstore.LifecycleHistory{}
		clear(current.state.abortRequests)
		clear(current.state.abortResults)
		clear(current.state.controlInputs)
		clear(current.state.controlCompleted)
		clear(current.state.controlOperations)
		current.clearToolGrace()

	case sessionstore.ItemInput:
		input, ok := item.Data.(inbox.Input)
		if !ok {
			return sessionstore.Item{}, fmt.Errorf(
				"input data is %T, want inbox.Input",
				item.Data,
			)
		}
		if err := input.Validate(); err != nil {
			return sessionstore.Item{}, fmt.Errorf("invalid input: %w", err)
		}
		if err := current.state.lifecycle.Observe(input); err != nil {
			return sessionstore.Item{}, err
		}
		if input.Kind == inbox.InputExternal {
			if err := current.dependencies.ContextBuilder.AddExternalInput(input); err != nil {
				return sessionstore.Item{}, fmt.Errorf(
					"add input %q to context: %w",
					input.ID,
					err,
				)
			}
			current.state.availableInputs++
			current.state.externalInputs++
			current.state.lastFailure = nil
		}
		if input.Kind == inbox.InputControl {
			request, err := input.DecodeControlMessage()
			if err != nil {
				return sessionstore.Item{}, err
			}
			current.dependencies.ContextBuilder.AddControlMessage(request)
			switch request.Mode {
			case inbox.AbortForeground:
				if current.state.abortRequests == nil {
					current.state.abortRequests = make(map[inbox.ID]inbox.ForegroundAbort)
				}
				value := request.Parameters.(inbox.ForegroundAbort)
				if prior, exists := current.state.abortRequests[input.ID]; exists && prior != value {
					return sessionstore.Item{}, fmt.Errorf("conflicting foreground abort identity %s", input.ID)
				}
				current.state.abortRequests[input.ID] = value
			case inbox.ForegroundAbortFinished:
				result := request.Parameters.(inbox.ForegroundAbortResult)
				intent, exists := current.state.abortRequests[result.InputID]
				if !exists || intent.TargetTurnID != result.TargetTurnID {
					return sessionstore.Item{}, fmt.Errorf("foreground abort outcome has no matching intent/target")
				}
				if prior, exists := current.state.abortResults[result.InputID]; exists && prior.Status != "uncertain" && prior != result {
					return sessionstore.Item{}, fmt.Errorf("conflicting final foreground abort outcome")
				}
				if current.state.abortResults == nil {
					current.state.abortResults = make(map[inbox.ID]inbox.ForegroundAbortResult)
				}
				current.state.abortResults[result.InputID] = result
				if result.Status == "completed" && result.TargetTurnID == string(current.state.currentTurnID) {
					current.consumeAbortedTurn()
				}
			case inbox.InvokeTool:
				current.state.controlInputs[input.ID] = request.Parameters.(inbox.ToolInvocation)
			case inbox.ToolInvocationFinished:
				result := request.Parameters.(inbox.ToolInvocationResult)
				current.state.controlCompleted[result.InputID] = true
				delete(current.state.controlInputs, result.InputID)
			case inbox.CompactionRequest:
				if err := current.observeCompactionRequest(request.Parameters.(inbox.CompactionRequestRecord)); err != nil {
					return sessionstore.Item{}, err
				}
			case inbox.ModelRequest:
				if err := current.observeRequestRecord(request.Parameters.(inbox.ModelRequestRecord)); err != nil {
					return sessionstore.Item{}, err
				}
			case inbox.StopHard:
				if a := current.requests.latest; a != nil && !a.completed && (a.record.Action == "started" || a.record.Action == "interrupted") {
					a.stoppedByControl = true
				}
			case inbox.SetCompactionPolicy:
				current.state.compactionFailureLimit = request.Parameters.(inbox.CompactionPolicy).FailureLimit
			case inbox.CompactionDisabled:
				current.state.compactionDisabled = true
			case inbox.Compact:
				current.state.manualCompaction = true
				current.state.manualCompactionInput = input.ID
			}
			if request.Mode == inbox.Heartbeat {
				current.state.availableInputs++
			}
		}

	case sessionstore.ItemTurn:
		turn, ok := item.Data.(session.Turn)
		if !ok {
			return sessionstore.Item{}, fmt.Errorf(
				"turn data is %T, want session.Turn",
				item.Data,
			)
		}
		if err := current.observeRequestTurn(turn); err != nil {
			return sessionstore.Item{}, err
		}
		current.state.currentTurnID = turn.ID
		current.state.currentTurnType = turn.Type
		current.state.currentTurnInputs = current.state.availableInputs
		current.state.currentTurnExternalInputs = current.state.externalInputs
		// A regular retry must not consume a newer manual intent. Compaction
		// recovery handles original vs newer manual identities before charging.
		if a := current.requests.byTurn[turn.ID]; a == nil || a.mode != inbox.ModelRequest || a.record.Attempt == 1 {
			current.state.manualCompaction = false
			current.state.manualCompactionInput = ""
		}
		if turn.Type != session.TurnCompaction {
			current.dependencies.ContextBuilder.Commit()
		}

	case sessionstore.ItemModelResponse:
		response, ok := item.Data.(sessionstore.ModelResponse)
		if !ok {
			return sessionstore.Item{}, fmt.Errorf(
				"model response data is %T, want sessionstore.ModelResponse",
				item.Data,
			)
		}
		if _, duplicate := current.state.respondedTurns[response.TurnID]; duplicate {
			return item, nil
		}
		current.state.respondedTurns[response.TurnID] = struct{}{}
		if current.requestResponseInert(response.TurnID) {
			current.state.failedTurns[response.TurnID] = struct{}{}
			return item, nil
		}
		if a := current.requests.byTurn[response.TurnID]; a != nil {
			a.completed = true
		}
		if current.state.currentTurnType == session.TurnCompaction && response.TurnID == current.state.currentTurnID {
			current.applyCompactionResponse(response.Response)
			return item, nil
		}
		if response.Response.Failure != nil || response.Response.Stop != llm.StopComplete {
			current.state.failedTurns[response.TurnID] = struct{}{}
		}
		// The complete output includes messages, reasoning, and tool calls.
		current.dependencies.ContextBuilder.AddModelResponse(response.Response)
		if response.TurnID == current.state.currentTurnID {
			current.state.deliveredInputs = current.state.currentTurnInputs
			current.state.lastFailure = nil
			if terminalModelFailure(response.Response) && current.state.externalInputs == current.state.currentTurnExternalInputs {
				current.state.lastFailure = &ModelFailureError{TurnID: response.TurnID, Failure: *response.Response.Failure}
			}
		}
		current.addToolCallsToLocalState(response)

	case sessionstore.ItemToolCallStatus:
		status, ok := item.Data.(sessionstore.ToolCallStatus)
		if !ok {
			return sessionstore.Item{}, fmt.Errorf(
				"tool-call status data is %T, want sessionstore.ToolCallStatus",
				item.Data,
			)
		}
		// Older versions could schedule partial calls from failed responses.
		// Never revive that work, even when a legacy status was recorded.
		if _, failed := current.state.failedTurns[status.TurnID]; failed {
			for _, value := range status.Operations {
				current.state.rejectedOperations[value.ID] = struct{}{}
			}
			return item, nil
		}
		for _, value := range status.Operations {
			current.addOperationToLocalState(value)
		}
		current.addToolCallOperationsToLocalState(status)
		if err := current.addToolResultToLocalState(status); err != nil {
			return sessionstore.Item{}, err
		}

	default:
		return sessionstore.Item{}, fmt.Errorf("unsupported item kind %q", item.Kind)
	}

	return item, nil
}

func (current *coordinator) addToolCallsToLocalState(response sessionstore.ModelResponse) {
	if response.Response.Failure != nil || response.Response.Stop != llm.StopComplete || current.requestResponseInert(response.TurnID) {
		return
	}
	for _, output := range response.Response.Output {
		if output.Type != llm.ItemToolCall {
			continue
		}
		call := output.Data.(llm.ToolCall)
		current.state.toolCalls[toolCallKey{
			turnID: response.TurnID,
			callID: call.CallID,
		}] = toolCallState{
			toolCall:   call,
			operations: make(map[operation.ID]struct{}),
		}
	}
}

func (current *coordinator) addToolCallOperationsToLocalState(
	status sessionstore.ToolCallStatus,
) {
	call, exists := current.state.toolCalls[toolCallKey{
		turnID: status.TurnID,
		callID: status.CallID,
	}]
	if !exists {
		return
	}
	statusValue := status.Status
	call.status = &statusValue
	for _, id := range status.Status.WaitingFor {
		call.operations[id] = struct{}{}
	}
	current.state.toolCalls[toolCallKey{
		turnID: status.TurnID,
		callID: status.CallID,
	}] = call
}

func (current *coordinator) finishToolCall(
	turnID session.TurnID,
	callID string,
) {
	key := toolCallKey{turnID: turnID, callID: callID}
	delete(current.state.toolCalls, key)
	delete(current.state.graceToolCalls, key)
	if len(current.state.graceToolCalls) == 0 {
		current.clearToolGrace()
	}
	current.state.availableInputs++
}

func (current *coordinator) toolCallOperationsAreTerminal(
	turnID session.TurnID,
	callID string,
) bool {
	call := current.state.toolCalls[toolCallKey{
		turnID: turnID,
		callID: callID,
	}]
	for id := range call.operations {
		value, exists := current.state.operations[id]
		if !exists || !operationIsTerminal(value.Status) {
			return false
		}
	}
	return true
}

func (current *coordinator) addToolResultToLocalState(
	status sessionstore.ToolCallStatus,
) error {
	call, exists := current.state.toolCalls[toolCallKey{
		turnID: status.TurnID,
		callID: status.CallID,
	}]
	if !exists {
		return nil
	}
	translator, exists := current.dependencies.Tools.Resolve(call.toolCall.Name)
	if !exists {
		if !toolCallRequiresTranslator(status) {
			current.dependencies.ContextBuilder.AddToolResult(status.CallID, []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: status.Status.Error}}, false)
			current.finishToolCall(status.TurnID, status.CallID)
		}
		return nil
	}

	operations := make([]operation.Operation, 0, len(status.Status.WaitingFor))
	for _, id := range status.Status.WaitingFor {
		if _, exists := call.operations[id]; !exists {
			return nil
		}
		value, exists := current.state.operations[id]
		if !exists {
			return nil
		}
		operations = append(operations, value)
	}
	result, err := translator.TranslateResult(status.CallID, status.Status, operations)
	if err != nil {
		return fmt.Errorf("add tool call %q result to context: %w", status.CallID, err)
	}
	modelResult := result.ToLLMResult()
	running := !current.toolCallOperationsAreTerminal(status.TurnID, status.CallID)
	current.dependencies.ContextBuilder.AddToolResult(
		status.CallID,
		modelResult.Output,
		running,
	)
	if !running {
		current.finishToolCall(status.TurnID, status.CallID)
	}
	return nil
}

func (current *coordinator) addOperationToLocalState(
	value operation.Operation,
) operation.Operation {
	current.state.operations[value.ID] = value
	return current.state.operations[value.ID]
}

func (current *coordinator) scheduleToolCalls(
	ctx context.Context,
) ([]sessionstore.ToolCallStatus, error) {
	statuses := make([]sessionstore.ToolCallStatus, 0)
	for key, call := range current.state.toolCalls {
		if call.status != nil {
			continue
		}
		status, err := current.scheduleToolCall(ctx, key, call.toolCall)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (current *coordinator) scheduleToolCall(
	ctx context.Context,
	key toolCallKey,
	call llm.ToolCall,
) (sessionstore.ToolCallStatus, error) {
	translator, exists := current.dependencies.Tools.Resolve(call.Name)
	toolContext := &toolCallContext{turnID: key.turnID, now: time.Now().UTC()}
	var status tool.CallStatus
	if exists {
		status = translator.Translate(toolContext, call)
	} else {
		status = tool.ErrorStatus(fmt.Sprintf("tool %q is not available", call.Name), 0)
	}
	operations := make([]operation.Operation, 0, len(toolContext.operations))
	for _, value := range toolContext.operations {
		operations = append(operations, current.addOperationToLocalState(value))
	}
	toolCallStatus := sessionstore.ToolCallStatus{
		TurnID:     key.turnID,
		CallID:     key.callID,
		Status:     status,
		Operations: operations,
	}
	item, err := current.addItemToLocalState(sessionstore.Item{
		Kind: sessionstore.ItemToolCallStatus,
		Data: toolCallStatus,
	})
	if err != nil {
		return sessionstore.ToolCallStatus{}, err
	}
	if err := current.storeItemInSessionStore(ctx, item); err != nil {
		return sessionstore.ToolCallStatus{}, err
	}
	return toolCallStatus, nil
}

// CurrentTurnID is optional pure turn metadata; Context.Submit remains unchanged.
func (current *toolCallContext) CurrentTimeUTC() time.Time { return current.now }

func (current *toolCallContext) CurrentTurnID() string { return string(current.turnID) }

func (current *toolCallContext) Submit(spec operation.Spec) operation.ID {
	id := operation.ID(uuid.New().String())
	current.operations = append(current.operations, operation.Operation{
		MaxOutputLength: spec.MaxOutputLength,
		ID:              id,
		Type:            spec.Type,
		Version:         spec.Version,
		Status:          operation.StatusReady,
		State:           spec.State,
		Idempotency:     spec.Idempotency,
	})
	return id
}

func (current *coordinator) reconcileToolCalls(
	ctx context.Context,
) ([]sessionstore.ToolCallStatus, error) {
	completed := make([]sessionstore.ToolCallStatus, 0)
	for key := range current.state.toolCalls {
		if !current.toolCallOperationsAreTerminal(key.turnID, key.callID) {
			continue
		}
		call := current.state.toolCalls[key]
		if call.status == nil {
			return nil, fmt.Errorf("reconcile untranslated tool call %q in turn %q", key.callID, key.turnID)
		}
		operations := make([]operation.Operation, 0, len(call.status.WaitingFor))
		for _, id := range call.status.WaitingFor {
			operations = append(operations, current.state.operations[id])
		}
		status := sessionstore.ToolCallStatus{
			TurnID:     key.turnID,
			CallID:     key.callID,
			Status:     *call.status,
			Operations: operations,
		}
		item, err := current.addItemToLocalState(sessionstore.Item{
			Kind: sessionstore.ItemToolCallStatus,
			Data: status,
		})
		if err != nil {
			return nil, err
		}
		if err := current.storeItemInSessionStore(ctx, item); err != nil {
			return nil, err
		}
		if _, exists := current.state.toolCalls[key]; !exists {
			completed = append(completed, status)
		}
	}
	return completed, nil
}

func toolCallStatusesRequireModelResponse(statuses []sessionstore.ToolCallStatus) bool {
	for _, status := range statuses {
		if status.Status.Error != "" || len(status.Status.WaitingFor) == 0 {
			return true
		}
	}
	return false
}

func (current *coordinator) storeItemInSessionStore(
	ctx context.Context,
	item sessionstore.Item,
) error {
	switch item.Kind {
	case sessionstore.ItemInput:
		input := item.Data.(inbox.Input)
		if err := current.dependencies.Sessions.AppendInput(
			ctx,
			current.dependencies.SessionID,
			input,
		); err != nil {
			return fmt.Errorf("store input %q: %w", input.ID, err)
		}

	case sessionstore.ItemTurn:
		turn := item.Data.(session.Turn)
		if err := current.dependencies.Sessions.AppendTurn(
			ctx,
			current.dependencies.SessionID,
			turn,
		); err != nil {
			return fmt.Errorf("store turn %q: %w", turn.ID, err)
		}

	case sessionstore.ItemModelResponse:
		response := item.Data.(sessionstore.ModelResponse)
		if err := current.dependencies.Sessions.AppendModelResponse(
			ctx,
			current.dependencies.SessionID,
			response,
		); err != nil {
			return fmt.Errorf("store turn %q response: %w", response.TurnID, err)
		}

	case sessionstore.ItemToolCallStatus:
		status := item.Data.(sessionstore.ToolCallStatus)
		if err := current.dependencies.Sessions.AppendToolCallStatus(
			ctx,
			current.dependencies.SessionID,
			status,
		); err != nil {
			return fmt.Errorf("store tool call %q status: %w", status.CallID, err)
		}

	default:
		return fmt.Errorf("unsupported local item kind %q", item.Kind)
	}
	return nil
}

func (current *coordinator) storeOperationInSessionStore(
	ctx context.Context,
	value operation.Operation,
) error {
	if err := current.dependencies.Sessions.SaveOperation(
		ctx,
		current.dependencies.SessionID,
		value,
	); err != nil {
		return fmt.Errorf("store operation %q: %w", value.ID, err)
	}
	return nil
}

func (current *coordinator) dispatchOperationsToManager() error {
	for _, value := range current.state.operations {
		if err := current.dispatchOperationToManager(value); err != nil {
			return err
		}
	}
	return nil
}

func (current *coordinator) dispatchOperationToManager(value operation.Operation) error {
	if operationIsTerminal(value.Status) {
		return nil
	}
	value.State = value.State.Clone()
	value.Idempotency = value.Idempotency.Clone()
	if err := current.dependencies.Operations.Add(value); err != nil {
		return fmt.Errorf("dispatch operation %q: %w", value.ID, err)
	}
	return nil
}

func operationIsTerminal(status operation.Status) bool {
	switch status {
	case operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled:
		return true
	default:
		return false
	}
}

func closedInputError(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%s closed", name)
}
