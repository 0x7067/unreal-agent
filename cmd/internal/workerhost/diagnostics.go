package workerhost

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// queryActivity is a live-only request, fenced against worker replacement. The
// single query slot bounds waiters retained in the process; callers waiting for
// admission only own their own stack. No goroutine is created for a query, and
// cancellation does not pretend the worker stopped or a control was consumed.
func (h *Host) queryActivity(ctx context.Context) (workerproto.Event, uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case h.query <- struct{}{}:
		defer func() { <-h.query }()
	case <-ctx.Done():
		return workerproto.Event{}, 0, context.Cause(ctx)
	case <-h.ctx.Done():
		return workerproto.Event{}, 0, ErrClosed
	}
	if err := h.lockDelivery(ctx); err != nil {
		return workerproto.Event{}, 0, err
	}
	if err := context.Cause(ctx); err != nil {
		h.unlockDelivery()
		return workerproto.Event{}, 0, err
	}
	h.mu.Lock()
	p, unavailable := h.current, h.closing || h.closed
	h.mu.Unlock()
	if p == nil || !p.accepting.Load() || unavailable || p.retired.Load() {
		h.unlockDelivery()
		return workerproto.Event{}, 0, ErrStopped
	}
	id := uuid.New().String()
	reply := make(chan workerproto.Event, 1)
	p.mu.Lock()
	p.waiting[id] = reply
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.waiting, id); p.mu.Unlock() }()
	// All production worker stdin pipes are os.Files supporting write deadlines.
	// A deadline after a partial write makes transport uncertain: retire rather
	// than reuse a potentially corrupt JSONL stream. Never spawn a leaked writer.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(5 * time.Second)
	}
	writer, ok := p.input.(interface{ SetWriteDeadline(time.Time) error })
	if !ok {
		h.unlockDelivery()
		return workerproto.Event{}, p.generation, errors.New("worker input does not support bounded diagnostic writes")
	}
	if err := writer.SetWriteDeadline(deadline); err != nil {
		h.unlockDelivery()
		return workerproto.Event{}, p.generation, err
	}
	// At most one cancellation callback exists per host (the query slot is held).
	// Join it before restoring the descriptor deadline, so a late callback cannot
	// poison the next ordered ingress write.
	interrupted := make(chan struct{})
	stopWrite := context.AfterFunc(ctx, func() { _ = writer.SetWriteDeadline(time.Now()); close(interrupted) })
	err := workerproto.Write(p.input, workerproto.Request{Type: "activity_snapshot", ID: id, Protocol: workerproto.Version})
	if !stopWrite() {
		<-interrupted
	}
	resetErr := writer.SetWriteDeadline(time.Time{})
	h.unlockDelivery()
	if err != nil || resetErr != nil {
		p.cancel()
		return workerproto.Event{}, p.generation, fmt.Errorf("diagnostic transport uncertain: %w", errors.Join(err, resetErr, context.Cause(ctx)))
	}
	select {
	case event := <-reply:
		return event, p.generation, nil
	case <-ctx.Done():
		return workerproto.Event{}, p.generation, fmt.Errorf("activity snapshot unavailable: %w", context.Cause(ctx))
	case <-p.done:
		return workerproto.Event{}, p.generation, errors.Join(ErrStopped, p.result())
	}
}

// Activity is detached live-only evidence from one executing worker generation.
// An unavailable query is not an empty fleet, idle proof, or a drain claim.
type Activity struct {
	Generation uint64
	Snapshot   subagent.FleetSnapshot
	Resources  *operation.ActivitySnapshot
}

func (h *Host) ActivitySnapshot(ctx context.Context) (Activity, error) {
	event, generation, err := h.queryActivity(ctx)
	if err != nil {
		return Activity{Generation: generation}, err
	}
	if event.Activity == nil {
		return Activity{Generation: generation}, errors.New("worker returned no activity snapshot")
	}
	return Activity{Generation: generation, Snapshot: *event.Activity, Resources: event.Resources}, nil
}
