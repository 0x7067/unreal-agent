package subagent

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func activityLine(t *testing.T, seq sessionstore.Sequence, data any) []byte {
	t.Helper()
	kind := sessionstore.ItemToolCallStatus
	if _, ok := data.(sessionstore.ModelResponse); ok {
		kind = sessionstore.ItemModelResponse
	}
	line, err := json.Marshal(sessionstore.Item{Sequence: seq, Kind: kind, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return line
}

func activityTool(id string, status operation.Status) sessionstore.ToolCallStatus {
	return sessionstore.ToolCallStatus{TurnID: "turn", CallID: "shell-call", Operations: []operation.Operation{{
		ID: operation.ID(id), Type: operation.TypeShell, Status: status,
		State: []byte(`{"Phase":"process","Input":{"Command":"SECRET_COMMAND","Directory":"SECRET_WORKSPACE"},"InlineOut":"SECRET_OUTPUT"}`),
	}}}
}

func TestActivityProjectionTextBlockerReplayAndGeneration(t *testing.T) {
	now := time.Unix(1000, 0)
	p := &activityProjection{snapshot: ActivitySnapshot{ScopeID: "scope"}}
	p.begin("reviewer", "session", 1, "agent-op", now)
	observe := func(seq sessionstore.Sequence, data any) {
		p.observeLine("session", 1, activityLine(t, seq, data), now.Add(time.Second))
	}
	observe(1, activityTool("shell-op", operation.StatusAwaiting))
	observe(2, sessionstore.ModelResponse{Response: llm.Response{Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Text: "SECRET_REPORT"}}}}})
	c := p.snapshotAt(now).Children[0]
	if !c.ModelTextObserved || c.ModelTextSequence != 2 || c.ReaderReturned || c.ProcessWaitReturned || c.OwnerTerminalEnqueued {
		t.Fatalf("text was confused with execution completion: %+v", c)
	}
	if c.RelevantOperation == nil || c.RelevantOperation.ID != "shell-op" || c.RelevantOperation.CallID != "shell-call" || c.RelevantOperation.ShellPhase != operation.ShellPhaseProcess || c.ObservedNonterminalOperations != 1 {
		t.Fatalf("missing relevant last-observed blocker: %+v", c)
	}
	observe(3, activityTool("shell-op", operation.StatusCompleted))
	observe(2, activityTool("shell-op", operation.StatusAwaiting)) // Replay.
	observe(4, activityTool("shell-op", operation.StatusAwaiting)) // Even a newer snapshot cannot resurrect the same terminal ID.
	c = p.snapshotAt(now).Children[0]
	if c.RelevantOperation != nil || c.Operations[0].Status != operation.StatusCompleted || c.LastSequence != 4 {
		t.Fatalf("terminal resurrected: %+v", c)
	}
	p.begin("reviewer", "session", 2, "message-owner", now.Add(time.Minute))
	observe(100, activityTool("stale-generation", operation.StatusAwaiting))
	p.observeLine("session", 2, activityLine(t, 2, activityTool("old-sequence", operation.StatusAwaiting)), now)
	c = p.snapshotAt(now).Children[0]
	if c.Generation != 2 || c.OwnerOperationID != "message-owner" || c.ModelTextObserved || len(c.Operations) != 0 || c.LastSequence != 4 {
		t.Fatalf("restart leaked/replayed old attempt: %+v", c)
	}
	p.observeLine("session", 2, activityLine(t, 5, activityTool("fresh", operation.StatusReady)), now)
	p.change("session", 2, func(c *ChildActivity) { c.ReaderReturned = true })
	p.observeLine("session", 2, activityLine(t, 6, activityTool("too-late", operation.StatusAwaiting)), now)
	c = p.snapshotAt(now).Children[0]
	if len(c.Operations) != 1 || c.Operations[0].ID != "fresh" || c.LastSequence != 5 {
		t.Fatalf("post-reader frame applied: %+v", c)
	}
}

