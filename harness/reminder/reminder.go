// Package reminder implements a bounded, pure durable timer reducer. It owns no
// goroutines or I/O. A host must commit checkpoints before using delivery intents.
package reminder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	PlanVersion       uint32 = 1
	CheckpointVersion uint32 = 1
	MaxTimers                = 128
	MaxOverdueBatch          = 16
	MaxDelay                 = 30 * 24 * time.Hour
	MaxMessageBytes          = 4096
)

var ErrUnsupported = errors.New("unsupported reminder data version")

type Ownership struct {
	Mode        string `json:"mode"` // foreground or background
	OwnerTurnID string `json:"ownerTurnID,omitempty"`
}
type Plan struct {
	Version     uint32    `json:"version"`
	TimerID     string    `json:"timerID"`
	ParentID    string    `json:"parentID"`
	OperationID string    `json:"operationID"`
	Kind        string    `json:"kind"` // reminder or wait
	CreatedAt   time.Time `json:"createdAt"`
	DueTime     time.Time `json:"dueTime"` // absolute UTC, never recomputed on restart
	Ownership   Ownership `json:"ownership"`
	Message     string    `json:"message"`
}
type State string

const (
	Scheduled           State = "scheduled"
	NotificationPending State = "notificationPending"
	Delivered           State = "inboxAccepted"
	Canceled            State = "canceled"
)

type Timer struct {
	Plan         Plan   `json:"plan"`
	State        State  `json:"state"`
	InputID      string `json:"inputID,omitempty"`
	CancelReason string `json:"cancelReason,omitempty"`
}
type Checkpoint struct {
	Version uint32  `json:"version"`
	Timers  []Timer `json:"timers"`
}
type NotificationIntent struct {
	TimerID     string
	InputID     string
	ParentID    string
	OperationID string
	Kind        string
	Message     string
	Ownership   Ownership
}

// CommitReceipt identifies exact checkpoint bytes. It is a required host
// acknowledgment, NOT evidence that this pure package performed persistence.
// The host may pass RequiredCommit to Deliveries only AFTER durable commit.
type CommitReceipt struct{ CheckpointSHA256 string }
type Transition struct {
	Checkpoint     Checkpoint
	RequiredCommit CommitReceipt
}

