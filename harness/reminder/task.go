package reminder

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

const TaskType operation.RemoteJobPlanType = "durable_timer"
const TaskVersion operation.RemoteJobPlanVersion = 1

// TaskPlan is inert. Operation identity is supplied only by manager Add.
type TaskPlan struct {
	Version   uint32    `json:"version"`
	ParentID  string    `json:"parentID"`
	DueTime   time.Time `json:"dueTime"`
	Ownership Ownership `json:"ownership"`
	Message   string    `json:"message"`
}

func taskID(s string) bool {
	return s != "" && len(s) <= 1024 && strings.TrimSpace(s) == s && !strings.ContainsFunc(s, unicode.IsControl)
}
func (p TaskPlan) Validate() error {
	_, zone := p.DueTime.Zone()
	if p.Version != 1 {
		return ErrUnsupported
	}
	if !taskID(p.ParentID) || p.DueTime.IsZero() || zone != 0 || len(p.Message) > MaxMessageBytes {
		return errors.New("invalid timer task plan")
	}
	switch p.Ownership.Mode {
	case "foreground":
		if !taskID(p.Ownership.OwnerTurnID) {
			return errors.New("exact foreground owner required")
		}
	case "background":
		if p.Ownership.OwnerTurnID != "" {
			return errors.New("background owner must be empty")
		}
	default:
		return errors.New("explicit ownership required")
	}
	return nil
}
func NewTaskSpec(p TaskPlan) (operation.Spec, error) {
	if e := p.Validate(); e != nil {
		return operation.Spec{}, e
	}
	b, e := json.Marshal(p)
	if e != nil {
		return operation.Spec{}, e
	}
	return operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: TaskType, Version: TaskVersion, Data: b})
}

// DecodeTask rejects extension fields rather than silently weakening recovery.
// Handle contains the strict reducer checkpoint; Plan never changes.
func DecodeTask(op operation.Operation) (TaskPlan, *Checkpoint, error) {
	var p TaskPlan
	if len(op.State) > 64*1024 {
		return p, nil, errors.New("task state too large")
	}
	state, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		return p, nil, e
	}
	var strict operation.RemoteJobState
	if e = json.Unmarshal(op.State, &strict, json.RejectUnknownMembers(true)); e != nil {
		return p, nil, e
	}
	if state.Plan.Type != TaskType || state.Plan.Version != TaskVersion {
		return p, nil, ErrUnsupported
	}
	if e = json.Unmarshal(state.Plan.Data, &p, json.RejectUnknownMembers(true)); e != nil {
		return p, nil, e
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(state.Plan.Data, &fields); err != nil {
		return p, nil, err
	}
	for _, name := range []string{"version", "parentID", "dueTime", "ownership", "message"} {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return p, nil, errors.New("required timer plan field missing/null")
		}
	}
	if e = p.Validate(); e != nil {
		return p, nil, e
	}
	if state.OutputDirectory != "" || state.ResultPath != "" || state.ErrorPath != "" { /* manager-owned artifact paths are inert */
	}
	if len(state.OutstandingInput) != 0 || len(state.Subscription) != 0 || !state.NextInspectionAt.IsZero() || state.TerminalError != "" {
		return p, nil, errors.New("unexpected task metadata")
	}
	var cp *Checkpoint
	if len(state.Handle) != 0 {
		c, e := Decode(state.Handle)
		if e != nil {
			return p, nil, e
		}
		cp = &c
		if len(c.Timers) != 1 {
			return p, nil, errors.New("task requires exactly one timer")
		}
		a := c.Timers[0].Plan
		if !taskID(string(op.ID)) || a.ParentID != p.ParentID || a.OperationID != string(op.ID) || a.Kind != "reminder" || !a.DueTime.Equal(p.DueTime) || a.Ownership != p.Ownership || a.Message != p.Message {
			return p, nil, errors.New("task checkpoint changed plan or identity")
		}
	}
	switch op.Status {
	case operation.StatusReady:
		if cp != nil || state.TerminalResult != "" {
			return p, nil, errors.New("ready task has checkpoint or result")
		}
	case operation.StatusAwaiting:
		if cp == nil || (cp.Timers[0].State != Scheduled && cp.Timers[0].State != NotificationPending) || state.TerminalResult != "" {
			return p, nil, errors.New("invalid awaiting task")
		}
	case operation.StatusCompleted:
		if cp == nil || cp.Timers[0].State != NotificationPending {
			return p, nil, errors.New("terminal notification requires pending proof")
		}
		want, _ := json.Marshal(NotificationResult{StableNotificationID: cp.Timers[0].InputID, Message: p.Message})
		if state.TerminalResult != string(want) {
			return p, nil, errors.New("terminal notification mismatch")
		}
	case operation.StatusCanceled:
		if cp == nil || cp.Timers[0].State != Canceled || state.TerminalResult != "" {
			return p, nil, errors.New("invalid canceled task")
		}
	default:
		return p, nil, errors.New("unsupported task status")
	}
	return p, cp, nil
}
func ValidateTaskRecovery(op operation.Operation) error { _, _, e := DecodeTask(op); return e }

