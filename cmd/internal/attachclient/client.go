// Package attachclient connects to an existing authenticated resident host.
// It never starts, configures, or closes the host.
package attachclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// ErrUncertain means admission may have committed. Retry the same input ID and
// payload after reconnecting, rather than creating a second input identity.
var ErrUncertain = errors.New("submission durability uncertain")

// ReplyError is a correlated rejection by the host.
type ReplyError struct {
	Code    string
	InputID inbox.ID
	Message string
}

func (e *ReplyError) Error() string {
	message := fmt.Sprintf("host %s for input-id %q: %s", e.Code, e.InputID, e.Message)
	if e.Code == "uncertain" {
		message += "; reconnect and retry the same input ID and payload"
	}
	return message
}
func (e *ReplyError) Unwrap() error {
	if e.Code == "uncertain" {
		return ErrUncertain
	}
	return nil
}

// Snapshot is published only after the complete, bounded snapshot validates.
// When Header.Base is nonnil, Items contains only Base.Head+1 through Header.Head.
// Pending, status and model are complete captured current state, not event replay.
type Snapshot struct {
	Hello   attachproto.Event
	Header  attachproto.Event
	Items   []sessionstore.Item
	Pending []inbox.Input
}
type waiter struct {
	requestID string
	inputID   inbox.ID
	reply     chan error
}

// Client has exactly one read owner: the caller of Next. Submit requires that
// owner to remain active until a correlated durable reply arrives. Lifecycle
// admission uses the same owner and does not wait for execution or cleanup.
type Client struct {
	conn                *net.UnixConn
	scanner             *bufio.Scanner
	view                *attachproto.View
	snapshot            Snapshot
	capabilities        map[string]bool
	lifecycleScope      string
	lifecycleGeneration uint64
	writeMu             sync.Mutex
	runtimeWaiting      *runtimeWaiter
	runtimeCancel       *runtimeWaiter
	submitMu            sync.Mutex
	mu                  sync.Mutex
	closed              bool
	waiting             *waiter
	stop                func() bool
}

// Dial authenticates the peer, negotiates capabilities, and atomically obtains a
// fresh snapshot. Cancellation of ctx closes only this client's transport.
func Dial(ctx context.Context, runtimeDir, stateDir string, id session.ID) (*Client, error) {
	return DialFrom(ctx, runtimeDir, stateDir, id, nil)
}

// DialFrom requests a canonical delta plus complete current pending/status/model.
// Older strict v1 hosts may close on Cursor: retry once without the extension.
// Both attempts bind the resulting client to the original caller context.
func DialFrom(ctx context.Context, runtimeDir, stateDir string, id session.ID, cursor *attachproto.Cursor) (*Client, error) {
	if cursor != nil {
		copy := *cursor
		cursor = &copy
	}
	c, err := dialFrom(ctx, runtimeDir, stateDir, id, cursor)
	if cursor != nil && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) && ctx.Err() == nil {
		c, err = dialFrom(ctx, runtimeDir, stateDir, id, nil)
		if err == nil {
			c.snapshot.Header.Code = "cursor_unsupported"
		}
	}
	return c, err
}

