package activitysummary

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

var now = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

func TestLabelsAreBoundedAndCannotInjectLines(t *testing.T) {
	for _, input := range []string{strings.Repeat("界", 100) + "\nforged status", strings.Repeat("é", 100), "a\xffb"} {
		got := label(input)
		if len(got) > 128 || !utf8.ValidString(got) || strings.ContainsAny(got, "\n\r") {
			t.Fatalf("label=%q", got)
		}
	}
	if got := label("a\nb\x00c\t\r\x1b\u2028\u2029\u202e"); got != "a b c"+strings.Repeat(" ", 6) {
		t.Fatalf("control characters not normalized: %q", got)
	}
	if label("") != "unknown" {
		t.Fatal("empty label is not explicit")
	}
}

func TestAgesUseCallerTimeAndClampFuture(t *testing.T) {
	for _, tc := range []struct {
		start time.Time
		want  string
	}{
		{time.Time{}, "unknown"},
		{now.Add(time.Hour), "0s"},
		{now, "0s"},
		{now.Add(-1500 * time.Millisecond), "2s"},
	} {
		if got := age(now, tc.start); got != tc.want {
			t.Errorf("age(%v)=%s, want %s", tc.start, got, tc.want)
		}
		fleet := &subagent.FleetSnapshot{Agents: []subagent.AgentActivity{{ChildActivity: subagent.ChildActivity{AttemptedAt: tc.start}}}}
		activity := &operation.ActivitySnapshot{Operations: []operation.OperationActivity{{Owned: true, StartedAt: tc.start}}}
		if text := Format(now, fleet, activity); strings.Count(text, "age="+tc.want+" ") != 2 {
			t.Fatalf("caller clock not used for both projections: %s", text)
		}
	}
}

func TestRanksCancellationThenReportThenOldestWithNoMutation(t *testing.T) {
	fleet := subagent.FleetSnapshot{Agents: []subagent.AgentActivity{
		{Handle: "other-new", ChildActivity: subagent.ChildActivity{AttemptedAt: now}},
		{Handle: "report", ChildActivity: subagent.ChildActivity{ReportAvailable: true, AttemptedAt: now}},
		{Handle: "other-b", ChildActivity: subagent.ChildActivity{AttemptedAt: now.Add(-time.Hour)}},
		{Handle: "cancel", ChildActivity: subagent.ChildActivity{CancellationRequested: true, ReportAvailable: true}},
		{Handle: "other-a", ChildActivity: subagent.ChildActivity{AttemptedAt: now.Add(-time.Hour)}},
		{Handle: "terminal-secret", ChildActivity: subagent.ChildActivity{OwnerTerminalEnqueued: true, CancellationRequested: true}},
		{Handle: "failed-secret", ChildActivity: subagent.ChildActivity{StartFailed: true, CancellationRequested: true}},
	}}
	activity := operation.ActivitySnapshot{Operations: []operation.OperationActivity{{ID: "owned", Owned: true}}}
	before, err := json.Marshal([]any{fleet, activity})
	if err != nil {
		t.Fatal(err)
	}
	text := Format(now, &fleet, &activity)
	previous := -1
	for _, handle := range []string{"cancel", "report", "other-a", "other-b"} {
		position := strings.Index(text, "agent="+handle+" ")
		if position <= previous {
			t.Fatalf("wrong ranking: %s", text)
		}
		previous = position
	}
	for _, absent := range []string{"other-new", "terminal-secret", "failed-secret"} {
		if strings.Contains(text, absent) {
			t.Fatalf("unexpected agent %q: %s", absent, text)
		}
	}
	if !strings.Contains(text, "omitted_agents=1;") {
		t.Fatal(text)
	}
	if again := Format(now, &fleet, &activity); again != text {
		t.Fatal("nondeterministic rendering")
	}
	after, err := json.Marshal([]any{fleet, activity})
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("input mutated: %s -> %s (%v)", before, after, err)
	}
}

