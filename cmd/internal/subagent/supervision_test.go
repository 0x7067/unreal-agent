package subagent

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

const supervisionHelperEnv = "UNREAL_SUPERVISION_HELPER"

func TestSupervisionChild(t *testing.T) {
	dir := os.Getenv(supervisionHelperEnv)
	if dir == "" {
		return
	}
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)
	reader := bufio.NewReader(os.Stdin)
	if _, err := reader.ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
	tool := activityTool("blocked-scan", operation.StatusAwaiting)
	tool.Operations[0].State = []byte(`{"Phase":"process","Input":{"Command":"find / --SECRET_TOKEN"}}`)
	_, _ = os.Stdout.Write(append(activityLine(t, 1, tool), '\n'))
	report := sessionstore.ModelResponse{TurnID: "report", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Text: "final review report"}}}}}
	_, _ = os.Stdout.Write(append(activityLine(t, 2, report), '\n'))
	_ = os.WriteFile(filepath.Join(dir, "reported"), nil, 0600)
	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var input inbox.Input
		if json.Unmarshal(line, &input) != nil {
			return
		}
		_ = os.WriteFile(filepath.Join(dir, "input-written"), nil, 0600)
		for {
			if _, err := os.Stat(filepath.Join(dir, "ack")); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		encoded, _ := json.Marshal(sessionstore.Item{Sequence: 3, Kind: sessionstore.ItemInput, Data: input})
		_, _ = os.Stdout.Write(append(encoded, '\n'))
	}()
	<-interrupted
	_ = os.WriteFile(filepath.Join(dir, "stop-requested"), nil, 0600)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("helper never released")
		}
		time.Sleep(time.Millisecond)
	}
}
func supervisionOperation(t *testing.T, id string, typ operation.RemoteJobPlanType, plan any) operation.Operation {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: 1, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
}
func supervisionStart(t *testing.T, hub *Hub, lifetime, id string) (*Supervisor, string, context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	s, err := New(ctx, Config{Hub: hub, Command: []string{os.Args[0], "-test.run=^TestSupervisionChild$", "--"}, Env: append(os.Environ(), supervisionHelperEnv+"="+dir), Inbox: lifecycleInbox{}, Stderr: os.Stderr})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "ack"), nil, 0600)
		_ = os.WriteFile(filepath.Join(dir, "release"), nil, 0600)
		cancel()
		wait, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := s.Wait(wait); err != nil {
			t.Errorf("owned helper cleanup: %v", err)
		}
	})
	op := supervisionOperation(t, id, toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: "reviewer", Prompt: "review", Purpose: "scan review", ParentTask: "parent-task", Lifetime: lifetime, OwnedResources: []string{"owned-shell"}, GraphID: "graph", TaskID: "review", TaskGeneration: 7})
	if err := s.run.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	lifecycleFile(t, filepath.Join(dir, "reported"))
	activityAwait(t, s, func(a ActivitySnapshot) bool { return len(a.Children) == 1 && a.Children[0].ReportAvailable })
	return s, dir, cancel
}
func supervisionHub(t *testing.T) *Hub {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	h := NewHub(ctx)
	t.Cleanup(func() {
		cancel()
		wait, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := h.Wait(wait); err != nil {
			t.Errorf("hub cleanup: %v", err)
		}
	})
	return h
}
func controlSubmit(t *testing.T, h *Hub, id, action string, a ChildActivity, message string) operation.Operation {
	t.Helper()
	op := supervisionOperation(t, id, ControlPlanType, toolsubagent.ControlPlan{Action: action, Handle: a.Handle, Generation: uint64(a.Generation), Message: message})
	if err := h.control.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	return op
}
func controlAwait(t *testing.T, h *Hub, id operation.ID, terminal bool) operation.Operation {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case op := <-h.control.updates:
			if op.ID == id && (!terminal || activityTerminal(op.Status)) {
				return op
			}
		case <-timeout.C:
			t.Fatalf("control %s not published", id)
		}
	}
}
func controlResult(t *testing.T, op operation.Operation) toolsubagent.ControlOutcome {
	t.Helper()
	r, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	var out toolsubagent.ControlOutcome
	if err := json.Unmarshal([]byte(r.TerminalResult), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestTaskReportCleanupAndExplicitBackground(t *testing.T) {
	hub := supervisionHub(t)
	task, taskDir, _ := supervisionStart(t, hub, "task", "task-owner")
	lifecycleFile(t, filepath.Join(taskDir, "stop-requested"))
	snap := hub.Snapshot()
	a := snap.Agents[0]
	if !a.ReportAvailable || !a.CancellationRequested || a.ProcessWaitReturned || a.OwnerTerminalEnqueued || a.Phase != "stopping" {
		t.Fatalf("report forged drain: %+v", a)
	}
	if a.Purpose != "scan review" || a.Lifetime != "task" || a.ParentTask != "parent-task" || a.TaskGeneration != 7 || a.RelevantOperation == nil || a.RelevantOperation.Executable != "find" || len(a.BlockerPath) != 1 {
		t.Fatalf("missing causal metadata: %+v", a)
	}
	encoded, _ := json.Marshal(snap)
	if strings.Contains(string(encoded), "SECRET_TOKEN") {
		t.Fatal("argv leaked")
	}
	background, bgDir, _ := supervisionStart(t, hub, "background", "bg-owner")
	if background.ActivitySnapshot().Children[0].CancellationRequested {
		t.Fatal("background was automatically stopped")
	}
	if err := os.WriteFile(filepath.Join(taskDir, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	activityAwait(t, task, func(s ActivitySnapshot) bool { return s.Children[0].OwnerTerminalEnqueued })
	// Self-cleanup must never publish Canceling->Completed: remote lifecycle
	// reducers intentionally prohibit that non-monotone owner transition.
	terminal := false
	for !terminal {
		select {
		case update := <-task.run.updates:
			if update.Status == operation.StatusCanceling {
				t.Fatal("self-cleanup published owner cancellation")
			}
			remote, err := operation.DecodeRemoteJobState(update)
			if err != nil {
				t.Fatal(err)
			}
			var handle toolsubagent.RunHandle
			if err := json.Unmarshal(remote.Handle, &handle); err != nil {
				t.Fatal(err)
			}
			if handle.Handle == "" || handle.Generation != 1 {
				t.Fatalf("owner capability not published: %+v", handle)
			}
			if activityTerminal(update.Status) {
				if update.Status != operation.StatusCompleted {
					t.Fatalf("task result: %s", update.Status)
				}
				terminal = true
			}
		case <-time.After(5 * time.Second):
			t.Fatal("missing task terminal update")
		}
	}
	if background.ActivitySnapshot().Children[0].CancellationRequested {
		t.Fatal("unrelated completion stopped background")
	}
	child := background.ActivitySnapshot().Children[0]
	op := controlSubmit(t, hub, "stop-background", "stop", child, "")
	result := controlResult(t, controlAwait(t, hub, op.ID, true))
	if result.State != "cancellation_requested" || result.DrainConfirmed {
		t.Fatalf("stop lied: %+v", result)
	}
	lifecycleFile(t, filepath.Join(bgDir, "stop-requested"))
	if background.ActivitySnapshot().Children[0].ProcessWaitReturned {
		t.Fatal("stop request counted as process exit")
	}
	_ = os.WriteFile(filepath.Join(bgDir, "release"), nil, 0600)
	activityAwait(t, background, func(s ActivitySnapshot) bool { return s.Children[0].OwnerTerminalEnqueued })
	op = controlSubmit(t, hub, "stop-again", "stop", child, "")
	if out := controlResult(t, controlAwait(t, hub, op.ID, true)); out.State != "drained" || !out.DrainConfirmed {
		t.Fatalf("idempotent stop: %+v", out)
	}
}
func TestGraphScopedControlsAndPersistedMessageBarrier(t *testing.T) {
	hub := supervisionHub(t)
	first, dir, _ := supervisionStart(t, hub, "background", "first-owner")
	sibling, _, _ := supervisionStart(t, hub, "background", "sibling-owner")
	a := first.ActivitySnapshot().Children[0]
	b := sibling.ActivitySnapshot().Children[0]
	if a.Name != b.Name || a.Handle == b.Handle || len(hub.Snapshot().Agents) != 2 {
		t.Fatal("supervisor names became authority")
	}
	op := controlSubmit(t, hub, "feedback", "message", a, "review feedback")
	update := controlAwait(t, hub, op.ID, false)
	if update.Status != operation.StatusAwaiting {
		t.Fatalf("queued message completed before persistence: %s", update.Status)
	}
	lifecycleFile(t, filepath.Join(dir, "input-written"))
	if first.ActivitySnapshot().Children[0].InputBarrier == "persisted" {
		t.Fatal("stdin write counted as persistence")
	}
	select {
	case update := <-hub.control.updates:
		if update.ID == op.ID && activityTerminal(update.Status) {
			t.Fatal("message completed before ack")
		}
	default:
	}
	_ = os.WriteFile(filepath.Join(dir, "ack"), nil, 0600)
	out := controlResult(t, controlAwait(t, hub, op.ID, true))
	if out.State != "delivered" || out.Delivery != "persisted" || out.DrainConfirmed {
		t.Fatalf("wrong barrier: %+v", out)
	}
	stop := controlSubmit(t, hub, "stop-first", "stop", a, "")
	_ = controlAwait(t, hub, stop.ID, true)
	lifecycleFile(t, filepath.Join(dir, "stop-requested"))
	if sibling.ActivitySnapshot().Children[0].CancellationRequested {
		t.Fatal("target stop affected same-name sibling")
	}
	other := supervisionHub(t)
	cross := controlSubmit(t, other, "cross", "stop", a, "")
	if got := controlAwait(t, other, cross.ID, true); got.Status != operation.StatusFailed {
		t.Fatal("cross-Hub handle accepted")
	}
	stale := a
	stale.Generation++
	wrong := controlSubmit(t, hub, "wrong-generation", "stop", stale, "")
	if got := controlAwait(t, hub, wrong.ID, true); got.Status != operation.StatusFailed {
		t.Fatal("wrong generation accepted")
	}
}

func TestPreparationBlockedAdmissionStopAndNoLateLaunch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := supervisionHub(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		s, err := New(ctx, Config{Hub: hub, Command: []string{"unused"}, Inbox: lifecycleInbox{}, prepare: func(op operation.Operation, r operation.RemoteJobState, status operation.Status) (operation.Step, error) {
			if op.ID == "blocked" && status == operation.StatusAwaiting {
				once.Do(func() { close(entered) })
				<-release
			}
			return operation.UpdateRemoteJob(op, r, status)
		}})
		if err != nil {
			t.Fatal(err)
		}
		op := supervisionOperation(t, "blocked", toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: "reviewer", Prompt: "review"})
		if err := s.run.AddRemoteJob(op); err != nil {
			t.Fatal(err)
		}
		<-entered
		a := s.ActivitySnapshot().Children[0]
		if a.Handle == "" || !a.ProcessStartedAt.IsZero() || a.OwnerTerminalEnqueued {
			t.Fatalf("admission unavailable: %+v", a)
		}
		stop := controlSubmit(t, hub, "cancel-prelaunch", "stop", a, "")
		// Receipt publication shares the blocked run preparation FIFO, but admission
		// and actual cancellation state do not. Status remains snapshot-only.
		synctest.Wait()
		if !s.ActivitySnapshot().Children[0].CancellationRequested {
			t.Fatal("stop admission blocked behind archive")
		}
		cancel()
		wait, done := context.WithTimeout(t.Context(), time.Second)
		if err := s.Wait(wait); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Wait did not retain blocked prep: %v", err)
		}
		done()
		close(release)
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		final := s.ActivitySnapshot().Children[0]
		if !final.ProcessStartedAt.IsZero() || !final.OwnerTerminalEnqueued {
			t.Fatalf("canceled deferred launch ran: %+v", final)
		}
		_ = stop
	})
}