func TestActivityProjectionBoundsRedactionAndDetachedCopy(t *testing.T) {
	now := time.Unix(1000, 0)
	p := &activityProjection{}
	p.begin("reviewer", "session", 1, "owner", now)
	data := activityTool("unused", operation.StatusCompleted)
	data.CallID = strings.Repeat("é", activityIdentityBytes)
	data.Operations = nil
	for i := range MaxActivityOperations + 10 {
		op := activityTool(fmt.Sprint(i), operation.StatusCompleted).Operations[0]
		data.Operations = append(data.Operations, op)
	}
	p.observeLine("session", 1, activityLine(t, 1, data), now)
	p.observeLine("session", 1, activityLine(t, 2, activityTool("new-live-op", operation.StatusAwaiting)), now)
	for i := range 2 {
		// Oversized identities sharing a prefix must be omitted, never merged.
		p.observeLine("session", 1, activityLine(t, sessionstore.Sequence(i+3), activityTool(strings.Repeat("x", activityIdentityBytes)+fmt.Sprint(i), operation.StatusAwaiting)), now)
	}
	p.observeLine("session", 1, []byte(strings.Repeat("x", activityFrameBytes+1)), now)
	p.observeLine("session", 1, []byte(`not-json`), now)
	p.observeLine("session", 1, activityLine(t, 0, activityTool("zero", operation.StatusAwaiting)), now)
	for i := 1; i < MaxActivityChildren+5; i++ {
		p.begin("other", fmt.Sprint(i), 1, "op", now)
	}
	p.begin("restored", strings.Repeat("x", activityIdentityBytes+1), 1, "op", now)
	snapshot := p.snapshotAt(now)
	c := snapshot.Children[0]
	if len(snapshot.Children) != MaxActivityChildren || snapshot.UntrackedStarts != 6 || len(c.Operations) != MaxActivityOperations || c.OmittedOperationObservations != 12 || c.SkippedFrames != 1 {
		t.Fatalf("incorrect bounds/counters: children=%d untracked=%d child=%+v", len(snapshot.Children), snapshot.UntrackedStarts, c)
	}
	if !c.Operations[0].IdentityTruncated || len(c.Operations[0].CallID) > activityIdentityBytes || c.RelevantOperation == nil || c.RelevantOperation.ID != "new-live-op" {
		t.Fatalf("incorrect retained slots/identity: %+v", c)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(encoded), "SECRET_") {
		t.Fatalf("free-form diagnostic leak: %s, %v", encoded, err)
	}
	snapshot.Children[0].Operations[0].ID = "mutated"
	snapshot.Children[0].Name = "mutated"
	if c = p.snapshotAt(now).Children[0]; c.Name == "mutated" || c.Operations[0].ID == "mutated" {
		t.Fatal("snapshot exposed mutable projection storage")
	}
	// Unknown/free-form enums and oversized shell states cannot leak into summaries.
	q := &activityProjection{}
	q.begin("child", "s", 1, "o", now)
	data = activityTool("op", "SECRET_STATUS")
	data.Operations[0].Type = "SECRET_TYPE"
	q.observeLine("s", 1, activityLine(t, 1, data), now)
	data = activityTool("shell", operation.StatusAwaiting)
	data.Operations[0].State = []byte(`{"Phase":"SECRET_PHASE"}`)
	q.observeLine("s", 1, activityLine(t, 2, data), now)
	data.Operations[0].ID = "large-shell"
	data.Operations[0].State = []byte(`{"Input":"` + strings.Repeat("x", activityStateBytes) + `"}`)
	q.observeLine("s", 1, activityLine(t, 3, data), now)
	c = q.snapshotAt(now).Children[0]
	if c.Operations[0].Type != "unknown" || c.Operations[0].Status != "unknown" || !c.Operations[1].DetailsUnavailable || !c.Operations[2].DetailsUnavailable {
		t.Fatalf("unsafe enum/state extraction: %+v", c)
	}
	// Relevant-operation pointers are detached too.
	c.RelevantOperation.ID = "mutated"
	if q.snapshotAt(now).Children[0].RelevantOperation.ID == "mutated" {
		t.Fatal("snapshot exposed relevant-operation pointer")
	}
}

func TestActivitySnapshotBlockedActorAndShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s, err := New(ctx, Config{Command: []string{"unused"}, Inbox: lifecycleInbox{}})
		if err != nil {
			t.Fatal(err)
		}
		initial := s.ActivitySnapshot()
		if initial.ScopeID == "" || initial.SupervisorJoined || len(initial.Children) != 0 {
			t.Fatalf("startup snapshot: %+v", initial)
		}
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		defer unblock()
		if err := s.do(func(*state) { close(entered); <-release }); err != nil {
			t.Fatal(err)
		}
		<-entered
		cancel()
		before := time.Now()
		for range 100 {
			got := s.ActivitySnapshot()
			if got.SupervisorJoined || !got.ShutdownRequested || got.ScopeID != initial.ScopeID {
				t.Fatalf("cancellation falsely joined blocked actor: %+v", got)
			}
		}
		if time.Now() != before {
			t.Fatal("snapshot waited for work in synthetic time")
		}
		unblock()
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		if got := s.ActivitySnapshot(); !got.SupervisorJoined || !got.ShutdownRequested {
			t.Fatalf("joined snapshot: %+v", got)
		}
	})
}

type activityGatedInbox struct{ entered, release chan struct{} }

func (g activityGatedInbox) Submit(ctx context.Context, input inbox.Input) error {
	close(g.entered)
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

const activityHelperEnv = "UNREAL_SUBAGENT_ACTIVITY_HELPER"

func TestActivityChild(t *testing.T) {
	if os.Getenv(activityHelperEnv) != "1" {
		return
	}
	stdin := bufio.NewReader(os.Stdin)
	if _, err := stdin.ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
	_, _ = os.Stdout.Write(append(activityLine(t, 1, activityTool("owned-shell", operation.StatusAwaiting)), '\n'))
	response := sessionstore.ModelResponse{TurnID: "report-turn", Response: llm.Response{Output: []llm.Item{
		{Type: llm.ItemMessage, Data: llm.Message{Text: "SECRET_REPORT"}},
		{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "parent-message", Name: tool.SendMessageName, Arguments: `{"to":"parent","message":"SECRET_MESSAGE"}`}},
	}}}
	_, _ = os.Stdout.Write(append(activityLine(t, 2, response), '\n'))
	_ = os.Stdout.Close() // EOF is independent of process exit.
	if _, err := stdin.ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
}

func activityRunOperation(t *testing.T, name string) operation.Operation {
	t.Helper()
	data, err := json.Marshal(toolsubagent.RunPlan{Name: name, Prompt: "SECRET_PROMPT"})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: "owner-op", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
}

