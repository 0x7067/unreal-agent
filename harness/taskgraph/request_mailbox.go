package taskgraph

import "context"

// runtimeRequests separates admission from execution. A single mailbox actor
// owns the unbounded FIFO shared by both handlers, so a slow runtime callback
// cannot backpressure the operation manager. Only Runtime.loop executes requests
// and touches graph state; admission is not an execution acknowledgment.
//
// Cancellation stops admission and discards pending requests. Like requests
// already accepted by the operation manager, queued requests are volatile.
// An executing callback must still return before runtime shutdown can finish.
type runtimeRequests struct {
	ctx         context.Context
	submissions chan func(*runtimeState)
	output      chan func(*runtimeState)
	done        chan struct{}
}

func newRuntimeRequests(ctx context.Context) *runtimeRequests {
	q := &runtimeRequests{
		ctx:         ctx,
		submissions: make(chan func(*runtimeState)),
		output:      make(chan func(*runtimeState)),
		done:        make(chan struct{}),
	}
	go q.loop()
	return q
}

func (q *runtimeRequests) submit(fn func(*runtimeState)) error {
	// A ready actor must not cause a call made after cancellation to report
	// success through select's random choice of ready cases.
	if err := q.ctx.Err(); err != nil {
		return err
	}
	select {
	case q.submissions <- fn:
		return nil
	case <-q.ctx.Done():
		return q.ctx.Err()
	}
}

func (q *runtimeRequests) loop() {
	defer close(q.done)
	var queued []func(*runtimeState)
	for {
		if q.ctx.Err() != nil {
			return
		}
		var output chan func(*runtimeState)
		var next func(*runtimeState)
		if len(queued) > 0 {
			output, next = q.output, queued[0]
		}
		select {
		case fn := <-q.submissions:
			queued = append(queued, fn)
		case output <- next:
			queued[0] = nil // Do not retain operations captured by executed closures.
			queued = queued[1:]
			if len(queued) == 0 {
				queued = nil
			}
		case <-q.ctx.Done():
			return
		}
	}
}