// TaskTerminalProof validates a terminal value, not whether anyone saved it.
func TaskTerminalProof(op operation.Operation) bool {
	return (op.Status == operation.StatusCompleted || op.Status == operation.StatusCanceled) && ValidateTaskRecovery(op) == nil
}

type NotificationResult struct {
	StableNotificationID string `json:"stableNotificationID"`
	Message              string `json:"message"`
}

type Config struct{ Now func() time.Time }
type taskReceipt struct {
	id    operation.ID
	state []byte
}
type Handler struct {
	mu       sync.Mutex
	now      func() time.Time
	tasks    map[operation.ID]operation.Operation
	updates  chan operation.Operation
	receipts chan taskReceipt
	closed   bool
	stopped  chan struct{}
}

// NewHandler starts no goroutines or timers. The owner drives Poll and Close.
func NewHandler(c Config) *Handler {
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Handler{now: c.Now, tasks: make(map[operation.ID]operation.Operation), updates: make(chan operation.Operation, MaxTimers*3), receipts: make(chan taskReceipt, MaxTimers), stopped: make(chan struct{})}
}
func (*Handler) RemoteJobPlanType() operation.RemoteJobPlanType       { return TaskType }
func (*Handler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion { return TaskVersion }
func (h *Handler) RemoteJobUpdates() <-chan operation.Operation       { return h.updates }
func cloneTask(op operation.Operation) operation.Operation {
	op.State = op.State.Clone()
	op.Idempotency = op.Idempotency.Clone()
	return op
}
func (h *Handler) emit(op operation.Operation) error {
	select {
	case h.updates <- cloneTask(op):
		h.tasks[op.ID] = cloneTask(op)
		return nil
	default:
		return errors.New("timer update queue full")
	}
}
func taskUpdate(op operation.Operation, cp Checkpoint, status operation.Status) (operation.Operation, error) {
	state, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		return op, e
	}
	b, e := Encode(cp)
	if e != nil {
		return op, e
	}
	state.Handle = b
	if status == operation.StatusCompleted {
		p, _, e := DecodeTask(op)
		if e != nil {
			return op, e
		}
		result, _ := json.Marshal(NotificationResult{StableNotificationID: cp.Timers[0].InputID, Message: p.Message})
		state.TerminalResult = string(result)
	}
	step, e := operation.UpdateRemoteJob(op, state, status)
	if e != nil {
		return op, e
	}
	return *step.Operation, nil
}
func (h *Handler) AddRemoteJob(op operation.Operation) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("timer handler closed")
	}
	p, cp, e := DecodeTask(op)
	if e != nil {
		return e
	}
	if !taskID(string(op.ID)) {
		return errors.New("manager operation ID required")
	}
	if old, ok := h.tasks[op.ID]; ok {
		a, _ := operation.DecodeRemoteJobState(old)
		b, _ := operation.DecodeRemoteJobState(op)
		if !bytes.Equal(a.Plan.Data, b.Plan.Data) {
			return errors.New("timer identity reused")
		}
		return nil
	}
	if len(h.tasks) >= MaxTimers {
		return errors.New("timer capacity exceeded")
	}
	if TaskTerminalProof(op) {
		h.tasks[op.ID] = cloneTask(op)
		return nil
	}
	if cp == nil {
		now := h.now().UTC()
		if now.IsZero() {
			return errors.New("clock required")
		}
		created := now
		if p.DueTime.Before(created) {
			created = p.DueTime
		}
		plan, e := NewPlan(p.ParentID, string(op.ID), "reminder", created, p.DueTime, p.Ownership, p.Message)
		if e != nil {
			return e
		}
		tr, e := Add(NewCheckpoint(), plan)
		if e != nil {
			return e
		}
		cp = &tr.Checkpoint
	}
	next, e := taskUpdate(op, *cp, operation.StatusAwaiting)
	if e != nil {
		return e
	}
	return h.emit(next)
}

