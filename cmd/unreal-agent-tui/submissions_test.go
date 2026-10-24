package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type submissionWriter func(context.Context, inbox.Input) error

func (w submissionWriter) Submit(ctx context.Context, input inbox.Input) error { return w(ctx, input) }

func submissionUpdate(m model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

func submissionSend(t *testing.T, m model, text string) (model, tea.Cmd) {
	t.Helper()
	m.composer.SetValue(text)
	updated, cmd := m.Update(fileKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("send did not return command")
	}
	return updated.(model), cmd
}

func submissionView(t *testing.T, m model, present, absent []string) {
	t.Helper()
	view := ansi.Strip(m.View().Content)
	for _, text := range present {
		if !strings.Contains(view, text) {
			t.Fatalf("View missing %q:\n%s", text, view)
		}
	}
	for _, text := range absent {
		if strings.Contains(view, text) {
			t.Fatalf("View unexpectedly contains %q:\n%s", text, view)
		}
	}
}

func TestSubmissionVisibleBeforeCommandAndIDReconciliation(t *testing.T) {
	for _, palette := range []string{"lite", "turbo-vision"} {
		for _, observerFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/observer-first=%v", palette, observerFirst), func(t *testing.T) {
				m := fileTestModel(t, t.TempDir(), palette)
				var input inbox.Input
				m.inputs = submissionWriter(func(_ context.Context, i inbox.Input) error { input = i; return nil })
				m, cmd := submissionSend(t, m, "stable-pending-message")
				if len(m.lines) != 1 || m.lines[0].inputID == "" || m.composer.Value() != "" || !m.sending {
					t.Fatalf("local state = %+v", m.lines)
				}
				id := m.lines[0].inputID
				submissionView(t, m, []string{"stable-pending-message", "Submitting", "not yet persisted"}, nil)
				result := cmd().(submitted)
				if result.id != id || input.ID != id || input.Kind != inbox.InputExternal {
					t.Fatalf("ID changed: local=%s result=%s input=%+v", id, result.id, input)
				}
				var payload string
				if err := json.Unmarshal(input.Payload, &payload); err != nil || payload != "stable-pending-message" {
					t.Fatalf("payload=%q err=%v", payload, err)
				}
				item := sessionstore.Item{Kind: sessionstore.ItemInput, Data: input}
				if observerFirst {
					m = submissionUpdate(m, item)
					submissionView(t, m, []string{payload}, []string{"not yet persisted"})
					m = submissionUpdate(m, result)
				} else {
					m = submissionUpdate(m, result)
					submissionView(t, m, []string{payload, "Queued", "not yet persisted"}, nil)
					m = submissionUpdate(m, item)
				}
				m = submissionUpdate(m, item) // duplicate observer must also be harmless
				if len(m.lines) != 1 || m.lines[0].submission != submissionPersisted || m.sending {
					t.Fatalf("not reconciled exactly once: %+v", m.lines)
				}
				submissionView(t, m, []string{payload}, []string{"Queued", "Submitting", "not yet persisted"})
				if strings.Count(ansi.Strip(m.View().Content), payload) != 1 {
					t.Fatal("duplicate visible user message")
				}
			})
		}
	}
}

func TestSubmissionExplicitSendRevealsLatestBackgroundPreservesScroll(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), defaultTheme)
	m.inputs = submissionWriter(func(context.Context, inbox.Input) error { return nil })
	for i := range 35 {
		m.append("Agent", fmt.Sprintf("Old response %d\nSecond line", i))
	}
	m.conversation.GotoTop()
	m.selection.capture(m.conversation)
	m.selection.dragging = true
	m, cmd := submissionSend(t, m, "revealed-local-message")
	if len(m.selection.lines) != 0 || !m.conversation.AtBottom() {
		t.Fatal("explicit send retained selection or old scroll")
	}
	submissionView(t, m, []string{"revealed-local-message", "Submitting"}, nil)
	m = submissionUpdate(m, cmd())
	m.conversation.GotoTop()
	m.selection.capture(m.conversation)
	before := m.conversation.YOffset()
	m.append("Agent", "unrelated background message")
	m = submissionUpdate(m, sessionstore.Item{Data: inbox.Input{ID: m.lines[35].inputID, Kind: inbox.InputExternal}})
	if m.conversation.YOffset() != before || m.conversation.AtBottom() {
		t.Fatal("background message or acknowledgment stole scroll")
	}
	submissionView(t, m, nil, []string{"revealed-local-message", "unrelated background message"})
}