func dialFrom(ctx context.Context, runtimeDir, stateDir string, id session.ID, cursor *attachproto.Cursor) (_ *Client, result error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := attachproto.Dial(dialCtx, runtimeDir, stateDir, id)
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, scanner: attachproto.Scanner(conn)}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	c.mu.Lock()
	c.stop = stop
	closed := c.closed
	c.mu.Unlock()
	if closed {
		stop()
	}
	defer func() {
		if result != nil {
			_ = c.Close()
		}
	}()
	snapshotDeadline := time.Now().Add(30 * time.Second)
	if err = conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, err
	}
	if err = attachproto.Write(conn, attachproto.Request{Type: "attach", ID: "attach", Protocol: attachproto.Version, SessionID: id, Cursor: cursor}); err != nil {
		return nil, err
	}
	hello, err := attachproto.ReadEvent(c.scanner)
	if err != nil {
		return nil, err
	}
	if hello.Type != "hello" || hello.ID != "attach" || hello.Protocol != attachproto.Version || hello.SessionID != id || hello.Connection == "" || hello.Incarnation == "" || hello.Base != nil {
		return nil, fmt.Errorf("attach rejected or incompatible: %s %s", hello.Code, hello.Error)
	}
	caps := map[string]bool{}
	for _, cap := range hello.Capabilities {
		caps[cap] = true
	}
	for _, cap := range []string{"fresh_snapshot", "history_watch", "submit_external"} {
		if !caps[cap] {
			return nil, fmt.Errorf("host missing required capability %q", cap)
		}
	}
	if err = conn.SetReadDeadline(snapshotDeadline); err != nil {
		return nil, err
	}
	c.capabilities = caps
	c.snapshot.Hello = hello
	c.view = attachproto.NewViewFrom(hello, cursor)
	total := len(c.scanner.Bytes()) + 1
	pendingBytes := 0
	for {
		e, err := attachproto.ReadEvent(c.scanner)
		if err != nil {
			return nil, err
		}
		total += len(c.scanner.Bytes()) + 1
		if total > 256<<20 {
			return nil, errors.New("snapshot exceeds byte budget")
		}
		if err = c.view.Accept(e); err != nil {
			return nil, err
		}
		switch e.Type {
		case "snapshot":
			c.snapshot.Header = e
		case "snapshot_item":
			c.snapshot.Items = append(c.snapshot.Items, *e.Item)
		case "snapshot_pending":
			in := e.Pending[0]
			pendingBytes += len(in.Payload)
			if len(c.snapshot.Pending) >= 64 || pendingBytes > 8<<20 {
				return nil, errors.New("snapshot pending exceeds budget")
			}
			c.snapshot.Pending = append(c.snapshot.Pending, in)
		case "snapshot_end":
			c.mu.Lock()
			c.lifecycleGeneration = c.snapshot.Header.Status.Generation
			c.lifecycleScope = fmt.Sprintf("%s/%d", hello.Incarnation, c.lifecycleGeneration)
			c.mu.Unlock()
			if err = conn.SetDeadline(time.Time{}); err != nil {
				return nil, err
			}
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			return c, nil
		}
	}
}

// Snapshot returns the initial complete view; live events are delivered by Next.
func (c *Client) Snapshot() Snapshot { return c.snapshot }

// ControlScope returns the trusted host incarnation and latest observed runtime
// generation. A caller must still select a fresh foreground target; the server
// rejects a scope that becomes stale before admission.
func (c *Client) ControlScope() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lifecycleScope
}

// advanceControlScopeLocked never lets delayed status/reply events reopen an
// older lifecycle fence. Only validated live status or correlated runtime events
// may call this helper; the root host incarnation remains the attached one.
func (c *Client) advanceControlScopeLocked(generation uint64) {
	if generation > c.lifecycleGeneration {
		c.lifecycleGeneration = generation
		c.lifecycleScope = fmt.Sprintf("%s/%d", c.snapshot.Hello.Incarnation, generation)
	}
}

func uncertain(id inbox.ID, cause error) error {
	return fmt.Errorf("%w for input-id %q; reconnect and retry the same input ID and payload: %v", ErrUncertain, id, cause)
}

// Close is idempotent and socket-only. It does not wait for Next or Submit.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.closeRuntimeLocked(io.ErrClosedPipe)
	stop := c.stop
	if w := c.waiting; w != nil {
		c.waiting = nil
		w.reply <- uncertain(w.inputID, io.ErrClosedPipe)
	}
	c.mu.Unlock()
	if stop != nil {
		stop()
	}
	return c.conn.Close()
}

