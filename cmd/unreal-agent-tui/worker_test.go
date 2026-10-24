package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestWorkerOptionsExplicitTrustAndResume(t *testing.T) {
	getenv := func(name string) string {
		if strings.Contains(name, "WORKER") || strings.Contains(name, "SESSION") {
			t.Fatalf("worker/session must not be environment defaults: %s", name)
		}
		return ""
	}
	opts, err := parseOptions([]string{"-worker-executable", "trusted-build", "-session", "resume-123"}, getenv, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs("trusted-build")
	if err != nil {
		t.Fatal(err)
	}
	if opts.workerExecutable != want || opts.sessionID != "resume-123" {
		t.Fatalf("options = %+v", opts)
	}
	opts, err = parseOptions(nil, getenv, io.Discard)
	if err != nil || opts.workerExecutable != "" || opts.sessionID != "" {
		t.Fatalf("defaults = %+v, %v", opts, err)
	}
	for _, invalid := range []string{"../other", "a/b", `a\b`, "with space", "_", "\n", "ø"} {
		if _, err := parseOptions([]string{"-session", invalid}, getenv, io.Discard); err == nil {
			t.Fatalf("accepted session %q", invalid)
		}
	}
}

type neverWorkerReader struct{}

func (neverWorkerReader) Read([]byte) (int, error) { panic("probe consumed stdin") }

func TestWorkerProbeBypassesEnvironmentAndStartup(t *testing.T) {
	t.Chdir(t.TempDir())
	var output bytes.Buffer
	handled, err := runWorkerCommand(t.Context(), []string{"worker-probe"}, neverWorkerReader{}, &output, io.Discard, func(string) string {
		t.Fatal("probe consulted environment/config/auth")
		return ""
	})
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	scanner := workerproto.Scanner(&output)
	if !scanner.Scan() {
		t.Fatal("missing probe frame")
	}
	var event workerproto.Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "hello" || event.Identity == nil || *event.Identity != workerIdentity() {
		t.Fatalf("probe = %+v", event)
	}
	if scanner.Scan() || scanner.Err() != nil {
		t.Fatal("probe emitted more than one frame or invalid framing")
	}
	for _, args := range [][]string{{"worker", "extra"}, {"worker-probe", "-load-dotenv"}} {
		if handled, err := runWorkerCommand(t.Context(), args, neverWorkerReader{}, io.Discard, io.Discard, nil); !handled || err == nil {
			t.Fatalf("arguments %q: handled=%v err=%v", args, handled, err)
		}
	}
	for _, args := range [][]string{nil, {"-version"}, {"agent-runner"}} {
		if handled, err := runWorkerCommand(t.Context(), args, neverWorkerReader{}, io.Discard, io.Discard, nil); handled || err != nil {
			t.Fatalf("non-worker %q: handled=%v err=%v", args, handled, err)
		}
	}
}

type reloadTestWriter struct {
	inputs    []inbox.Input
	calls     int
	candidate []string
	reload    func(context.Context) error
}

func (w *reloadTestWriter) Submit(_ context.Context, input inbox.Input) error {
	w.inputs = append(w.inputs, input)
	return nil
}

func (w *reloadTestWriter) Reload(ctx context.Context, candidate []string) error {
	w.calls++
	w.candidate = candidate
	if w.reload != nil {
		return w.reload(ctx)
	}
	return nil
}

func TestReloadLocalNeverSubmittedOrEchoed(t *testing.T) {
	for _, palette := range []string{"lite", "turbo-vision"} {
		t.Run(palette, func(t *testing.T) {
			m := fileTestModel(t, t.TempDir(), palette)
			writer := &reloadTestWriter{}
			m.inputs = writer
			for i := range 35 {
				m.append("Agent", fmt.Sprintf("Retained history %d\nsecond line", i))
			}
			m.conversation.SetYOffset(3)
			m.selection.capture(m.conversation)
			beforeOffset, beforeSelection := m.conversation.YOffset(), m.selection
			before := append([]message(nil), m.lines...)
			m, cmd := submissionSend(t, m, " /reload ")
			if m.composer.Value() != "" || !m.reloading || m.sending || !reflect.DeepEqual(m.lines, before) {
				t.Fatal("reload altered transcript or became model submission")
			}
			if m.conversation.YOffset() != beforeOffset || !reflect.DeepEqual(m.selection, beforeSelection) {
				t.Fatal("reload stole scroll or transcript selection")
			}
			m.composer.SetValue("new draft during reload")
			m = submissionUpdate(m, cmd())
			if writer.calls != 1 || writer.candidate != nil || len(writer.inputs) != 0 {
				t.Fatalf("reload=%d candidate=%q model inputs=%+v", writer.calls, writer.candidate, writer.inputs)
			}
			if m.reloading || m.ended || m.composer.Value() != "new draft during reload" || m.conversation.YOffset() != beforeOffset || !reflect.DeepEqual(m.lines, before) {
				t.Fatal("reload result lost retained UI state")
			}
		})
	}
}

func TestReloadRetainsUIAcrossGenerationsAndFailures(t *testing.T) {
	m := toolsModel(t, "turbo-vision", 150, 40, 12)
	writer := &reloadTestWriter{reload: func(context.Context) error { return workerhost.ErrBusy }}
	m.inputs = writer
	for i := range 35 {
		m.append("Agent", fmt.Sprintf("Earlier %d\nsecond line", i))
	}
	m = submissionUpdate(m, workerhost.Status{Generation: 1, Build: "old", Message: "Worker ready"})
	m.conversation.SetYOffset(3)
	m.detailsOpen, m.toolsFocused, m.toolsCollapsed, m.hideReasoning = true, true, false, true
	m.selected = 4
	m.refreshDetails()
	m.details.SetYOffset(1)
	m.composer.SetValue("preserved draft")
	beforeConversation, beforeDetails, beforeTools := m.conversation.YOffset(), m.details.YOffset(), append([]toolCard(nil), m.tools...)
	beforeLines := append([]message(nil), m.lines...)
	cmd := m.startReload()
	if cmd == nil || !m.reloading {
		t.Fatal("no async reload")
	}
	if duplicate := m.startReload(); duplicate != nil {
		t.Fatal("admitted concurrent reload command")
	}
	m = submissionUpdate(m, cmd())
	if m.reloading || !strings.Contains(m.reloadError, "active") {
		t.Fatalf("busy error not shown: %q", m.reloadError)
	}
	writer.reload = nil
	cmd = m.startReload()
	// Status events can precede the command result. Neither expected retirement
	// nor a stale old worker completion may disable the new generation.
	m = submissionUpdate(m, workerhost.Status{Message: "Old worker exited; starting replacement", Stopped: true})
	m = submissionUpdate(m, workerhost.Status{Generation: 2, Build: "replacement", Message: "Worker ready"})
	m = submissionUpdate(m, workerhost.Status{Generation: 1, Build: "old", Message: "late failure", Err: errors.New("old failure"), Stopped: true})
	m = submissionUpdate(m, runEnded{err: errors.New("stale old coordinator")})
	m = submissionUpdate(m, cmd())
	if m.workerGeneration != 2 || m.workerBuild != "replacement" || m.workerStopped || m.ended || m.reloadError != "" || m.workerStatus != "Worker ready" {
		t.Fatalf("replacement disabled: generation=%d build=%q status=%q stopped=%v ended=%v", m.workerGeneration, m.workerBuild, m.workerStatus, m.workerStopped, m.ended)
	}
	if m.composer.Value() != "preserved draft" || m.conversation.YOffset() != beforeConversation || m.details.YOffset() != beforeDetails || !m.detailsOpen || !m.toolsFocused || m.toolsCollapsed || !m.hideReasoning || m.selected != 4 || !reflect.DeepEqual(m.tools, beforeTools) || !reflect.DeepEqual(m.lines, beforeLines) {
		t.Fatal("worker lifecycle reset retained frontend state")
	}
	if len(writer.inputs) != 0 {
		t.Fatal("reload entered model ingress")
	}
}

func TestReloadCommandCapturesOnlyImmutableInputs(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), "lite")
	started, release := make(chan struct{}), make(chan struct{})
	writer := &reloadTestWriter{reload: func(ctx context.Context) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	m.inputs = writer
	cmd := m.startReload()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("reload did not start asynchronously")
	}
	m.inputs = submissionWriter(func(context.Context, inbox.Input) error { t.Fatal("captured mutated writer"); return nil })
	m.composer.SetValue("typed while worker reload blocked")
	m.conversation.GotoTop()
	close(release)
	m = submissionUpdate(m, <-done)
	if m.composer.Value() != "typed while worker reload blocked" || m.reloading {
		t.Fatal("async reload captured/reset mutable UI")
	}
}

