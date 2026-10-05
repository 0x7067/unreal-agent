package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type mouseFrame struct{}

// Bubble Tea renders after every Update, so wheel bursts are coalesced in the filter.
type mouseScreen struct {
	model
	framePending bool
}

func (s *mouseScreen) filter(_ tea.Model, msg tea.Msg) tea.Msg {
	wheel, ok := msg.(tea.MouseWheelMsg)
	if !ok {
		return msg
	}
	target := s.mouseViewport(wheel.Mouse())
	if target == nil || !wheel.Mod.Contains(tea.ModShift) &&
		(wheel.Button == tea.MouseWheelDown && target.AtBottom() || wheel.Button == tea.MouseWheelUp && target.AtTop()) {
		return nil
	}
	if s.framePending {
		s.scrollWheel(wheel)
		return nil
	}
	return msg
}

func (s *mouseScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(mouseFrame); ok {
		s.framePending = false
		return s, nil
	}
	next, cmd := s.model.Update(msg)
	s.model = next.(model)
	if _, ok := msg.(tea.MouseWheelMsg); ok {
		s.framePending = true
		cmd = tea.Tick(time.Second/60, func(time.Time) tea.Msg { return mouseFrame{} })
	}
	return s, cmd
}

func (m *model) scrollWheel(msg tea.MouseWheelMsg) {
	m.lastClickAt = time.Time{}
	if target := m.mouseViewport(msg.Mouse()); target != nil {
		*target, _ = target.Update(msg)
		if m.selection.dragging && m.selection.details == m.detailsOpen {
			m.extendSelection(msg.Mouse(), false)
		}
	}
}
