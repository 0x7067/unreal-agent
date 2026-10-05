package main

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

type textSelection struct {
	anchor, head      image.Point
	lines             []string
	content           string
	details, dragging bool
}

func (m model) followConversation() bool {
	return m.conversation.AtBottom() && (len(m.selection.lines) == 0 || m.selection.details)
}

func (m model) composerOrigin() image.Point {
	x, y := m.padding()
	left := 2
	if m.theme.Delimiter != "" {
		left += x
		y = 1
	}
	return image.Pt(left, 3+y+m.conversation.Height()+m.filePickerHeight())
}

func (m *model) selectionViewport() *viewport.Model {
	if m.selection.details {
		return &m.details
	}
	return &m.conversation
}

func (m *model) syncSelection(details bool) {
	if len(m.selection.lines) == 0 || m.selection.details != details {
		return
	}
	v := m.selectionViewport()
	if !strings.HasPrefix(v.GetContent(), m.selection.content) {
		m.selection = textSelection{}
		return
	}
	m.selection.capture(*v)
}

func (s *textSelection) capture(v viewport.Model) {
	s.content = v.GetContent()
	s.lines = strings.Split(ansi.Strip(s.content), "\n")
}

func (m *model) beginSelection(mouse tea.Mouse) {
	m.selection = textSelection{}
	m.dragComposer = false
	m.composer.ClearSelection()
	if v := m.mouseViewport(mouse); v != nil {
		m.selection = textSelection{details: m.detailsOpen, dragging: true}
		m.selection.capture(*v)
		m.extendSelection(mouse, false)
		m.selection.anchor = m.selection.head
		return
	}
	p := image.Pt(mouse.X, mouse.Y).Sub(m.composerOrigin())
	if !m.toolsFocused && p.In(image.Rect(0, 0, m.composer.Width(), m.composer.Height())) {
		m.composer.BeginSelection(p.X, p.Y)
		m.dragComposer = true
	}
}

func (m *model) extendSelection(mouse tea.Mouse, scroll bool) {
	if m.dragComposer {
		p := image.Pt(mouse.X, mouse.Y).Sub(m.composerOrigin())
		m.composer.ExtendSelection(p.X, p.Y)
		return
	}
	if !m.selection.dragging || len(m.selection.lines) == 0 {
		return
	}
	v := m.selectionViewport()
	if scroll {
		if mouse.Y < 1 {
			v.ScrollUp(1)
		} else if mouse.Y > v.Height() {
			v.ScrollDown(1)
		}
	}
	left, _ := m.padding()
	if m.theme.Delimiter != "" {
		left++
	}
	row := min(v.YOffset()+max(0, min(mouse.Y-1, v.Height()-1)), len(m.selection.lines)-1)
	column := min(v.XOffset()+max(0, min(mouse.X-left, v.Width())), ansi.StringWidth(m.selection.lines[row]))
	m.selection.head = image.Pt(column, row)
}

func (m *model) endSelection(mouse tea.Mouse) {
	if mouse.Button != tea.MouseLeft && mouse.Button != tea.MouseNone {
		return
	}
	m.extendSelection(mouse, false)
	m.selection.dragging, m.dragComposer = false, false
	m.composer.EndSelection()
	if !m.hasSelection() {
		m.selection = textSelection{}
	}
}

func (m model) selectedText() string {
	if len(m.selection.lines) > 0 {
		return m.selection.text()
	}
	return m.composer.SelectedText()
}

func (m model) hasSelection() bool {
	return len(m.selection.lines) > 0 && m.selection.anchor != m.selection.head || m.composer.HasSelection()
}

func copyText(text string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			return tea.SetClipboard(text)()
		}
		return nil
	}
}

func (s textSelection) columns(row int) (int, int) {
	start, end := s.anchor, s.head
	if end.Y < start.Y || end.Y == start.Y && end.X < start.X {
		start, end = end, start
	}
	if start == end || row < start.Y || row > end.Y || row >= len(s.lines) {
		return 0, 0
	}
	from, to := 0, ansi.StringWidth(s.lines[row])
	if row == start.Y {
		from = start.X
	}
	if row == end.Y {
		to = end.X
	}
	column := 0
	graphemes := uniseg.NewGraphemes(s.lines[row])
	for graphemes.Next() {
		next := column + graphemes.Width()
		if from > column && from < next {
			from = column
		}
		if to > column && to < next {
			to = next
		}
		column = next
	}
	return from, to
}

func (s textSelection) text() string {
	if s.anchor == s.head {
		return ""
	}
	var lines []string
	for row := min(s.anchor.Y, s.head.Y); row <= max(s.anchor.Y, s.head.Y) && row < len(s.lines); row++ {
		from, to := s.columns(row)
		lines = append(lines, strings.TrimRight(ansi.Cut(s.lines[row], from, to), " "))
	}
	return strings.Join(lines, "\n")
}

func (s textSelection) render(v viewport.Model, details bool, palette theme) string {
	view := v.View()
	if len(s.lines) == 0 || s.details != details {
		return view
	}
	lines := strings.Split(view, "\n")
	style := textStyle(palette.SelectionForeground).Background(lipgloss.Color(palette.Selection))
	for i := range lines {
		from, to := s.columns(v.YOffset() + i)
		from, to = max(0, from-v.XOffset()), min(v.Width(), to-v.XOffset())
		if from < to {
			lines[i] = lipgloss.StyleRanges(lines[i], lipgloss.NewRange(from, to, style))
		}
	}
	return strings.Join(lines, "\n")
}