func TestBoundedJSONLCompatibilityAndSafeSummary(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
		err  error
	}{{"{\"a\":1}\n", "{\"a\":1}\n", nil}, {"final without newline", "final without newline", io.EOF}, {"", "", io.EOF}} {
		got, err := boundedLine(bufio.NewReader(strings.NewReader(tc.line)))
		if string(got) != tc.want || !errors.Is(err, tc.err) {
			t.Fatalf("line=%q err=%v", got, err)
		}
	}
	if _, err := boundedLine(bufio.NewReader(strings.NewReader(strings.Repeat("x", MaxSubagentFrameBytes+1)))); err == nil {
		t.Fatal("oversized unterminated frame accepted")
	}
	for command, want := range map[string]string{"find / --SECRET": "find", "TOKEN=secret find /": "", "/secret/path/find /": "", "secret-command --token": "", "find;secret /": "", "go test -token SECRET": "go"} {
		if got := safeExecutable(command); got != want {
			t.Fatalf("summary %q=%q", command, got)
		}
	}
}

func TestBlockedParentDeliveryDoesNotBlockReapingOrControl(t *testing.T) {
	hub := supervisionHub(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	gate := activityGatedInbox{make(chan struct{}), make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	s, err := New(ctx, Config{Hub: hub, Command: []string{os.Args[0], "-test.run=^TestActivityChild$", "--"}, Env: append(os.Environ(), activityHelperEnv+"=1"), Inbox: gate, Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		release()
		cancel()
		wait, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := s.Wait(wait); err != nil {
			t.Error(err)
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
	// This actor request would deadlock on the baseline's synchronous Submit.
	if err := s.do(func(state *state) { _, _ = io.WriteString(state.children["reviewer"].stdin, "release\n") }); err != nil {
		t.Fatal(err)
	}
	activityAwait(t, s, func(s ActivitySnapshot) bool {
		return s.Children[0].ProcessWaitReturned && s.Children[0].ReaderReturned
	})
	child := s.ActivitySnapshot().Children[0]
	if child.OwnerTerminalEnqueued || child.OwnerExitProcessed {
		t.Fatal("terminal overtook committed parent submission")
	}
	op := controlSubmit(t, hub, "status-blocked-delivery", "status", child, "")
	if got := controlAwait(t, hub, op.ID, true); got.Status != operation.StatusCompleted {
		t.Fatal("status blocked behind delivery")
	}
	release()
	activityAwait(t, s, func(s ActivitySnapshot) bool { return s.Children[0].OwnerTerminalEnqueued })
}

type supervisionBlockingWriter struct {
	entered, release chan struct{}
	once             sync.Once
	mu               sync.Mutex
	writes           int
}

func (w *supervisionBlockingWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	w.writes++
	w.mu.Unlock()
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(data), nil
}
func (*supervisionBlockingWriter) Close() error { return nil }
func TestBlockedInputAndQueuedCancellationRemainResponsive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := supervisionHub(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s, err := New(ctx, Config{Hub: hub, Command: []string{"unused"}, Inbox: lifecycleInbox{}})
		if err != nil {
			t.Fatal(err)
		}
		writer := &supervisionBlockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
		owner := activityRunOperation(t, "reviewer")
		ready := make(chan struct{})
		if err := s.do(func(state *state) {
			c := &child{name: "reviewer", sessionID: "session"}
			state.children[c.name] = c
			if !state.reserveAttempt(c, &owner) {
				panic("reserve")
			}
			c.reserved = false
			c.stdin = writer
			c.stop = func() {}
			c.writer = s.newInputWriter(s.workCtx, writer, c.sessionID, c.generation)
			_, _ = c.writer.enqueue([]byte(`{"initial":true}`), "")
			close(ready)
		}); err != nil {
			t.Fatal(err)
		}
		<-ready
		<-writer.entered
		a := s.ActivitySnapshot().Children[0]
		message := controlSubmit(t, hub, "queued-message", "message", a, "queued feedback")
		_ = controlAwait(t, hub, message.ID, false)
		if err := hub.control.CancelRemoteJob(message.ID, "retract"); err != nil {
			t.Fatal(err)
		}
		canceled := controlAwait(t, hub, message.ID, true)
		if canceled.Status != operation.StatusCanceled {
			t.Fatalf("message cancel: %s", canceled.Status)
		}
		activity := s.ActivitySnapshot().Children[0]
		if len(activity.Inputs) != 1 || !activity.Inputs[0].CanceledBeforeWrite {
			t.Fatalf("queued frame not retracted: %+v", activity.Inputs)
		}
		stop := controlSubmit(t, hub, "stop-blocked-input", "stop", a, "")
		if result := controlResult(t, controlAwait(t, hub, stop.ID, true)); result.DrainConfirmed {
			t.Fatal("blocked input forged drain")
		}
		if !s.ActivitySnapshot().Children[0].CancellationRequested {
			t.Fatal("stop not observed")
		}
		cancel()
		wait, done := context.WithTimeout(t.Context(), time.Second)
		if err := s.Wait(wait); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked writer was not joined: %v", err)
		}
		done()
		close(writer.release)
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		writer.mu.Lock()
		writes := writer.writes
		writer.mu.Unlock()
		if writes != 1 {
			t.Fatalf("canceled queued frame was written: %d", writes)
		}
	})
}

func TestInputPersistenceStickyAndSnapshotDetached(t *testing.T) {
	p := &activityProjection{}
	p.begin("reviewer", "session", 1, "owner", time.Now())
	p.input("session", 1, "input", "admitted")
	p.input("session", 1, "input", "queued")
	p.input("session", 1, "input", "write_started")
	p.input("session", 1, "input", "persisted")
	p.input("session", 1, "input", "written")
	snap := p.snapshotAt(time.Now())
	if snap.Children[0].Inputs[0].Barrier != "persisted" {
		t.Fatal("writer completion regressed persistence")
	}
	snap.Children[0].Inputs[0].Barrier = "forged"
	if p.snapshotAt(time.Now()).Children[0].Inputs[0].Barrier != "persisted" {
		t.Fatal("snapshot aliases projection")
	}
}

func TestNestedGraphBlockerPathAndRedaction(t *testing.T) {
	shell := activityTool("nested-shell", operation.StatusAwaiting).Operations[0]
	shell.State = []byte(`{"Phase":"process","Input":{"Command":"find / --SECRET_CREDENTIAL"}}`)
	checkpoint, _ := json.Marshal(map[string]any{"version": 1, "attempts": map[string]any{"worker": map[string]any{"child_operation": shell}}})
	remote, _ := json.Marshal(operation.RemoteJobState{Plan: operation.RemoteJobPlan{Type: "taskgraph.run", Version: 1, Data: []byte(`{}`)}, Handle: checkpoint})
	graph := operation.Operation{ID: "graph-op", Type: operation.TypeRemoteJob, Status: operation.StatusAwaiting, State: remote}
	p := &activityProjection{}
	p.begin("reviewer", "session", 1, "owner", time.Now())
	p.observeLine("session", 1, activityLine(t, 1, sessionstore.ToolCallStatus{CallID: "graph", Operations: []operation.Operation{graph}}), time.Now())
	got := p.snapshotAt(time.Now()).Children[0]
	if got.RelevantOperation == nil || len(got.RelevantOperation.Descendants) != 1 || got.RelevantOperation.Descendants[0].Executable != "find" {
		t.Fatalf("nested blocker missing: %+v", got)
	}
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "SECRET_CREDENTIAL") {
		t.Fatal("nested command argv leaked")
	}
	got.Operations[0].Descendants[0].Executable = "forged"
	if p.snapshotAt(time.Now()).Children[0].RelevantOperation.Descendants[0].Executable != "find" {
		t.Fatal("nested snapshot aliases projection")
	}
}

func TestRestartHandleFencingAndOldSessionHandleRead(t *testing.T) {
	hub := supervisionHub(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, err := New(ctx, Config{Hub: hub, Command: []string{"/nonexistent-supervision-child"}, Inbox: lifecycleInbox{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := s.Wait(context.Background()); err != nil {
			t.Error(err)
		}
	})
	op := supervisionOperation(t, "restored-owner", toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: "restored", Prompt: "resume"})
	remote, _ := operation.DecodeRemoteJobState(op)
	// Old version-1 sessions only recorded SessionID, with no capability fields.
	remote.Handle = []byte(`{"SessionID":"existing-session"}`)
	op.State, _ = json.Marshal(remote)
	if err := s.run.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	first := activityAwait(t, s, func(a ActivitySnapshot) bool { return len(a.Children) == 1 && a.Children[0].OwnerTerminalEnqueued }).Children[0]
	if first.SessionID != "existing-session" || first.Generation != 1 || first.Handle == "" {
		t.Fatalf("old handle unreadable: %+v", first)
	}
	message := supervisionOperation(t, "resumed-owner", toolsubagent.MessagePlanType, toolsubagent.MessagePlan{To: "restored", Message: "continue"})
	if err := s.message.AddRemoteJob(message); err != nil {
		t.Fatal(err)
	}
	second := activityAwait(t, s, func(a ActivitySnapshot) bool {
		return a.Children[0].Generation == 2 && a.Children[0].OwnerTerminalEnqueued
	}).Children[0]
	if first.Handle == second.Handle {
		t.Fatal("replacement reused capability")
	}
	stale := controlSubmit(t, hub, "stale-old-generation", "stop", first, "")
	if got := controlAwait(t, hub, stale.ID, true); got.Status != operation.StatusFailed {
		t.Fatal("old capability reached replacement")
	}
	// Duplicate admission is inert, and cannot resurrect the stopped generation.
	if err := hub.control.AddRemoteJob(stale); err != nil {
		t.Fatal(err)
	}
	if got := s.ActivitySnapshot().Children[0]; got.Generation != second.Generation || got.CancellationRequested {
		t.Fatal("duplicate stale control affected replacement")
	}
}

func TestInputQueueAndHubAdmissionCapacity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s, err := New(ctx, Config{Command: []string{"unused"}, Inbox: lifecycleInbox{}})
		if err != nil {
			t.Fatal(err)
		}
		gate := &supervisionBlockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
		w := s.newInputWriter(s.workCtx, gate, "none", 1)
		if _, err := w.enqueue([]byte("first"), ""); err != nil {
			t.Fatal(err)
		}
		<-gate.entered
		for range maxInputFrames {
			if _, err := w.enqueue([]byte("queued"), ""); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := w.enqueue([]byte("overflow"), ""); err == nil {
			t.Fatal("input count capacity not enforced")
		}
		w.stop()
		close(gate.release)
		cancel()
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		hctx, hcancel := context.WithCancel(t.Context())
		h := NewHub(hctx)
		for i := 0; i < MaxFleetAgents; i++ {
			if _, err := h.bind(s, "session", i+1); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := h.bind(s, "session", MaxFleetAgents+1); err == nil {
			t.Fatal("handle tombstone capacity not enforced")
		}
		if h.Snapshot().OmittedAgents != 1 {
			t.Fatal("capacity failure not surfaced")
		}
		hcancel()
		if err := h.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPreparationConcurrencyButOrderedPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		entered, release, secondPrepared := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var first, second sync.Once
		s, err := New(ctx, Config{Command: []string{"/nonexistent-supervision-child"}, Inbox: lifecycleInbox{}, prepare: func(op operation.Operation, r operation.RemoteJobState, status operation.Status) (operation.Step, error) {
			if op.ID == "first" && status == operation.StatusAwaiting {
				first.Do(func() { close(entered) })
				<-release
			}
			step, err := operation.UpdateRemoteJob(op, r, status)
			if op.ID == "second" && status == operation.StatusAwaiting {
				second.Do(func() { close(secondPrepared) })
			}
			return step, err
		}})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"first", "second"} {
			op := supervisionOperation(t, name, toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: name, Prompt: "review"})
			if err := s.run.AddRemoteJob(op); err != nil {
				t.Fatal(err)
			}
		}
		<-entered
		<-secondPrepared
		synctest.Wait()
		ready := make(chan bool, 1)
		_ = s.do(func(state *state) {
			ready <- len(state.runQueue) == 0 && state.runPublished == 0 && len(state.runPrepared) == 1
		})
		if !<-ready {
			t.Fatal("prepared second result overtook first archive")
		}
		if err := s.run.CancelRemoteJob("second", "cancel while prepared"); err != nil {
			t.Fatal(err)
		}
		close(release)
		cancel()
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		snap := s.ActivitySnapshot()
		for _, child := range snap.Children {
			if !child.ProcessStartedAt.IsZero() {
				t.Fatal("canceled prepared launch dispatched")
			}
		}
	})
}