func TestSubmissionBlockedErrorPreservesNewDraft(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), defaultTheme)
	started, release := make(chan inbox.Input, 1), make(chan struct{})
	m.inputs = submissionWriter(func(ctx context.Context, input inbox.Input) error {
		started <- input
		select {
		case <-release:
			return errors.New("synthetic rejection")
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	m, cmd := submissionSend(t, m, "first-send")
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	input := <-started
	if input.ID != m.lines[0].inputID {
		t.Fatal("writer did not receive stable local ID")
	}
	m.composer.SetValue("newer-draft")
	submissionView(t, m, []string{"first-send", "Submitting", "newer-draft"}, nil)
	close(release)
	m = submissionUpdate(m, <-done)
	if m.sending || m.composer.Value() != "first-send\nnewer-draft" || m.lines[0].submission != submissionFailed {
		t.Fatalf("failure state draft=%q lines=%+v", m.composer.Value(), m.lines)
	}
	submissionView(t, m, []string{"first-send", "Send failed", "not persisted", "newer-draft"}, []string{"Queued", "Submitting"})
}

func TestSubmissionRunEndedRetainsUnconfirmedText(t *testing.T) {
	for _, submitFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("submit-first=%v", submitFirst), func(t *testing.T) {
			m := fileTestModel(t, t.TempDir(), defaultTheme)
			m.inputs = submissionWriter(func(context.Context, inbox.Input) error { return nil })
			m, cmd := submissionSend(t, m, "unresolved-text")
			result := cmd()
			if submitFirst {
				m = submissionUpdate(m, result)
			}
			m = submissionUpdate(m, runEnded{err: errors.New("synthetic run failure")})
			if !submitFirst {
				m = submissionUpdate(m, result)
			}
			if m.sending || !m.ended || m.lines[0].submission != submissionUnconfirmed {
				t.Fatalf("unresolved state=%+v", m.lines)
			}
			submissionView(t, m, []string{"unresolved-text", "persistence unconfirmed"}, []string{"Queued", "Submitting"})
			m = submissionUpdate(m, sessionstore.Item{Data: inbox.Input{ID: m.lines[0].inputID, Kind: inbox.InputExternal}})
			if len(m.lines) != 2 || m.lines[0].submission != submissionPersisted {
				t.Fatal("late persistence was not reconciled")
			}
			submissionView(t, m, []string{"unresolved-text"}, []string{"persistence unconfirmed"})
		})
	}
}

func TestSubmissionPersistedBeforeErrorIsNotDowngraded(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), defaultTheme)
	m.inputs = submissionWriter(func(context.Context, inbox.Input) error { return errors.New("late rejection") })
	m, cmd := submissionSend(t, m, "persisted-text")
	m = submissionUpdate(m, sessionstore.Item{Data: inbox.Input{ID: m.lines[0].inputID, Kind: inbox.InputExternal}})
	m.composer.SetValue("newer-draft")
	m = submissionUpdate(m, cmd())
	if len(m.lines) != 1 || m.composer.Value() != "newer-draft" || m.sending {
		t.Fatal("late submit error overrode authoritative persistence")
	}
	submissionView(t, m, []string{"persisted-text"}, []string{"Send failed", "not persisted"})
}

func TestSubmissionIdenticalTextUsesDistinctIDs(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), defaultTheme)
	m.inputs = submissionWriter(func(context.Context, inbox.Input) error { return nil })
	m, cmd := submissionSend(t, m, "same-text")
	m = submissionUpdate(m, cmd())
	m, cmd = submissionSend(t, m, "same-text")
	m = submissionUpdate(m, cmd())
	if len(m.lines) != 2 || m.lines[0].inputID == m.lines[1].inputID {
		t.Fatal("IDs not unique")
	}
	m = submissionUpdate(m, sessionstore.Item{Data: inbox.Input{ID: m.lines[1].inputID, Kind: inbox.InputExternal}})
	if m.lines[0].submission != submissionQueued || m.lines[1].submission != submissionPersisted {
		t.Fatal("reconciled by text instead of ID")
	}
	m = submissionUpdate(m, runEnded{})
	if m.lines[0].submission != submissionUnconfirmed || m.lines[1].submission != submissionPersisted {
		t.Fatal("run end changed persisted input")
	}
}

