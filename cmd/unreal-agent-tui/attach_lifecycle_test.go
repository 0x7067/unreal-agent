package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

type lifecycleTestWriter struct {
	calls  []inbox.Input
	before func()
	err    error
}

func (w *lifecycleTestWriter) Submit(context.Context, inbox.Input) error {
	panic("lifecycle entered external submit")
}
func (w *lifecycleTestWriter) SubmitLifecycle(_ context.Context, in inbox.Input) error {
	if w.before != nil {
		w.before()
	}
	in.Payload = in.Payload.Clone()
	w.calls = append(w.calls, in)
	return w.err
}
func lifecycleTestModel(t *testing.T) (model, *lifecycleTestWriter) {
	t.Helper()
	m := toolsModel(t, defaultTheme, 100, 30, 0)
	m.attached, m.attachConnected = true, true
	m.workerGeneration = 1
	snap := attachclient.Snapshot{Hello: attachproto.Event{Incarnation: "host-one", Capabilities: []string{attachproto.CapabilityForegroundAbort, attachproto.CapabilitySessionShutdown}}, Header: attachproto.Event{Status: &attachproto.Status{Generation: 1}}}
	m.workerEvents = &attachEvents{lifecycle: newAttachLifecycle(snap)}
	m.attachNamespace = strings.Repeat("a", 64)
	w := &lifecycleTestWriter{}
	m.inputs = w
	return m, w
}
func controlFixture(t *testing.T, mode inbox.ControlMode) attachLocalControl {
	t.Helper()
	c := attachLocalControl{ID: "32345678-1234-4234-8234-123456789abc", Scope: "host-one/1"}
	var p any = inbox.ShutdownSessionRequest{Version: 1, Scope: c.Scope}
	if mode == inbox.AbortForeground {
		c.TargetTurnID = "turn-one"
		p = inbox.ForegroundAbort{Version: 1, Scope: c.Scope, TargetTurnID: c.TargetTurnID}
	}
	b, err := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: p})
	if err != nil {
		t.Fatal(err)
	}
	c.Input = inbox.Input{ID: inbox.ID(c.ID), Kind: inbox.InputControl, Payload: b}
	return c
}
func TestAttachLifecyclePersistsBeforeNetworkAndKeepsExternalOutboxSeparate(t *testing.T) {
	m, w := lifecycleTestModel(t)
	m.apply(sessionstore.Item{Data: session.Turn{ID: "turn-one"}})
	// The original model response does not remove the exact turn selector during
	// its Bash/tool phase. The host is authoritative about current execution.
	m.apply(sessionstore.Item{Data: sessionstore.ModelResponse{TurnID: "turn-one"}})
	var saved attachLocalState
	m.attachSave = func(s attachLocalState) error { saved = s; return validateAttachLocalState(s) }
	w.before = func() {
		if len(saved.Controls) != 1 || saved.Controls[0].TargetTurnID != "turn-one" {
			t.Fatal("network before original control persisted")
		}
	}
	m.composer.SetValue("/abort")
	cmd := m.startSubmission("/abort")
	if cmd == nil {
		t.Fatal("abort unavailable during tool phase")
	}
	if len(w.calls) != 0 || len(m.attachOutbox) != 0 || len(m.pendingSubmissions) != 0 || len(saved.Retained) != 0 {
		t.Fatal("control escaped separate journal")
	}
	result := cmd().(workerEvent)
	if !result.local {
		t.Fatal("local admission response can pump an extra reader")
	}
	if len(w.calls) != 1 || !bytes.Equal(w.calls[0].Payload, saved.Controls[0].Input.Payload) {
		t.Fatal("original payload changed")
	}
	m.applyAttachedEvent(result.message.(attachedEvent).event)
	if len(m.attachedLifecycle().outcomes) != 0 {
		t.Fatal("admission treated as canonical completion")
	}
	c := saved.Controls[0]
	m.apply(sessionstore.Item{Data: c.Input})
	m.apply(sessionstore.Item{Data: lifecycleResultInput(inbox.ForegroundAbortFinished, inbox.ForegroundAbortResult{Version: 1, InputID: c.Input.ID, TargetTurnID: c.TargetTurnID, Status: "completed"})})
	if m.attachedLifecycle().outcomes[c.ID] != "completed" {
		t.Fatal("canonical result absent")
	}
	if cmd := m.startSubmission("/retry-control " + c.ID); cmd != nil {
		t.Fatal("final canonical control retried")
	}
}
func TestAttachLifecycleUnsupportedOrUnsafeSendsNothing(t *testing.T) {
	for _, kind := range []string{"old-host", "no-target", "stale-generation", "disconnected", "save-failed", "no-save", "argument", "capacity"} {
		t.Run(kind, func(t *testing.T) {
			m, w := lifecycleTestModel(t)
			l := m.attachedLifecycle()
			l.turn = "turn-one"
			m.attachSave = func(attachLocalState) error { return nil }
			text := "/abort"
			switch kind {
			case "old-host":
				l.caps = map[string]bool{}
			case "no-target":
				l.turn = ""
			case "stale-generation":
				m.workerGeneration = 2
			case "disconnected":
				m.attachConnected = false
			case "save-failed":
				m.attachSave = func(attachLocalState) error { return errors.New("disk full") }
			case "no-save":
				m.attachSave = nil
			case "argument":
				text = "/abort turn-new"
			case "capacity":
				l.controls = make([]attachLocalControl, attachControlLimit)
			}
			if cmd := m.startSubmission(text); cmd != nil {
				t.Fatal("unsafe control produced network command")
			}
			if len(w.calls) != 0 || len(m.attachOutbox) != 0 {
				t.Fatal("unsafe control submitted")
			}
		})
	}
}
func TestAttachLifecycleRetryNeverRebindsOriginal(t *testing.T) {
	m, w := lifecycleTestModel(t)
	l := m.attachedLifecycle()
	c := controlFixture(t, inbox.AbortForeground)
	l.controls = []attachLocalControl{c}
	l.turn = c.TargetTurnID
	m.attachSave = func(attachLocalState) error { return nil }
	w.err = attachclient.ErrUncertain
	cmd := m.startSubmission("/retry-control " + c.ID)
	if cmd == nil {
		t.Fatal("explicit original retry unavailable")
	}
	result := cmd().(workerEvent)
	if !result.local {
		t.Fatal("local admission response can pump an extra reader")
	}
	m.applyAttachedEvent(result.message.(attachedEvent).event)
	if len(w.calls) != 1 || !sameAttachedControl(c, l.controls[0]) || !bytes.Equal(w.calls[0].Payload, c.Input.Payload) {
		t.Fatal("original identity modified")
	}
	l.turn = "turn-new"
	if m.startSubmission("/retry-control "+c.ID) != nil {
		t.Fatal("old abort rebound to a new turn")
	}
	l.turn = c.TargetTurnID
	l.scope = "host-two/1"
	if m.startSubmission("/retry-control "+c.ID) != nil {
		t.Fatal("old abort rebound to new incarnation")
	}
	l.scope = c.Scope
	m.workerGeneration = 2
	if m.startSubmission("/retry-control "+c.ID) != nil {
		t.Fatal("old abort rebound to a new generation")
	}
	if !sameAttachedControl(c, l.controls[0]) || len(w.calls) != 1 {
		t.Fatal("retry changed retained original")
	}
}
func TestAttachLifecycleCanonicalRequiresMatchingOriginalIntent(t *testing.T) {
	m, _ := lifecycleTestModel(t)
	l := m.attachedLifecycle()
	c := controlFixture(t, inbox.ShutdownSession)
	l.controls = []attachLocalControl{c}
	result := lifecycleResultInput(inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: c.Input.ID, Scope: c.Scope, Phase: "completed"})
	m.observeAttachedLifecycle(sessionstore.Item{Data: result})
	if l.outcomes[c.ID] != "" {
		t.Fatal("orphan outcome accepted")
	}
	m.observeAttachedLifecycle(sessionstore.Item{Data: c.Input})
	bad := lifecycleResultInput(inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: c.Input.ID, Scope: "other/1", Phase: "completed"})
	m.observeAttachedLifecycle(sessionstore.Item{Data: bad})
	if l.outcomes[c.ID] != "" {
		t.Fatal("wrong scope outcome accepted")
	}
	m.observeAttachedLifecycle(sessionstore.Item{Data: result})
	if l.outcomes[c.ID] != "completed" {
		t.Fatal("matching canonical result not surfaced")
	}
}
func TestAttachLifecycleMultipleControlsVersion2AndBackwardVersion1(t *testing.T) {
	dir := localStateDirectory(t)
	s := localStateFixture()
	s.Version = 2
	a, b := controlFixture(t, inbox.AbortForeground), controlFixture(t, inbox.ShutdownSession)
	b.ID = "22345678-1234-4234-8234-123456789abc"
	b.Input.ID = inbox.ID(b.ID)
	s.Controls = []attachLocalControl{a, b}
	if err := saveAttachLocalState(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := loadAttachLocalState(dir, s.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Controls) != 2 || !sameAttachedControl(got.Controls[0], a) || !sameAttachedControl(got.Controls[1], b) || got.Draft != s.Draft || got.Retained[0] != s.Retained[0] || got.ToolsOffset != s.ToolsOffset {
		t.Fatal("version2 lost original controls or presentation/outbox")
	}
	s.Version = 1
	s.Controls = nil
	if err := saveAttachLocalState(dir, s); err != nil {
		t.Fatal(err)
	}
	if got, err := loadAttachLocalState(dir, s.Namespace); err != nil || got.Version != 1 {
		t.Fatal("version1 backward read", err)
	}
}
func TestAttachLifecycleStrictLocalJournal(t *testing.T) {
	for _, kind := range []string{"version1", "duplicate", "count", "id-size", "scope", "target", "external", "receipt", "payload", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			s := localStateFixture()
			s.Version = 2
			s.Controls = []attachLocalControl{controlFixture(t, inbox.AbortForeground)}
			switch kind {
			case "version1":
				s.Version = 1
			case "duplicate":
				s.Controls = append(s.Controls, s.Controls[0])
			case "count":
				s.Controls = make([]attachLocalControl, attachControlLimit+1)
			case "id-size":
				s.Controls[0].ID = strings.Repeat("a", 129)
			case "scope":
				s.Controls[0].Scope = "other/1"
			case "target":
				s.Controls[0].TargetTurnID = "new-turn"
			case "external":
				s.Controls[0].Input.Kind = inbox.InputExternal
			case "receipt":
				s.Controls[0].Input = lifecycleResultInput(inbox.ExecutionDrained, inbox.ExecutionDrainReceipt{Version: 1, Scope: s.Controls[0].Scope})
				s.Controls[0].Input.ID = inbox.ID(s.Controls[0].ID)
			case "payload":
				s.Controls[0].Input.Payload = []byte(strings.Repeat(" ", attachproto.MaxPayload+1))
			case "unknown":
				s.Controls[0].Input.Payload = []byte(`{"Mode":"abort_foreground","Parameters":{"Version":1,"TargetTurnID":"turn-one","Scope":"host-one/1","Injected":true}}`)
			}
			if err := validateAttachLocalState(s); err == nil {
				t.Fatal("invalid control journal accepted")
			}
		})
	}
}
func TestAttachLifecycleOfflineReconcilesOnlyOriginalIDsWithoutCanonicalMutation(t *testing.T) {
	root, err := testtmp.MkdirTemp("ul-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	state := filepath.Join(root, "state")
	runtime := filepath.Join(root, "runtime")
	clients := filepath.Join(root, "clients")
	for _, dir := range []string{state, runtime, clients} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	endpoint, err := attachproto.EndpointPath(runtime, state, "private-test")
	if err != nil {
		t.Fatal(err)
	}
	ns := strings.TrimSuffix(filepath.Base(endpoint), ".sock")
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), "private-test"); err != nil {
		t.Fatal(err)
	}
	c := controlFixture(t, inbox.ShutdownSession)
	if err := store.AppendInput(t.Context(), "private-test", c.Input); err != nil {
		t.Fatal(err)
	}
	s := localStateFixture()
	s.Version = 2
	s.Namespace = ns
	s.Controls = []attachLocalControl{c}
	if err := saveAttachLocalState(clients, s); err != nil {
		t.Fatal(err)
	}
	opts := attachOptions{runtime: runtime, state: state, local: clients, ui: options{sessionID: "private-test"}}
	var out bytes.Buffer
	if handled, err := reconcileUnavailableAttach(t.Context(), opts, &out); !handled || err != nil || !strings.Contains(out.String(), "unknown;") {
		t.Fatal("EOF invented completion", handled, err, out.String())
	}
	result := lifecycleResultInput(inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: c.Input.ID, Scope: c.Scope, Phase: "completed"})
	result.ID = "canonical-result"
	if err := store.AppendInput(t.Context(), "private-test", result); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "private-test.session.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	out.Reset()
	if handled, err := reconcileUnavailableAttach(t.Context(), opts, &out); !handled || err != nil || !strings.Contains(out.String(), c.ID+" · completed") {
		t.Fatal("canonical reconciliation failed", handled, err, out.String())
	}
	after, _ := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	entries, _ := os.ReadDir(state)
	if !bytes.Equal(before, after) || !info.ModTime().Equal(afterInfo.ModTime()) || len(entries) != 1 {
		t.Fatal("read-only reconciliation mutated canonical namespace")
	}
	// Never follow a replaced session file to another path.
	outside := filepath.Join(root, "outside")
	if err := os.Rename(path, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if _, err := reconcileUnavailableAttach(t.Context(), opts, &out); err == nil {
		t.Fatal("symlink canonical history accepted")
	}
}

func TestAttachLifecycleQuitHintIsDetachInEveryPane(t *testing.T) {
	for _, pane := range []string{"composer", "files", "tools", "details"} {
		t.Run(pane, func(t *testing.T) {
			m, _ := lifecycleTestModel(t)
			m.files.open = pane == "files"
			m.toolsFocused = pane == "tools"
			m.detailsOpen = pane == "details"
			if got := m.helpView(400); !strings.Contains(got, "detach") || strings.Contains(got, "exit") {
				t.Fatalf("attached quit hint: %q", got)
			}
			m.workerEvents = nil
			if got := m.helpView(400); !strings.Contains(got, "exit") || strings.Contains(got, "detach") {
				t.Fatalf("local quit hint: %q", got)
			}
		})
	}
}

func TestLocalLifecycleAdmissionDoesNotPumpConcurrentSocketReader(t *testing.T) {
	m, _ := lifecycleTestModel(t)
	event := workerEvent{local: true, epoch: m.attachEpoch, message: attachedEvent{attachproto.Event{Type: "local_lifecycle_admission", InputID: "local-private", Code: "accepted"}}}
	_, cmd := m.Update(event)
	if cmd != nil {
		t.Fatal("local admission completion scheduled another socket Next")
	}
}