func activityAwait(t *testing.T, s *Supervisor, predicate func(ActivitySnapshot) bool) ActivitySnapshot {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		got := s.ActivitySnapshot()
		if predicate(got) {
			return got
		}
		select {
		case <-deadline.C:
			t.Fatalf("activity condition not observed: %+v", got)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestActivitySnapshotReportBlockedParentIOAndChildJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	gate := activityGatedInbox{make(chan struct{}), make(chan struct{})}
	s, err := New(ctx, Config{
		Command: []string{os.Args[0], "-test.run=^TestActivityChild$", "--"},
		Env:     append(os.Environ(), activityHelperEnv+"=1"), Inbox: gate, Stderr: os.Stderr,
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	var once sync.Once
	unblock := func() { once.Do(func() { close(gate.release) }) }
	t.Cleanup(func() {
		unblock()
		cancel()
		waitCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := s.Wait(waitCtx); err != nil {
			t.Errorf("helper cleanup: %v", err)
		}
	})
	if err := s.run.AddRemoteJob(activityRunOperation(t, "reviewer")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("parent delivery not entered")
	}
	// The actor is inside blocked parent Submit; the typed frame was projected
	// before actor delivery. Query completion does not depend on releasing I/O.
	result := make(chan ActivitySnapshot, 1)
	go func() { result <- s.ActivitySnapshot() }()
	var got ActivitySnapshot
	select {
	case got = <-result:
	case <-time.After(time.Second):
		t.Fatal("snapshot blocked behind actor I/O")
	}
	c := got.Children[0]
	if !c.ModelTextObserved || c.ProcessWaitReturned || c.ReaderReturned || c.OwnerTerminalEnqueued || c.OwnerOperationID != "owner-op" || c.RelevantOperation == nil || c.RelevantOperation.ID != "owned-shell" {
		t.Fatalf("report/blocker/lifecycle evidence wrong: %+v", c)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "SECRET_") {
		t.Fatalf("diagnostic secret leak: %s", encoded)
	}
	unblock()
	activityAwait(t, s, func(s ActivitySnapshot) bool { return s.Children[0].OutputEOFObserved })
	if c := s.ActivitySnapshot().Children[0]; c.ProcessWaitReturned || c.ReaderReturned || c.OwnerTerminalEnqueued {
		t.Fatalf("EOF falsely proved process/operation completion: %+v", c)
	}
	if err := s.do(func(state *state) { _, _ = io.WriteString(state.children["reviewer"].stdin, "release\n") }); err != nil {
		t.Fatal(err)
	}
	got = activityAwait(t, s, func(s ActivitySnapshot) bool {
		c := s.Children[0]
		return c.ReaderReturned && c.OwnerExitProcessed && c.OwnerTerminalEnqueued
	})
	c = got.Children[0]
	if !c.ProcessWaitReturned || !c.OwnerTerminalEnqueued || c.StartFailed || got.SupervisorJoined {
		t.Fatalf("child join/result enqueue barriers wrong: %+v", c)
	}
	// The old awaiting tool snapshot remains explicitly historical even though
	// this process has joined; it is never relabeled as current manager state.
	if c.RelevantOperation == nil || c.RelevantOperation.Status != operation.StatusAwaiting {
		t.Fatalf("unexpected historical snapshot mutation: %+v", c)
	}
	cancel()
	if err := s.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !s.ActivitySnapshot().SupervisorJoined {
		t.Fatal("successful Wait not reflected")
	}
}

func TestActivitySnapshotStartFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, err := New(ctx, Config{Command: []string{"/nonexistent-unreal-activity-test-runner"}, Inbox: lifecycleInbox{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = s.Wait(context.Background()) })
	if err := s.run.AddRemoteJob(activityRunOperation(t, "failed")); err != nil {
		t.Fatal(err)
	}
	got := activityAwait(t, s, func(s ActivitySnapshot) bool {
		return len(s.Children) == 1 && s.Children[0].OwnerExitProcessed && s.Children[0].OwnerTerminalEnqueued
	})
	c := got.Children[0]
	if !c.StartFailed || !c.OwnerTerminalEnqueued || !c.ProcessStartedAt.IsZero() || c.ProcessWaitReturned || c.ReaderReturned {
		t.Fatalf("startup failure invented execution: %+v", c)
	}
}

type activityGatedStdin struct{ entered, release chan struct{} }

func (g activityGatedStdin) Write(data []byte) (int, error) {
	close(g.entered)
	<-g.release
	return len(data), nil
}
func (activityGatedStdin) Close() error { return nil }

func TestActivitySnapshotCancellationBlockedChildStdin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s, err := New(ctx, Config{Command: []string{"unused"}, Inbox: lifecycleInbox{}})
		if err != nil {
			t.Fatal(err)
		}
		gate := activityGatedStdin{make(chan struct{}), make(chan struct{})}
		var once sync.Once
		unblock := func() { once.Do(func() { close(gate.release) }) }
		defer unblock()
		owner := activityRunOperation(t, "gated")
		// A private fake child isolates the actor's real cancel->stdin path;
		// no subprocess or model is needed to prove this blocking boundary.
		if err := s.do(func(state *state) {
			c := &child{name: "gated", sessionID: "session", generation: 1, owner: &owner, stdin: gate}
			state.children[c.name], state.owners[owner.ID] = c, c
			s.activity.begin(c.name, c.sessionID, c.generation, owner.ID, time.Now())
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.run.CancelRemoteJob(owner.ID, "stop fake child"); err != nil {
			t.Fatal(err)
		}
		<-gate.entered
		before := time.Now()
		c := s.ActivitySnapshot().Children[0]
		if !c.CancellationRequested || c.ProcessWaitReturned || c.ReaderReturned || c.OwnerExitProcessed || c.OwnerTerminalEnqueued {
			t.Fatalf("blocked stop write falsely acknowledged termination: %+v", c)
		}
		if time.Now() != before {
			t.Fatal("snapshot waited for child stdin")
		}
		cancel()
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := s.Wait(waitCtx); err != context.DeadlineExceeded {
			t.Fatalf("Wait while stdin blocked = %v", err)
		}
		got := s.ActivitySnapshot()
		if !got.ShutdownRequested || got.SupervisorJoined {
			t.Fatalf("shutdown request confused with join: %+v", got)
		}
		unblock()
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestActivityOverflowRetainsQuietNestedBlockerAfterTerminalHistory(t *testing.T) {
	now := time.Unix(2000, 0)
	p := &activityProjection{}
	p.begin("reviewer", "session", 1, "owner", now)
	sequence := sessionstore.Sequence(0)
	observe := func(op operation.Operation) {
		sequence++
		p.observeLine("session", 1, activityLine(t, sequence, sessionstore.ToolCallStatus{CallID: "quiet-call", Operations: []operation.Operation{op}}), now.Add(time.Duration(sequence)*time.Second))
	}
	for i := 0; i < MaxActivityOperations+8; i++ {
		observe(activityTool(fmt.Sprintf("finished-%d", i), operation.StatusCompleted).Operations[0])
	}
	shell := activityTool("quiet-find", operation.StatusAwaiting).Operations[0]
	shell.State = []byte(`{"Phase":"process","Input":{"Command":"find / --SECRET_TOKEN"}}`)
	checkpoint, _ := json.Marshal(map[string]any{"version": 1, "attempts": map[string]any{"worker": map[string]any{"child_operation": shell}}})
	remote, _ := json.Marshal(operation.RemoteJobState{Plan: operation.RemoteJobPlan{Type: "taskgraph.run", Version: 1, Data: []byte(`{}`)}, Handle: checkpoint})
	graph := operation.Operation{ID: "quiet-graph", Type: operation.TypeRemoteJob, Status: operation.StatusAwaiting, State: remote}
	observe(graph)
	got := p.snapshotAt(now).Children[0]
	if len(got.Operations) != MaxActivityOperations || got.OverflowOperation == nil || got.RelevantOperation == nil || got.RelevantOperation.ID != "quiet-graph" || got.ObservedNonterminalOperations != 1 {
		t.Fatalf("quiet blocker hidden by terminal history: %+v", got)
	}
	if got.OmittedOperationObservations != 8 || len(got.RelevantOperation.Descendants) != 1 || got.RelevantOperation.Descendants[0].Executable != "find" {
		t.Fatalf("nested blocker or omissions incorrect: %+v", got)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "SECRET_TOKEN") {
		t.Fatal("overflow argv leaked")
	}
	got.OverflowOperation.Descendants[0].Executable = "mutated"
	got.RelevantOperation.Descendants[0].ID = "mutated"
	fresh := p.snapshotAt(now).Children[0]
	if fresh.RelevantOperation.Descendants[0].Executable != "find" || fresh.RelevantOperation.Descendants[0].ID != "quiet-find" {
		t.Fatal("overflow snapshot aliases retained descendants")
	}
	graph.Status = operation.StatusCompleted
	observe(graph)
	graph.Status = operation.StatusAwaiting
	observe(graph) // New sequence cannot regress a retained terminal ID.
	observe(activityTool("finished-0", operation.StatusAwaiting).Operations[0])
	got = p.snapshotAt(now).Children[0]
	if got.RelevantOperation != nil || got.ObservedNonterminalOperations != 0 || got.OverflowOperation.Status != operation.StatusCompleted {
		t.Fatalf("retained tombstone resurrected: %+v", got)
	}
	observe(shell) // A distinct quiet live operation can replace the terminal overflow tombstone.
	got = p.snapshotAt(now).Children[0]
	if got.RelevantOperation == nil || got.RelevantOperation.ID != "quiet-find" || got.RelevantOperation.Executable != "find" || got.OverflowEvictions != 1 || got.OmittedOperationObservations != 8 {
		t.Fatalf("overflow replacement policy incorrect: %+v", got)
	}
	another := shell
	another.ID = "omitted-concurrent"
	observe(another)
	got = p.snapshotAt(now).Children[0]
	if got.RelevantOperation.ID != "quiet-find" || got.OmittedOperationObservations != 9 {
		t.Fatalf("live overflow blocker displaced or omission hidden: %+v", got)
	}
}