// Integration boundary: actual running local TaskGraph + inbox + disk store
// observer. Persistence is driven explicitly, not by a provider/coordinator.
func TestSubmissionDuringBlockedTaskGraph(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	workspace := t.TempDir()
	inputs, err := inbox.New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, manager, err := newTaskOperations(ctx, workspace, t.TempDir(), "/bin/sh", inputs, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		for range manager.Updates() {
		}
	}()
	translator, ok := registry.Resolve(tool.TaskGraphName)
	if !ok {
		t.Fatal("TaskGraph missing")
	}
	submitter := &graphSubmitter{manager: manager}
	status := translator.Translate(submitter, llm.ToolCall{CallID: "blocked", Arguments: `{"action":"start","name":"pending-ui","tasks":[{"id":"blocked","command":"printf started > started; while [ ! -f release ]; do sleep 0.01; done; printf proof > result.txt","reads":["release"],"writes":["started","result.txt"],"acceptance":"proof after release"}]}`})
	if status.Error != "" || submitter.err != nil {
		t.Fatalf("start=%+v err=%v", status, submitter.err)
	}
	m := fileTestModel(t, workspace, defaultTheme)
	m.inputs, m.registry, m.responding = inputs, registry, true
	m.workStarted = time.Now()
	for {
		select {
		case op := <-manager.Updates():
			if op.ID == status.WaitingFor[0] && op.Status == operation.StatusAwaiting {
				card := m.card(callKey{call: "blocked"})
				card.name = tool.TaskGraphName
				m.readToolResult(card, sessionstore.ToolCallStatus{CallID: "blocked", Status: status, Operations: []operation.Operation{op}})
				goto awaitingGraph
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
awaitingGraph:
	for {
		if _, err := os.Stat(filepath.Join(workspace, "started")); err == nil {
			break
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for i := range 35 {
		m.append("Agent", fmt.Sprintf("Old history %d\nsecond line", i))
	}
	m.conversation.GotoTop()
	m, cmd := submissionSend(t, m, "steer-running-taskgraph")
	submissionView(t, m, []string{"steer-running-taskgraph", "Submitting"}, nil)
	m = submissionUpdate(m, cmd())
	submissionView(t, m, []string{"steer-running-taskgraph", "Queued", "not yet persisted"}, nil)
	var userInput inbox.Input
	for userInput.ID != m.lines[35].inputID {
		select {
		case userInput = <-inputs.Output():
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	store, err := localfile.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := session.ID("pending-ui")
	if _, err := store.Create(ctx, id); err != nil {
		t.Fatal(err)
	}
	store.AddObserver(func(_ session.ID, item sessionstore.Item) { m = submissionUpdate(m, item) })
	if err := store.AppendInput(ctx, id, userInput); err != nil {
		t.Fatal(err)
	}
	if len(m.lines) != 36 || m.lines[35].submission != submissionPersisted {
		t.Fatal("disk observer duplicated or lost submission")
	}
	submissionView(t, m, []string{"steer-running-taskgraph"}, []string{"not yet persisted", "Queued"})
	if _, err := os.Stat(filepath.Join(workspace, "result.txt")); !os.IsNotExist(err) {
		t.Fatal("graph was not blocked during acknowledgment")
	}
	if err := os.WriteFile(filepath.Join(workspace, "release"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Empty/terminal-control-only observer inputs must never steal an earlier ID.
func TestSubmissionBlankObserverCorrelation(t *testing.T) {
	for _, text := range []string{"", " \t\n", "\x1b[2J\x00\a"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/existing=%v", text, existing), func(t *testing.T) {
				m := fileTestModel(t, t.TempDir(), "lite")
				if existing {
					m.append("You", "previous")
					m.lines[0].inputID = "previous-id"
				}
				before := len(m.lines)
				payload, err := json.Marshal(text)
				if err != nil {
					t.Fatal(err)
				}
				input := inbox.Input{ID: "empty-id", Kind: inbox.InputExternal, Payload: payload}
				m.applyInput(input)
				if existing && m.lines[0].inputID != "previous-id" {
					t.Fatal("previous input ID overwritten")
				}
				if len(m.lines) > before {
					if m.lines[before].inputID != input.ID {
						t.Fatal("new entry lost correlation")
					}
					m.applyInput(input)
					if len(m.lines) != before+1 {
						t.Fatal("observer duplicated blank input")
					}
				}
			})
		}
	}
}

func TestSubmissionCompactControlReconciliation(t *testing.T) {
	for _, observerFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("observer-first=%v", observerFirst), func(t *testing.T) {
			m := fileTestModel(t, t.TempDir(), "lite")
			var input inbox.Input
			m.inputs = submissionWriter(func(_ context.Context, value inbox.Input) error { input = value; return nil })
			m, cmd := submissionSend(t, m, " /compact ")
			submissionView(t, m, []string{"/compact", "Submitting"}, nil)
			id := m.lines[0].inputID
			result := cmd().(submitted)
			control, err := input.DecodeControlMessage()
			if err != nil || control.Mode != inbox.Compact || input.ID != id || result.id != id {
				t.Fatalf("input=%+v control=%+v err=%v", input, control, err)
			}
			item := sessionstore.Item{Kind: sessionstore.ItemInput, Data: input}
			if observerFirst {
				m = submissionUpdate(m, item)
				m = submissionUpdate(m, result)
			} else {
				m = submissionUpdate(m, result)
				m = submissionUpdate(m, item)
			}
			if m.lines[0].submission != submissionPersisted || m.sending {
				t.Fatalf("not reconciled: %+v", m.lines)
			}
			submissionView(t, m, []string{"Manual compaction requested."}, []string{"Submitting", "Queued", "not yet persisted"})
		})
	}
}
