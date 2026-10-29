package resident

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

type Config struct {
	RuntimeReplacement bool
	RuntimeDirectory   string
	Worker             workerhost.Config
}
type Info struct {
	Endpoint, Incarnation string
	HostPID               int
	SessionID             session.ID
}
type client struct {
	conn          *net.UnixConn
	token         string
	queue         chan []byte
	bytes         int // publisher.mu
	stopped       chan struct{}
	once          sync.Once
	runtimeMu     sync.Mutex
	runtimeID     string
	runtimeCancel context.CancelFunc
	runtimeWG     sync.WaitGroup
}

func (c *client) stop() {
	c.once.Do(func() {
		close(c.stopped)
		c.runtimeMu.Lock()
		cancel := c.runtimeCancel
		c.runtimeMu.Unlock()
		if cancel != nil {
			cancel()
		}
	})
}
func randomToken() (string, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}

// Serve owns the worker independently of client lifetimes. Shutdown always gets
// a fresh budget; cancellation of a client never propagates into the host.
func Serve(ctx context.Context, config Config, ready func(Info)) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := attachproto.ValidateRuntimeDirectory(config.RuntimeDirectory); err != nil {
		return err
	}
	incarnation, err := randomToken()
	if err != nil {
		return err
	}
	p := &publisher{runtimeEnabled: config.RuntimeReplacement, info: Info{Incarnation: incarnation, HostPID: os.Getpid(), SessionID: config.Worker.Bootstrap.SessionID}, available: true, seen: make(map[inbox.ID]bool), model: config.Worker.Bootstrap.Model}
	wc := config.Worker
	wc.ControlScopePrefix = incarnation
	wc.OnItem = func(i sessionstore.Item) {
		p.item(i)
		if config.Worker.OnItem != nil {
			config.Worker.OnItem(i)
		}
	}
	wc.OnAdmission = func(i inbox.Input) {
		p.admission(i)
		if config.Worker.OnAdmission != nil {
			config.Worker.OnAdmission(i)
		}
	}
	wc.OnStatus = func(s workerhost.Status) {
		p.statusChanged(s)
		if config.Worker.OnStatus != nil {
			config.Worker.OnStatus(s)
		}
	}
	// Startup follows caller cancellation, but a ready host is shut down only
	// through Close with its own cleanup budget, not through caller cancellation.
	ownerCtx, ownerCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer ownerCancel()
	startupCanceled := make(chan struct{})
	disarmStartup := context.AfterFunc(ctx, func() { ownerCancel(); close(startupCanceled) })
	host, err := workerhost.Start(ownerCtx, wc)
	if !disarmStartup() {
		<-startupCanceled
	}
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result = errors.Join(result, host.Close(closeCtx))
	}()
	endpoint, err := attachproto.Listen(config.RuntimeDirectory, wc.Bootstrap.Directory, p.info.SessionID)
	if err != nil {
		return err
	}
	defer endpoint.Close()
	p.info.Endpoint = endpoint.Path
	store, err := localfile.New(wc.Bootstrap.Directory)
	if err != nil {
		return err
	}
	serving, cancel := context.WithCancel(ctx)
	defer cancel()
	var clients sync.WaitGroup
	handshake := make(chan struct{}, 1)
	acceptDone := make(chan error, 1)
	acceptStopped := make(chan struct{})
	go func() {
		defer close(acceptStopped)
		for {
			conn, err := endpoint.Listener.AcceptUnix()
			if err != nil {
				acceptDone <- err
				return
			}
			select {
			case handshake <- struct{}{}:
				clients.Add(1)
				go func() { defer clients.Done(); serveClient(serving, p, host, store, conn, func() { <-handshake }) }()
			default:
				_ = conn.Close()
			}
		}
	}()
	if ready != nil {
		ready(p.info)
	}
	select {
	case <-ctx.Done():
	case err := <-host.Done():
		result = err
		// An opt-in replacement failure is a stopped durable agent, not an
		// instruction to tear down its current control reply/ingress owner.
		// Keep the latched Host (no fallback/retry) until operator shutdown.
		if config.RuntimeReplacement && err != nil {
			select {
			case <-ctx.Done():
			case acceptErr := <-acceptDone:
				result = errors.Join(result, acceptErr)
			}
		}
	case err := <-acceptDone:
		result = err
	}
	cancel()
	_ = endpoint.Close()
	<-acceptStopped // Fence WaitGroup.Add before joining all client owners.
	clients.Wait()
	return result
}

