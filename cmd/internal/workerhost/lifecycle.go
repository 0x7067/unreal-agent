package workerhost

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

var ErrShutdownUncertain = errors.New("session shutdown incomplete; host authority remains fenced")

type sessionShutdown struct {
	input          inbox.Input
	identity       []byte
	process        *process
	accepted, done chan struct{}
	// Set before closing the corresponding channel. Readers must await it.
	admissionErr, result error
}

// ShutdownSealed is diagnostic state, not a cleanup receipt. A sealed host keeps
// its parent authority even when its worker is gone or a caller stops waiting.
func (h *Host) ShutdownSealed() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.sealed }

func (h *Host) ControlScope() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.shutdown != nil {
		return h.shutdown.process.controlScope
	}
	if h.current == nil {
		return ""
	}
	return h.current.controlScope
}

// SubmitLifecycle accepts only scoped, typed public intents. A nil result proves
// durable ingress admission, NOT work cleanup, worker exit, or authority release.
// The request context cancels only admission waiting; shutdown has its own owner.
func (h *Host) SubmitLifecycle(ctx context.Context, input inbox.Input) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	identity, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(identity, &input); err != nil {
		return err
	}
	h.mu.Lock()
	if existing := h.shutdown; existing != nil {
		if input.ID != existing.input.ID || !bytes.Equal(identity, existing.identity) {
			h.mu.Unlock()
			return ErrClosed
		}
		h.mu.Unlock()
		return awaitAdmission(ctx, existing)
	}
	p := h.current
	if h.closing || h.closed || h.sealed || p == nil || h.failure != nil || context.Cause(h.ctx) != nil {
		h.mu.Unlock()
		return ErrStopped
	}
	if err = attachproto.ValidateLifecycle(input, p.controlScope); err != nil {
		h.mu.Unlock()
		return err
	}
	control, _ := input.DecodeControlMessage()
	if control.Mode == inbox.AbortForeground {
		h.admissions.Add(1)
		h.mu.Unlock()
		defer h.admissions.Done()
		return h.admitLifecycle(ctx, input)
	}
	if h.reload.Load() {
		h.mu.Unlock()
		return ErrBusy
	}
	// Seal before Wait: every old admission has already Add'ed under this mutex;
	// no later admission can race the durable intent or be lost behind shutdown.
	state := &sessionShutdown{input: input, identity: identity, process: p, accepted: make(chan struct{}), done: make(chan struct{})}
	h.sealed = true
	h.shutdown = state
	go h.runShutdown(state)
	h.mu.Unlock()
	return awaitAdmission(ctx, state)
}

func awaitAdmission(ctx context.Context, s *sessionShutdown) error {
	select {
	case <-s.accepted:
		return s.admissionErr
	case <-ctx.Done():
		return fmt.Errorf("shutdown admission outcome unconfirmed: %w", errors.Join(ingress.ErrUncertain, context.Cause(ctx)))
	}
}

func (h *Host) admitLifecycle(ctx context.Context, input inbox.Input) error {
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

// AwaitShutdown never cancels cleanup, kills a process, closes ingress, or drops a
// lease. In particular Close cannot bypass this fence after explicit admission.
func (h *Host) AwaitShutdown(ctx context.Context) error {
	h.mu.Lock()
	state := h.shutdown
	h.mu.Unlock()
	if state == nil {
		return errors.New("no deliberate session shutdown")
	}
	select {
	case <-state.done:
		return state.result
	case <-ctx.Done():
		return errors.Join(ErrShutdownUncertain, context.Cause(ctx))
	}
}

func (h *Host) runShutdown(state *sessionShutdown) {
	h.admissions.Wait()
	// Background is intentional. No client/parent deadline authorizes abandoning
	// a possibly persisted stop intent or canceling its cleanup owner.
	// No cancellation/force timer: the durable control initiates graceful drain.
	state.process.expectedStop.Store(true)
	err := h.admitLifecycle(context.Background(), state.input)
	state.admissionErr = err
	if errors.Is(err, ingress.ErrConflict) {
		// Definitive identity rejection committed no intent. Restore normal
		// admission when the same worker is still present; unknown writes NEVER
		// take this path. Existing admissions have already joined.
		h.mu.Lock()
		if h.current == state.process && h.shutdown == state {
			state.process.expectedStop.Store(false)
			h.sealed = false
			h.shutdown = nil
			if h.parentCanceled {
				h.cancel()
			}
		}
		h.mu.Unlock()
		state.result = err
		close(state.accepted)
		close(state.done)
		return
	}
	close(state.accepted)
	if err == nil {
		h.wake()
		<-state.process.done // exact process + private output decoder + callbacks joined
		if state.process.result() != nil || !state.process.cleanupProven.Load() {
			err = errors.Join(ErrShutdownUncertain, state.process.result())
		} else {
			err = h.finalizeShutdown(state)
		}
	}
	if err != nil {
		state.result = errors.Join(ErrShutdownUncertain, err)
		h.status(Status{Generation: state.process.generation, Build: state.process.build, Message: "Shutdown unproven; host authority retained, automatic takeover forbidden", Err: state.result, Stopped: true})
		close(state.done)
		return
	}
	// Completed canonical outcome committed under parent authority. Only now may
	// delivery/journal/stages and the final host lease be released.
	h.mu.Lock()
	h.closing = true
	h.mu.Unlock()
	if h.stopParent != nil {
		h.stopParent()
	}
	h.cancel()
	<-h.dispatcherDone
	_ = h.lockDelivery(context.Background())
	err = errors.Join(h.journal.Close(), h.parentLease.Close())
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	h.removeStages()
	h.unlockDelivery()
	state.result = err
	close(state.done)
	h.finish(err)
}

func (h *Host) finalizeShutdown(state *sessionShutdown) (result error) {
	directory, id := h.config.Bootstrap.Directory, h.config.Bootstrap.SessionID
	lease, err := localfile.AcquireLease(directory, id)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, lease.Close()) }()
	store, err := localfile.New(directory)
	if err != nil {
		return err
	}
	history, err := sessionstore.ReadLifecycle(context.Background(), store, id)
	if err != nil {
		return err
	}
	outcome, ok := history.ShutdownResults[state.input.ID]
	if !ok || outcome.Scope != state.process.controlScope || outcome.Phase != "work_drained" {
		return errors.New("missing exact canonical work-drained outcome")
	}
	// p.cleanupProven is fresh private evidence (post-ready only), not a replayed
	// historical receipt, terminal operation flag, EOF, or missing PID.
	control, _ := state.input.DecodeControlMessage()
	if history.Shutdowns[state.input.ID] != control.Parameters.(inbox.ShutdownSessionRequest) {
		return errors.New("canonical shutdown identity mismatch")
	}
	var publicationErr error
	observer := store.AddObserver(func(_ session.ID, item sessionstore.Item) {
		h.mu.Lock()
		if item.Sequence != h.sequence+1 {
			publicationErr = errors.New("host final outcome lost canonical continuity")
		} else {
			h.sequence = item.Sequence
		}
		h.mu.Unlock()
		if publicationErr == nil && h.config.OnItem != nil {
			h.config.OnItem(item)
		}
	})
	defer store.RemoveObserver(observer)
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.SessionShutdownFinished, Parameters: inbox.SessionShutdownResult{Version: 1, InputID: state.input.ID, Scope: state.process.controlScope, Phase: "completed"}})
	if err != nil {
		return err
	}
	err = store.AppendInput(context.Background(), id, inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload})
	return errors.Join(err, publicationErr)
}
