package inbox

import (
	"context"
	"fmt"
	"sync/atomic"
)

type Inbox struct {
	ctx         context.Context
	submissions chan submission
	output      chan Input
	controls    chan admissionRequest
	pending     atomic.Int64
}

var _ Writer = (*Inbox)(nil)

func New(ctx context.Context, seenIDs []ID) (*Inbox, error) {
	seen := make(map[ID]struct{}, len(seenIDs))
	for _, id := range seenIDs {
		if id == "" {
			return nil, fmt.Errorf("seen input ID is empty")
		}
		seen[id] = struct{}{}
	}
	inputs := &Inbox{
		ctx:         ctx,
		submissions: make(chan submission),
		output:      make(chan Input),
		controls:    make(chan admissionRequest),
	}
	go inputs.run(seen, inputs.submissions)
	return inputs, nil
}

func (inputs *Inbox) Submit(ctx context.Context, input Input) error {
	if err := input.Validate(); err != nil {
		return fmt.Errorf("submit input: %w", err)
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := context.Cause(inputs.ctx); err != nil {
		return err
	}
	input = cloneInput(input)

	request := submission{input: input, admitted: make(chan struct{})}
	select {
	case inputs.submissions <- request:
		// The actor publishes queued ownership before acknowledging. Once
		// admitted, late caller cancellation cannot hide accepted input.
		<-request.admitted
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-inputs.ctx.Done():
		return context.Cause(inputs.ctx)
	}
}

type submission struct {
	input    Input
	admitted chan struct{}
}

// HasPending reports actor-owned, accepted but undelivered input. Read only
// after checking producer ownership: producers release ownership after Submit
// acknowledges, so required completion traffic cannot disappear between checks.
func (inputs *Inbox) HasPending() bool { return inputs.pending.Load() != 0 }

func (inputs *Inbox) Output() <-chan Input {
	return inputs.output
}

// Freeze atomically stops admission and output, transferring the already admitted
// FIFO to its caller. Submit calls racing the fence are either included in that
// FIFO (or already delivered on Output), or block until Thaw or cancellation.
// Freeze/Thaw have one owner; freezing an already frozen inbox is idempotent.
// Cancellation before the actor commits aborts the request. Once committed, the
// definitive result wins over caller cancellation, so a transferred FIFO is never
// silently abandoned. The caller must process the returned inputs before thawing.
func (inputs *Inbox) Freeze(ctx context.Context) ([]Input, error) {
	result, err := inputs.changeAdmission(ctx, true)
	return result.inputs, err
}

// Thaw resumes admission and output without resetting input deduplication.
func (inputs *Inbox) Thaw(ctx context.Context) error {
	_, err := inputs.changeAdmission(ctx, false)
	return err
}

type admissionRequest struct {
	ctx    context.Context
	freeze bool
	result chan admissionResult
}

type admissionResult struct {
	inputs []Input
	err    error
}

func (inputs *Inbox) changeAdmission(ctx context.Context, freeze bool) (admissionResult, error) {
	if err := context.Cause(ctx); err != nil {
		return admissionResult{}, err
	}
	if err := context.Cause(inputs.ctx); err != nil {
		return admissionResult{}, err
	}
	request := admissionRequest{ctx: ctx, freeze: freeze, result: make(chan admissionResult, 1)}
	select {
	case inputs.controls <- request:
	case <-ctx.Done():
		return admissionResult{}, context.Cause(ctx)
	case <-inputs.ctx.Done():
		return admissionResult{}, context.Cause(inputs.ctx)
	}
	// Receiving the control commits the actor to sending a buffered reply before
	// it can observe shutdown. Waiting for that reply avoids losing a transferred
	// FIFO when inbox cancellation races the acknowledgment.
	result := <-request.result
	return result, result.err
}

func (inputs *Inbox) run(seen map[ID]struct{}, submissions <-chan submission) {
	defer close(inputs.output)
	defer inputs.pending.Store(0)

	queued := make([]Input, 0)
	frozen := false
	for {
		var output chan Input
		var next Input
		if !frozen && len(queued) != 0 {
			output = inputs.output
			next = queued[0]
		}

		var admission <-chan submission
		if !frozen {
			admission = submissions
		}
		select {
		case request := <-inputs.controls:
			if err := context.Cause(request.ctx); err != nil {
				request.result <- admissionResult{err: err}
				continue
			}
			frozen = request.freeze
			var drained []Input
			if frozen {
				drained = queued
				queued = nil
				inputs.pending.Store(0)
			}
			request.result <- admissionResult{inputs: drained}

		case request := <-admission:
			input := request.input
			if _, duplicate := seen[input.ID]; !duplicate {
				seen[input.ID] = struct{}{}
				queued = append(queued, input)
				inputs.pending.Add(1)
			}
			close(request.admitted)

		case output <- next:
			queued[0] = Input{}
			queued = queued[1:]
			inputs.pending.Add(-1)

		case <-inputs.ctx.Done():
			return
		}
	}
}

func cloneInput(input Input) Input {
	input.Payload = input.Payload.Clone()
	return input
}
