package operation

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// ErrCleanupUncertain means the local actor stopped, but cleanup of some owned
// work cannot be proven. It must not be interpreted as successful cleanup.
var ErrCleanupUncertain = errors.New("operation cleanup is uncertain")

// CleanupUncertainError preserves canonical operation IDs for independent owner
// reconciliation. UnprovenRemoteIDs includes attempted remote admissions, even
// jobs reporting terminal status: RemoteJobHandler has no effect-drain contract.
// UnknownRemoteIDs is the subset whose tracked outcome remained unknown at stop;
// a forwarding-worker join or a terminal StatusUnsupported cannot resolve it.
// UnsupportedOperationIDs records unsupported operations, not cleanup proof.
// AwaitStopped returns fresh slices so callers cannot mutate retained evidence.
type CleanupUncertainError struct {
	UnprovenRemoteIDs       []ID
	UnknownRemoteIDs        []ID
	UnsupportedOperationIDs []ID
}

func (e *CleanupUncertainError) Error() string {
	return fmt.Sprintf("%v: unproven remote operations %v; unknown remote outcomes %v; unsupported operations %v", ErrCleanupUncertain, e.UnprovenRemoteIDs, e.UnknownRemoteIDs, e.UnsupportedOperationIDs)
}

func (e *CleanupUncertainError) Unwrap() error { return ErrCleanupUncertain }

// AwaitStopped waits for the manager actor and its remote-update forwarding
// workers to return, after all local primitive completion events are consumed.
// Call after canceling the manager lifetime context. Concurrent calls, including
// calls before actor exit, are supported. Canceling ctx cancels only this wait;
// neither a deadline nor cancellation counts as successful cleanup.
//
// Local completion events follow effect/output cleanup (process wait and output
// worker joins, file closes, or compute callback return/unwind). Primitive APIs
// do not expose a join for the terminal-event sender's final goroutine epilogue.
// No external remote-effect drain is implied by forwarding-worker exit or by a
// terminal remote status. Such work yields ErrCleanupUncertain unless there was
// no attempted remote admission. Pending consumer updates need not be read.
func (m *LocalOperationManager) AwaitStopped(ctx context.Context) error {
	select {
	case <-m.stopped:
		return m.stoppedError()
	default:
	}
	select {
	case <-m.stopped:
		return m.stoppedError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *LocalOperationManager) stoppedError() error {
	if m.stopErr == nil {
		return nil
	}
	return &CleanupUncertainError{
		UnprovenRemoteIDs:       slices.Clone(m.stopErr.UnprovenRemoteIDs),
		UnknownRemoteIDs:        slices.Clone(m.stopErr.UnknownRemoteIDs),
		UnsupportedOperationIDs: slices.Clone(m.stopErr.UnsupportedOperationIDs),
	}
}

// Only the actor writes stop evidence; closing stopped publishes it to waiters.
func (m *LocalOperationManager) captureStopUncertainty(operations map[ID]*localRunningOperation) {
	evidence := &CleanupUncertainError{}
	for id := range m.unprovenRemoteIDs {
		evidence.UnprovenRemoteIDs = append(evidence.UnprovenRemoteIDs, id)
	}
	for id := range m.unsupportedOperationIDs {
		evidence.UnsupportedOperationIDs = append(evidence.UnsupportedOperationIDs, id)
	}
	for id, current := range operations {
		if current.remoteUnknown {
			evidence.UnknownRemoteIDs = append(evidence.UnknownRemoteIDs, id)
		}
		if current.remoteJobHandlerIndex >= 0 && !current.drained {
			if _, tracked := m.unprovenRemoteIDs[id]; !tracked {
				evidence.UnprovenRemoteIDs = append(evidence.UnprovenRemoteIDs, id)
			}
		}
	}
	if len(evidence.UnprovenRemoteIDs)+len(evidence.UnknownRemoteIDs)+len(evidence.UnsupportedOperationIDs) == 0 {
		return
	}
	slices.Sort(evidence.UnprovenRemoteIDs)
	slices.Sort(evidence.UnknownRemoteIDs)
	slices.Sort(evidence.UnsupportedOperationIDs)
	m.stopErr = evidence
}

// LocalCleanupVerifier is an optional live, nonblocking proof capability. It does
// not retire the manager or cancel unrelated background owners.
type LocalCleanupVerifier interface {
	LocalCleanupProof([]ID) error
}

// ErrLocalCleanupUnproven means at least one requested ID has no retained local
// cleanup receipt. The operation may still be running, remote, unknown, unsupported,
// never admitted, or evicted. It is not evidence that cleanup failed or succeeded.
var ErrLocalCleanupUnproven = errors.New("local operation cleanup is unproven")

const localCleanupReceiptLimit = 256

var _ LocalCleanupVerifier = (*LocalOperationManager)(nil)

// LocalCleanupProof proves only local effect/output cleanup for all requested
// IDs. It neither waits nor cancels anything and never infers proof from terminal
// status. Missing receipts fail closed. The latest 256 receipts are retained;
// callers must use globally unique IDs, as manager admission already requires.
// The primitive sender epilogue and detached-daemon limitations of AwaitStopped
// apply here too. An empty target list is vacuously proven.
func (m *LocalOperationManager) LocalCleanupProof(ids []ID) error {
	m.activityMu.RLock()
	defer m.activityMu.RUnlock()
	for _, id := range ids {
		if !slices.Contains(m.localCleanupReceipts, id) {
			return fmt.Errorf("%w: operation %q", ErrLocalCleanupUnproven, id)
		}
	}
	return nil
}

// All actor finish paths go through this wrapper. finishOwned is the existing
// ownership/drain barrier; record a receipt only after that barrier succeeded.
func (m *LocalOperationManager) finishOwnedWithLocalReceipt(operations map[ID]*localRunningOperation, current *localRunningOperation) {
	m.finishOwned(operations, current)
	if !current.drained || current.activePrimitives != 0 || current.remoteUnknown || current.operation.Status == StatusUnsupported {
		return
	}
	switch current.operation.Type {
	case TypeShell:
		if current.operation.Version != VersionShell && current.operation.Version != VersionShellBudget {
			return
		}
	case TypeValue:
		if current.operation.Version != VersionValue {
			return
		}
	case TypeSkillUse:
		if current.operation.Version != VersionSkillUse {
			return
		}
	case TypeViewImage:
		if current.operation.Version != VersionViewImage {
			return
		}
	default:
		return
	}
	if _, unsupported := m.unsupportedOperationIDs[current.operation.ID]; unsupported {
		return
	}
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	if slices.Contains(m.localCleanupReceipts, current.operation.ID) {
		return
	}
	if len(m.localCleanupReceipts) == localCleanupReceiptLimit {
		copy(m.localCleanupReceipts, m.localCleanupReceipts[1:])
		m.localCleanupReceipts = m.localCleanupReceipts[:localCleanupReceiptLimit-1]
	}
	m.localCleanupReceipts = append(m.localCleanupReceipts, current.operation.ID)
}
