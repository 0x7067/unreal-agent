package durablechild

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Handle separates immutable execution linkage from a freshly bound parent
// control capability. Neither a saved capability nor a PID grants reattachment.
type Handle struct {
	Version          int
	SessionID        string
	Handle           string
	Generation       uint64
	Link             Link
	Phase            string
	AnswerSequence   sessionstore.Sequence `json:",omitzero"`
	AnswerTurnID     session.TurnID        `json:",omitzero"`
	ResultDeliveryID string
	Diagnostic       string `json:",omitzero"`
}
type HandlerConfig struct {
	Owner    Config
	ParentID session.ID
	Hub      *subagent.Hub
}
type Handler struct {
	ctx     context.Context
	config  HandlerConfig
	mu      sync.Mutex
	entries map[operation.ID]*entry
	bytes   int
	closed  bool
	updates chan operation.Operation
	workers sync.WaitGroup
	done    chan struct{}
}
type entry struct {
	mu         sync.Mutex
	original   operation.Operation
	plan       toolsubagent.DurableRunPlan
	request    Request
	connection *Connection
	handle     Handle
	wake       chan struct{}
	finished   bool
	answer     string
	launch     *Connection
}

func NewHandler(ctx context.Context, cfg HandlerConfig) (*Handler, error) {
	if cfg.ParentID == "" || cfg.Hub == nil {
		return nil, errors.New("durable child requires parent and fresh control hub")
	}
	h := &Handler{ctx: ctx, config: cfg, entries: map[operation.ID]*entry{}, updates: make(chan operation.Operation), done: make(chan struct{})}
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		h.workers.Wait()
		close(h.updates)
		close(h.done)
	}()
	return h, nil
}
func (*Handler) RemoteJobPlanType() operation.RemoteJobPlanType { return toolsubagent.RunPlanType }
func (*Handler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion {
	return toolsubagent.DurablePlanVersion
}
func (h *Handler) RemoteJobUpdates() <-chan operation.Operation { return h.updates }
func (h *Handler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func requestFor(parent session.ID, op operation.Operation) (toolsubagent.DurableRunPlan, Request, error) {
	p, err := toolsubagent.DecodeDurableRun(op)
	if err != nil {
		return p, Request{}, err
	}
	if p.ParentID != parent {
		return p, Request{}, errors.New("durable child parent identity mismatch")
	}
	return p, Request{ParentID: parent, OperationID: op.ID, Name: p.Run.Name, Prompt: p.Run.Prompt, Lifetime: p.Run.Lifetime}, nil
}
func validateHandle(op operation.Operation, expected Link) error {
	s, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return err
	}
	if len(s.Handle) == 0 {
		if op.Status == operation.StatusCompleted {
			return errors.New("completed durable child requires committed linkage")
		}
		return nil
	}
	var old Handle
	if err = json.Unmarshal(s.Handle, &old, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if old.Version != 1 || old.Link != expected || old.SessionID != string(expected.ChildID) || old.Handle == "" || len(old.Handle) > 128 || len(old.Diagnostic) > 1024 || old.Generation == 0 || old.ResultDeliveryID != "child-result-"+expected.Key {
		return errors.New("unsupported or mismatched durable child linkage")
	}
	switch old.Phase {
	case "connecting", "awaiting_answer", "uncertain":
		if old.AnswerSequence != 0 || old.AnswerTurnID != "" {
			return errors.New("pending child carries answer authority")
		}
	case "reported_owner_retained":
		if old.AnswerSequence == 0 || old.AnswerTurnID == "" {
			return errors.New("reported child requires canonical answer reference")
		}
	default:
		return errors.New("unsupported durable child phase")
	}
	return nil
}
func (h *Handler) AddRemoteJob(op operation.Operation) error { return h.bind(op, false) }

// AdoptRetained binds fresh observation/control authority for terminal child
// history without rewriting its result or dispatching its initial prompt.
func (h *Handler) AdoptRetained(op operation.Operation) error {
	if op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed && op.Status != operation.StatusCanceled {
		return errors.New("retained adoption requires terminal operation")
	}
	return h.bind(op, true)
}
func (h *Handler) bind(op operation.Operation, retained bool) error {
	p, req, err := requestFor(h.config.ParentID, op)
	if err != nil {
		return err
	}
	r, err := plan(h.config.Owner, req)
	if err != nil {
		return err
	}
	if err = validateHandle(op, r.Link); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.ctx.Err() != nil {
		return h.ctx.Err()
	}
	if old := h.entries[op.ID]; old != nil {
		if !reflect.DeepEqual(old.plan, p) || old.original.MaxOutputLength != op.MaxOutputLength {
			return ErrConflict
		}
		return nil
	}
	if len(h.entries) >= 128 || len(op.State) > 1<<20 || h.bytes+len(op.State) > 16<<20 {
		return errors.New("durable child capacity exhausted")
	}
	e := &entry{original: op, plan: p, request: req, wake: make(chan struct{}, 1), handle: Handle{Version: 1, SessionID: string(r.Link.ChildID), Link: r.Link, ResultDeliveryID: "child-result-" + r.Link.Key, Phase: "connecting"}}
	activity := subagent.AgentActivity{ChildActivity: subagent.ChildActivity{Name: p.Run.Name, SessionID: string(r.Link.ChildID), OwnerOperationID: string(op.ID), Generation: 1}, Purpose: p.Run.Purpose, Lifetime: p.Run.Lifetime, Phase: "connecting"}
	handle, generation, err := h.config.Hub.BindExternal(activity, func(ctx context.Context, p subagent.ControlPlan) (any, error) { return h.control(ctx, e, p) })
	if err != nil {
		return err
	}
	e.handle.Handle = handle
	e.handle.Generation = generation
	h.entries[op.ID] = e
	h.bytes += len(op.State)
	h.workers.Add(1)
	go func() {
		defer h.workers.Done()
		if retained {
			h.observe(e)
		} else {
			h.run(e)
		}
	}()
	return nil
}
func (h *Handler) emit(e *entry, status operation.Status) bool {
	h.publishActivity(e)
	e.mu.Lock()
	handle := e.handle
	e.mu.Unlock()
	s, err := operation.DecodeRemoteJobState(e.original)
	if err != nil {
		return false
	}
	s.Handle, err = json.Marshal(handle)
	if err != nil {
		return false
	}
	if status == operation.StatusCompleted {
		e.mu.Lock()
		answer := e.answer
		e.mu.Unlock()
		s.TerminalResult = answer
		s.TerminalError = ""
	}
	step, err := operation.UpdateRemoteJob(e.original, s, status)
	if err != nil {
		return false
	}
	select {
	case h.updates <- *step.Operation:
		return true
	case <-h.ctx.Done():
		return false
	}
}
func (h *Handler) run(e *entry) {
	defer func() {
		e.mu.Lock()
		c := e.connection
		e.mu.Unlock()
		if c != nil {
			_ = c.Close()
		}
	}()
	for {
		e.mu.Lock()
		c := e.connection
		e.mu.Unlock()
		if c == nil {
			attempt, cancel := context.WithTimeout(h.ctx, 6*time.Second)
			fresh, err := Open(attempt, h.config.Owner, e.request, true)
			cancel()
			if err == nil {
				e.mu.Lock()
				e.connection = fresh
				e.handle.Phase = "awaiting_answer"
				e.handle.Diagnostic = ""
				e.mu.Unlock()
				c = fresh
			} else {
				if fresh != nil {
					_ = fresh.Close()
					e.mu.Lock()
					e.launch = fresh
					e.mu.Unlock()
				}
				e.mu.Lock()
				e.handle.Phase = "uncertain"
				e.handle.Diagnostic = fmt.Sprintf("owner unavailable; no child relaunch: %.512s", err)
				e.mu.Unlock()
			}
			if !h.emit(e, operation.StatusAwaiting) {
				return
			}
			if c == nil {
				select {
				case <-e.wake:
					continue
				case <-h.ctx.Done():
					return
				}
			}
		}
		attempt, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		err := c.SubmitInitial(attempt)
		cancel()
		if err != nil {
			_ = c.Close()
			e.mu.Lock()
			e.connection = nil
			e.handle.Phase = "uncertain"
			e.handle.Diagnostic = fmt.Sprintf("prompt admission uncertain; retry retains original identity: %.512s", err)
			e.mu.Unlock()
			if !h.emit(e, operation.StatusAwaiting) {
				return
			}
			select {
			case <-e.wake:
				continue
			case <-h.ctx.Done():
				return
			}
		}
		tick := time.NewTicker(25 * time.Millisecond)
		for {
			select {
			case <-c.readerDone:
				_ = c.Close()
				e.mu.Lock()
				e.connection = nil
				e.handle.Phase = "uncertain"
				e.handle.Diagnostic = "independent child transport closed; execution/result require reconciliation; no child relaunch"
				e.mu.Unlock()
				_ = h.emit(e, operation.StatusAwaiting)
				tick.Stop()
				select {
				case <-e.wake:
					goto retry
				case <-h.ctx.Done():
					return
				}
			default:
			}
			answer, found, err := c.Answer(h.ctx)
			if err != nil {
				e.mu.Lock()
				e.handle.Phase = "uncertain"
				e.handle.Diagnostic = fmt.Sprintf("child answer unavailable: %.512s", err)
				e.mu.Unlock()
				_ = h.emit(e, operation.StatusAwaiting)
				tick.Stop()
				select {
				case <-e.wake:
					goto retry
				case <-h.ctx.Done():
					return
				}
			}
			if found {
				e.mu.Lock()
				e.finished = true
				e.handle.Phase = "reported_owner_retained"
				e.handle.AnswerSequence = answer.Sequence
				e.handle.AnswerTurnID = answer.TurnID
				e.handle.Diagnostic = ""
				e.answer = answer.Text
				e.mu.Unlock()
				h.publishActivity(e)
				_ = h.emit(e, operation.StatusCompleted)
				tick.Stop()
				<-h.ctx.Done()
				return
			}
			select {
			case <-tick.C:
			case <-e.wake:
			case <-h.ctx.Done():
				tick.Stop()
				return
			}
		}
	retry:
	}
}
func (h *Handler) publishActivity(e *entry) {
	e.mu.Lock()
	v := e.handle
	finished := e.finished
	e.mu.Unlock()
	_ = h.config.Hub.UpdateExternal(v.Handle, subagent.AgentActivity{ChildActivity: subagent.ChildActivity{Name: e.plan.Run.Name, SessionID: v.SessionID, OwnerOperationID: string(e.original.ID), Generation: 1, ReportAvailable: finished}, Purpose: e.plan.Run.Purpose, Lifetime: e.plan.Run.Lifetime, Phase: v.Phase})
}
func (h *Handler) control(ctx context.Context, e *entry, p subagent.ControlPlan) (any, error) {
	if err := h.ctx.Err(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	v := e.handle
	c := e.connection
	e.mu.Unlock()
	if p.Handle != v.Handle || p.Generation != v.Generation {
		return nil, errors.New("stale durable child parent authority")
	}
	switch p.Action {
	case "status":
		h.publishActivity(e)
		select {
		case e.wake <- struct{}{}:
		default:
		}
		return v, nil
	case "stop":
		if c == nil {
			return nil, ErrUncertain
		}
		if err := c.Shutdown(ctx); err != nil {
			return nil, err
		}
		return toolsubagent.ControlOutcome{Action: "stop", Handle: v.Handle, Generation: v.Generation, State: "cancellation_requested", DrainConfirmed: false}, nil
	default:
		return nil, errors.New("durable child messages require a new explicit run; inherited messages are not replayed")
	}
}
func (h *Handler) CancelRemoteJob(id operation.ID, reason string) error {
	h.mu.Lock()
	e := h.entries[id]
	h.mu.Unlock()
	if e == nil {
		return errors.New("unknown durable child operation")
	}
	e.mu.Lock()
	v := e.handle
	e.mu.Unlock()
	_, err := h.control(h.ctx, e, subagent.ControlPlan{Action: "stop", Handle: v.Handle, Generation: v.Generation, Message: reason})
	return err
}

// ReconcileHistory permits only exact independently owned v2 children. It never
// submits prompts, launches hosts, rewrites history, or invents a global drain.
func ReconcileHistory(ctx context.Context, cfg Config, parent session.ID, history sessionstore.LifecycleHistory, latest []operation.Operation) error {
	if err := history.ValidateResume(); err == nil {
		return nil
	}
	allowed := map[operation.ID]bool{}
	for id := range history.RetainedRemoteOperations {
		allowed[id] = true
	}
	if err := history.ValidateResumeWithOwners(allowed); err != nil {
		return err
	}
	ops := map[operation.ID]operation.Operation{}
	for id, op := range history.RetainedRemoteOperations {
		ops[id] = op
	}
	for _, op := range latest {
		if _, ok := ops[op.ID]; ok {
			ops[op.ID] = op
		}
	}
	for id, op := range ops {
		if InertControl(op) {
			continue
		}
		_, req, err := requestFor(parent, op)
		if err != nil {
			return fmt.Errorf("retained owner %s: %w", id, err)
		}
		r, err := plan(cfg, req)
		if err != nil {
			return err
		}
		if err = validateHandle(op, r.Link); err != nil {
			return err
		}
		connection, err := Open(ctx, cfg, req, false)
		if err != nil && connection != nil {
			_ = connection.Close()
		}
		if errors.Is(err, ErrNotSubmitted) {
			state, _ := operation.DecodeRemoteJobState(op)
			if len(state.Handle) > 0 {
				return fmt.Errorf("%w: committed child linkage lost reservation", ErrUncertain)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("retained child %s requires ownership reconciliation: %w", id, err)
		}
		if err = connection.Close(); err != nil {
			return err
		}
	}
	return history.ValidateResumeWithOwners(allowed)
}

// observe retains a separately owned terminal child in the new Hub. No run,
// prompt, or result is replayed; shutdown admission remains explicitly not drain.
func (h *Handler) observe(e *entry) {
	ctx, cancel := context.WithTimeout(h.ctx, 6*time.Second)
	c, err := Open(ctx, h.config.Owner, e.request, false)
	cancel()
	if err != nil && c != nil {
		_ = c.Close()
	}
	e.mu.Lock()
	e.finished = true
	if err != nil {
		e.handle.Phase = "uncertain"
		e.handle.Diagnostic = fmt.Sprintf("retained owner unavailable: %.512s", err)
	} else {
		e.connection = c
		e.handle.Phase = "reported_owner_retained"
	}
	e.mu.Unlock()
	h.publishActivity(e)
	if err == nil {
		select {
		case <-h.ctx.Done():
		case <-c.readerDone:
			e.mu.Lock()
			e.handle.Phase = "uncertain"
			e.handle.Diagnostic = "retained independent child transport closed; result remains committed but owner reconciliation required"
			e.mu.Unlock()
			h.publishActivity(e)
			<-h.ctx.Done()
		}
	} else {
		<-h.ctx.Done()
	}
	if err == nil {
		_ = c.Close()
	}
}

// InertControl classifies only terminal read/stop controls, which never create
// a child. It does not authorize replay or classify any child they referenced.
// Messages can resume legacy children and are deliberately excluded.
func InertControl(op operation.Operation) bool {
	if op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed && op.Status != operation.StatusCanceled {
		return false
	}
	s, err := operation.DecodeRemoteJobState(op)
	if err != nil || s.Plan.Type != toolsubagent.ControlPlanType || s.Plan.Version != toolsubagent.PlanVersion {
		return false
	}
	var p toolsubagent.ControlPlan
	if json.Unmarshal(s.Plan.Data, &p, json.RejectUnknownMembers(true)) != nil || p.Validate() != nil {
		return false
	}
	return p.Action == "list" || p.Action == "status" || p.Action == "stop"
}

// OwnershipProof deliberately cannot convert independent retention to global
// drain, even when no terminal operation needed manager admission after replay.
func (h *Handler) OwnershipProof() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) > 0 {
		return errors.New("independent durable child ownership retained; global execution drain unproven")
	}
	return nil
}
