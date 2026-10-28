package main

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestScreenFiltersRedundantSizeNotifications(t *testing.T) {
	// The initial size must reach Update even when it matches the defaults:
	// startup may still have a saved attached view to restore.
	screen := &mouseScreen{model: toolsModel(t, defaultTheme, 80, 24, 0)}
	for _, tc := range []struct {
		size tea.WindowSizeMsg
		pass bool
	}{
		{tea.WindowSizeMsg{Width: 0, Height: 0}, false},
		{tea.WindowSizeMsg{Width: 80, Height: 24}, true},
		{tea.WindowSizeMsg{Width: 80, Height: 24}, false},
		{tea.WindowSizeMsg{Width: 0, Height: 24}, false},
		{tea.WindowSizeMsg{Width: 80, Height: -1}, false},
		{tea.WindowSizeMsg{Width: 80, Height: 24}, false},
		{tea.WindowSizeMsg{Width: 120, Height: 24}, true},
		{tea.WindowSizeMsg{Width: 120, Height: 40}, true},
		{tea.WindowSizeMsg{Width: 120, Height: 40}, false},
		{tea.WindowSizeMsg{Width: 80, Height: 24}, true},
	} {
		got := screen.filter(screen, tc.size)
		if (got != nil) != tc.pass {
			t.Fatalf("size %+v: filter = %v, want pass = %v", tc.size, got, tc.pass)
		}
		if got != nil {
			if got != tc.size {
				t.Fatalf("filter changed size: got %v, want %v", got, tc.size)
			}
			screen.Update(got)
			if screen.width != tc.size.Width || screen.height != tc.size.Height {
				t.Fatalf("real resize not applied: %dx%d", screen.width, screen.height)
			}
		}
	}
}

func TestDuplicateSizePreservesViewState(t *testing.T) {
	screen := &mouseScreen{model: toolsModel(t, defaultTheme, 120, 40, 10)}
	size := tea.WindowSizeMsg{Width: 120, Height: 40}
	screen.Update(screen.filter(screen, size))
	screen.append("Agent", strings.Repeat("A line of history\n\n", 100))
	screen.conversation.SetYOffset(5)
	screen.composer.SetValue("unfinished draft")
	screen.resize()
	screen.selection = textSelection{anchor: image.Pt(0, 5), head: image.Pt(4, 5)}
	screen.selection.capture(screen.conversation)
	before := screen.View()
	selected := screen.selectedText()
	if selected == "" {
		t.Fatal("fixture has no selection")
	}

	for range 3 {
		if msg := screen.filter(screen, size); msg != nil {
			t.Fatal("duplicate size would reach Bubble Tea's full-screen erase")
		}
	}
	if after := screen.View(); after.Content != before.Content || screen.selectedText() != selected ||
		screen.conversation.YOffset() != 5 || screen.composer.Value() != "unfinished draft" {
		t.Fatalf("duplicate size: content equal=%v selection=%q (want %q) offset=%d draft=%q", after.Content == before.Content, screen.selectedText(), selected, screen.conversation.YOffset(), screen.composer.Value())
	}

	// A genuine size change still reflows the view and clears stale selection.
	resized := tea.WindowSizeMsg{Width: 100, Height: 35}
	screen.Update(screen.filter(screen, resized))
	if len(screen.selection.lines) != 0 || screen.width != 100 || screen.height != 35 {
		t.Fatal("real resize did not update layout and clear stale selection")
	}
	if screen.composer.Value() != "unfinished draft" {
		t.Fatal("real resize lost draft")
	}
}
