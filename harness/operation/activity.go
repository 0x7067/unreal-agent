package operation

import (
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

type ActivityPhase string

const (
	ActivityRunning               ActivityPhase = "running"
	ActivityWarning               ActivityPhase = "warning"
	ActivityCancellationRequested ActivityPhase = "cancellation_requested"
	ActivityDraining              ActivityPhase = "draining"
	ActivityTerminal              ActivityPhase = "terminal"
)

// OperationActivity is a live observation, not a durable checkpoint or proof of
// coordinator persistence. No command text, output, or credentials are included.
// Silence is not classified as a stall. A process primitive completion proves
// its own wait/read drain barrier, not remote child or whole-session termination.
type OperationActivity struct {
	ID                        ID
	Type                      Type
	Status                    Status
	Phase                     ActivityPhase
	StartedAt                 time.Time
	LastObservedAt            time.Time
	WarningAt                 time.Time
	CancellationRequestedAt   time.Time
	DrainingAt                time.Time
	TerminalAt                time.Time
	Budget                    executionbudget.Policy
	Owned                     bool
	ActivePrimitives          int
	ProcessStarted            bool
	ProcessCompletionObserved bool
	CancellationReason        string
}

type ActivitySnapshot struct {
	// Operations contains at most 128 last observations. Omitted includes
	// historical entries evicted from this live-only view.
	Operations []OperationActivity
	Omitted    int
	Account    executionbudget.Snapshot
}

// ActivitySnapshot returns a bounded, side-effect-free projection of the last
// actor observations. It never calls a handler or performs IO. Active ownership
// omitted by the view limit is still counted by the shared account.
func (m *LocalOperationManager) ActivitySnapshot() ActivitySnapshot {
	m.activityMu.RLock()
	result := ActivitySnapshot{Operations: append([]OperationActivity(nil), m.activity...), Omitted: m.activityOmitted}
	m.activityMu.RUnlock()
	result.Account = m.options.ResourceAccount.Snapshot()
	return result
}

const activityLimit = 128

func (m *LocalOperationManager) observe(current *localRunningOperation) {
	a := current.activity
	a.Status = current.operation.Status
	a.ActivePrimitives = current.activePrimitives
	a.Owned = current.lease != nil || !current.drained
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	for i := range m.activity {
		if m.activity[i].ID == a.ID {
			m.activity[i] = a
			return
		}
	}
	if len(m.activity) == activityLimit {
		// Prefer dropping a drained historical observation, never its ownership.
		index := 0
		for i := range m.activity {
			if !m.activity[i].Owned {
				index = i
				break
			}
		}
		copy(m.activity[index:], m.activity[index+1:])
		m.activity = m.activity[:len(m.activity)-1]
		m.activityOmitted++
	}
	m.activity = append(m.activity, a)
}

func (m *LocalOperationManager) finishOwned(operations map[ID]*localRunningOperation, current *localRunningOperation) {
	if current.activePrimitives != 0 || current.remoteUnknown {
		m.observe(current)
		return
	}
	if current.pendingTerminal {
		m.appendLocalUpdate(current.operation)
		current.pendingTerminal = false
	}
	current.lease.Release()
	current.lease = nil
	current.drained = true
	current.activity.Phase = ActivityTerminal
	current.activity.TerminalAt = m.now()
	current.cancel()
	m.observe(current)
	delete(operations, current.operation.ID)
}

func (m *LocalOperationManager) now() time.Time {
	if m.options.now != nil {
		return m.options.now()
	}
	return time.Now()
}

func (m *LocalOperationManager) requestCancellation(current *localRunningOperation, reason string) error {
	if !current.activity.CancellationRequestedAt.IsZero() {
		return nil
	}
	now := m.now()
	current.activity.Phase = ActivityCancellationRequested
	current.activity.CancellationRequestedAt = now
	// Reason is a bounded categorical label, not arbitrary user/command text.
	current.activity.CancellationReason = "cancel_requested"
	if reason == "wall_budget_exceeded" {
		current.activity.CancellationReason = reason
	}
	m.observe(current)
	var err error
	if current.remoteJobHandlerIndex >= 0 {
		err = m.remoteJobs.cancel(current.remoteJobHandlerIndex, current.operation.ID, reason)
	} else {
		current.cancel()
	}
	current.activity.Phase = ActivityDraining
	current.activity.DrainingAt = now
	m.observe(current)
	return err
}

func (m *LocalOperationManager) checkBudgets(operations map[ID]*localRunningOperation) {
	now := m.now()
	for _, current := range operations {
		if current.terminal || !current.activity.CancellationRequestedAt.IsZero() {
			continue
		}
		elapsed := now.Sub(current.activity.StartedAt)
		p := current.activity.Budget
		if p.WarnAfter > 0 && elapsed >= p.WarnAfter && current.activity.WarningAt.IsZero() {
			current.activity.WarningAt = now
			current.activity.Phase = ActivityWarning
			m.observe(current)
		}
		if p.Timeout > 0 && elapsed >= p.Timeout {
			if err := m.requestCancellation(current, "wall_budget_exceeded"); err != nil {
				current.remoteUnknown = true
				m.failLocalOperation(operations, current, err)
			}
		}
	}
}
