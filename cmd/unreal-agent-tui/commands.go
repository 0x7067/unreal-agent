package main

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type slashCommand struct {
	name, args, about string
}

// slashCommands lists only standalone commands; other slash-prefixed text is
// ordinary prompt input, as it is upstream.
var slashCommands = []slashCommand{
	{"/review", "[cancel | review run flags]", "Review uncommitted changes in the background"},
}

type commandPicker struct {
	open             bool
	query, dismissed string
	matches          []slashCommand
	selected         int
}

func (m *model) syncCommandPicker() {
	c := &m.commands
	query := m.composer.Value()
	if query != c.dismissed {
		c.dismissed = ""
	}
	if !strings.HasPrefix(query, "/") || strings.ContainsFunc(query, unicode.IsSpace) || c.dismissed != "" || m.toolsFocused || m.detailsOpen || m.ended {
		c.open = false
		return
	}
	// The selection resets only when the query changes.
	if query != c.query {
		c.selected = 0
	}
	c.open, c.query, c.matches = true, query, nil
	for _, command := range slashCommands {
		if strings.HasPrefix(command.name, query) {
			c.matches = append(c.matches, command)
		}
	}
	c.selected = min(c.selected, max(0, len(c.matches)-1))
}

// commandPickerKey moves the selection and completes the command. Enter
// leaves the completed command in the composer and returns false, so the
// usual send path runs it.
func (m *model) commandPickerKey(msg tea.KeyPressMsg) bool {
	c := &m.commands
	// A picker too small to show is closed, so Enter never runs a hidden choice.
	if !c.open || m.commandPickerHeight() == 0 || len(c.matches) == 0 && msg.String() != "esc" {
		return false
	}
	switch msg.String() {
	case "up", "down", "ctrl+p", "ctrl+n":
		step := 1
		if msg.String() == "up" || msg.String() == "ctrl+p" {
			step = -1
		}
		c.selected = (c.selected + step + len(c.matches)) % len(c.matches)
	case "tab":
		m.composer.SetValue(c.matches[c.selected].name + " ")
		m.composer.CursorEnd()
		c.open = false
	case "enter":
		m.composer.SetValue(c.matches[c.selected].name)
		c.open = false
		return false
	case "esc":
		c.dismissed, c.open = c.query, false
	default:
		return false
	}
	m.resize()
	return true
}

func (m model) commandPickerHeight() int {
	return m.pickerHeight(m.commands.open, len(m.commands.matches))
}

func (m model) commandPickerView(width int) string {
	matches := m.commands.matches
	return m.pickerPane("Commands", fmt.Sprintf("%d commands", len(matches)), "No matching commands", m.commandPickerHeight(), width, len(matches), m.commands.selected, func(i int, selected bool, labelWidth int) string {
		command := matches[i]
		args, about := textStyle(m.theme.SurfaceMuted).Render(command.args), textStyle(m.theme.SurfaceHint).Render(command.about)
		if selected {
			args, about = command.args, command.about
		}
		label := lipgloss.NewStyle().Bold(true).Render(command.name) + " " + args
		return ansi.Truncate(label+strings.Repeat(" ", max(2, 44-ansi.StringWidth(label)))+about, labelWidth, "…")
	})
}
