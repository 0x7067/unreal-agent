package coordinator

import (
	"context"
	"errors"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

type quiesceRequest struct {
	ctx    context.Context
	result chan error
}

func (current *coordinator) initLifecycle() {
	current.lifecycleOnce.Do(func() {
		current.quiesce = make(chan quiesceRequest)
		current.runDone = make(chan struct{})
	})
}

func (current *coordinator) quiesceAfterExit() error {
	if current.runResult == ErrQuiesced {
		return nil
	}
	return errors.Join(ErrStopped, current.runResult)
}

func (current *coordinator) TryQuiesce(ctx context.Context) error {
	current.initLifecycle()
	// Completed retirement takes precedence over a late caller cancellation.
	select {
	case <-current.runDone:
		return current.quiesceAfterExit()
	default:
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	request := quiesceRequest{ctx: ctx, result: make(chan error, 1)}
	select {
	case current.quiesce <- request:
	case <-current.runDone:
		return current.quiesceAfterExit()
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	// The loop always replies to an admitted request. Do not race that answer
	// with ctx.Done: doing so could report cancellation for committed retirement.
	return <-request.result
}

func (current *coordinator) canQuiesce() bool {
	return current.isIdle() && !current.state.callModel &&
		!current.state.manualCompaction && current.state.grace == nil &&
		len(current.state.graceToolCalls) == 0 && current.state.lastFailure == nil &&
		current.stop.request.Mode == ""
}

// handleQuiesce runs only on Run's goroutine. It never schedules, cancels or
// dispatches work; draining accepted input can create work for the next iteration.
// The second return value is a fatal loop error, separate from the RPC answer.
func (current *coordinator) handleQuiesce(ctx context.Context, request quiesceRequest) (retired bool, fatal error) {
	var answer error
	defer func() { request.result <- answer }()
	if answer = context.Cause(request.ctx); answer != nil {
		return false, nil
	}
	if answer = context.Cause(ctx); answer != nil {
		return false, nil
	}
	if !current.canQuiesce() {
		answer = ErrBusy
		return false, nil
	}

	// Merge lifetimes so shutdown can abort admission too. Freeze acknowledges
	// its definitive actor transition even if this context is canceled late.
	freezeCtx, cancel := context.WithCancelCause(ctx)
	stopCancel := context.AfterFunc(request.ctx, func() { cancel(context.Cause(request.ctx)) })
	defer cancel(nil)
	defer stopCancel()
	var queued []inbox.Input
	queued, answer = current.dependencies.Inbox.Freeze(freezeCtx)
	if answer != nil {
		return false, nil
	}
	defer func() {
		if !retired {
			// Caller/Run cancellation must not strand a live inbox frozen. The
			// inbox's own lifetime still bounds this RPC during worker shutdown.
			if err := current.dependencies.Inbox.Thaw(context.Background()); err != nil {
				answer = errors.Join(answer, err)
			}
		}
	}()
	// These inputs were admitted before the fence. Never drop them, including
	// when the request was canceled during Freeze. Use Run's context to persist.
	if err := current.processInputs(ctx, queued); err != nil {
		answer = err
		return false, err
	}
	// Consume updates already waiting at the fence too. Normally every update
	// belongs to a known operation (which made the initial idle check fail),
	// but an unsolicited/restored update must fail safe rather than retire over
	// newly visible nonterminal work. Bound this drain so a noisy producer cannot
	// monopolize Run; reaching the bound conservatively refuses retirement.
	updates := current.dependencies.Operations.Updates()
	for count := 0; ; count++ {
		if count == slurpMaxItems {
			answer = ErrBusy
			return false, nil
		}
		select {
		case update, open := <-updates:
			if !open {
				answer = closedInputError(ctx, "operation updates")
				return false, answer
			}
			if err := current.processOperations(ctx, []operation.Operation{update}); err != nil {
				answer = err
				return false, err
			}
		default:
			if answer = context.Cause(request.ctx); answer != nil {
				return false, nil
			}
			if answer = context.Cause(ctx); answer != nil {
				return false, nil
			}
			if !current.canQuiesce() {
				answer = ErrBusy
				return false, nil
			}
			return true, nil
		}
	}
}
