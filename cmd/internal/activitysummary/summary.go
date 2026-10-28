// Package activitysummary renders detached, redacted live observations. It has
// no scheduling or lifecycle authority; callers capture snapshots and the clock.
package activitysummary

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// Format is pure and does not mutate its inputs. Nil means unavailable, not idle.
// Resource counts cover in-process operation envelopes, never process trees.
func Format(now time.Time, fleet *subagent.FleetSnapshot, activity *operation.ActivitySnapshot) string {
	var b strings.Builder
	b.WriteString("Live supervision snapshot (not semantic acceptance, durable terminal or idle proof). Pending persisted tools remain authoritative. Reports and cancellation do not prove execution/output drain; silence alone is not a stall;")
	if activity != nil {
		s := activity.Account
		fmt.Fprintf(&b, " resources scope=%s owned=%d peak=%d work_owned=%d work_limit=%d management_owned=%d management_limit=%d rejected=%d;", label(s.Scope), s.Owned, s.Peak, s.WorkOwned, s.Limit, s.ManagementOwned, s.ManagementLimit, s.Rejected)
	} else {
		b.WriteString(" resources unavailable;")
	}
	if fleet != nil {
		b.WriteString(" AgentControl(list/status/message/stop) requires exact handle+generation; message/stop admission is not model consumption or drain;")
		agents := make([]subagent.AgentActivity, 0, len(fleet.Agents))
		for _, a := range fleet.Agents {
			if !a.OwnerTerminalEnqueued && !a.StartFailed {
				agents = append(agents, a)
			}
		}
		sort.SliceStable(agents, func(i, j int) bool {
			a, b := agents[i], agents[j]
			if rank(a) != rank(b) {
				return rank(a) < rank(b)
			}
			if !a.AttemptedAt.Equal(b.AttemptedAt) {
				return a.AttemptedAt.Before(b.AttemptedAt)
			}
			return a.Handle < b.Handle
		})
		fmt.Fprintf(&b, " tracked_active_agents=%d omitted_agents=%d;", len(agents), fleet.OmittedAgents+uint64(max(0, len(agents)-4)))
		for _, a := range agents[:min(4, len(agents))] {
			fmt.Fprintf(&b, " agent=%s generation=%d graph=%s task=%s phase=%s age=%s report_available=%t cancellation_requested=%t execution_drain_observed=%t pending_operations=%d;", label(a.Handle), a.Generation, label(a.GraphID), label(a.TaskID), label(a.Phase), age(now, a.AttemptedAt), a.ReportAvailable, a.CancellationRequested, a.ProcessWaitReturned && a.OutputEOFObserved && a.ReaderReturned && a.OwnerExitProcessed, a.ObservedNonterminalOperations)
			path := a.BlockerPath
			if len(path) == 0 && a.RelevantOperation != nil {
				path = []subagent.OperationActivity{*a.RelevantOperation}
			}
			fmt.Fprintf(&b, " omitted_blockers=%d omitted_operation_observations=%d;", max(0, len(path)-3), a.OmittedOperationObservations)
			for _, op := range path[:min(3, len(path))] {
				fmt.Fprintf(&b, " blocker=%s type=%s status=%s phase=%s executable=%s (arguments omitted);", label(op.ID), label(string(op.Type)), label(string(op.Status)), label(string(op.ShellPhase)), label(op.Executable))
			}
		}
	} else {
		b.WriteString(" agents unavailable;")
	}
	if activity != nil {
		active, shown := 0, 0
		for _, op := range activity.Operations {
			if !op.Owned {
				continue
			}
			active++
			if shown < 4 {
				shown++
				fmt.Fprintf(&b, " operation=%s type=%s status=%s phase=%s age=%s drain_confirmed=false;", label(string(op.ID)), label(string(op.Type)), label(string(op.Status)), label(string(op.Phase)), age(now, op.StartedAt))
			}
		}
		fmt.Fprintf(&b, " omitted_active_operations=%d omitted_operation_observations=%d;", active-shown, activity.Omitted)
	}
	const suffix = " [summary truncated]"
	text := b.String()
	if len(text) > 4096 {
		return prefix(text, 4096-len(suffix)) + suffix
	}
	return text
}

func rank(a subagent.AgentActivity) int {
	if a.CancellationRequested {
		return 0
	}
	if a.ReportAvailable {
		return 1
	}
	return 2
}

func label(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, s)
	if s == "" {
		return "unknown"
	}
	return prefix(s, 128)
}

func prefix(s string, limit int) string {
	s = s[:min(len(s), limit)]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func age(now, start time.Time) string {
	if start.IsZero() {
		return "unknown"
	}
	return max(time.Duration(0), now.Sub(start)).Round(time.Second).String()
}