// Next directly reads the existing scanner. Only one caller may own Next; no
// background reader or event mailbox exists. Replies also appear in this stream.
func (c *Client) Next() (attachproto.Event, error) {
	for {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return attachproto.Event{}, io.ErrClosedPipe
		}
		e, err := attachproto.ReadEvent(c.scanner)
		if err == nil {
			err = c.view.Accept(e)
		}
		if errors.Is(err, attachproto.ErrDuplicate) {
			continue
		}
		if err != nil {
			_ = c.Close()
			return attachproto.Event{}, err
		}
		if e.Type == "status" {
			c.mu.Lock()
			c.advanceControlScopeLocked(e.Status.Generation)
			c.mu.Unlock()
		}
		if e.Type == "runtime" {
			if err := c.acceptRuntime(e); err != nil {
				_ = c.Close()
				return attachproto.Event{}, err
			}
		}
		if e.Type == "accepted" || e.Type == "error" {
			c.mu.Lock()
			w := c.waiting
			if w == nil || w.requestID != e.ID || w.inputID != e.InputID {
				c.mu.Unlock()
				_ = c.Close()
				return attachproto.Event{}, errors.New("uncorrelated submission reply")
			}
			c.waiting = nil
			var reply error
			if e.Type == "error" {
				reply = &ReplyError{Code: e.Code, InputID: e.InputID, Message: e.Error}
			}
			w.reply <- reply
			c.mu.Unlock()
		}
		return e, nil
	}
}

// Submit serializes submissions and waits for a confirmed durable host reply.
// A concurrently active Next consumer is required. Once writing begins, any
// cancellation or transport failure is uncertain and closes the connection.
func (c *Client) Submit(ctx context.Context, in inbox.Input) error {
	if in.Kind != inbox.InputExternal || in.ID == "" || len(in.ID) > 128 || len(in.Payload) > attachproto.MaxPayload {
		return errors.New("submission requires external input with bounded stable ID and payload")
	}
	if err := in.Validate(); err != nil {
		return err
	}
	return c.submit(ctx, "submit", in)
}

// SubmitLifecycle waits only for durable admission of a typed lifecycle intent,
// never execution, an outcome, or cleanup. The intent must match ControlScope,
// which advances with trusted live runtime generation observations.
// Callers retain responsibility for selecting a fresh foreground target.
// A concurrently active Next consumer is required, just as for Submit.
func (c *Client) SubmitLifecycle(ctx context.Context, in inbox.Input) error {
	if err := attachproto.ValidateLifecycle(in, c.ControlScope()); err != nil {
		return err
	}
	message, err := in.DecodeControlMessage()
	if err != nil {
		return err
	}
	capability := attachproto.CapabilityForegroundAbort
	if message.Mode == inbox.ShutdownSession {
		capability = attachproto.CapabilitySessionShutdown
	}
	if !c.capabilities[capability] {
		return fmt.Errorf("host missing required capability %q", capability)
	}
	return c.submit(ctx, "lifecycle", in)
}

// submit is the single transport/waiter path for both admission APIs. It clones
// the caller's stable input without replacing its ID, and uses a fresh RPC ID.
func (c *Client) submit(ctx context.Context, requestType string, in inbox.Input) error {
	in.Payload = in.Payload.Clone()
	c.submitMu.Lock()
	defer c.submitMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	w := &waiter{requestID: uuid.New().String(), inputID: in.ID, reply: make(chan error, 1)}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return io.ErrClosedPipe
	}
	if requestType == "lifecycle" {
		if err := attachproto.ValidateLifecycle(in, c.lifecycleScope); err != nil {
			c.mu.Unlock()
			return err
		}
	}
	c.waiting = w
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
		err = attachproto.Write(c.conn, attachproto.Request{Type: requestType, ID: w.requestID, Protocol: attachproto.Version, SessionID: c.snapshot.Hello.SessionID, Connection: c.snapshot.Hello.Connection, Input: &in})
	}
	c.writeMu.Unlock()
	if err != nil {
		_ = c.Close()
	}
	// Close and Next always resolve this bounded, single-waiter channel.
	return <-w.reply
}
