package operation

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

type LocalOperationManager struct {
	stopped                 chan struct{}
	stopErr                 *CleanupUncertainError
	unprovenRemoteIDs       map[ID]struct{}
	unsupportedOperationIDs map[ID]struct{}
	localCleanupReceipts    []ID
	activityMu              sync.RWMutex
	activity                []OperationActivity
	activityOmitted         int
	ctx                     context.Context
	options                 LocalOperationManagerOptions
	remoteJobs              *remoteJobHandlers
	adds                    chan localAddRequest
	cancellations           chan localCancelRequest
	primitiveEvents         chan primitives.PrimitiveEvent
	updates                 chan Operation
	pendingUpdates          []Operation
}

type localAddRequest struct {
	operation Operation
}

type localCancelRequest struct {
	id     ID
	reason string
}

type localRunningOperation struct {
	activePrimitives      int
	terminal              bool
	drained               bool
	remoteUnknown         bool
	pendingTerminal       bool
	remoteCheckpoint      Operation
	lease                 *executionbudget.Lease
	activity              OperationActivity
	ctx                   context.Context
	cancel                context.CancelFunc
	operation             Operation
	remoteJobHandlerIndex int
	handle                func(*primitives.PrimitiveEvent) (Step, error)
}

func (current *localRunningOperation) initialize() error {
	switch current.operation.Type {
	case TypeShell:
		shell, err := NewShell(current.operation)
		if err != nil {
			return err
		}
		current.handle = shell.Handle
	case TypeViewImage:
		view, err := NewViewImage(current.operation)
		if err != nil {
			return err
		}
		current.handle = view.Handle
	default:
		current.handle = func(event *primitives.PrimitiveEvent) (Step, error) {
			return advanceLocalOperation(current.operation, event)
		}
	}
	return nil
}

var _ Manager = (*LocalOperationManager)(nil)

func NewLocalOperationManager(ctx context.Context, remoteJobHandlers ...RemoteJobHandler) *LocalOperationManager {
	return NewLocalOperationManagerWithOptions(ctx, LocalOperationManagerOptions{}, remoteJobHandlers...)
}

// LocalOperationManagerOptions configures session-scoped output artifacts. An
// empty OutputDirectory preserves the legacy, inline-only behavior.
type LocalOperationManagerOptions struct {
	OutputDirectory string
	// Budget is a live admission-relative wall policy. Zero preserves legacy behavior.
	Budget executionbudget.Policy
	// ResourceAccount must be shared by root and nested managers to share capacity.
	// If nil, it is inherited from ContextWithAccount when present.
	ResourceAccount *executionbudget.Account
	// Tests may replace the wall clock and tick source without real sleeps.
	now         func() time.Time
	budgetTicks <-chan time.Time
}

func NewLocalOperationManagerWithOptions(ctx context.Context, options LocalOperationManagerOptions, remoteJobHandlers ...RemoteJobHandler) *LocalOperationManager {
	if options.ResourceAccount == nil {
		options.ResourceAccount = executionbudget.AccountFromContext(ctx)
	}
	manager := &LocalOperationManager{
		stopped:                 make(chan struct{}),
		unprovenRemoteIDs:       make(map[ID]struct{}),
		unsupportedOperationIDs: make(map[ID]struct{}),
		options:                 options,
		ctx:                     ctx,
		remoteJobs:              newRemoteJobHandlers(ctx, remoteJobHandlers),
		adds:                    make(chan localAddRequest),
		cancellations:           make(chan localCancelRequest),
		primitiveEvents:         make(chan primitives.PrimitiveEvent),
		updates:                 make(chan Operation),
	}
	go func() {
		manager.run()
		// run includes the forwarding-worker wait and primitive-event drain.
		close(manager.stopped)
	}()
	return manager
}

func (manager *LocalOperationManager) Add(operation Operation) error {
	request := localAddRequest{operation: operation}
	select {
	case manager.adds <- request:
		return nil
	case <-manager.ctx.Done():
		return manager.ctx.Err()
	}
}

func (manager *LocalOperationManager) Cancel(id ID, reason string) error {
	request := localCancelRequest{id: id, reason: reason}
	select {
	case manager.cancellations <- request:
		return nil
	case <-manager.ctx.Done():
		return manager.ctx.Err()
	}
}

func (manager *LocalOperationManager) Updates() <-chan Operation {
	return manager.updates
}

