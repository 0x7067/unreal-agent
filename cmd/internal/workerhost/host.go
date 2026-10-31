// Package workerhost keeps the frontend and durable ingress alive while a
// private resident worker is replaced at a proven quiescent boundary.
package workerhost

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/lifecycleguard"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

var (
	ErrBusy    = errors.New("reload blocked by active or persistence-unconfirmed work")
	ErrClosed  = errors.New("worker host is closed")
	ErrStopped = errors.New("worker stopped; messages remain in durable ingress")
)

type Config struct {
	// ControlScopePrefix is stable for one host incarnation, never a replacement.
	ControlScopePrefix string
	Command            []string
	Candidate          []string
	Bootstrap          workerproto.Bootstrap
	OnItem             func(sessionstore.Item)
	// OnAdmission observes confirmed ingress acceptance (including recovered
	// pending inputs before worker launch). It runs before Submit returns, not
	// necessarily before an already-running dispatcher persists the input.
	// Callbacks must not block on client I/O or call back into the host.
	OnAdmission func(inbox.Input)
	OnStatus    func(Status)
	Stderr      io.Writer
	// ShutdownGrace bounds worker cleanup before forced termination (default 20s). Callers of
	// Close should allow additional time to join the process and release leases.
	ShutdownGrace time.Duration
}

type Status struct {
	Generation     uint64
	Build          string
	InstalledBuild string
	ReloadState    string
	Message        string
	Err            error
	Stopped        bool
}

// admissionJournal keeps durable ingress semantics independent of host lifecycle
// locking. Implementations must serialize their own operations, as ingress does.
type admissionJournal interface {
	Submit(context.Context, inbox.Input) error
	Acknowledge(context.Context, inbox.Input) error
	Pending() []inbox.Input
	Close() error
}

type Host struct {
	parentCanceled bool
	sealed         bool
	shutdown       *sessionShutdown
	stopParent     func() bool
	config         Config
	ctx            context.Context
	cancel         context.CancelFunc
	journal        admissionJournal
	parentLease    *localfile.Lease
	mu             sync.Mutex
	current        *process
	generation     uint64
	sequence       sessionstore.Sequence
	stages         []string
	closing        bool
	closed         bool
	stopped        bool // true only after a proven retirement; not arbitrary crash
	failure        error
	done           chan error
	doneOnce       sync.Once
	kick           chan struct{}
	dispatcherDone chan struct{}
	delivery       chan struct{} // serializes pipe delivery, cutoff, and intervention closure
	reload         atomic.Bool
	reloadDone     chan struct{}
	handoffState   *reloadHandoff
	admissions     sync.WaitGroup // Add only under mu, before closing is set
	admissionOnce  sync.Once
	admissionDone  chan struct{}
	query          chan struct{} // bounded one live snapshot query per host
	installedBuild string
}

type process struct {
	host          *Host
	command       *exec.Cmd
	input         io.WriteCloser
	cancel        context.CancelFunc
	generation    uint64
	controlScope  string
	cleanupProven atomic.Bool
	build         string
	hello         chan error
	ready         chan error
	done          chan struct{}
	mu            sync.Mutex
	err           error
	waiting       map[string]chan workerproto.Event
	sequence      sessionstore.Sequence
	sent          map[inbox.ID]bool // only under host.delivery
	draining      atomic.Bool       // process-lifetime delivery cutoff; never undone
	accepting     atomic.Bool       // ready handshake done; the first request must be start
	retired       atomic.Bool
	expectedStop  atomic.Bool
	waited        chan error
	scanDone      chan struct{}
}

func Start(ctx context.Context, config Config) (*Host, error) {
	return start(ctx, config, nil)
}