func NewCheckpoint() Checkpoint { return Checkpoint{Version: CheckpointVersion, Timers: []Timer{}} }
func StableTimerID(parentID, operationID string) string {
	b, _ := json.Marshal([]string{parentID, operationID})
	h := sha256.Sum256(b)
	return "timer_" + hex.EncodeToString(h[:])
}
func LogicalInputID(timerID string) string { return "reminder:" + timerID }
func NewPlan(parent, operation, kind string, created, due time.Time, ownership Ownership, message string) (Plan, error) {
	p := Plan{Version: PlanVersion, TimerID: StableTimerID(parent, operation), ParentID: parent, OperationID: operation, Kind: kind, CreatedAt: created.UTC(), DueTime: due.UTC(), Ownership: ownership, Message: message}
	return p, p.Validate()
}
func (p Plan) Validate() error {
	if p.Version != PlanVersion {
		return fmt.Errorf("plan version %d: %w", p.Version, ErrUnsupported)
	}
	if p.ParentID == "" || p.OperationID == "" || len(p.ParentID) > 1024 || len(p.OperationID) > 1024 || p.TimerID != StableTimerID(p.ParentID, p.OperationID) {
		return errors.New("timer identity must bind parent and operation")
	}
	if p.Kind != "reminder" && p.Kind != "wait" {
		return errors.New("kind must be reminder or wait")
	}
	_, cz := p.CreatedAt.Zone()
	_, dz := p.DueTime.Zone()
	if p.CreatedAt.IsZero() || p.DueTime.IsZero() || cz != 0 || dz != 0 || p.DueTime.Before(p.CreatedAt) || p.DueTime.Sub(p.CreatedAt) > MaxDelay {
		return errors.New("dueTime must be absolute UTC within bounded delay from createdAt")
	}
	if len(p.Message) > MaxMessageBytes {
		return errors.New("message too large")
	}
	switch p.Ownership.Mode {
	case "foreground":
		if p.Ownership.OwnerTurnID == "" || len(p.Ownership.OwnerTurnID) > 1024 {
			return errors.New("foreground requires bounded ownerTurnID")
		}
	case "background":
		if p.Ownership.OwnerTurnID != "" {
			return errors.New("background cannot carry ownerTurnID")
		}
	default:
		return errors.New("explicit ownership required")
	}
	return nil
}
func (c Checkpoint) Validate() error {
	if c.Version != CheckpointVersion {
		return fmt.Errorf("checkpoint version %d: %w", c.Version, ErrUnsupported)
	}
	if len(c.Timers) > MaxTimers {
		return errors.New("timer capacity exceeded")
	}
	seen := map[string]bool{}
	pending := 0
	for _, t := range c.Timers {
		if err := t.Plan.Validate(); err != nil {
			return err
		}
		if seen[t.Plan.TimerID] {
			return errors.New("duplicate timer")
		}
		seen[t.Plan.TimerID] = true
		switch t.State {
		case Scheduled:
			if t.InputID != "" || t.CancelReason != "" {
				return errors.New("scheduled timer has terminal metadata")
			}
		case NotificationPending, Delivered:
			if t.InputID != LogicalInputID(t.Plan.TimerID) || t.CancelReason != "" {
				return errors.New("invalid notification identity")
			}
			if t.State == NotificationPending {
				pending++
			}
		case Canceled:
			if t.CancelReason == "" || len(t.CancelReason) > 1024 || (t.InputID != "" && t.InputID != LogicalInputID(t.Plan.TimerID)) {
				return errors.New("invalid cancellation")
			}
		default:
			return errors.New("unsupported timer state")
		}
	}
	if pending > MaxOverdueBatch {
		return errors.New("pending delivery capacity exceeded")
	}
	return nil
}
func EncodePlan(p Plan) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(p)
}
func DecodePlan(b []byte) (Plan, error) {
	var p Plan
	err := json.Unmarshal(b, &p, json.RejectUnknownMembers(true))
	if err == nil {
		err = p.Validate()
	}
	return p, err
}
func Encode(c Checkpoint) ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}
func Decode(b []byte) (Checkpoint, error) {
	var c Checkpoint
	err := json.Unmarshal(b, &c, json.RejectUnknownMembers(true))
	if err == nil {
		err = c.Validate()
	}
	return c, err
}
func clone(c Checkpoint) Checkpoint { c.Timers = append([]Timer{}, c.Timers...); return c }
func transition(c Checkpoint) (Transition, error) {
	b, err := Encode(c)
	if err != nil {
		return Transition{}, err
	}
	h := sha256.Sum256(b)
	return Transition{Checkpoint: c, RequiredCommit: CommitReceipt{CheckpointSHA256: hex.EncodeToString(h[:])}}, nil
}

// Add is idempotent for identical plans, and rejects identity reuse with changes.
func Add(c Checkpoint, p Plan) (Transition, error) {
	if err := c.Validate(); err != nil {
		return Transition{}, err
	}
	if err := p.Validate(); err != nil {
		return Transition{}, err
	}
	for _, t := range c.Timers {
		if t.Plan.TimerID == p.TimerID {
			a, _ := EncodePlan(t.Plan)
			b, _ := EncodePlan(p)
			if string(a) != string(b) {
				return Transition{}, errors.New("timer identity reused with different plan")
			}
			return transition(clone(c))
		}
	}
	if len(c.Timers) >= MaxTimers {
		return Transition{}, errors.New("timer capacity exceeded")
	}
	c = clone(c)
	c.Timers = append(c.Timers, Timer{Plan: p, State: Scheduled})
	return transition(c)
}