func valid(r attachproto.Request, p *publisher, token string) bool {
	return r.Protocol == attachproto.Version && r.SessionID == p.info.SessionID && r.ID != "" && len(r.ID) <= 128 && r.Connection == token && r.After == 0
}

func serveClient(ctx context.Context, p *publisher, host *workerhost.Host, store *localfile.Store, conn *net.UnixConn, release func()) {
	defer conn.Close()
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	if attachproto.CheckPeer(conn) != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := attachproto.Scanner(conn)
	hello, err := attachproto.ReadRequest(scanner)
	if err != nil || hello.Type != "attach" || !valid(hello, p, "") || (hello.Input != nil || hello.Runtime != nil) {
		return
	}
	token, err := randomToken()
	if err != nil {
		return
	}
	c := &client{conn: conn, token: token, queue: make(chan []byte, attachproto.MaxQueuedFrames), stopped: make(chan struct{})}
	p.mu.Lock()
	if p.active != nil {
		e := p.event("error")
		e.ID = hello.ID
		e.Code = "busy"
		p.mu.Unlock()
		// Free the handshake slot before replying: a client that reads busy and
		// redials at once must not be shed by a slot this rejection still holds.
		release()
		released = true
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = attachproto.Write(conn, e)
		return
	}
	p.active = c
	snap := p.event("snapshot")
	snap.Base, snap.Code = p.cursorBase(hello.Cursor)
	status, model := p.status, p.model
	snap.Status = &status
	snap.Model = &model
	snap.Connection = token
	pending := append([]inbox.Input(nil), p.pending...)
	greeting := p.event("hello")
	greeting.ID = hello.ID
	greeting.Connection = token
	greeting.HostPID = p.info.HostPID
	greeting.Capabilities = []string{"fresh_snapshot", "history_watch", "submit_external", "cursor_snapshot", attachproto.CapabilityForegroundAbort, attachproto.CapabilitySessionShutdown, attachproto.CapabilityIntervention}
	if p.runtimeEnabled {
		greeting.Capabilities = append(greeting.Capabilities, attachproto.CapabilityRuntime, attachproto.CapabilityRuntimeDrain, attachproto.CapabilityHostRetire)
	}
	p.mu.Unlock()
	release()
	released = true
	defer func() {
		p.mu.Lock()
		p.stopClient(c)
		if p.active == c {
			p.active = nil
		}
		p.mu.Unlock()
	}()
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		select {
		case <-ctx.Done():
		case <-c.stopped:
		}
		p.mu.Lock()
		p.stopClient(c)
		p.mu.Unlock()
		_ = conn.Close()
	}()
	defer func() { c.stop(); <-monitorDone }()
	_ = conn.SetReadDeadline(time.Time{})
	snapshotCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	total := 0
	writeSnapshot := func(e attachproto.Event) error {
		if err := snapshotCtx.Err(); err != nil {
			return err
		}
		frame, err := attachproto.Encode(e)
		if err != nil {
			return err
		}
		total += len(frame)
		if total > 256<<20 {
			return errors.New("snapshot exceeds byte budget")
		}
		deadline := time.Now().Add(5 * time.Second)
		if d, ok := snapshotCtx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		_ = conn.SetWriteDeadline(deadline)
		return writeFrame(conn, frame)
	}
	if writeSnapshot(greeting) != nil || writeSnapshot(snap) != nil {
		return
	}
	for _, in := range pending {
		e := snap
		e.Type = "snapshot_pending"
		e.Base = nil
		e.Status = nil
		e.Model = nil
		e.Pending = []inbox.Input{in}
		if writeSnapshot(e) != nil {
			return
		}
	}
	var after sessionstore.Sequence
	if snap.Base != nil {
		after = snap.Base.Head
	}
	for after < snap.Head {
		page, err := store.Items(snapshotCtx, p.info.SessionID, after, 256)
		if err != nil || len(page.Items) == 0 {
			return
		}
		for _, item := range page.Items {
			if item.Sequence > snap.Head {
				break
			}
			e := snap
			e.Type = "snapshot_item"
			e.Base = nil
			e.Status = nil
			e.Model = nil
			e.Item = &item
			if writeSnapshot(e) != nil {
				return
			}
			after = item.Sequence
		}
		if page.NextAfter <= after && after < snap.Head && !page.More {
			return
		}
	}
	end := snap
	end.Type = "snapshot_end"
	end.Base = nil
	end.Status = nil
	end.Model = nil
	if writeSnapshot(end) != nil {
		return
	}
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer func() { p.mu.Lock(); p.stopClient(c); p.mu.Unlock() }()
		for {
			select {
			case <-c.stopped:
				return
			case frame := <-c.queue:
				p.mu.Lock()
				c.bytes -= len(frame)
				select {
				case <-c.stopped:
					p.mu.Unlock()
					return
				default:
				}
				p.mu.Unlock()
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if writeFrame(conn, frame) != nil {
					return
				}
			}
		}
	}()
	readRequests(ctx, p, host, c, scanner)
	p.mu.Lock()
	p.stopClient(c)
	p.mu.Unlock()
	<-writerDone
	c.runtimeWG.Wait()
}
func writeFrame(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func readRequests(ctx context.Context, p *publisher, host *workerhost.Host, c *client, scanner *bufio.Scanner) {
	for {
		r, err := attachproto.ReadRequest(scanner)
		if err != nil {
			return
		}
		if !valid(r, p, c.token) || r.Cursor != nil {
			return
		}
		if r.Type == "runtime" {
			if r.Input != nil || r.Runtime == nil || attachproto.ValidateRuntimeRequest(*r.Runtime) != nil {
				return
			}
			p.startRuntime(ctx, host, c, r)
			continue
		}
		if r.Runtime != nil {
			return
		}
		if r.Type == "detach" {
			if r.Input != nil {
				return
			}
			return
		}
		if r.Input == nil || r.Input.ID == "" || len(r.Input.ID) > 128 || len(r.Input.Payload) > attachproto.MaxPayload || r.Input.Validate() != nil {
			return
		}
		switch r.Type {
		case "submit":
			if r.Input.Kind != inbox.InputExternal {
				return
			}
		case "lifecycle":
			if attachproto.ValidateLifecycle(*r.Input, host.ControlScope()) != nil {
				return
			}
		case "intervention":
			if attachproto.ValidateIntervention(*r.Input, host.ControlScope()) != nil {
				return
			}
		default:
			return
		}
		in := *r.Input
		in.Payload = in.Payload.Clone()
		p.mu.Lock()
		// This is the admission linearization point. Once invalidated, even
		// requests already buffered by Scanner must never mutate the host.
		select {
		case <-c.stopped:
			p.mu.Unlock()
			return
		default:
		}
		if p.active != c || ctx.Err() != nil {
			p.mu.Unlock()
			return
		}
		allowed := p.canAdmit(in)
		p.mu.Unlock()
		var submitErr error
		if !allowed {
			submitErr = fmt.Errorf("pending mirror unavailable or full")
		} else if r.Type == "intervention" {
			submitErr = host.SubmitIntervention(ctx, in)
		} else if r.Type == "lifecycle" {
			submitErr = host.SubmitLifecycle(ctx, in)
		} else {
			submitErr = host.Submit(ctx, in)
		}
		if errors.Is(submitErr, ingress.ErrUncertain) {
			p.uncertain()
		}
		p.mu.Lock()
		e := p.event("accepted")
		e.ID = r.ID
		e.InputID = in.ID
		e.Connection = c.token
		if submitErr != nil {
			e.Type = "error"
			e.Error = submitErr.Error()
			e.Code = "not_accepted"
			if errors.Is(submitErr, ingress.ErrConflict) {
				e.Code = "conflict"
			}
			if errors.Is(submitErr, ingress.ErrUncertain) {
				e.Code = "uncertain"
			}
		}
		p.enqueue(c, e)
		p.mu.Unlock()
	}
}
