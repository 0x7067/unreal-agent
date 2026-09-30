package main

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
)

const awaiting = "Awaiting result"

type toolCard struct {
	key                                       callKey
	name, arguments, summary, status, failure string
	started, finished                         time.Time
}

func (m *model) card(key callKey) *toolCard {
	index := slices.IndexFunc(m.tools, func(card toolCard) bool { return card.key == key })
	if index < 0 {
		if len(m.tools) > 0 && m.toolsFocused {
			m.selected++
		} else {
			m.selected = 0
		}
		m.tools = slices.Insert(m.tools, 0, toolCard{key: key, name: "Tool", status: awaiting})
		index = 0
	}
	return &m.tools[index]
}

func toolSummary(name, arguments string) string {
	var input struct {
		Command string `json:"command"`
		Path    string `json:"path"`
	}
	if json.Unmarshal([]byte(arguments), &input) == nil {
		switch name {
		case "Bash":
			arguments = input.Command
		case "ViewImage":
			arguments = input.Path
		}
	}
	return ansi.Truncate(singleLine(arguments), 160, "…")
}

func singleLine(text string) string {
	return strings.Join(strings.Fields(terminaltext.Clean(text)), " ")
}

func (m model) toolCounts() (active, failed int) {
	for _, card := range m.tools {
		if card.status == awaiting {
			active++
		}
		if card.failure != "" {
			failed++
		}
	}
	return
}

func (m model) toolPane(width, height int) string {
	fit := func(text string) string { return ansi.Truncate(singleLine(text), width, "…") }
	rows := max(1, (height-1)/3)
	start := max(0, m.selected-rows+1)
	end := min(len(m.tools), start+rows)
	header := "Tools · Tab to focus"
	if m.toolsFocused {
		header = "Tools · ↑/↓ select · Esc back"
	}
	if len(m.tools) > 0 {
		header = fmt.Sprintf("Tools %d–%d/%d", start+1, end, len(m.tools))
	}
	lines := []string{lipgloss.NewStyle().Bold(true).Render(fit(header))}
	if len(m.tools) == 0 {
		lines = append(lines, fit("No tool calls yet"))
	}
	for i := start; i < end; i++ {
		card := m.tools[i]
		name := card.name
		if i == m.selected && m.toolsFocused {
			name = "> " + name
		}
		status := card.status
		if m.ended && status == awaiting {
			status = "Run ended; result unavailable"
		}
		if !card.started.IsZero() {
			until := card.finished
			if until.IsZero() {
				until = m.now
			}
			status += fmt.Sprintf(" · %s", max(time.Duration(0), until.Sub(card.started)).Truncate(time.Second))
		}
		if card.failure != "" {
			status += " · " + card.failure
		}
		style := lipgloss.NewStyle()
		if card.failure != "" {
			style = style.Foreground(lipgloss.Color("1"))
		}
		if i == m.selected && m.toolsFocused {
			style = style.Reverse(true)
		}
		lines = append(lines, style.Render(fit(name)), fit(card.summary), style.Render(fit(status)))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines[:min(len(lines), height)], "\n"))
}