// Advance persists notificationPending before the host can obtain intents.
// Existing pending notifications reserve batch capacity and retain their InputID.
func Advance(c Checkpoint, now time.Time) (Transition, error) {
	if err := c.Validate(); err != nil {
		return Transition{}, err
	}
	if now.IsZero() {
		return Transition{}, errors.New("clock time required")
	}
	c = clone(c)
	pending := 0
	var due []int
	for i, t := range c.Timers {
		if t.State == NotificationPending {
			pending++
		}
		if t.State == Scheduled && !now.Before(t.Plan.DueTime) {
			due = append(due, i)
		}
	}
	sort.Slice(due, func(i, j int) bool {
		a, b := c.Timers[due[i]].Plan, c.Timers[due[j]].Plan
		if a.DueTime.Equal(b.DueTime) {
			return a.TimerID < b.TimerID
		}
		return a.DueTime.Before(b.DueTime)
	})
	for _, i := range due {
		if pending == MaxOverdueBatch {
			break
		}
		c.Timers[i].State = NotificationPending
		c.Timers[i].InputID = LogicalInputID(c.Timers[i].Plan.TimerID)
		pending++
	}
	return transition(c)
}
func Reconcile(c Checkpoint, now time.Time) (Transition, error) { return Advance(c, now) }

// Deliveries requires the host's receipt for a successfully committed checkpoint.
// Repeated calls intentionally redeliver the same logical InputID. Inbox/session
// deduplication is required; exactly-once third-party delivery is NOT promised.
func Deliveries(c Checkpoint, committed CommitReceipt) ([]NotificationIntent, error) {
	tr, err := transition(c)
	if err != nil {
		return nil, err
	}
	if committed != tr.RequiredCommit {
		return nil, errors.New("receipt does not match committed checkpoint")
	}
	out := make([]NotificationIntent, 0, MaxOverdueBatch)
	for _, t := range c.Timers {
		if t.State == NotificationPending {
			p := t.Plan
			out = append(out, NotificationIntent{TimerID: p.TimerID, InputID: t.InputID, ParentID: p.ParentID, OperationID: p.OperationID, Kind: p.Kind, Message: p.Message, Ownership: p.Ownership})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TimerID < out[j].TimerID })
	return out, nil
}

// Acknowledge records durable inbox/session acceptance, not client presentation.
func Acknowledge(c Checkpoint, timerID, inputID string) (Transition, error) {
	if err := c.Validate(); err != nil {
		return Transition{}, err
	}
	c = clone(c)
	for i, t := range c.Timers {
		if t.Plan.TimerID == timerID {
			if (t.State != NotificationPending && t.State != Delivered) || t.InputID != inputID {
				return Transition{}, errors.New("notification acknowledgement mismatch")
			}
			c.Timers[i].State = Delivered
			return transition(c)
		}
	}
	return Transition{}, errors.New("unknown timer")
}
func cancel(c Checkpoint, match func(Timer) bool, reason string) (Transition, error) {
	if err := c.Validate(); err != nil {
		return Transition{}, err
	}
	if reason == "" || len(reason) > 1024 {
		return Transition{}, errors.New("bounded cancellation reason required")
	}
	c = clone(c)
	for i, t := range c.Timers {
		if match(t) && (t.State == Scheduled || t.State == NotificationPending) {
			c.Timers[i].State = Canceled
			c.Timers[i].CancelReason = reason
		}
	}
	return transition(c)
}

// Cancel cannot retract a notification already accepted by an external inbox.
func Cancel(c Checkpoint, timerID, reason string) (Transition, error) {
	found := false
	for _, t := range c.Timers {
		if t.Plan.TimerID == timerID {
			found = true
		}
	}
	if !found {
		return Transition{}, errors.New("unknown timer")
	}
	return cancel(c, func(t Timer) bool { return t.Plan.TimerID == timerID }, reason)
}
func CancelForeground(c Checkpoint, ownerTurnID, reason string) (Transition, error) {
	if ownerTurnID == "" {
		return Transition{}, errors.New("ownerTurnID required")
	}
	return cancel(c, func(t Timer) bool {
		return t.Plan.Ownership.Mode == "foreground" && t.Plan.Ownership.OwnerTurnID == ownerTurnID
	}, reason)
}

// Clock and Poll are synchronous: the host owns wakeups and persistence.
type Clock interface{ Now() time.Time }

func Poll(c Checkpoint, clock Clock) (Transition, error) {
	if clock == nil {
		return Transition{}, errors.New("clock required")
	}
	return Advance(c, clock.Now())
}
