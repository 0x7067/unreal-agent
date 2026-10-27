package workerruntime

import (
	"fmt"
	"sort"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
)

// summary is a bounded, redacted live observation suitable for the coordinator
// heartbeat. It performs no scheduling, filesystem access, model call or join.
func (o *ownedExecution) summary() string {
	if o.hub == nil {
		return "Live supervision unavailable; pending tools remain owned."
	}
	snapshot := o.hub.Snapshot()
	sort.Slice(snapshot.Agents, func(i, j int) bool {
		a, b := snapshot.Agents[i], snapshot.Agents[j]
		ra, rb := summaryRank(a), summaryRank(b)
		if ra != rb {
			return ra < rb
		}
		if !a.AttemptedAt.Equal(b.AttemptedAt) {
			return a.AttemptedAt.Before(b.AttemptedAt)
		}
		return a.Handle < b.Handle
	})
	for _, a := range snapshot.Agents {
		if a.OwnerTerminalEnqueued || a.StartFailed {
			continue
		}
		phase := a.Phase
		if a.CancellationRequested {
			phase = "cancellation requested; execution/output drain unconfirmed"
		} else if a.ReportAvailable {
			phase = "report available; owned execution still pending"
		}
		text := fmt.Sprintf("Live-only activity: agent %q handle %s generation %d; %s; observed age %s; %d observed pending operations.", a.Name, a.Handle, a.Generation, phase, time.Since(a.AttemptedAt).Round(time.Second), a.ObservedNonterminalOperations)
		if op := a.RelevantOperation; op != nil {
			text += fmt.Sprintf(" Blocker: %s phase %s status %s", op.Type, op.ShellPhase, op.Status)
			if op.Executable != "" {
				text += " executable " + op.Executable + " (arguments omitted)"
			}
			text += "."
		}
		text += " Use AgentControl status/message/stop with this exact handle and generation. Message/stop admission is not model consumption or drain; silence alone is not a stall."
		if runes := []rune(text); len(runes) > 2048 {
			text = string(runes[:2048])
		}
		return text
	}
	if snapshot.OmittedAgents > 0 {
		return fmt.Sprintf("Live supervision snapshot omitted %d agents; absence is not idle proof.", snapshot.OmittedAgents)
	}
	if o.account != nil {
		s := o.account.Snapshot()
		return fmt.Sprintf("Live supervision: no active tracked agents; %d in-process operation envelopes owned (limit %d, peak %d, rejected %d). Pending persisted tools remain authoritative; this is not process-tree drain proof.", s.Owned, s.Limit, s.Peak, s.Rejected)
	}
	return "No active tracked agents in the live-only snapshot; pending tools remain authoritative."
}
func summaryRank(a subagent.AgentActivity) int {
	if a.CancellationRequested {
		return 0
	}
	if a.ReportAvailable {
		return 1
	}
	return 2
}
