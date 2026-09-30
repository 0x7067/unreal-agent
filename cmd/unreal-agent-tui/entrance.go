package main

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const entranceDuration = 600 * time.Millisecond

func (m model) entranceView(view tea.View, now time.Time) tea.View {
	elapsed := now.Sub(m.entranceStarted)
	if !m.animations || m.entranceStarted.IsZero() || elapsed >= entranceDuration || m.ended ||
		len(m.lines) > 0 || len(m.tools) > 0 || m.responding || m.sending || m.detailsOpen || m.toolsFocused {
		return view
	}
	progress := func(delay, duration time.Duration) float64 {
		return min(1, max(0, float64(elapsed-delay)/float64(duration)))
	}
	canvas := lipgloss.NewCanvas(m.width, m.height).Compose(lipgloss.NewLayer(view.Content))
	background := lipgloss.Color(m.theme.Background)
	surface := lipgloss.Blend1D(12, background, lipgloss.Color(m.theme.Surface))
	status := lipgloss.Blend1D(12, background, lipgloss.Color(m.theme.Status))
	statusY := m.conversation.Height() + 2
	sidebarX := m.width
	if m.width >= 90 && !m.toolsCollapsed {
		sidebarX -= m.sidebarWidth()
	}
	inset, _ := m.padding()
	textOffset := m.introTextOffset()
	mascotHeight := lipgloss.Height(mascot)
	frame := int(max(0, elapsed) / (time.Second / 30))
	for row := range m.height {
		for column := range m.width {
			cell := canvas.CellAt(column, row)
			if cell.Style.Bg == nil {
				cell.Style.Bg = lipgloss.Color(m.theme.Surface)
			}
			position := float64(column) / float64(max(1, m.width-1))
			textProgress := progress(80*time.Millisecond, 400*time.Millisecond)
			textColumn, textWidth := column-inset-2-textOffset, max(1, sidebarX-inset-3-textOffset)
			mascotRow := row - 1 + m.conversation.YOffset() - m.mascotTop
			isMascot := row > 0 && row < statusY && column < sidebarX && mascotRow >= 0 && mascotRow < mascotHeight
			if textOffset > 0 {
				isMascot = isMascot && column < inset+2+textOffset
			}
			switch {
			case row >= statusY:
				colors := surface
				fill := progress(260*time.Millisecond, 340*time.Millisecond)
				if row == statusY {
					colors = status
					fill = progress(180*time.Millisecond, 340*time.Millisecond)
				}
				shade := int(min(1, max(0, (fill*1.12-position)/0.12)) * float64(len(colors)-1))
				cell.Style.Bg = colors[shade]
				textProgress = fill
				textColumn, textWidth = column, m.width
			case column >= sidebarX:
				position = float64(row) / float64(max(1, statusY-1))
				fill := progress(40*time.Millisecond, 380*time.Millisecond)
				shade := int(min(1, max(0, (fill*1.12-position)/0.12)) * float64(len(surface)-1))
				cell.Style.Bg = surface[shade]
				textProgress = progress(130*time.Millisecond, 350*time.Millisecond)
				textColumn, textWidth = column-sidebarX-2, max(1, m.sidebarWidth()-4)
			case isMascot:
				textProgress = progress(0, 240*time.Millisecond)
				textColumn, textWidth = mascotRow, mascotHeight
			}
			front := int(textProgress * float64(textWidth))
			if textProgress < 1 && textColumn >= front {
				if !isMascot && textProgress > 0 && textColumn < front+4 && cell.Width == 1 && strings.TrimSpace(cell.Content) != "" {
					cell.Content = string(revealGlyphs[(frame+column*3)%len(revealGlyphs)])
				} else {
					cell.Style.Fg = cell.Style.Bg
				}
			}
		}
	}
	view.Content = canvas.Render()
	view.BackgroundColor = background
	view.Cursor = nil
	return view
}
