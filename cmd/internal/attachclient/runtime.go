package attachclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
)

// ErrRuntimeUncertain means transport loss prevented confirmation of a runtime
// operation's outcome. It does not authorize replay of runtime or durable input.
var ErrRuntimeUncertain = errors.New("runtime control outcome uncertain")

// RuntimeError preserves a confirmed unsuccessful terminal runtime status.
type RuntimeError struct{ Status attachproto.RuntimeStatus }

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("runtime %s: %s %s", e.Status.State, e.Status.Message, e.Status.Error)
}

type runtimeResult struct {
	status attachproto.RuntimeStatus
	err    error
}
type runtimeWaiter struct {
	id    string
	reply chan runtimeResult
}

func (c *Client) closeRuntimeLocked(cause error) {
	for _, w := range []*runtimeWaiter{c.runtimeWaiting, c.runtimeCancel} {
		if w != nil {
			w.reply <- runtimeResult{err: fmt.Errorf("%w: %v", ErrRuntimeUncertain, cause)}
		}
	}
	c.runtimeWaiting, c.runtimeCancel = nil, nil
}

// acceptRuntime is called only by Next, after connection/view validation. Even
// consumed replies are returned by Next so the frontend can display progress.
func (c *Client) acceptRuntime(e attachproto.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.runtimeWaiting
	cancel := false
	if w == nil || w.id != e.ID {
		w, cancel = c.runtimeCancel, true
	}
	if w == nil || w.id != e.ID || e.Runtime == nil {
		return errors.New("uncorrelated runtime reply")
	}
	s := *e.Runtime
	c.advanceControlScopeLocked(s.Generation)
	if !s.Terminal() {
		return nil
	}
	if cancel {
		c.runtimeCancel = nil
	} else {
		c.runtimeWaiting = nil
	}
	var err error
	switch s.State {
	case "blocked", "failed", "stopped", "canceled":
		err = &RuntimeError{Status: s}
	}
	w.reply <- runtimeResult{status: s, err: err}
	return nil
}

// RuntimeControl requires a concurrently active Next consumer. It never retries.
// One check/replacement and one cancellation may be outstanding, independently
// of Submit. Cancellation after registration closes this transport: the server
// cancels connection-owned control, but its outcome is explicitly uncertain.
func (c *Client) RuntimeControl(ctx context.Context, request attachproto.RuntimeRequest) (attachproto.RuntimeStatus, error) {
	var zero attachproto.RuntimeStatus
	if err := attachproto.ValidateRuntimeRequest(request); err != nil {
		return zero, err
	}
	if request.Action == "retire-host" && !c.capabilities[attachproto.CapabilityHostRetire] {
		return zero, fmt.Errorf("host missing required capability %q", attachproto.CapabilityHostRetire)
	}
	if request.Action == "drain" && !c.capabilities[attachproto.CapabilityRuntimeDrain] {
		return zero, fmt.Errorf("host missing required capability %q", attachproto.CapabilityRuntimeDrain)
	}
	if !c.capabilities[attachproto.CapabilityRuntime] {
		return zero, fmt.Errorf("host missing required capability %q", attachproto.CapabilityRuntime)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	w := &runtimeWaiter{id: uuid.New().String(), reply: make(chan runtimeResult, 1)}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return zero, io.ErrClosedPipe
	}
	if request.Action == "cancel" {
		if c.runtimeCancel != nil || c.runtimeWaiting == nil || c.runtimeWaiting.id != request.TargetID {
			c.mu.Unlock()
			return zero, errors.New("runtime cancel requires the pending local control ID and no pending cancel")
		}
		c.runtimeCancel = w
	} else {
		if c.runtimeWaiting != nil {
			c.mu.Unlock()
			return zero, errors.New("runtime control already pending")
		}
		c.runtimeWaiting = w
	}
	c.mu.Unlock()
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	c.writeMu.Lock()
	err := c.conn.SetWriteDeadline(deadline)
	if err == nil {
		err = attachproto.Write(c.conn, attachproto.Request{Type: "runtime", ID: w.id, Protocol: attachproto.Version, SessionID: c.snapshot.Hello.SessionID, Connection: c.snapshot.Hello.Connection, Runtime: &request})
	}
	c.writeMu.Unlock()
	if err != nil {
		_ = c.Close()
	}
	result := <-w.reply
	return result.status, result.err
}

// CancelRuntime targets the request ID exposed in Next's runtime progress event.
// It does not send session shutdown, abort, or any durable input.
func (c *Client) CancelRuntime(ctx context.Context, targetID string) error {
	_, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "cancel", TargetID: targetID})
	return err
}
