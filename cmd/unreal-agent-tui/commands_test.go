package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

type promptWriter struct {
	inputs []inbox.Input
	err    error
}

func (w *promptWriter) Submit(_ context.Context, input inbox.Input) error {
	w.inputs = append(w.inputs, input)
	return w.err
}

func TestCommandPickerOnlyReview(t *testing.T) {
	for _, palette := range []string{"turbo-vision", "lite"} {
		t.Run(palette, func(t *testing.T) {
			m := toolsModel(t, palette, 120, 40, 0)
			m.composer.SetValue("/")
			m.syncFilePicker()
			m.resize()
			if !m.commands.open || len(m.commands.matches) != 1 || m.commands.matches[0].name != "/review" {
				t.Fatalf("commands: %+v", m.commands)
			}
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "Commands") || !strings.Contains(view, "/review") || strings.Contains(view, "/compact") {
				t.Fatalf("picker view: %s", view)
			}
			for _, code := range []rune{tea.KeyUp, tea.KeyDown} {
				if !m.commandPickerKey(fileKey(code)) || m.commands.selected != 0 {
					t.Fatal("single command selection changed")
				}
			}
			if !m.commandPickerKey(fileKey(tea.KeyTab)) || m.composer.Value() != "/review " || m.commands.open {
				t.Fatal("Tab did not complete")
			}
			m.syncFilePicker()
			if m.commands.open {
				t.Fatal("argument input left command picker open")
			}
		})
	}
}

func TestCommandPickerFilterDismissAndEnter(t *testing.T) {
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	m.composer.SetValue("/rev")
	m.syncFilePicker()
	if !m.commandPickerKey(fileKey(tea.KeyEscape)) || m.commands.open {
		t.Fatal("Esc did not dismiss")
	}
	m.syncFilePicker()
	if m.commands.open {
		t.Fatal("dismissal did not persist")
	}
	m.composer.SetValue("/revi")
	m.syncFilePicker()
	if !m.commands.open || len(m.commands.matches) != 1 {
		t.Fatal("edited query did not reopen picker")
	}
	if m.commandPickerKey(fileKey(tea.KeyEnter)) || m.composer.Value() != "/review" {
		t.Fatal("Enter did not leave completed command for dispatch")
	}
	m.composer.SetValue("/not-a-command")
	m.syncFilePicker()
	if len(m.commands.matches) != 0 || m.commandPickerKey(fileKey(tea.KeyEnter)) {
		t.Fatal("unknown command intercepted")
	}
}

func TestHiddenCommandPickerDoesNotComplete(t *testing.T) {
	for _, size := range [][2]int{{19, 40}, {80, 7}, {20, 8}} {
		m := toolsModel(t, defaultTheme, size[0], size[1], 0)
		m.composer.SetValue("/rev")
		m.syncFilePicker()
		m.resize()
		if m.commandPickerHeight() != 0 {
			t.Fatalf("fixture is not hidden: %v", size)
		}
		if m.commandPickerKey(fileKey(tea.KeyEnter)) || m.composer.Value() != "/rev" {
			t.Fatal("hidden command was completed")
		}
	}
}

func TestOrdinaryPromptsPreserveUpstreamSemantics(t *testing.T) {
	for _, text := range []string{"ordinary prompt", "/noncommand", "/compact", "/agents list", "/reviewer", "  whitespace\n stays  "} {
		t.Run(text, func(t *testing.T) {
			m := toolsModel(t, defaultTheme, 120, 40, 0)
			writer := &promptWriter{}
			m.inputs = writer
			m.composer.SetValue(text)
			updated, cmd := m.Update(fileKey(tea.KeyEnter))
			m = applyFileCommands(updated.(model), cmd)
			if len(writer.inputs) != 1 || writer.inputs[0].Kind != inbox.InputExternal || writer.inputs[0].ID == "" {
				t.Fatalf("not sent: %+v", writer.inputs)
			}
			var got string
			if err := json.Unmarshal(writer.inputs[0].Payload, &got); err != nil || got != text {
				t.Fatalf("prompt = %q, %v", got, err)
			}
			if m.composer.Value() != "" || m.sending {
				t.Fatal("send completion did not clear sending state")
			}
		})
	}
}

func TestDirectSendFailureRestoresPrompt(t *testing.T) {
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	m.inputs = &promptWriter{err: errors.New("offline")}
	m.composer.SetValue("/noncommand")
	cmd := m.startPrompt()
	m.composer.SetValue("new draft")
	m = applyFileCommands(m, cmd)
	if m.sending || m.composer.Value() != "/noncommand\nnew draft" || m.lines[len(m.lines)-1].label != "Send failed" {
		t.Fatalf("send failure state: %+v", m)
	}
}

func TestCommandPickerClosesAwayFromComposer(t *testing.T) {
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	for _, mode := range []string{"tools", "details", "ended"} {
		m.toolsFocused, m.detailsOpen, m.ended = mode == "tools", mode == "details", mode == "ended"
		m.composer.SetValue("/")
		m.syncFilePicker()
		if m.commands.open {
			t.Fatalf("picker open in %s mode", mode)
		}
	}
}
