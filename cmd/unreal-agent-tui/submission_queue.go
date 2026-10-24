package main

import (
	"context"
	"errors"
	"sync"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

// submissionQueue is an unbounded frontend FIFO, not a durable acknowledgment.
// The UI enqueues synchronously; one sender owns all calls to captured writers.
// Bubble Tea commands only wait for results, so their scheduling cannot reorder
// ingress. No session/tool execution or journal ownership belongs here.
type submissionQueue struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	queued []queuedSubmission
	wake   chan struct{}
	done   chan struct{}
}

type queuedSubmission struct {
	input  inbox.Input
	text   string
	target inbox.Writer
	err    error
	result chan submitted
}

func newSubmissionQueue(parent context.Context) *submissionQueue {
	ctx, cancel := context.WithCancel(parent)
	q := &submissionQueue{ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go q.run()
	return q
}

func (q *submissionQueue) enqueue(input inbox.Input, text string, target inbox.Writer, err error) <-chan submitted {
	record := queuedSubmission{input: input, text: text, target: target, err: err, result: make(chan submitted, 1)}
	// The record owns its payload even if the caller reuses the input buffer.
	record.input.Payload = append(record.input.Payload[:0:0], input.Payload...)
	q.mu.Lock()
	if canceled := q.ctx.Err(); canceled != nil {
		q.mu.Unlock()
		record.complete(canceled)
		return record.result
	}
	q.queued = append(q.queued, record)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return record.result
}

func (record queuedSubmission) complete(err error) {
	record.result <- submitted{id: record.input.ID, text: record.text, err: err}
}

func (q *submissionQueue) rejectPending() {
	q.mu.Lock()
	pending := q.queued
	q.queued = nil
	q.mu.Unlock()
	for _, record := range pending {
		record.complete(q.ctx.Err())
	}
}

func (q *submissionQueue) run() {
	defer close(q.done)
	// One lifecycle callback drains queued promises even if an active writer is
	// still settling cancellation. Wait still joins that active call.
	drained := make(chan struct{})
	stopDrain := context.AfterFunc(q.ctx, func() {
		q.rejectPending()
		close(drained)
	})
	defer func() {
		if !stopDrain() {
			<-drained
		}
		q.rejectPending()
	}()
	for {
		q.mu.Lock()
		if q.ctx.Err() != nil {
			q.mu.Unlock()
			return
		}
		if len(q.queued) == 0 {
			q.mu.Unlock()
			select {
			case <-q.ctx.Done():
				return
			case <-q.wake:
				continue
			}
		}
		record := q.queued[0]
		q.queued[0] = queuedSubmission{}
		q.queued = q.queued[1:]
		if len(q.queued) == 0 {
			q.queued = nil
		}
		q.mu.Unlock()
		err := record.err
		if err == nil {
			if record.target == nil {
				err = errors.New("submission writer is unavailable")
			} else {
				err = record.target.Submit(q.ctx, record.input)
			}
		}
		record.complete(err)
	}
}

func (q *submissionQueue) Cancel() {
	q.cancel()
	q.rejectPending()
}

// Wait does not relinquish active writer ownership on timeout. Production must
// also close/join the host, which tracks durable admissions independently.
func (q *submissionQueue) Wait(ctx context.Context) error {
	select {
	case <-q.done:
		return nil
	default:
	}
	select {
	case <-q.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
