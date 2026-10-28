// Package recoverygate validates the narrow allow-list for automatic recovery.
// It performs no dispatch or I/O; callers must separately enforce ownership and
// keep terminal history inert.
package recoverygate

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Validate refuses every nonterminal operation except a supported, decodable
// ready Value, ready/awaiting ViewImage without a result, or a recorded-exit
// shell-v3/v4 output-read checkpoint. Terminal history is not decoded:
// accepting it here never authorizes redispatch. The input is not modified.
// Refusals use a full stable ordering, independent of input/map iteration order.
func Validate(operations []operation.Operation) error {
	ordered := slices.Clone(operations)
	slices.SortFunc(ordered, compare)
	for _, op := range ordered {
		switch op.Status {
		case operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled:
			continue
		}
		if op.ID == "" {
			return refuse(op, "missing operation identity", operation.ErrUnsupported)
		}
		if op.Type == operation.TypeRemoteJob {
			state, err := operation.DecodeRemoteJobState(op)
			if err == nil && state.Plan.Type == taskgraph.RunPlanType && state.Plan.Version == taskgraph.DurablePlanVersion {
				if op.Status != operation.StatusReady && op.Status != operation.StatusAwaiting && op.Status != operation.StatusCanceling {
					return refuse(op, "unapproved graph recovery status", operation.ErrUnsupported)
				}
				plan, _, err := taskgraph.DecodeDurableRun(op)
				if err == nil {
					err = taskgraph.ValidateDurableRecovery(op, plan.ParentID)
				}
				if err != nil || state.TerminalError != "" {
					return refuse(op, "unclassified graph checkpoint", fmt.Errorf("%w: %v", operation.ErrUnsupported, err))
				}
				continue
			}
			if op.Status != operation.StatusReady && op.Status != operation.StatusAwaiting {
				return refuse(op, "unapproved remote recovery status", operation.ErrUnsupported)
			}
			if _, err := toolsubagent.DecodeDurableRun(op); err != nil {
				return refuse(op, "unclassified remote plan", fmt.Errorf("%w: %w", operation.ErrUnsupported, err))
			}
			state, err = operation.DecodeRemoteJobState(op)
			if err != nil || state.TerminalResult != "" || state.TerminalError != "" {
				return refuse(op, "invalid unfinished durable child", operation.ErrUnsupported)
			}
			continue // separately reconciled under the command-layer owner fence.
		}
		if op.Type == operation.TypeShell {
			if err := validateShellRead(op); err != nil {
				return err
			}
			continue
		}
		if op.Status != operation.StatusReady && !(op.Type == operation.TypeViewImage && op.Version == operation.VersionViewImage && op.Status == operation.StatusAwaiting) {
			return refuse(op, "only approved recovery states may recover", operation.ErrUnsupported)
		}
		switch {
		case op.Type == operation.TypeValue && op.Version == operation.VersionValue:
			if _, err := operation.DecodeValue(op); err != nil {
				return refuse(op, "invalid value state", err)
			}
		case op.Type == operation.TypeViewImage && op.Version == operation.VersionViewImage:
			state, err := operation.DecodeViewImageState(op)
			if err != nil {
				return refuse(op, "invalid image state", err)
			}
			if state.Result != nil {
				return refuse(op, "unfinished image state contains a result", operation.ErrUnsupported)
			}
		default:
			return refuse(op, "unclassified initial operation intent", operation.ErrUnsupported)
		}
	}
	return nil
}

