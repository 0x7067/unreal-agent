package main

import (
	"encoding/json/v2"
	"slices"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

const awaiting = "Awaiting result"

type toolCard struct {
	key                                       callKey
	name, arguments, summary, status, failure string
	output, paths                             string
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
		Name    string `json:"name"`
		Prompt  string `json:"prompt"`
		To      string `json:"to"`
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(arguments), &input) == nil {
		switch name {
		case "Bash":
			arguments = input.Command
		case "ViewImage":
			arguments = input.Path
		case "Agent":
			arguments = input.Name + ": " + input.Prompt
		case "SendMessage":
			arguments = "to " + input.To + ": " + input.Message
		}
	}
	return ansi.Truncate(singleLine(arguments), 160, "…")
}

func singleLine(text string) string {
	return strings.Join(strings.Fields(terminaltext.Clean(text)), " ")
}

func boundedDetail(text string, limit int) string {
	text, truncated := operation.BoundOutput(terminaltext.Clean(text), limit)
	if truncated {
		text += "\n[display shortened; inspect run files for full text]"
	}
	return text
}

func (m model) labelValue(width int, label, value string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		textStyle(m.theme.Foreground).Width(11).Render(label),
		textStyle(m.theme.Muted).Width(max(1, width-11)).Render(value))
}

func (m *model) refreshDetails() {
	if !m.detailsOpen || m.selected >= len(m.tools) {
		return
	}
	card := m.tools[m.selected]
	arguments := card.arguments
	if card.name == "Bash" {
		var input struct {
			Command *string `json:"command"`
		}
		if json.Unmarshal([]byte(arguments), &input) == nil && input.Command != nil {
			arguments = *input.Command
		}
	}
	status := card.status
	color := m.theme.Warning
	switch status {
	case "Completed":
		color = m.theme.Success
	case "Canceled":
		color = m.theme.Muted
	}
	if m.ended && status == awaiting {
		status = "Run ended; result unavailable"
		color = m.theme.Muted
	}
	if card.failure != "" || card.status == "Failed" {
		color = m.theme.Error
	}
	title := textStyle(m.theme.Accent).Bold(true).Render(singleLine(card.name)) +
		textStyle(m.theme.Muted).Render(" · ") + textStyle(color).Render(status)
	output := terminaltext.Clean(card.output)
	if renderer, err := glamour.NewTermRenderer(glamour.WithStyles(m.theme.markdownStyles()), glamour.WithWordWrap(max(1, m.details.Width()-1)), glamour.WithChromaFormatter("terminal16m")); err == nil {
		if rendered, err := renderer.Render(output); err == nil {
			output = strings.Trim(terminaltext.CleanStyled(rendered), "\n")
		}
	}
	if card.paths != "" {
		output += "\n\n" + textStyle(m.theme.Muted).Render(card.paths)
	}
	output = renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).
		Width(m.details.Width()).Padding(0, 0, 1, 1), output)
	content := title + "\n\n" +
		m.labelValue(m.details.Width(), "Call", singleLine(card.key.call)) + "\n" +
		m.labelValue(m.details.Width(), "Arguments", boundedDetail(arguments, 16000)) +
		"\n\n" + textStyle(m.theme.Foreground).Render("Result") + "\n" + output
	m.details.SetContent(ansi.Hardwrap(content, max(1, m.details.Width()), true))
	m.syncSelection(true)
}

func (m model) toolIndicator(card toolCard) string {
	icon, color := "●", m.theme.Warning
	switch card.status {
	case "Completed":
		color = m.theme.Success
	case "Canceled":
		icon, color = "○", m.theme.Muted
	}
	if m.ended && card.status == awaiting {
		icon, color = "○", m.theme.Muted
	}
	if card.failure != "" || card.status == "Failed" {
		color = m.theme.Error
	}
	return textStyle(color).Render(icon)
}

func (m model) toolIndicators(width int) string {
	var indicators []string
	for _, card := range m.tools {
		if card.status != awaiting || m.ended {
			continue
		}
		indicators = append(indicators, m.toolIndicator(card))
		if len(indicators)*2-1 > width {
			break
		}
	}
	return ansi.Truncate(strings.Join(indicators, " "), width, "…")
}

func (m model) toolPane(width, height int) string {
	framed := m.theme.Delimiter != ""
	if framed {
		width -= 2
		height -= 2
	}
	innerWidth := max(1, width-4)
	lines := []string{"", m.heading("Tools", "Shift+Tab ›", m.theme.Surface, m.theme.SurfaceAccent, m.theme.SurfaceHint, width), ""}
	if framed {
		lines = []string{""}
	}
	available := max(0, height-len(lines))
	start := max(0, m.selected-max(1, available)+1)
	end := min(len(m.tools), start+available)
	if len(m.tools) == 0 {
		lines = append(lines, textStyle(m.theme.SurfaceMuted).Padding(0, 2).Render(ansi.Truncate("No tool calls yet", innerWidth, "…")))
	}
	for i := start; i < end; i++ {
		card := m.tools[i]
		focused := i == m.selected && m.toolsFocused
		foreground, muted := m.theme.SurfaceForeground, m.theme.SurfaceMuted
		style := textStyle(foreground).Background(lipgloss.Color(m.theme.Surface)).Width(width).Padding(0, 2)
		if focused {
			foreground, muted = m.theme.SelectionForeground, m.theme.Muted
			style = style.Foreground(lipgloss.Color(foreground)).Background(lipgloss.Color(m.theme.Selection))
		}
		text := textStyle(foreground).Bold(focused).Render(singleLine(card.name)) + " " +
			textStyle(muted).Render(card.summary)
		row := m.toolIndicator(card) + " " + ansi.Truncate(text, max(0, innerWidth-2), "…")
		row = renderSurface(style, ansi.Truncate(row, innerWidth, "…"))
		lines = append(lines, row)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	content := renderSurface(textStyle(m.theme.SurfaceForeground).Background(lipgloss.Color(m.theme.Surface)).Width(width), strings.Join(lines[:min(len(lines), height)], "\n"))
	if framed {
		return m.framedPane("Tools", "Shift+Tab ›", content, m.theme.Surface, width+2)
	}
	return content
}