func TestOmissionsAndOwnedOperations(t *testing.T) {
	fleet := subagent.FleetSnapshot{OmittedAgents: 3}
	for i := range 6 {
		fleet.Agents = append(fleet.Agents, subagent.AgentActivity{Handle: fmt.Sprint(i)})
	}
	for i := range 5 {
		fleet.Agents[0].BlockerPath = append(fleet.Agents[0].BlockerPath, subagent.OperationActivity{ID: fmt.Sprint(i)})
	}
	fleet.Agents[0].OmittedOperationObservations = 7
	activity := operation.ActivitySnapshot{Omitted: 9}
	for i := range 6 {
		activity.Operations = append(activity.Operations, operation.OperationActivity{ID: operation.ID(fmt.Sprint(i)), Owned: true})
	}
	activity.Operations = append(activity.Operations, operation.OperationActivity{ID: "unowned-secret"})
	text := Format(now, &fleet, &activity)
	for _, want := range []string{"omitted_agents=5;", "omitted_blockers=2", "omitted_operation_observations=7", "omitted_active_operations=2", "omitted_operation_observations=9"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	for token, count := range map[string]int{" agent=": 4, " blocker=": 3, " operation=": 4} {
		if got := strings.Count(text, token); got != count {
			t.Fatalf("%q count=%d, want %d: %s", token, got, count, text)
		}
	}
	if strings.Contains(text, "unowned-secret") || strings.Contains(text, "summary truncated") {
		t.Fatal(text)
	}
}

func TestPrivacyAndReportDrainBarriers(t *testing.T) {
	secret := "provider-prompt-command-arguments-output-pid-secret"
	blocker := subagent.OperationActivity{ID: "scan", Type: operation.TypeShell, ShellPhase: operation.ShellPhaseProcess, Executable: "find", TurnID: secret, CallID: secret}
	fleet := subagent.FleetSnapshot{Agents: []subagent.AgentActivity{{
		Handle: "reviewer", ScopeID: secret, Purpose: secret, Lifetime: secret,
		ChildActivity: subagent.ChildActivity{Name: secret, SessionID: secret, OwnerOperationID: secret, ProgressEvidence: secret, ParentTask: secret, OwnedResources: []string{secret}, ReportAvailable: true, RelevantOperation: &blocker},
	}}}
	activity := operation.ActivitySnapshot{Account: executionbudget.Snapshot{Scope: "in_process_operation_envelopes", Owned: 2, Limit: 4}, Operations: []operation.OperationActivity{{ID: "live", Owned: true, CancellationReason: secret}}}
	text := Format(now, &fleet, &activity)
	for _, want := range []string{"scope=in_process_operation_envelopes", "owned=2", "work_limit=4", "report_available=true", "execution_drain_observed=false", "drain_confirmed=false", "blocker=scan", "executable=find (arguments omitted)", "Reports and cancellation do not prove execution/output drain", "admission is not model consumption or drain", "Pending persisted tools remain authoritative", "not semantic acceptance, durable terminal or idle proof", "silence alone is not a stall"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, secret) {
		t.Fatal("private metadata leaked")
	}
	a := &fleet.Agents[0]
	a.ProcessWaitReturned, a.OutputEOFObserved, a.ReaderReturned = true, true, true
	if text := Format(now, &fleet, nil); !strings.Contains(text, "execution_drain_observed=false") {
		t.Fatal("missing owner exit barrier: " + text)
	}
	a.OwnerExitProcessed = true
	if text := Format(now, &fleet, nil); !strings.Contains(text, "execution_drain_observed=true") {
		t.Fatal("drain barriers lost: " + text)
	}
}

func TestUTF8ByteBoundAndControlInjectionAcrossFields(t *testing.T) {
	long := strings.Repeat("界\n\r\x00\u2028\u202e", 100)
	fleet := subagent.FleetSnapshot{OmittedAgents: 5}
	activity := operation.ActivitySnapshot{Account: executionbudget.Snapshot{Scope: long}}
	for range 4 {
		a := subagent.AgentActivity{Handle: long, GraphID: long, TaskID: long, Phase: long}
		for range 3 {
			a.BlockerPath = append(a.BlockerPath, subagent.OperationActivity{ID: long, Type: operation.Type(long), Status: operation.Status(long), ShellPhase: operation.ShellPhase(long), Executable: long})
		}
		fleet.Agents = append(fleet.Agents, a)
		activity.Operations = append(activity.Operations, operation.OperationActivity{ID: operation.ID(long), Type: operation.Type(long), Status: operation.Status(long), Phase: operation.ActivityPhase(long), Owned: true})
	}
	text := Format(now, &fleet, &activity)
	if len(text) > 4096 || len(text) < 4093 || !utf8.ValidString(text) || !strings.HasSuffix(text, " [summary truncated]") {
		t.Fatalf("invalid bounded summary (%d bytes): %q", len(text), text)
	}
	for _, r := range text {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			t.Fatalf("control injection: %q", text)
		}
	}
	if !strings.Contains(text, "admission is not model consumption or drain") {
		t.Fatal("truncation dropped control caveat: " + text)
	}
	// Exercise operation labels even when agent text would consume the bound.
	text = Format(now, nil, &activity)
	if len(text) > 4096 || !utf8.ValidString(text) || strings.ContainsAny(text, "\n\r\x00\u2028\u202e") {
		t.Fatalf("operation control injection: %q", text)
	}
}

func TestUnavailableAndEmptyAreNotIdlePermission(t *testing.T) {
	for _, text := range []string{Format(now, nil, nil), Format(now, &subagent.FleetSnapshot{OmittedAgents: 2}, &operation.ActivitySnapshot{})} {
		if !strings.Contains(text, "idle proof") || !strings.Contains(text, "Pending persisted tools remain authoritative") {
			t.Fatal(text)
		}
	}
	if text := Format(now, nil, nil); !strings.Contains(text, "agents unavailable") || !strings.Contains(text, "resources unavailable") || strings.Contains(text, "AgentControl") {
		t.Fatal(text)
	}
	if text := Format(now, &subagent.FleetSnapshot{OmittedAgents: 2}, nil); !strings.Contains(text, "tracked_active_agents=0 omitted_agents=2") {
		t.Fatal(text)
	}
}
