package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func queueReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for submission queue")
		var zero T
		return zero
	}
}

func joinSubmissionQueue(t *testing.T, q *submissionQueue) {
	t.Helper()
	q.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := q.Wait(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSubmissionQueueEnterOrderIndependentOfCommandScheduling(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), defaultTheme)
	started, release := make(chan struct{}), make(chan struct{})
	inputs := make(chan inbox.Input, 3)
	m.inputs = submissionWriter(func(ctx context.Context, input inbox.Input) error {
		inputs <- input
		if len(input.Payload) > 0 && string(input.Payload) == `"first"` {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	var commands [3]tea.Cmd
	m, commands[0] = submissionSend(t, m, "first")
	t.Cleanup(func() { joinSubmissionQueue(t, m.submissionQueue) })
	queueReceive(t, started) // No command has run; enqueue itself started admission.
	m, commands[1] = submissionSend(t, m, "second")
	m, commands[2] = submissionSend(t, m, " /compact ")
	if len(m.lines) != 3 || len(m.pendingSubmissions) != 3 || !m.sending || m.composer.Value() != "" {
		t.Fatalf("Enter did not immediately accept every input: lines=%+v pending=%d", m.lines, len(m.pendingSubmissions))
	}
	for i := range 3 {
		if m.lines[i].submission != submissionSending || m.lines[i].inputID == "" {
			t.Fatalf("missing optimistic echo %d: %+v", i, m.lines[i])
		}
	}
	// Typing and worker tool updates remain responsive while disk admission is gated.
	m = submissionUpdate(m, tea.PasteMsg{Content: "new draft"})
	m = submissionUpdate(m, sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{
		TurnID: "turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{
			CallID: "live", Name: tool.BashName, Arguments: `{"command":"must not execute in frontend"}`,
		}}}},
	}})
	if m.composer.Value() != "new draft" || len(m.tools) != 1 {
		t.Fatalf("UI stopped processing events: draft=%q tools=%d", m.composer.Value(), len(m.tools))
	}
	close(release)
	// Deliberately execute result waiters in reverse Enter order. A parallel
	// Submit tea.Cmd implementation would deliver compact, second, first here.
	for i := 2; i >= 0; i-- {
		result := commands[i]().(submitted)
		if result.id != m.lines[i].inputID || result.err != nil {
			t.Fatalf("wrong result for %d: %+v", i, result)
		}
		m = submissionUpdate(m, result)
		if len(m.pendingSubmissions) != i || m.sending != (i > 0) {
			t.Fatalf("completion cleared unrelated pending IDs: count=%d sending=%v", len(m.pendingSubmissions), m.sending)
		}
	}
	for i := range 3 {
		input := queueReceive(t, inputs)
		if input.ID != m.lines[i].inputID {
			t.Fatalf("backend order %d: got %s want %s", i, input.ID, m.lines[i].inputID)
		}
		if i == 2 {
			control, err := input.DecodeControlMessage()
			if err != nil || control.Mode != inbox.Compact {
				t.Fatalf("compact bypassed FIFO: %+v %v", input, err)
			}
		}
	}
	if m.composer.Value() != "new draft" {
		t.Fatal("completion stomped newer draft")
	}
}

