package main

import (
	"context"
	"errors"
	"image"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

type savedIdentityWriter struct {
	saved atomic.Bool
	calls chan inbox.Input
	t     *testing.T
}

func (w *savedIdentityWriter) Submit(_ context.Context, in inbox.Input) error {
	if !w.saved.Load() {
		w.t.Error("network submission preceded stable identity save")
	}
	w.calls <- in
	return nil
}
func TestAttachLocalViewSavedBeforeDispatchAndFailureClosed(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{true: "save_failure", false: "saved_first"}[fail], func(t *testing.T) {
			writer := &savedIdentityWriter{calls: make(chan inbox.Input, 1), t: t}
			m := toolsModel(t, defaultTheme, 120, 40, 0)
			m.attached = true
			m.attachConnected = true
			m.inputs = writer
			m.composer.SetValue("draft")
			m.attachSave = func(state attachLocalState) error {
				if len(state.Retained) != 1 || state.Retained[0].Text != "draft" || state.Draft != "" {
					t.Error("identity/payload not checkpointed before send", state)
				}
				if fail {
					return errors.New("disk full")
				}
				writer.saved.Store(true)
				return nil
			}
			cmd := m.startSubmission("draft")
			if fail {
				if cmd != nil || m.composer.Value() != "draft" || len(writer.calls) != 0 || len(m.attachOutbox) != 0 {
					t.Fatal("failed save sent/lost draft")
				}
				return
			}
			m.finishSubmission(cmd().(submitted))
			if !writer.saved.Load() || len(writer.calls) != 1 {
				t.Fatal("not submitted after save")
			}
			m.submissionQueue.Cancel()
			if err := m.submissionQueue.Wait(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestAttachLocalViewRoundTripSelectionExpansionAndScroll(t *testing.T) {
	makeView := func() model {
		m := toolsModel(t, defaultTheme, 120, 40, 4)
		m.attached = true
		m.attachConnected = true
		m.attachNamespace = strings.Repeat("a", 64)
		for i := range 80 {
			m.append("Agent", strings.Repeat(string(rune('a'+i%26)), 80))
		}
		return m
	}
	old := makeView()
	old.composer.SetValue("unsent private draft")
	old.openTool(2)
	old.details.SetYOffset(2)
	old.conversation.SetYOffset(7)
	old.selection = textSelection{details: true}
	old.selection.capture(old.details)
	old.selection.anchor = image.Pt(1, 0)
	old.selection.head = image.Pt(5, 2)
	state := old.captureAttachView()
	directory := t.TempDir() + "/clients"
	if err := saveAttachLocalState(directory, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadAttachLocalState(directory, state.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	fresh := makeView()
	fresh.composer.SetValue(loaded.Draft)
	fresh.restoreAttachView(loaded)
	if fresh.composer.Value() != old.composer.Value() || fresh.conversation.YOffset() != old.conversation.YOffset() || !fresh.detailsOpen || fresh.tools[fresh.selected].key != old.tools[old.selected].key || fresh.details.YOffset() != old.details.YOffset() {
		t.Fatal("draft/scroll/selection/expanded detail did not restore")
	}
	if !reflect.DeepEqual(fresh.selection.anchor, old.selection.anchor) || !reflect.DeepEqual(fresh.selection.head, old.selection.head) {
		t.Fatal("text selection did not restore", fresh.selection, old.selection)
	}
}
func TestAttachLocalViewRetainedNeverAutomaticallyExecutes(t *testing.T) {
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	m.attached = true
	writer := &uncertainInputWriter{}
	m.inputs = writer
	state := attachLocalState{Retained: []attachLocalInput{{ID: "00000000-0000-4000-8000-000000000001", Text: "only an explicit retry may send"}}}
	m.restoreRetainedAttached(state)
	if writer.input.ID != "" || m.submissionQueue != nil || len(m.attachOutbox) != 1 || len(m.lines) != 1 {
		t.Fatal("local snapshot caused input dispatch or lost echo")
	}
}

func TestAttachLocalViewConflictingRetainedPayloadCannotReplaceAcceptedEcho(t *testing.T) {
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	m.attached = true
	m.attachConnected = true
	id := inbox.ID("00000000-0000-4000-8000-000000000002")
	accepted := inbox.Input{ID: id, Kind: inbox.InputExternal, Payload: []byte(`"durable original"`)}
	m.applyAttachedPending(accepted)
	m.restoreRetainedAttached(attachLocalState{Retained: []attachLocalInput{{ID: string(id), Text: "corrupted replacement"}}})
	if len(m.attachOutbox) != 0 || m.lines[m.submissionIndex(id)].text != "durable original" || m.submissionQueue != nil {
		t.Fatal("local snapshot replaced/resent confirmed host payload")
	}
}