func TestReloadOptionalForExistingWriters(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), "lite")
	m.inputs = submissionWriter(func(context.Context, inbox.Input) error { t.Fatal("reload submitted to writer"); return nil })
	m.composer.SetValue("draft")
	cmd := m.startSubmission("/reload")
	m = submissionUpdate(m, cmd())
	if !strings.Contains(m.reloadError, "does not support") || m.composer.Value() != "draft" || len(m.lines) != 0 {
		t.Fatal("unsupported local reload modified chat")
	}
	writer := &reloadTestWriter{}
	m.inputs = writer
	m, cmd = submissionSend(t, m, "/reload is just part of this message")
	m = submissionUpdate(m, cmd())
	if len(writer.inputs) != 1 || writer.calls != 0 || writer.inputs[0].Kind != inbox.InputExternal {
		t.Fatal("ordinary text containing /reload was intercepted")
	}
}

func TestWorkerEventsBufferReplayBeforeUIStarts(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := newWorkerEvents(ctx)
	for i := range 300 {
		payload, err := json.Marshal(fmt.Sprintf("history-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		events.item(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{ID: inbox.ID(fmt.Sprint(i)), Kind: inbox.InputExternal, Payload: payload}})
	}
	events.status(workerhost.Status{Generation: 1, Build: "buffered", Message: "Worker ready"})
	m := fileTestModel(t, t.TempDir(), "lite")
	m.workerEvents = events
	for i := range 301 {
		msg := events.next().(workerEvent)
		m = submissionUpdate(m, msg)
		if i < 300 && m.lines[i].text != fmt.Sprintf("history-%d", i) {
			t.Fatalf("reordered replay at %d", i)
		}
	}
	if len(m.lines) != 300 || m.workerBuild != "buffered" || m.ended {
		t.Fatal("lost startup replay or ready event")
	}
	cancel()
	if msg := events.next().(workerEvent); msg.message != nil {
		t.Fatal("canceled mailbox did not stop")
	}
}

func TestUIRegistryAdvertisesWithoutRuntimes(t *testing.T) {
	for _, subagents := range []bool{false, true} {
		registry := newUIRegistry(subagents, true)
		var names []string
		for _, definition := range registry.StaticDefinitions() {
			names = append(names, definition.Tool.Name)
		}
		for _, name := range []string{tool.BashName, tool.ViewImageName, tool.TaskGraphName, tool.SkillUseName} {
			if !strings.Contains(strings.Join(names, ","), name) {
				t.Fatalf("missing capability %s in %v", name, names)
			}
		}
		_, agent := registry.Resolve(tool.AgentName)
		if agent != subagents {
			t.Fatal("subagent setting not reflected in UI definitions")
		}
	}
}

func TestWorkerIngressAcknowledgmentIsNotSessionPersistence(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), "lite")
	writer := &reloadTestWriter{}
	m.inputs = writer
	m, cmd := submissionSend(t, m, "durable ingress only")
	m = submissionUpdate(m, cmd())
	view := ansi.Strip(m.View().Content)
	if m.lines[0].submission != submissionQueued || !strings.Contains(view, "not yet persisted") {
		t.Fatal("host ingress acknowledgment falsely claimed session persistence")
	}
	m = submissionUpdate(m, workerhost.Status{Generation: 1, Message: "starting replacement", Stopped: true})
	if m.lines[0].submission != submissionQueued || m.ended {
		t.Fatal("retirement downgraded ingress or ended retained UI")
	}
	m = submissionUpdate(m, sessionstore.Item{Kind: sessionstore.ItemInput, Data: writer.inputs[0]})
	if m.lines[0].submission != submissionPersisted {
		t.Fatal("typed persistence item did not reconcile input")
	}
}
