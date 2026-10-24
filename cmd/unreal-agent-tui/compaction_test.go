package main

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type captureCompactionInput struct{ inputs []inbox.Input }

func (capture *captureCompactionInput) Submit(_ context.Context, input inbox.Input) error {
	capture.inputs = append(capture.inputs, input)
	return nil
}

func TestCompactCommandAndNotice(t *testing.T) {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	capture := &captureCompactionInput{}
	m := newModel(t.Context(), capture, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	m.composer.SetValue(" /compact ")
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("no command submitted")
	}
	// Update batches the input command with file-picker work. Execute the batch
	// exactly as Bubble Tea would, without opening a terminal.
	var execute func(tea.Cmd)
	execute = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, child := range msg {
				execute(child)
			}
		case submitted:
			if msg.err != nil {
				t.Fatal(msg.err)
			}
		}
	}
	execute(cmd)
	if len(capture.inputs) != 1 {
		t.Fatalf("inputs=%d", len(capture.inputs))
	}
	control, err := capture.inputs[0].DecodeControlMessage()
	if err != nil || control.Mode != inbox.Compact {
		t.Fatalf("control=%+v error=%v", control, err)
	}
	notice := "Automatic compaction disabled. Original context preserved; use /compact."
	payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.CompactionDisabled, Reason: notice})
	m.apply(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "notice", Kind: inbox.InputControl, Payload: payload}})
	if len(m.lines) == 0 || !strings.Contains(m.lines[len(m.lines)-1].text, "/compact") {
		t.Fatal("notice not visible")
	}
}

func TestFailedResponseDoesNotDisplayPendingTools(t *testing.T) {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	m.apply(sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "failed", Response: llm.Response{
		Failure: &llm.Failure{Code: "failed", Message: "fixture failure"}, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "partial", Name: "Bash"}}},
	}}})
	if len(m.tools) != 0 {
		t.Fatal("failed partial call displayed as pending tool")
	}
	if len(m.lines) != 1 || m.lines[0].label != "Model failed" {
		t.Fatal("failure not displayed")
	}
}