// validateShellRead authorizes only replay of capture reads, never process start.
// A persisted PGID is an observation, not ownership, death, or drain evidence.
func validateShellRead(op operation.Operation) error {
	deny := func(reason string) error { return refuse(op, reason, operation.ErrUnsupported) }
	if op.Status != operation.StatusAwaiting || (op.Version != operation.VersionShell && op.Version != operation.VersionShellBudget) {
		return deny("prior execution uncertain; ownership reconciliation required")
	}
	// NewShell includes the real constructor's configuration, size and inline-limit
	// validation. Decode alone does not establish a valid recovery cell.
	if _, err := operation.NewShell(op); err != nil {
		return refuse(op, "invalid shell state; prior execution uncertain; ownership reconciliation required", fmt.Errorf("%w: %w", operation.ErrUnsupported, err))
	}
	var state operation.ShellState
	err := json.Unmarshal(op.State, &state, json.RejectUnknownMembers(true))
	if err != nil {
		return refuse(op, "invalid shell state", fmt.Errorf("%w: %w", operation.ErrUnsupported, err))
	}
	switch state.Phase {
	case operation.ShellPhaseReadOut, operation.ShellPhaseReadOutTail, operation.ShellPhaseReadErr, operation.ShellPhaseReadErrTail:
	default:
		return deny("prior execution uncertain; ownership reconciliation required")
	}
	if state.Result != nil || state.TerminalError != "" || state.ErrorTruncated || state.ProcessGroupID != 0 || state.PendingExitCode == nil {
		return deny("shell read requires recorded exit, no PGID, and no terminal result/error; prior execution uncertain; ownership reconciliation required")
	}
	name := string(op.ID)
	if name == "." || name == ".." || filepath.IsAbs(name) || filepath.Base(name) != name {
		return deny("shell identity is not a path component")
	}
	directory := filepath.Join(state.BaseDirectory, name)
	if state.OutPath != filepath.Join(directory, operation.ShellOutFilename) || state.ErrPath != filepath.Join(directory, operation.ShellErrFilename) {
		return deny("inconsistent shell capture paths")
	}
	// The reducer persists only completed read buffers, never an in-flight cursor.
	// Restarting a read discards its transient chunks. Check the exact persisted
	// prefix/tail shape of each stream at this phase, not invented cursor fields.
	limit := int64(op.MaxOutputLength) * utf8.UTFMax
	tail := int64(op.MaxOutputLength-op.MaxOutputLength/2) * utf8.UTFMax
	complete := func(size int64, head, end []byte) bool {
		if size <= limit {
			return int64(len(head)) == size && len(end) == 0
		}
		return int64(len(head)) == limit-tail && int64(len(end)) == tail
	}
	pendingTail := func(size int64, head, end []byte) bool {
		return size > limit && int64(len(head)) == limit-tail && len(end) == 0
	}
	empty := func(size int64, head, end []byte) bool { return size == 0 && len(head) == 0 && len(end) == 0 }
	valid := false
	switch state.Phase {
	case operation.ShellPhaseReadOut:
		valid = empty(state.OutSize, state.InlineOut, state.InlineOutTail) && empty(state.ErrSize, state.InlineErr, state.InlineErrTail)
	case operation.ShellPhaseReadOutTail:
		valid = pendingTail(state.OutSize, state.InlineOut, state.InlineOutTail) && empty(state.ErrSize, state.InlineErr, state.InlineErrTail)
	case operation.ShellPhaseReadErr:
		valid = complete(state.OutSize, state.InlineOut, state.InlineOutTail) && empty(state.ErrSize, state.InlineErr, state.InlineErrTail)
	case operation.ShellPhaseReadErrTail:
		valid = complete(state.OutSize, state.InlineOut, state.InlineOutTail) && pendingTail(state.ErrSize, state.InlineErr, state.InlineErrTail)
	}
	if !valid || !state.OutTruncated || !state.ErrTruncated {
		return deny("inconsistent shell read sizes, partial buffers, or truncation state")
	}
	return nil
}

// compare includes every serialized field, so duplicate IDs cannot make the
// selected diagnostic depend on the order of a map-derived input slice.
func compare(a, b operation.Operation) int {
	if n := compareOrdered(a.ID, b.ID); n != 0 {
		return n
	}
	if n := compareOrdered(a.Type, b.Type); n != 0 {
		return n
	}
	if n := compareOrdered(a.Version, b.Version); n != 0 {
		return n
	}
	if n := compareOrdered(a.Status, b.Status); n != 0 {
		return n
	}
	if n := bytes.Compare(a.State, b.State); n != 0 {
		return n
	}
	if n := bytes.Compare(a.Idempotency, b.Idempotency); n != 0 {
		return n
	}
	return compareOrdered(a.MaxOutputLength, b.MaxOutputLength)
}

func compareOrdered[T ~string | ~uint32 | ~int](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// refusal retains the original decoder cause for errors.Is/As/Unwrap while
// exposing only bounded diagnostic text (at most 2048 bytes).
type refusal struct {
	message string
	cause   error
}

func (e *refusal) Error() string { return e.message }
func (e *refusal) Unwrap() error { return e.cause }

func refuse(op operation.Operation, reason string, cause error) error {
	// Quote bounded prefixes before formatting; hostile control bytes cannot
	// inject log lines, and arbitrarily large identifiers cannot expand the log.
	field := func(s string) string { return strconv.QuoteToASCII(clip(s, 48)) }
	message := fmt.Sprintf("recovery refused: id=%s type=%s version=%d status=%s: %s: %s",
		field(string(op.ID)), field(string(op.Type)), op.Version, field(string(op.Status)), reason, clip(cause.Error(), 512))
	return &refusal{message: clip(message, 2048), cause: cause}
}

func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}