// NotifyCheckpointCommitted is bounded and never waits on the handler lock or
// an update consumer. Only a real successful coordinator save authorizes it.
func (h *Handler) NotifyCheckpointCommitted(id operation.ID, stateBytes []byte) error {
	if len(stateBytes) > 64*1024 {
		return errors.New("receipt state too large")
	}
	select {
	case <-h.stopped:
		return errors.New("timer handler closed")
	default:
	}
	r := taskReceipt{id: id, state: bytes.Clone(stateBytes)}
	select {
	case h.receipts <- r:
		return nil
	default:
		return errors.New("timer commit queue full")
	}
}
func (h *Handler) Poll() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("timer handler closed")
	}
	for i := 0; i < MaxOverdueBatch; i++ {
		select {
		case r := <-h.receipts:
			op, ok := h.tasks[r.id]
			if !ok || op.Status != operation.StatusAwaiting || !bytes.Equal(op.State, r.state) {
				continue
			}
			_, cp, e := DecodeTask(op)
			if e != nil {
				return e
			}
			if cp.Timers[0].State != NotificationPending {
				continue
			}
			next, e := taskUpdate(op, *cp, operation.StatusCompleted)
			if e != nil {
				return e
			}
			if e = h.emit(next); e != nil {
				return e
			}
		default:
			i = MaxOverdueBatch
		}
	}
	now := h.now()
	if now.IsZero() {
		return errors.New("clock required")
	}
	count := 0
	// Bound overdue work independently of total capacity; stable IDs survive any ordering.
	ids := make([]operation.ID, 0, len(h.tasks))
	for id := range h.tasks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, _, _ := DecodeTask(h.tasks[ids[i]])
		b, _, _ := DecodeTask(h.tasks[ids[j]])
		if a.DueTime.Equal(b.DueTime) {
			return ids[i] < ids[j]
		}
		return a.DueTime.Before(b.DueTime)
	})
	for _, id := range ids {
		op := h.tasks[id]
		if count == MaxOverdueBatch {
			break
		}
		if op.Status != operation.StatusAwaiting {
			continue
		}
		_, cp, e := DecodeTask(op)
		if e != nil {
			return e
		}
		if cp.Timers[0].State != Scheduled || now.Before(cp.Timers[0].Plan.DueTime) {
			continue
		}
		tr, e := Advance(*cp, now)
		if e != nil {
			return e
		}
		next, e := taskUpdate(op, tr.Checkpoint, operation.StatusAwaiting)
		if e != nil {
			return e
		}
		if e = h.emit(next); e != nil {
			return e
		}
		count++
	}
	return nil
}
func (h *Handler) CancelRemoteJob(id operation.ID, reason string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("timer handler closed")
	}
	op, ok := h.tasks[id]
	if !ok {
		return errors.New("unknown timer")
	}
	if TaskTerminalProof(op) {
		return nil
	}
	_, cp, e := DecodeTask(op)
	if e != nil {
		return e
	}
	tr, e := Cancel(*cp, cp.Timers[0].Plan.TimerID, reason)
	if e != nil {
		return e
	}
	next, e := taskUpdate(op, tr.Checkpoint, operation.StatusCanceled)
	if e != nil {
		return e
	}
	return h.emit(next)
}

// Close joins all local activity (there are no workers). It does not forge
// completion/cancellation checkpoints for unsaved tasks or send notifications.
func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		h.closed = true
		close(h.stopped)
		close(h.updates)
	}
	return nil
}
func (h *Handler) AwaitStopped(ctx context.Context) error {
	select {
	case <-h.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ operation.RemoteJobHandler = (*Handler)(nil)
