package main

import (
	"encoding/json/v2"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
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
	shell                                     *shellDetail
	terminal                                  bool
	diagnostic                                string
}

func (m *model) card(key callKey) *toolCard {
	index := slices.IndexFunc(m.tools, func(card toolCard) bool { return card.key == key })
	if index < 0 {
		if len(m.tools) > 0 && m.toolsFocused {
			m.selected++
		} else {
			m.selected = 0
		}
		if m.toolsOffset > 0 {
			m.toolsOffset++ // keep the cards the user scrolled to in view
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
	if card.name == "Bash" {
		if card.shell != nil {
			output = m.renderShellDetail(*card.shell)
		}
	} else if renderer, err := glamour.NewTermRenderer(glamour.WithStyles(m.theme.markdownStyles()), glamour.WithWordWrap(max(1, m.details.Width()-1)), glamour.WithChromaFormatter("terminal16m")); err == nil {
		if rendered, err := renderer.Render(output); err == nil {
			output = strings.Trim(terminaltext.CleanStyled(rendered), "\n")
		}
	}
	if card.paths != "" {
		output += "\n\n" + textStyle(m.theme.Muted).Render(card.paths)
	}
	// Reuse the translated error, including for old sessions that only store
	// TerminalError. Keep it visible before potentially long output and render
	// it as bounded literal text, not model-controlled Markdown or terminal ANSI.
	if card.failure != "" {
		output = textStyle(m.theme.Error).Bold(true).Render("Failure:") + "\n" +
			textStyle(m.theme.Foreground).Render(boundedDetail(card.failure, 4000)) + "\n\n" + output
	}
	// Expand each tab to four spaces before wrapping for stable layout. Lipgloss
	// also expands tabs during multiline alignment, even with NoTabConversion.
	// The shell capture and literal helper retain the original tab characters.
	output = renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).
		Width(m.details.Width()).TabWidth(4).Padding(0, 0, 1, 1), output)
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

// toolRows is the number of cards the Tools pane shows; both layouts reserve
// three of its conversation.Height()+2 rows for chrome.
func (m model) toolRows() int { return max(1, m.conversation.Height()-1) }

func (m model) toolStart() int { return max(0, min(m.toolsOffset, len(m.tools)-m.toolRows())) }

func (m *model) scrollTools(delta int) {
	m.toolsOffset = max(0, min(m.toolStart()+delta, len(m.tools)-m.toolRows()))
}

func (m *model) revealSelected() {
	start, rows := m.toolStart(), m.toolRows()
	m.toolsOffset = min(max(start, m.selected-rows+1), m.selected)
}

// toolAt reports the card under the mouse (-1 for none) and whether the mouse
// is over the Tools pane, following the layout in View.
func (m model) toolAt(mouse tea.Mouse) (int, bool) {
	x, _ := m.padding()
	left := x
	switch {
	case m.width < 20 || m.height < 8 || m.detailsOpen:
		return -1, false
	case m.width >= 90 && !m.toolsCollapsed:
		left = m.width - m.sidebarWidth()
	case !m.toolsFocused:
		return -1, false
	}
	if mouse.X < left || mouse.X >= m.width || mouse.Y < 0 || mouse.Y > m.conversation.Height()+1 {
		return -1, false
	}
	first := 3 // blank, heading, blank
	if m.theme.Delimiter != "" {
		first = 2 // border, blank
	}
	row := mouse.Y - first
	if index := m.toolStart() + row; row >= 0 && row < m.toolRows() && index < len(m.tools) {
		return index, true
	}
	return -1, true
}

// openTool focuses the Tools pane on a card and opens its details, as Tab,
// arrows and Enter do.
func (m *model) openTool(index int) tea.Cmd {
	m.selection = textSelection{}
	m.toolsFocused, m.toolsCollapsed, m.detailsOpen = true, false, true
	m.selected = index
	m.revealSelected()
	m.composer.Blur()
	m.resize()
	m.refreshDetails()
	m.details.GotoTop()
	return m.syncFilePicker()
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
	start := m.toolStart()
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

// Summaries are a diagnostic surface, not a command viewer. Only a simple
// executable basename is retained; assignments, expansions and shell syntax
// fall back to a generic label. Arguments can contain credentials.
func safeExecutableSummary(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "shell command · arguments hidden"
	}
	executable := filepath.Base(fields[0])
	if len(executable) == 0 || len(executable) > 48 {
		return "shell command · arguments hidden"
	}
	for _, r := range executable {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return "shell command · arguments hidden"
		}
	}
	return executable + " · arguments hidden"
}
