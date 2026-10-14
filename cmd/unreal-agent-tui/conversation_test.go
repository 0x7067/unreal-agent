package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// Appending reuses rendered messages, so the result must match a fresh render
// of the same history at every width and reasoning setting.
func TestConversationMatchesFreshRender(t *testing.T) {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	history := []message{
		{label: "You", text: "Plan the change"},
		{label: "Reasoning", text: "First **thought**"},
		{label: "Reasoning", text: "Second thought"},
		{label: "Agent", text: "## Plan\n\nDo `this`."},
		{label: "Model failed", text: "timeout"},
		{label: "Reasoning", text: "Retry"},
		{label: "Agent", text: "Done."},
	}
	newTestModel := func() model {
		m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		return updated.(model)
	}
	fresh := func(width int, hideReasoning bool) string {
		m := newTestModel()
		m.hideReasoning = hideReasoning
		m.conversation.SetWidth(width)
		m.lines = append([]message(nil), history...)
		m.renderConversation()
		return m.conversation.GetContent()
	}

	m := newTestModel()
	for _, entry := range history {
		m.append(entry.label, entry.text)
	}
	width := m.conversation.Width()
	if got, want := m.conversation.GetContent(), fresh(width, false); got != want {
		t.Fatalf("after appends:\n%s\nwant:\n%s", got, want)
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyF3})
	m = updated.(model)
	if got, want := m.conversation.GetContent(), fresh(width, true); got != want {
		t.Fatalf("with reasoning hidden:\n%s\nwant:\n%s", got, want)
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updated.(model)
	if m.conversation.Width() == width {
		t.Fatal("resize did not change the conversation width")
	}
	if got, want := m.conversation.GetContent(), fresh(m.conversation.Width(), true); got != want {
		t.Fatalf("after resize:\n%s\nwant:\n%s", got, want)
	}
}