func TestSubmissionQueueCapturesWriterAndReconcilesEachID(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), defaultTheme)
	started, release := make(chan struct{}), make(chan struct{})
	firstErr := errors.New("first failed")
	m.inputs = submissionWriter(func(ctx context.Context, _ inbox.Input) error {
		close(started)
		select {
		case <-release:
			return firstErr
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	m, first := submissionSend(t, m, "first")
	t.Cleanup(func() { joinSubmissionQueue(t, m.submissionQueue) })
	queueReceive(t, started)
	secondInput := make(chan inbox.Input, 1)
	m.inputs = submissionWriter(func(_ context.Context, input inbox.Input) error {
		secondInput <- input
		return errors.New("late rejection after persistence")
	})
	m, second := submissionSend(t, m, "second")
	m.inputs = submissionWriter(func(context.Context, inbox.Input) error {
		t.Error("queued record used replacement writer")
		return nil
	})
	m = submissionUpdate(m, sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{
		ID: m.lines[1].inputID, Kind: inbox.InputExternal,
	}})
	m.composer.SetValue("latest draft")
	close(release)
	m = submissionUpdate(m, first())
	if !m.sending || len(m.pendingSubmissions) != 1 || m.lines[0].submission != submissionFailed || m.lines[1].submission != submissionPersisted {
		t.Fatalf("failure affected another ID: %+v", m.lines)
	}
	if m.composer.Value() != "first\nlatest draft" {
		t.Fatal("failure lost latest draft")
	}
	result := second().(submitted)
	m = submissionUpdate(m, result)
	m = submissionUpdate(m, result) // duplicate result is harmless
	if queueReceive(t, secondInput).ID != m.lines[1].inputID || m.sending || len(m.pendingSubmissions) != 0 || m.lines[1].submission != submissionPersisted || m.composer.Value() != "first\nlatest draft" {
		t.Fatal("late result downgraded persistence or restored already-persisted text")
	}
}

func TestSubmissionQueueCancellationDrainsAndJoinsActiveWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	q := newSubmissionQueue(ctx)
	t.Cleanup(func() { joinSubmissionQueue(t, q) })
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	writer := submissionWriter(func(ctx context.Context, _ inbox.Input) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release // Model an admission that must settle before its owner can join.
		return ctx.Err()
	})
	first := q.enqueue(inbox.Input{ID: "first"}, "first", writer, nil)
	queueReceive(t, started)
	second := q.enqueue(inbox.Input{ID: "second"}, "second", writer, nil)
	third := q.enqueue(inbox.Input{ID: "third"}, "third", writer, nil)
	cancel() // Parent context cancellation, not only explicit Cancel, drains promises.
	queueReceive(t, canceled)
	for _, result := range []<-chan submitted{second, third} {
		if msg := queueReceive(t, result); !errors.Is(msg.err, context.Canceled) {
			t.Fatalf("queued promise not rejected: %+v", msg)
		}
	}
	waitCtx, stop := context.WithCancel(context.Background())
	stop()
	if err := q.Wait(waitCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait relinquished active admission before join: %v", err)
	}
	close(release)
	released = true
	if msg := queueReceive(t, first); !errors.Is(msg.err, context.Canceled) {
		t.Fatalf("active promise not completed: %+v", msg)
	}
	joinSubmissionQueue(t, q)
	select {
	case <-q.done:
	default:
		t.Fatal("Wait returned before sender joined")
	}
}

func TestSubmissionQueuePayloadSnapshotAndCanceledEnqueue(t *testing.T) {
	q := newSubmissionQueue(t.Context())
	t.Cleanup(func() { joinSubmissionQueue(t, q) })
	started, release := make(chan struct{}), make(chan struct{})
	gate := submissionWriter(func(ctx context.Context, _ inbox.Input) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	first := q.enqueue(inbox.Input{ID: "first"}, "first", gate, nil)
	queueReceive(t, started)
	payload, _ := json.Marshal("original")
	captured := make(chan inbox.Input, 1)
	result := q.enqueue(inbox.Input{ID: "snapshot", Payload: payload}, "original", submissionWriter(func(_ context.Context, input inbox.Input) error {
		captured <- input
		return nil
	}), nil)
	copy(payload, []byte(`"mutated!"`))
	close(release)
	queueReceive(t, first)
	queueReceive(t, result)
	if string(queueReceive(t, captured).Payload) != `"original"` {
		t.Fatal("queued payload was not immutable")
	}
	q.Cancel()
	if msg := queueReceive(t, q.enqueue(inbox.Input{ID: "late"}, "late", gate, nil)); !errors.Is(msg.err, context.Canceled) || msg.id != "late" {
		t.Fatalf("enqueue after cancellation: %+v", msg)
	}
}
