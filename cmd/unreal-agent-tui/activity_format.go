package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
)

func activityAge(now, since time.Time) string {
	if since.IsZero() {
		return "age unavailable"
	}
	elapsed := max(time.Duration(0), now.Sub(since))
	if elapsed < time.Second {
		return "<1s"
	}
	return elapsed.Round(time.Second).String()
}

// Observation, report and drain are deliberately independent. A terminal
// handler enqueue is not proof of coordinator persistence, and process Wait
// alone is not proof that stdout readers/output delivery have drained.
func childActivityPhase(child subagent.ChildActivity) string {
	if child.StartFailed {
		return "Start failed"
	}
	if child.ProcessWaitReturned && child.ReaderReturned && child.OutputEOFObserved {
		if child.OwnerExitProcessed && child.OwnerTerminalEnqueued {
			return "Execution drained; terminal result enqueued"
		}
		return "Process/output drained; terminal publication pending"
	}
	if child.CancellationRequested {
		return "Stopping; waiting for owned execution drain"
	}
	if child.ReportAvailable {
		return "Review reported; cleanup pending"
	}
	if child.ModelTextObserved {
		return "Model text observed; execution still owned"
	}
	if child.ProcessStartedAt.IsZero() {
		return "Queued; process start not observed"
	}
	return "Executing"
}

func childActivityLine(child subagent.ChildActivity, now time.Time) string {
	text := childActivityPhase(child) + " · " + singleLine(child.Name) + " · " + activityAge(now, child.AttemptedAt)
	if op := child.RelevantOperation; op != nil {
		text += fmt.Sprintf(" · %d pending · %s · observed %s", child.ObservedNonterminalOperations, operationBlockerPath(*op), activityAge(now, op.FirstObservedAt))
	}
	if !child.LastFrameAt.IsZero() {
		text += " · last observed frame " + activityAge(now, child.LastFrameAt) + " ago"
	}
	return text
}

func childActivityDetail(child subagent.ChildActivity, handle string, now time.Time) string {
	var lines []string
	lines = append(lines, childActivityLine(child, now), "Handle: "+singleLine(handle), "Owner operation: "+singleLine(child.OwnerOperationID), "Session: "+singleLine(child.SessionID))
	for _, op := range child.Operations {
		phase := string(op.Status)
		if op.ShellPhase != "" {
			phase += " / " + string(op.ShellPhase)
		}
		lines = append(lines, fmt.Sprintf("  %s → %s · %s · %s", singleLine(child.Name), op.Type, singleLine(phase), activityAge(now, op.FirstObservedAt)))
	}
	lines = append(lines, fmt.Sprintf("Report available: %t (not proof of drain)\nProcess joined: %t · output EOF: %t · reader joined: %t · owner exit processed: %t\nTerminal enqueued: %t (not proof of parent session persistence)", child.ReportAvailable, child.ProcessWaitReturned, child.OutputEOFObserved, child.ReaderReturned, child.OwnerExitProcessed, child.OwnerTerminalEnqueued))
	if child.LastInputID != "" {
		lines = append(lines, "Last input: "+singleLine(child.LastInputID)+" · barrier: "+singleLine(child.InputBarrier)+" (not proof of model consumption)")
	}
	lines = append(lines, "Live-only; silence is not a stall verdict. Commands, arguments and output are omitted.", "Actions: /agents message "+handle+" TEXT · /agents stop "+handle)
	return strings.Join(lines, "\n")
}

func operationBlockerPath(op subagent.OperationActivity) string { return operationBlockerPathAt(op, 0) }
func operationBlockerPathAt(op subagent.OperationActivity, depth int) string {
	label := string(op.Type)
	if op.Executable != "" {
		label += " → " + singleLine(op.Executable) + " …"
	}
	phase := string(op.Status)
	if op.ShellPhase != "" {
		phase = string(op.ShellPhase)
	}
	label += " [" + singleLine(phase) + "]"
	if depth >= 3 {
		return label
	}
	for _, nested := range op.Descendants[:min(4, len(op.Descendants))] {
		label += " → " + operationBlockerPathAt(nested, depth+1)
	}
	return label
}