func (manager *LocalOperationManager) run() {
	defer close(manager.updates)
	defer manager.remoteJobs.workers.Wait()
	operations := make(map[ID]*localRunningOperation)
	accepted := make(map[ID]struct{})
	activePrimitives := 0
	ticks := manager.options.budgetTicks
	if ticks == nil {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		var updates chan Operation
		var update Operation
		if len(manager.pendingUpdates) != 0 {
			updates = manager.updates
			update = manager.pendingUpdates[0]
		}

		select {
		case updates <- update:
			manager.pendingUpdates[0] = Operation{}
			manager.pendingUpdates = manager.pendingUpdates[1:]

		case <-ticks:
			manager.checkBudgets(operations)

		case request := <-manager.adds:
			if _, exists := accepted[request.operation.ID]; exists {
				continue
			}
			accepted[request.operation.ID] = struct{}{}
			ctx, cancel := context.WithCancel(manager.ctx)
			current := &localRunningOperation{
				ctx:                   ctx,
				cancel:                cancel,
				operation:             request.operation,
				remoteJobHandlerIndex: -1,
			}
			current.activity = OperationActivity{ID: current.operation.ID, Type: current.operation.Type, Phase: ActivityRunning, StartedAt: manager.now(), Budget: manager.options.Budget}
			current.activity.LastObservedAt = current.activity.StartedAt
			if err := manager.options.Budget.Validate(); err != nil {
				manager.failLocalOperation(operations, current, err)
				continue
			}
			if current.operation.Type == TypeShell {
				state, err := DecodeShellState(current.operation)
				if err != nil {
					manager.failLocalOperation(operations, current, err)
					continue
				}
				current.activity.Budget = executionbudget.Constrain(manager.options.Budget, state.Input.Budget)
			}
			var lease *executionbudget.Lease
			var err error
			if isManagementOperation(current.operation) {
				lease, err = manager.options.ResourceAccount.AcquireManagement()
			} else {
				lease, err = manager.options.ResourceAccount.Acquire()
			}
			if err != nil {
				manager.failLocalOperation(operations, current, err)
				continue
			}
			current.lease = lease
			manager.observe(current)
			if request.operation.Type == TypeRemoteJob {
				if err := manager.configureRemoteJob(&current.operation); err != nil {
					manager.failLocalOperation(operations, current, err)
					continue
				}
				// Even a handler admission error cannot generically prove that no
				// external work was started. Registered but unused handlers do not
				// enter this set.
				manager.unprovenRemoteIDs[current.operation.ID] = struct{}{}
				handlerIndex, err := manager.remoteJobs.add(current.operation)
				if err != nil {
					manager.failLocalOperation(operations, current, err)
					continue
				}
				current.remoteJobHandlerIndex = handlerIndex
				current.remoteCheckpoint = current.operation
				operations[request.operation.ID] = current
				continue
			}
			if err := current.initialize(); err != nil {
				manager.failLocalOperation(operations, current, err)
				continue
			}
			step, err := current.handle(nil)
			if err != nil {
				manager.failLocalOperation(operations, current, err)
				continue
			}
			operations[request.operation.ID] = current
			activePrimitives += manager.acceptLocalStep(operations, current, step)

		case request := <-manager.cancellations:
			current, exists := operations[request.id]
			if !exists {
				continue
			}
			if err := manager.requestCancellation(current, request.reason); err != nil {
				current.remoteUnknown = true
				manager.failLocalOperation(operations, current, fmt.Errorf("remote job outcome is unknown because cancellation failed: %v", err))
			}

		case update := <-manager.remoteJobs.updates:
			if update.closed {
				manager.remoteJobs.markClosed(update.handlerIndex)
				for _, current := range operations {
					if current.remoteJobHandlerIndex == update.handlerIndex {
						current.remoteUnknown = true
						// A retained terminal operation already reported its one terminal update.
						if current.terminal {
							continue
						}
						manager.failLocalOperation(
							operations,
							current,
							fmt.Errorf("remote job handler %d stopped", update.handlerIndex),
						)
					}
				}
				continue
			}
			current, exists := operations[update.operation.ID]
			if !exists {
				continue
			}
			if current.remoteJobHandlerIndex != update.handlerIndex {
				if current.terminal {
					continue
				}
				manager.failRemoteJobOperation(
					operations,
					current,
					fmt.Errorf("remote job operation %q update came from the wrong handler", update.operation.ID),
				)
				continue
			}
			validation := current.operation
			if current.terminal {
				validation = current.remoteCheckpoint
			}
			if err := validateRemoteJobUpdate(validation, update.operation); err != nil {
				if current.terminal {
					continue
				}
				manager.failRemoteJobOperation(operations, current, err)
				continue
			}
			current.remoteCheckpoint = update.operation
			if current.terminal {
				if localOperationFinished(update.operation.Status) {
					current.remoteUnknown = false
					manager.finishOwnedWithLocalReceipt(operations, current)
				}
				continue
			}
			current.operation = update.operation
			current.activity.LastObservedAt = manager.now()
			manager.observe(current)
			manager.appendLocalUpdate(update.operation)
			if localOperationFinished(update.operation.Status) {
				current.terminal = true
				manager.finishOwnedWithLocalReceipt(operations, current)
			}

		case event := <-manager.primitiveEvents:
			completed := localPrimitiveCompleted(event.Type)
			if completed {
				activePrimitives--
			}
			current, exists := operations[ID(event.Source)]
			if !exists {
				continue
			}
			current.activity.LastObservedAt = manager.now()
			if event.Type == primitives.PrimitiveEventProcessStarted {
				current.activity.ProcessStarted = true
			}
			if completed {
				current.activePrimitives--
				if event.CorrelationID == primitives.CorrelationID(ShellPhaseProcess) {
					current.activity.ProcessCompletionObserved = true
				}
			}
			manager.observe(current)
			if current.terminal {
				manager.finishOwnedWithLocalReceipt(operations, current)
				continue
			}
			if completed && current.ctx.Err() != nil {
				event.Type = primitives.PrimitiveEventCanceled
				event.Result = nil
			}
			step, err := current.handle(&event)
			if err != nil {
				manager.failLocalOperation(operations, current, err)
				continue
			}
			activePrimitives += manager.acceptLocalStep(operations, current, step)

		case <-manager.ctx.Done():
			for _, current := range operations {
				current.cancel()
				if current.activity.CancellationRequestedAt.IsZero() {
					current.activity.CancellationRequestedAt = manager.now()
					current.activity.CancellationReason = "manager_shutdown"
				}
				current.activity.Phase = ActivityDraining
				current.activity.DrainingAt = manager.now()
				manager.observe(current)
			}
			manager.captureStopUncertainty(operations)
			manager.drainLocalPrimitives(activePrimitives)
			for _, current := range operations {
				if current.remoteJobHandlerIndex < 0 {
					current.activePrimitives = 0
					manager.finishOwnedWithLocalReceipt(operations, current)
				}
			}
			return
		}
	}
}

