package agentrunner

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// runnerActivitySummary only reads detached projections. It never asks actors to
// schedule, revalidate, persist, message, cancel, or join. Command arguments,
// output, prompts, process IDs and provider data do not enter heartbeat text.
func runnerActivitySummary(hub *subagent.Hub, manager *operation.LocalOperationManager) string {
	var b strings.Builder
	b.WriteString("Live supervision snapshot (not semantic acceptance or durable terminal proof):")
	if manager != nil {
		snapshot := manager.ActivitySnapshot()
		fmt.Fprintf(&b, " resources scope=%s owned=%d peak=%d work_owned=%d work_limit=%d management_owned=%d management_limit=%d;", snapshot.Account.Scope, snapshot.Account.Owned, snapshot.Account.Peak, snapshot.Account.WorkOwned, snapshot.Account.Limit, snapshot.Account.ManagementOwned, snapshot.Account.ManagementLimit)
		shown, active := 0, 0
		for _, op := range snapshot.Operations {
			if !op.Owned {
				continue
			}
			active++
			if shown == 4 {
				continue
			}
			shown++
			fmt.Fprintf(&b, " operation=%s type=%s status=%s phase=%s age=%s drain_confirmed=false;", summaryLabel(string(op.ID)), op.Type, op.Status, op.Phase, summaryAge(op.StartedAt))
		}
		if active > shown {
			fmt.Fprintf(&b, " omitted_active_operations=%d;", active-shown)
		}
		if snapshot.Omitted > 0 {
			fmt.Fprintf(&b, " omitted_operation_observations=%d;", snapshot.Omitted)
		}
	}
	if hub != nil {
		fleet := hub.Snapshot()
		const limit = 4
		for i, a := range fleet.Agents {
			if i == limit {
				break
			}
			fmt.Fprintf(&b, " agent=%s generation=%d graph=%s task=%s phase=%s age=%s report_available=%t cancellation_requested=%t execution_drain_observed=%t;", summaryLabel(a.Handle), a.Generation, summaryLabel(a.GraphID), summaryLabel(a.TaskID), a.Phase, summaryAge(a.AttemptedAt), a.ReportAvailable, a.CancellationRequested, a.ProcessWaitReturned && a.OutputEOFObserved && a.ReaderReturned && a.OwnerExitProcessed)
			for j, blocker := range a.BlockerPath {
				if j == 3 {
					break
				}
				fmt.Fprintf(&b, " blocker=%s type=%s status=%s executable=%s;", summaryLabel(blocker.ID), blocker.Type, blocker.Status, summaryLabel(blocker.Executable))
			}
		}
		omitted := fleet.OmittedAgents
		if len(fleet.Agents) > limit {
			omitted += uint64(len(fleet.Agents) - limit)
		}
		if omitted > 0 {
			fmt.Fprintf(&b, " omitted_agents=%d;", omitted)
		}
		b.WriteString(" controls=AgentControl(list/status/message/stop) with exact handle+generation;")
	}
	text := b.String()
	if len(text) > 4096 {
		text = text[:4096-len(" [summary truncated]")]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		text += " [summary truncated]"
	}
	return text
}
func summaryLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	if len(s) > 128 {
		s = s[:128]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	if s == "" {
		return "unknown"
	}
	return s
}
func summaryAge(start time.Time) string {
	if start.IsZero() {
		return "unknown"
	}
	return max(time.Duration(0), time.Since(start)).Round(time.Second).String()
}