// wrapJournal is an internal fault-injection seam, applied before any owners run.
func start(ctx context.Context, config Config, wrapJournal func(admissionJournal) admissionJournal) (*Host, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if config.Bootstrap.SessionID == "" || strings.ContainsAny(string(config.Bootstrap.SessionID), "/\\") {
		return nil, errors.New("valid stable session ID is required")
	}
	if !filepath.IsAbs(config.Bootstrap.Directory) || !filepath.IsAbs(config.Bootstrap.Workspace) {
		return nil, errors.New("worker session and workspace paths must be absolute")
	}
	if err := os.MkdirAll(config.Bootstrap.Directory, 0700); err != nil {
		return nil, err
	}
	directory, err := filepath.EvalSymlinks(config.Bootstrap.Directory)
	if err != nil {
		return nil, err
	}
	config.Bootstrap.Directory = directory
	workspace, err := filepath.EvalSymlinks(config.Bootstrap.Workspace)
	if err != nil {
		return nil, err
	}
	config.Bootstrap.Workspace = workspace
	if config.ShutdownGrace <= 0 {
		config.ShutdownGrace = 20 * time.Second
	}
	if config.Stderr == nil {
		config.Stderr = io.Discard
	}
	if config.ControlScopePrefix == "" {
		config.ControlScopePrefix = uuid.New().String()
	}
	if len(config.ControlScopePrefix) > 128 || strings.TrimSpace(config.ControlScopePrefix) == "" {
		return nil, errors.New("invalid control scope prefix")
	}
	config.Command = append([]string(nil), config.Command...)
	config.Candidate = append([]string(nil), config.Candidate...)
	if len(config.Candidate) == 0 {
		config.Candidate = append([]string(nil), config.Command...)
	}
	// Validate the shared state namespace before executing even a probe. A
	// separate parent lease also fences duplicate frontends for this session.
	parentLease, err := localfile.AcquireLease(directory, session.ID("host-"+string(config.Bootstrap.SessionID)))
	if err != nil {
		return nil, fmt.Errorf("own session frontend: %w", err)
	}
	root := filepath.Join(directory, "ingress")
	if err = privateDirectory(root); err != nil {
		_ = parentLease.Close()
		return nil, err
	}
	durable, err := ingress.Open(root, config.Bootstrap.SessionID)
	if err != nil {
		_ = parentLease.Close()
		return nil, fmt.Errorf("open durable ingress: %w", err)
	}
	if err := lifecycleguard.ValidatePending(durable.Pending()); err != nil {
		_ = durable.Close()
		_ = parentLease.Close()
		return nil, err
	}
	var journal admissionJournal = durable
	if wrapJournal != nil {
		journal = wrapJournal(journal)
	}
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	h := &Host{config: config, ctx: lifetime, cancel: cancel, journal: journal, parentLease: parentLease, done: make(chan error, 1), kick: make(chan struct{}, 1), dispatcherDone: make(chan struct{}), delivery: make(chan struct{}, 1), admissionDone: make(chan struct{}), query: make(chan struct{}, 1)}
	h.stopParent = context.AfterFunc(ctx, func() {
		h.mu.Lock()
		h.parentCanceled = true
		// A parent cancellation may request legacy teardown, but cannot
		// interrupt an independently owned, durably fenced shutdown.
		if !h.sealed {
			h.cancel()
		}
		h.mu.Unlock()
	})
	if config.OnAdmission != nil {
		for _, input := range journal.Pending() {
			input.Payload = input.Payload.Clone()
			config.OnAdmission(input)
		}
	}
	// Start must be bounded even if a candidate never identifies itself.
	startCtx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	stage, err := h.stage(startCtx, config.Command)
	if err == nil {
		_, err = h.launch(startCtx, stage)
	}
	if err != nil {
		h.mu.Lock()
		h.closing = true
		p := h.current
		h.mu.Unlock()
		if p != nil {
			p.expectedStop.Store(true)
			p.cancel()
			_ = p.input.Close()
			<-p.done
		}
		if h.stopParent != nil {
			h.stopParent()
		}
		cancel()
		_ = journal.Close()
		_ = parentLease.Close()
		h.removeStages()
		return nil, err
	}
	go h.dispatch()
	h.wake()
	return h, nil
}