func (manager *LocalOperationManager) acceptLocalStep(
	operations map[ID]*localRunningOperation,
	current *localRunningOperation,
	step Step,
) int {
	if step.Operation != nil {
		current.operation = *step.Operation
		if localOperationFinished(step.Operation.Status) {
			current.terminal = true
			current.cancel()
			if current.activePrimitives > 0 {
				current.pendingTerminal = true
				current.activity.Phase = ActivityDraining
				current.activity.DrainingAt = manager.now()
			} else {
				manager.appendLocalUpdate(*step.Operation)
			}
			manager.finishOwnedWithLocalReceipt(operations, current)
			return 0
		}
		manager.appendLocalUpdate(*step.Operation)
		manager.observe(current)
	}
	started := 0
	for _, dispatch := range step.Dispatches {
		if err := startLocalPrimitive(current.ctx, dispatch, manager.primitiveEvents); err != nil {
			failed, advanceErr := current.handle(&primitives.PrimitiveEvent{
				Type:   primitives.PrimitiveEventFailed,
				Source: primitives.SourceID(current.operation.ID),
				Result: primitives.PrimitiveFailureResult{Error: err.Error()},
			})
			if advanceErr != nil {
				manager.failLocalOperation(operations, current, advanceErr)
				return started
			}
			return started + manager.acceptLocalStep(operations, current, failed)
		}
		started++
		current.activePrimitives++
		manager.observe(current)
	}
	return started
}

func (manager *LocalOperationManager) failLocalOperation(
	operations map[ID]*localRunningOperation,
	current *localRunningOperation,
	err error,
) {
	failed := failLocalOperation(current.operation, err)
	if failed.Status == StatusUnsupported {
		if manager.unsupportedOperationIDs == nil {
			manager.unsupportedOperationIDs = make(map[ID]struct{})
		}
		manager.unsupportedOperationIDs[current.operation.ID] = struct{}{}
	}
	if current.remoteUnknown {
		current.activity.Phase = ActivityDraining
		if current.activity.DrainingAt.IsZero() {
			current.activity.DrainingAt = manager.now()
		}
	}
	if current.activePrimitives > 0 {
		current.pendingTerminal = true
		current.activity.Phase = ActivityDraining
		current.activity.DrainingAt = manager.now()
	} else {
		manager.appendLocalUpdate(failed)
	}
	current.operation = failed
	current.terminal = true
	current.cancel()
	manager.finishOwnedWithLocalReceipt(operations, current)
}

func (manager *LocalOperationManager) failRemoteJobOperation(
	operations map[ID]*localRunningOperation,
	current *localRunningOperation,
	cause error,
) {
	current.remoteUnknown = true
	if err := manager.remoteJobs.cancel(
		current.remoteJobHandlerIndex,
		current.operation.ID,
		cause.Error(),
	); err != nil {
		cause = errors.Join(cause, fmt.Errorf("remote job outcome is unknown because cancellation failed: %v", err))
	}
	manager.failLocalOperation(operations, current, cause)
}

