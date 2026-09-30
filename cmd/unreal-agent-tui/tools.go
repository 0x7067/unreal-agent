package main

import (
	"encoding/json/v2"
	"math"
	"slices"
	"strings"
	"time"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

const (
	awaiting         = "Awaiting result"
	toolStatusLinger = 5 * time.Second
)

type toolCard struct {
	key                                       callKey
	name, arguments, summary, status, failure string
	output, paths                             string
	created, completed                        time.Time
}

func (m *model) card(key callKey) *toolCard {
	index := slices.IndexFunc(m.tools, func(card toolCard) bool { return card.key == key })
	if index < 0 {
		if len(m.tools) > 0 && m.toolsFocused {
			m.selected++
		} else {
			m.selected = 0
		}
		m.tools = slices.Insert(m.tools, 0, toolCard{key: key, name: "Tool", status: awaiting, created: time.Now()})
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
	if renderer, err := glamour.NewTermRenderer(glamour.WithStyles(m.theme.markdownStyles()), glamour.WithWordWrap(max(1, m.details.Width()-1))); err == nil {
		if rendered, err := renderer.Render(output); err == nil {
			output = strings.Trim(terminaltext.CleanStyled(rendered), "\n")
		}
	}
	if card.paths != "" {
		output += "\n\n" + textStyle(m.theme.Muted).Render(card.paths)
	}
	output = renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).
		Width(m.details.Width()).Padding(0, 0, 1, 1), output)
	m.details.SetContent(title + "\n\n" +
		m.labelValue(m.details.Width(), "Call", singleLine(card.key.call)) + "\n" +
		m.labelValue(m.details.Width(), "Arguments", boundedDetail(arguments, 16000)) +
		"\n\n" + textStyle(m.theme.Foreground).Render("Result") + "\n" + output)
}

func (m model) revealTool(text string, elapsed time.Duration) string {
	if !m.animations || elapsed >= revealDuration || m.ended {
		return text
	}
	elapsed = max(0, elapsed)
	width := ansi.StringWidth(text)
	settled := int(elapsed * time.Duration(width) / revealDuration)
	prefix := ansi.Truncate(text, settled, "")
	var noise strings.Builder
	frame := int(elapsed / (time.Second / 30))
	for column := ansi.StringWidth(prefix); column < min(width, settled+4); column++ {
		noise.WriteByte(revealGlyphs[(frame+column*3)%len(revealGlyphs)])
	}
	return prefix + textStyle(m.theme.Accent).Render(noise.String())
}

func (m model) shimmerTool(row string, elapsed time.Duration) string {
	if !m.animations || m.ended || elapsed <= 0 || elapsed >= revealDuration {
		return row
	}
	width := ansi.StringWidth(row)
	canvas := lipgloss.NewCanvas(width, 1).Compose(lipgloss.NewLayer(row))
	colors := lipgloss.Blend1D(26, canvas.CellAt(0, 0).Style.Bg, lipgloss.Color(m.theme.Success))
	center := -0.3 + 1.6*float64(elapsed)/float64(revealDuration)
	for column := range width {
		distance := math.Abs(float64(column)/float64(max(1, width-1))-center) / 0.3
		shade := int(10 * (1 + math.Cos(math.Pi*min(1, distance))) / 2)
		if shade > 0 {
			canvas.CellAt(column, 0).Style.Bg = colors[shade]
		}
	}
	return canvas.Render()
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
	indicator := textStyle(color)
	if card.status == awaiting && card.failure == "" && !m.ended {
		icon = [...]string{"●", "◉", "○", "◉"}[m.toolPulseFrame/6]
		indicator = indicator.Foreground(m.toolColors[m.toolPulseFrame])
	}
	return indicator.Render(icon)
}

func (m model) toolIndicators(width int) string {
	now := time.Now()
	var indicators []string
	for _, card := range m.tools {
		if (card.status != awaiting || m.ended) && now.Sub(card.completed) >= toolStatusLinger {
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
	innerWidth := max(1, width-4)
	start := max(0, m.selected-max(1, height-3)+1)
	end := min(len(m.tools), start+max(1, height-3))
	lines := []string{"", m.heading("Tools", "Shift+Tab ›", m.theme.Surface, width), ""}
	if len(m.tools) == 0 {
		lines = append(lines, textStyle(m.theme.Muted).Padding(0, 2).Render(ansi.Truncate("No tool calls yet", innerWidth, "…")))
	}
	now := time.Now()
	for i := start; i < end; i++ {
		card := m.tools[i]
		focused := i == m.selected && m.toolsFocused
		style := textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Surface)).Width(width).Padding(0, 2)
		if focused {
			style = style.Background(lipgloss.Color(m.theme.Selection))
		}
		text := textStyle(m.theme.Foreground).Bold(focused).Render(singleLine(card.name)) + " " +
			textStyle(m.theme.Muted).Render(card.summary)
		row := m.toolIndicator(card) + " " + m.revealTool(ansi.Truncate(text, max(0, innerWidth-2), "…"), now.Sub(card.created))
		row = renderSurface(style, ansi.Truncate(row, innerWidth, "…"))
		if card.status == "Completed" && card.failure == "" && card.completed.Sub(card.created) > time.Second {
			row = m.shimmerTool(row, now.Sub(card.completed))
		}
		lines = append(lines, row)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Surface)).Width(width), strings.Join(lines[:min(len(lines), height)], "\n"))
}