func (h *Host) Done() <-chan error { return h.done }
func (h *Host) finish(err error)   { h.doneOnce.Do(func() { h.done <- err; close(h.done) }) }
func (h *Host) wake() {
	select {
	case h.kick <- struct{}{}:
	default:
	}
}
func (h *Host) status(s Status) {
	h.mu.Lock()
	s.InstalledBuild = h.installedBuild
	h.mu.Unlock()
	if h.reload.Load() {
		s.ReloadState = "reload in progress"
	} else if s.Build != "" && s.InstalledBuild == s.Build {
		s.ReloadState = "running worker matches checked installed candidate"
	} else if s.Build != "" && strings.HasPrefix(s.InstalledBuild, s.Build+" (launch candidate;") {
		s.ReloadState = "running matches launch candidate; /builds rechecks installed path"
	} else if s.InstalledBuild != "" && s.Build != "" {
		s.ReloadState = "installed candidate differs; explicit /reload required"
	} else {
		s.ReloadState = "installed candidate not checked; /builds"
	}
	if h.config.OnStatus != nil {
		h.config.OnStatus(s)
	}
}

// Submit acknowledges durable ingress ONLY. Worker/session persistence and model
// delivery remain separate states. New messages may be admitted during reload.
func (h *Host) Submit(ctx context.Context, input inbox.Input) error {
	// Store the wire-normalized identity: JSON framing and the session store
	// compact raw JSON payloads. Whitespace outside JSON strings is not a new
	// user message; string contents and conflicting values remain distinct.
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(encoded, &input); err != nil {
		return err
	}
	h.mu.Lock()
	if h.closing || h.closed || h.sealed {
		h.mu.Unlock()
		return ErrClosed
	}
	// Close fences future Add calls under the same mutex, then joins accepted
	// admissions before closing the journal. Never hold lifecycle mu over fsync.
	h.admissions.Add(1)
	h.mu.Unlock()
	defer h.admissions.Done()
	if err := h.journal.Submit(ctx, input); err != nil {
		return err
	}
	if h.config.OnAdmission != nil {
		input.Payload = input.Payload.Clone()
		h.config.OnAdmission(input)
	}
	h.wake()
	return nil
}

func (h *Host) dispatch() {
	defer close(h.dispatcherDone)
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.kick:
		}
		if err := h.lockDelivery(h.ctx); err != nil {
			return
		}
		h.mu.Lock()
		p := h.current
		closing := h.closing
		failure := h.failure
		h.mu.Unlock()
		if p != nil && p.accepting.Load() && !closing && failure == nil && !p.retired.Load() {
			retiring := p.draining.Load()
			for _, input := range h.journal.Pending() {
				if retiring && inbox.ValidateIntervention(input, p.controlScope) != nil {
					continue
				}
				if p.sent[input.ID] {
					continue
				}
				p.sent[input.ID] = true // failed writes have ambiguous admission; never blindly retry live worker
				if err := workerproto.Write(p.input, workerproto.Request{Type: "input", Protocol: workerproto.Version, Input: &input}); err != nil {
					p.cancel()
					h.status(Status{Generation: p.generation, Build: p.build, Message: "Worker input transport failed; input retained", Err: err})
					break
				}
			}
		}
		h.unlockDelivery()
	}
}