func (manager *LocalOperationManager) drainLocalPrimitives(active int) {
	for active != 0 {
		event := <-manager.primitiveEvents
		if localPrimitiveCompleted(event.Type) {
			active--
		}
	}
}

func (manager *LocalOperationManager) appendLocalUpdate(operation Operation) {
	operation.State = operation.State.Clone()
	operation.Idempotency = operation.Idempotency.Clone()
	manager.pendingUpdates = append(manager.pendingUpdates, operation)
}

func advanceLocalOperation(current Operation, event *primitives.PrimitiveEvent) (Step, error) {
	switch current.Type {
	case TypeValue:
		return AdvanceValue(current, event)
	case TypeSkillUse:
		return AdvanceSkillUse(current, event)
	default:
		return Step{}, fmt.Errorf(
			"local operation manager does not support type %q: %w",
			current.Type,
			ErrUnsupported,
		)
	}
}

func failLocalOperation(current Operation, err error) Operation {
	if errors.Is(err, ErrUnsupported) {
		current.Status = StatusUnsupported
		return current
	}
	switch current.Type {
	case TypeSkillUse:
		state, stateErr := skillUseOperationState(current)
		if stateErr == nil {
			step, stepErr := failSkillUse(current, state, err)
			if stepErr == nil {
				return *step.Operation
			}
		}
	case TypeShell:
		shell, stateErr := NewShell(current)
		if stateErr == nil {
			step, stepErr := shell.fail(err)
			if stepErr == nil {
				return *step.Operation
			}
		}
	case TypeViewImage:
		step, stateErr := failViewImage(current, err)
		if stateErr == nil {
			return *step.Operation
		}
	case TypeRemoteJob:
		step, stateErr := FailRemoteJob(current, err)
		if stateErr == nil {
			return *step.Operation
		}
	}
	current.Status = StatusUnsupported
	return current
}

func startLocalPrimitive(
	ctx context.Context,
	dispatch PrimitiveDispatch,
	events chan primitives.PrimitiveEvent,
) error {
	switch dispatch.Type {
	case primitives.PrimitiveDispatchIOCreate:
		request, ok := dispatch.Data.(primitives.IOCreateRequest)
		if !ok {
			return fmt.Errorf("io.create dispatch data is %T, want primitives.IOCreateRequest", dispatch.Data)
		}
		primitives.Create(ctx, request, events)

	case primitives.PrimitiveDispatchIORead:
		request, ok := dispatch.Data.(primitives.IOReadRequest)
		if !ok {
			return fmt.Errorf("io.read dispatch data is %T, want primitives.IOReadRequest", dispatch.Data)
		}
		primitives.ReadFile(ctx, request, events)

	case primitives.PrimitiveDispatchProcessStart:
		request, ok := dispatch.Data.(primitives.ProcessStartRequest)
		if !ok {
			return fmt.Errorf("process.start dispatch data is %T, want primitives.ProcessStartRequest", dispatch.Data)
		}
		primitives.StartProcess(ctx, request, events)

	case primitives.PrimitiveDispatchCompute:
		request, ok := dispatch.Data.(primitives.ComputeRequest)
		if !ok {
			return fmt.Errorf("compute dispatch data is %T, want primitives.ComputeRequest", dispatch.Data)
		}
		primitives.Compute(ctx, request, events)

	default:
		return fmt.Errorf("local operation manager does not support dispatch %q", dispatch.Type)
	}

	return nil
}

func localOperationFinished(status Status) bool {
	switch status {
	case StatusCompleted, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}
}

func localPrimitiveCompleted(eventType primitives.PrimitiveEventType) bool {
	switch eventType {
	case primitives.PrimitiveEventIOCreateCompleted,
		primitives.PrimitiveEventIOReadCompleted,
		primitives.PrimitiveEventProcessExited,
		primitives.PrimitiveEventFailed,
		primitives.PrimitiveEventCanceled,
		primitives.PrimitiveEventComputeCompleted:
		return true
	default:
		return false
	}
}

// Configure before AddRemoteJob: handlers may call UpdateRemoteJob themselves,
// so waiting for their update to arrive would already have lost the full text.
func (manager *LocalOperationManager) configureRemoteJob(current *Operation) error {
	state, err := DecodeRemoteJobState(*current)
	if err != nil {
		return err
	}
	state.OutputDirectory = manager.options.OutputDirectory
	if state.OutputDirectory != "" {
		if _, err := remoteJobArtifactName(current.ID, "result", ""); err != nil {
			return err
		}
		state.OutputDirectory, err = filepath.Abs(state.OutputDirectory)
		if err != nil {
			return err
		}
	}
	current.State, err = json.Marshal(state)
	return err
}
