package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

var cardSummary = regexp.MustCompile(`echo \d+`)

func toolsModel(t *testing.T, themeName string, width, height, cards int) model {
	t.Helper()
	palette, err := loadTheme(themeName)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updated.(model)
	for i := range cards {
		addCard(&m, i)
	}
	return m
}

func addCard(m *model, i int) {
	card := m.card(callKey{"turn", fmt.Sprint(i)})
	card.name, card.status, card.summary, card.output = "Bash", "Completed", fmt.Sprintf("echo %d", i), fmt.Sprintf("output %d", i)
}

// visibleCards lists card summaries on screen, top to bottom.
func visibleCards(m model) []string {
	return cardSummary.FindAllString(ansi.Strip(m.View().Content), -1)
}

// cardCell returns the screen cell of a card's summary.
func cardCell(t *testing.T, m model, summary string) (int, int) {
	t.Helper()
	pattern := regexp.MustCompile(regexp.QuoteMeta(summary) + `\b`)
	for y, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if loc := pattern.FindStringIndex(line); loc != nil {
			return ansi.StringWidth(line[:loc[0]]), y
		}
	}
	t.Fatalf("%q not on screen", summary)
	return 0, 0
}

func update(m model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

func TestToolsWheelScrollsWithoutTakingFocus(t *testing.T) {
	for _, themeName := range []string{"turbo-vision", "github-dark"} {
		t.Run(themeName, func(t *testing.T) {
			m := toolsModel(t, themeName, 160, 50, 120)
			if got := visibleCards(m)[0]; got != "echo 119" {
				t.Fatalf("top card = %q, want newest", got)
			}
			x, y := cardCell(t, m, "echo 110")
			screen := &mouseScreen{model: m}
			for range 3 {
				wheel := tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown}
				if screen.filter(screen, wheel) == nil {
					t.Fatal("wheel over the Tools pane was dropped")
				}
				screen.model = update(screen.model, wheel)
			}
			m = screen.model
			if got := visibleCards(m)[0]; got != "echo 116" {
				t.Fatalf("top card after 3 notches = %q, want echo 116", got)
			}
			if m.toolsFocused || m.View().Cursor == nil {
				t.Fatal("wheel took focus from the composer")
			}

			addCard(&m, 120)
			if got := visibleCards(m)[0]; got != "echo 116" {
				t.Fatalf("top card after a new call while scrolled = %q, want echo 116", got)
			}
			for range 10 {
				m = update(m, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp})
			}
			addCard(&m, 121)
			if got := visibleCards(m)[0]; got != "echo 121" {
				t.Fatalf("top card after a new call at the top = %q, want echo 121", got)
			}
		})
	}
}

func TestToolsKeysKeepSelectionVisible(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 120)
	m = update(m, tea.KeyPressMsg{Code: tea.KeyTab})
	rows := m.toolRows()
	for range rows + 5 {
		m = update(m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	want := fmt.Sprintf("echo %d", 119-(rows+5))
	if cards := visibleCards(m); cards[len(cards)-1] != want {
		t.Fatalf("bottom card = %q, want selected %q", cards[len(cards)-1], want)
	}
	m = update(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.detailsOpen || !strings.Contains(ansi.Strip(m.View().Content), "output "+strings.TrimPrefix(want, "echo ")) {
		t.Fatalf("Enter did not open the selected card %q", want)
	}
}

func TestToolsClickOpensDetails(t *testing.T) {
	for _, tc := range []struct {
		theme         string
		width, height int
		focus         bool
	}{
		{"turbo-vision", 160, 50, false},
		{"github-dark", 160, 50, false},
		{"turbo-vision", 80, 30, true}, // narrow: Tab swaps the conversation for the Tools pane
	} {
		t.Run(fmt.Sprintf("%s-%dx%d", tc.theme, tc.width, tc.height), func(t *testing.T) {
			m := toolsModel(t, tc.theme, tc.width, tc.height, 30)
			if tc.focus {
				m = update(m, tea.KeyPressMsg{Code: tea.KeyTab})
			}
			x, y := cardCell(t, m, "echo 25")
			m = update(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			screen := ansi.Strip(m.View().Content)
			if !m.detailsOpen || !strings.Contains(screen, "output 25") {
				t.Fatalf("click did not open details for echo 25:\n%s", screen)
			}
			m = update(m, tea.KeyPressMsg{Code: tea.KeyEscape})
			if m.detailsOpen || !m.toolsFocused || m.tools[m.selected].summary != "echo 25" {
				t.Fatalf("Esc should return to the focused list on echo 25, got details=%v focused=%v selected=%q",
					m.detailsOpen, m.toolsFocused, m.tools[m.selected].summary)
			}
		})
	}
}

func TestToolsClickOutsideCardsDoesNothing(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 3)
	x, _ := cardCell(t, m, "echo 2")
	for _, y := range []int{0, 1, 20} { // border, blank row, empty row below the cards
		got := update(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		if got.detailsOpen || got.toolsFocused {
			t.Fatalf("click on row %d opened details or focused the pane", y)
		}
	}
}