func (h *Host) launch(ctx context.Context, stage staged) (*process, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	processCtx, cancel := context.WithCancel(h.ctx)
	command := exec.CommandContext(processCtx, stage.command[0], stage.command[1:]...)
	command.Dir = h.config.Bootstrap.Workspace
	command.Stderr = h.config.Stderr
	isolateProcess(command)
	// Interrupt permits worker-owned cleanup. WaitDelay is a last-resort process
	// boundary, not evidence that arbitrary detached external effects were joined.
	var forceMu sync.Mutex
	var forceTimer *time.Timer
	command.Cancel = func() error {
		forceMu.Lock()
		forceTimer = time.AfterFunc(h.config.ShutdownGrace, func() { _ = killProcessGroup(command) })
		forceMu.Unlock()
		err := command.Process.Signal(os.Interrupt)
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	command.WaitDelay = h.config.ShutdownGrace
	// Own stdout ourselves: Wait must run concurrently with decoding so that
	// WaitDelay also bounds inherited stderr pipes after an unexpected exit.
	// StdoutPipe cannot be used here: Wait would close it before replay drained.
	output, childOutput, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, err
	}
	command.Stdout = childOutput
	input, err := command.StdinPipe()
	if err != nil {
		cancel()
		_ = childOutput.Close()
		_ = output.Close()
		return nil, err
	}
	h.mu.Lock()
	h.generation++
	if h.installedBuild == "" && slices.Equal(h.config.Command, h.config.Candidate) {
		h.installedBuild = stage.identity.Build + " [" + stage.digest[:12] + "] (launch candidate; recheck with /builds)"
	}
	generation := h.generation
	h.mu.Unlock()
	p := &process{host: h, command: command, input: input, cancel: cancel, generation: generation, controlScope: fmt.Sprintf("%s/%d", h.config.ControlScopePrefix, generation), build: stage.identity.Build + " [" + stage.digest[:12] + "]", hello: make(chan error, 1), ready: make(chan error, 1), done: make(chan struct{}), waiting: make(map[string]chan workerproto.Event), sent: make(map[inbox.ID]bool), waited: make(chan error, 1), scanDone: make(chan struct{})}
	if err = command.Start(); err != nil {
		_ = childOutput.Close()
		cancel()
		_ = output.Close()
		_ = input.Close()
		return nil, err
	}
	h.mu.Lock()
	h.current = p
	h.stopped = false
	h.mu.Unlock()
	_ = childOutput.Close()
	// Keep decoding during graceful cancellation: closing stdout immediately can
	// turn a final persistence/error frame into SIGPIPE during owner cleanup.
	go func() {
		err := command.Wait()
		forceMu.Lock()
		if forceTimer != nil {
			forceTimer.Stop()
		}
		forceMu.Unlock()
		// The direct child has been joined. Clean any abandoned same-group writers
		// before waiting for EOF; otherwise their inherited stdout can block forever.
		err = errors.Join(err, killProcessGroup(command))
		p.waited <- err
		// Do not close stdout on a timer: decoding may still be draining buffered
		// frames. An escaped writer outside this process group can prevent EOF;
		// Close must then report its caller's deadline, not claim a lossless join.
	}()
	go p.consume(output, stage.identity)
	if err = p.await(ctx, p.hello); err != nil {
		return p, err
	}
	bootstrap := h.config.Bootstrap
	bootstrap.ControlScope = p.controlScope
	// A candidate launches descendants from its staged executable, never from an
	// old frontend's path, and it must include embedded runner dispatch.
	bootstrap.SubagentCommand = []string{stage.command[0], "agent-runner"}
	// Snapshot before the worker can replay and acknowledge. These frames are
	// written, in this order, immediately after ready and before dispatch.
	startup := h.journal.Pending()
	bootstrap.StartupInputCount = len(startup)
	if err = workerproto.Write(input, workerproto.Request{Type: "start", Protocol: workerproto.Version, Bootstrap: &bootstrap}); err != nil {
		return p, err
	}
	if err = p.await(ctx, p.ready); err != nil {
		return p, err
	}
	// Hold delivery so dispatch cannot interleave pipe writes. accepting stays
	// false until the launch snapshot is on the pipe; inputs admitted during
	// handshake are still delivered by the wake below.
	if err = h.lockDelivery(ctx); err != nil {
		return p, err
	}
	for i := range startup {
		in := startup[i]
		p.sent[in.ID] = true
		if err = workerproto.Write(input, workerproto.Request{Type: "input", Protocol: workerproto.Version, Input: &in}); err != nil {
			h.unlockDelivery()
			return p, err
		}
	}
	p.accepting.Store(true)
	h.unlockDelivery()
	h.wake()
	h.status(Status{Generation: p.generation, Build: p.build, Message: "Worker ready"})
	return p, nil
}
func (p *process) await(ctx context.Context, ch <-chan error) error {
	select {
	case err := <-ch:
		return err
	case <-p.done:
		return p.result()
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
func (p *process) result() error { p.mu.Lock(); defer p.mu.Unlock(); return p.err }

func (p *process) consume(output io.ReadCloser, expected workerproto.Identity) {
	queue := newEventMailbox()
	go p.decode(output, queue)
	sawHello, sawReady := false, false
	var failure error
	for {
		event, err, ok := queue.next()
		if !ok {
			failure = err
			break
		}
		if !sawHello && event.Type != "hello" {
			failure = errors.New("worker event before hello")
			break
		}
		switch event.Type {
		case "hello":
			if sawHello || event.Identity == nil {
				failure = errors.New("invalid repeated worker hello")
				break
			}
			if err := event.Identity.Validate(); err != nil {
				failure = err
				break
			}
			if *event.Identity != expected {
				failure = errors.New("worker identity changed after probe")
				break
			}
			sawHello = true
			p.hello <- nil
		case "item":
			if event.Item == nil {
				failure = errors.New("worker item is missing")
				break
			}
			item := *event.Item
			if item.Sequence != p.sequence+1 {
				failure = fmt.Errorf("worker event sequence gap: got %d after %d", item.Sequence, p.sequence)
				break
			}
			p.sequence = item.Sequence
			if input, ok := item.Data.(inbox.Input); ok {
				if input.Kind == inbox.InputControl {
					control, err := input.DecodeControlMessage()
					if err != nil {
						failure = err
						break
					}
					if sawReady && control.Mode == inbox.ExecutionDrained && control.Parameters.(inbox.ExecutionDrainReceipt).Scope == p.controlScope {
						p.cleanupProven.Store(true)
					}
				}
				if err := p.host.journal.Acknowledge(context.Background(), input); err != nil {
					failure = fmt.Errorf("acknowledge durable ingress: %w", err)
					break
				}
			}
			p.host.mu.Lock()
			last := p.host.sequence
			if item.Sequence > last {
				if item.Sequence != last+1 {
					failure = errors.New("worker transcript lost continuity")
				} else {
					p.host.sequence = item.Sequence
				}
			}
			p.host.mu.Unlock()
			if failure == nil && item.Sequence > last && p.host.config.OnItem != nil {
				p.host.config.OnItem(item)
			}
		case "ready":
			if sawReady {
				failure = errors.New("repeated worker ready")
				break
			}
			p.host.mu.Lock()
			last := p.host.sequence
			p.host.mu.Unlock()
			if p.sequence < last {
				failure = errors.New("worker replay is shorter than previously observed history")
				break
			}
			sawReady = true
			p.ready <- nil
		case "drained", "activity_snapshot":
			p.mu.Lock()
			waiter := p.waiting[event.ID]
			delete(p.waiting, event.ID)
			p.mu.Unlock()
			if waiter == nil && event.Type == "activity_snapshot" {
				// A canceled live query can finish later; it has no side effect.
				continue
			}
			if waiter == nil {
				failure = errors.New("unsolicited worker control response")
				break
			}
			if event.Type == "drained" {
				// The same ordered gate admits interventions and closes their lane.
				// Previously admitted but unpersisted controls remain in ingress;
				// their exact old scope makes replacement reconciliation inert.
				if err := p.host.lockDelivery(p.host.ctx); err != nil {
					failure = err
					break
				}
				p.retired.Store(true)
				p.host.unlockDelivery()
			}
			waiter <- event
		case "error":
			failure = fmt.Errorf("worker: %s", event.Error)
		default:
			failure = fmt.Errorf("unknown worker event %q", event.Type)
		}
		if failure != nil {
			break
		}
	}
	if failure == nil && !p.retired.Load() && !p.expectedStop.Load() {
		// EOF is a transport failure, not proof of process exit. A live worker
		// that closes stdout must not leave Wait (and the session lease) stuck.
		failure = errors.New("worker output closed unexpectedly")
	}
	if failure != nil {
		p.cancel()
		_ = p.input.Close()
		_ = output.Close()
	}
	<-p.scanDone
	// Both ordered output consumption and process Wait are handoff barriers.
	waitErr := <-p.waited
	_ = output.Close()
	_ = p.input.Close()
	p.cancel()
	if !sawReady {
		failure = errors.Join(failure, errors.New("worker exited before ready"))
	}
	failure = errors.Join(failure, waitErr)
	if failure == nil && !p.retired.Load() && !p.expectedStop.Load() {
		failure = errors.New("worker exited unexpectedly")
	}
	p.mu.Lock()
	p.err = failure
	p.mu.Unlock()
	p.host.mu.Lock()
	owner := p.host.current == p
	if owner {
		if !(p.retired.Load() && failure == nil) && !p.host.closing {
			p.host.failure = errors.Join(ErrStopped, failure)
		}
	}
	closing := p.host.closing
	p.host.mu.Unlock()
	p.host.discardStage(p.command.Path)
	// Every joined retirement must be visible even if its Reload caller has
	// already canceled. A clean retirement is recoverable, not a host run end.
	reportStopped := owner && !closing
	p.host.mu.Lock()
	controlledShutdown := p.host.shutdown != nil && p.host.shutdown.process == p
	p.host.mu.Unlock()
	finishHost := reportStopped && !controlledShutdown && (!p.retired.Load() || failure != nil)
	if reportStopped {
		p.host.status(Status{Generation: p.generation, Build: p.build, Message: "Worker stopped; durable inputs retained", Err: failure, Stopped: true})
	}
	// Keep current visible to Close until every process callback has returned.
	p.host.mu.Lock()
	if p.host.current == p {
		p.host.current = nil
	}
	p.host.mu.Unlock()
	close(p.done)
	if finishHost {
		p.host.finish(errors.Join(ErrStopped, failure))
	}
}

// Reload stages once and establishes an ordered input cutoff. Busy work drains
// naturally; only exact scoped interventions pass the retiring generation.
// After admission ctx cancels the wait, never retirement or eventual replacement.
func (h *Host) Reload(ctx context.Context, candidate []string) error {
	return h.handoff(ctx, candidate, false)
}

// RetireForHostReplacement uses the same drain and joins but seals admission
// instead of starting a candidate. Ingress ownership survives until Close.
func (h *Host) RetireForHostReplacement(ctx context.Context) error {
	return h.handoff(ctx, nil, true)
}

type reloadHandoff struct {
	done   chan struct{}
	result error // published by closing done
}

// ReloadPending is observation only, never cleanup or exit proof.
func (h *Host) ReloadPending() bool { return h.reload.Load() }

// AwaitReload observes the host-owned result after a disconnected/canceled wait.
func (h *Host) AwaitReload(ctx context.Context) error {
	h.mu.Lock()
	state := h.handoffState
	h.mu.Unlock()
	if state == nil {
		return errors.New("no reload admitted")
	}
	return awaitReload(ctx, state)
}
func awaitReload(ctx context.Context, state *reloadHandoff) error {
	select {
	case <-state.done:
		return state.result
	case <-ctx.Done():
		return fmt.Errorf("reload wait ended; host still owns any admitted retirement: %w", context.Cause(ctx))
	}
}

func (h *Host) handoff(ctx context.Context, candidate []string, retireHost bool) error {
	if !h.reload.CompareAndSwap(false, true) {
		return fmt.Errorf("%w: another reload is in progress", ErrBusy)
	}
	armed := false
	state := &reloadHandoff{done: make(chan struct{})}
	h.mu.Lock()
	closing, failure := h.closing || h.sealed, h.failure
	h.reloadDone = state.done
	h.mu.Unlock()
	defer func() {
		if !armed {
			h.reload.Store(false)
			close(state.done)
		}
	}()
	if closing {
		return ErrClosed
	}
	if failure != nil {
		return failure
	}
	var stage staged
	var err error
	if !retireHost {
		if len(candidate) == 0 {
			candidate = h.config.Candidate
		}
		stage, err = h.stage(ctx, candidate)
		if err != nil {
			return fmt.Errorf("candidate rejected; current worker unchanged: %w", err)
		}
		defer func() {
			if !armed {
				h.discardStage(stage.command[0])
			}
		}()
	}
	if err := h.lockDelivery(ctx); err != nil {
		return err
	}
	h.mu.Lock()
	p, safeStopped := h.current, h.stopped
	closing, failure = h.closing || h.sealed, h.failure
	if err = context.Cause(ctx); err == nil && !closing && failure == nil && (p != nil || safeStopped) {
		// This is the irreversible admission boundary. The host, not this wait,
		// owns the pipe write and every subsequent cleanup/replacement step.
		h.handoffState, armed = state, true
		if p != nil {
			p.draining.Store(true)
		}
	}
	h.mu.Unlock()
	if !armed {
		h.unlockDelivery()
		if err != nil {
			return err
		}
		if closing {
			return ErrClosed
		}
		if failure != nil {
			return failure
		}
		return ErrStopped
	}
	go func() {
		state.result = h.completeHandoff(p, safeStopped, stage, retireHost)
		h.reload.Store(false)
		close(state.done)
		h.wake()
	}()
	return awaitReload(ctx, state)
}

// completeHandoff receives delivery ownership and always releases it after the
// ordered stop write. No client context is allowed past the admission boundary.
func (h *Host) completeHandoff(p *process, safeStopped bool, stage staged, retireHost bool) (result error) {
	used := false
	defer func() {
		if result != nil {
			h.mu.Lock()
			if !h.closing && h.current == nil && !h.stopped {
				h.failure = errors.Join(ErrStopped, result)
			}
			h.mu.Unlock()
		}
		if !retireHost && !used {
			h.discardStage(stage.command[0])
		}
	}()
	if p != nil {
		id := uuid.New().String()
		reply := make(chan workerproto.Event, 1)
		p.mu.Lock()
		p.waiting[id] = reply
		p.mu.Unlock()
		err := workerproto.Write(p.input, workerproto.Request{Type: "drain", Protocol: workerproto.Version, ID: id})
		h.unlockDelivery()
		h.wake()
		defer func() { p.mu.Lock(); delete(p.waiting, id); p.mu.Unlock() }()
		if err != nil {
			p.cancel()
			return fmt.Errorf("drain transport uncertain; no replacement started: %w", err)
		}
		h.status(Status{Generation: p.generation, Build: p.build, Message: "Replacement scheduled; ordinary submissions queued; scoped graph/helper interventions remain available"})
		select {
		case <-reply:
		case <-p.done:
			select {
			case <-reply:
			default:
				return errors.Join(ErrStopped, p.result())
			}
		case <-h.ctx.Done():
			return ErrClosed
		}
		select {
		case <-p.done:
		case <-h.ctx.Done():
			return ErrClosed
		}
		if err := p.result(); err != nil {
			return fmt.Errorf("old worker cleanup/exit failed: %w", err)
		}
		if !p.cleanupProven.Load() {
			return errors.New("old worker cleanup proof missing; no replacement started")
		}
		// Actually acquire/release the writer lease after the old process and all
		// output callbacks joined. A receipt or successful exit alone is not enough.
		lease, err := localfile.AcquireLease(h.config.Bootstrap.Directory, h.config.Bootstrap.SessionID)
		if err != nil {
			return fmt.Errorf("old worker lease release unconfirmed: %w", err)
		}
		if err := lease.Close(); err != nil {
			return err
		}
		safeStopped = true
		h.mu.Lock()
		h.stopped = true // only cleanup receipt + process/output joins + lease probe authorize this
		h.mu.Unlock()
	} else {
		h.unlockDelivery()
	}
	if !safeStopped {
		return ErrStopped
	}
	if retireHost {
		h.mu.Lock()
		h.sealed, h.failure = true, ErrStopped
		h.mu.Unlock()
		h.admissionOnce.Do(func() { go func() { h.admissions.Wait(); close(h.admissionDone) }() })
		<-h.admissionDone
		h.status(Status{Message: "Worker drained and joined; ingress retained until explicit host stop", Stopped: true})
		h.finish(ErrStopped)
		return nil
	}
	h.status(Status{Message: "Old worker exited; starting replacement", Stopped: true})
	// Close sets closing before canceling h.ctx and joins this handoff before
	// releasing ingress or leases, including any racing startup process.
	h.mu.Lock()
	closing := h.closing || h.sealed
	h.mu.Unlock()
	if closing {
		return ErrClosed
	}
	startup, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()
	next, err := h.launch(startup, stage)
	used = next != nil
	if err != nil {
		if next != nil {
			next.expectedStop.Store(true)
			next.cancel()
			_ = next.input.Close()
			<-next.done
		}
		h.status(Status{Message: "Replacement failed; session stopped, no automatic downgrade", Err: err, Stopped: true})
		return fmt.Errorf("replacement failed after retirement: %w", err)
	}
	return nil
}

// SubmitIntervention acknowledges durable admission only, not consumption or
// drain. Delivery/closure is generation-bound; an undelivered stale intent is
// reconciled as failed, never applied to a replacement generation.
func (h *Host) SubmitIntervention(ctx context.Context, input inbox.Input) error {
	if err := h.lockDelivery(ctx); err != nil {
		return err
	}
	defer h.unlockDelivery()
	h.mu.Lock()
	p := h.current
	closed := h.closing || h.closed || h.sealed || h.failure != nil
	h.mu.Unlock()
	if p == nil || closed || p.retired.Load() {
		return ErrStopped
	}
	if err := inbox.ValidateIntervention(input, p.controlScope); err != nil {
		return err
	}
	return h.Submit(ctx, input)
}

// Close is explicit user shutdown, not reload: canceling active work is allowed.
// A deadline never reports that the worker or lease has finished.
func (h *Host) Close(ctx context.Context) error {
	h.mu.Lock()
	if h.shutdown != nil {
		h.mu.Unlock()
		return h.AwaitShutdown(ctx)
	}

	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closing = true
	p, reloadDone := h.current, h.reloadDone
	h.mu.Unlock()
	// Cancel the host lifetime before waiting for a handoff. This prevents a
	// candidate launch racing a nil/current snapshot and interrupts active probes.
	if p != nil {
		p.expectedStop.Store(true)
		p.cancel()
		_ = p.input.Close()
	}
	h.cancel()
	// closing was set under mu before Wait can begin: no later Submit can Add.
	// One waiter for the whole host, not one goroutine per admission or Close.
	h.admissionOnce.Do(func() {
		go func() {
			h.admissions.Wait()
			close(h.admissionDone)
		}()
	})
	if reloadDone != nil {
		select {
		case <-reloadDone:
		case <-ctx.Done():
			return fmt.Errorf("reload cleanup incomplete: %w", context.Cause(ctx))
		}
	}
	select {
	case <-h.dispatcherDone:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	if err := h.lockDelivery(ctx); err != nil {
		return err
	}
	defer h.unlockDelivery()
	// Re-read after every launch path is fenced, not the pre-handoff snapshot.
	h.mu.Lock()
	p = h.current
	h.mu.Unlock()
	if p != nil {
		p.expectedStop.Store(true)
		p.cancel()
		_ = p.input.Close()
		select {
		case <-p.done:
		case <-ctx.Done():
			return fmt.Errorf("worker shutdown incomplete: %w", context.Cause(ctx))
		}
	}
	select {
	case <-h.admissionDone:
	case <-ctx.Done():
		return fmt.Errorf("durable admission cleanup incomplete: %w", context.Cause(ctx))
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	if h.stopParent != nil {
		h.stopParent()
	}
	err := errors.Join(h.journal.Close(), h.parentLease.Close())
	h.closed = true
	h.finish(err)
	for _, name := range h.stages {
		_ = os.Remove(name)
	}
	return err
}

// Kill is the user's forced exit after Close has started: the current worker's
// process group is stopped immediately with no graceful cleanup, and Close
// then joins the exit. Shell tools own separate groups and may be left behind.
func (h *Host) Kill() error {
	h.mu.Lock()
	p := h.current
	h.mu.Unlock()
	if p == nil {
		return nil
	}
	p.expectedStop.Store(true)
	return killProcessGroup(p.command)
}

func (h *Host) removeStages() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, name := range h.stages {
		_ = os.Remove(name)
	}
}

func (h *Host) lockDelivery(ctx context.Context) error {
	select {
	case h.delivery <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
func (h *Host) unlockDelivery() { <-h.delivery }

// Only unlaunched probes and fully exited worker generations are discarded.
func (h *Host) discardStage(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, name := range h.stages {
		if name == path {
			_ = os.Remove(name)
			h.stages = append(h.stages[:i], h.stages[i+1:]...)
			return
		}
	}
}
