package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type mouseFrame struct{}

// Bubble Tea renders after every Update, so redundant terminal events and wheel
// bursts are coalesced in the filter.
type mouseScreen struct {
	model
	framePending bool
	lastSize     tea.WindowSizeMsg
}

func (s *mouseScreen) filter(_ tea.Model, msg tea.Msg) tea.Msg {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		// Some terminals report their size again when switching tabs. Bubble
		// Tea erases the screen before Update for every size message, so this
		// must happen in the filter, not just in model.Update. Keep the first
		// valid size even if it matches newModel's default dimensions.
		if size.Width <= 0 || size.Height <= 0 || size == s.lastSize {
			return nil
		}
		s.lastSize = size
		return msg
	}
	wheel, ok := msg.(tea.MouseWheelMsg)
	if !ok {
		return msg
	}
	target := s.mouseViewport(wheel.Mouse())
	if _, overTools := s.toolAt(wheel.Mouse()); !overTools && (target == nil || !wheel.Mod.Contains(tea.ModShift) &&
		(wheel.Button == tea.MouseWheelDown && target.AtBottom() || wheel.Button == tea.MouseWheelUp && target.AtTop())) {
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
	if _, overTools := m.toolAt(msg.Mouse()); overTools {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollTools(-1)
		case tea.MouseWheelDown:
			m.scrollTools(1)
		}
		return
	}
	if target := m.mouseViewport(msg.Mouse()); target != nil {
		*target, _ = target.Update(msg)
		if m.selection.dragging && m.selection.details == m.detailsOpen {
			m.extendSelection(msg.Mouse(), false)
		}
	}
}
