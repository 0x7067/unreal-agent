package main

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func benchModel(b *testing.B, messages int) model {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		b.Fatal(err)
	}
	m := newModel(b.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = updated.(model)
	for i := range messages {
		m.append("Agent", fmt.Sprintf("## Step %d\n\n%s\n\n```go\nfunc f() int { return %d }\n```", i, strings.Repeat("Some **markdown** with `code`. ", 8), i))
	}
	return m
}

// BenchmarkAppend measures adding one message to an existing conversation.
func BenchmarkAppend(b *testing.B) {
	for _, history := range []int{60, 600} {
		b.Run(fmt.Sprintf("history=%d", history), func(b *testing.B) {
			m := benchModel(b, history)
			for b.Loop() {
				m.append("Agent", "One more **reply**.")
				m.lines = m.lines[:history]
			}
		})
	}
}

// BenchmarkView measures one frame at 160x50 with a 60-message conversation.
func BenchmarkView(b *testing.B) {
	m := benchModel(b, 60)
	for b.Loop() {
		_ = m.View()
	}
}
