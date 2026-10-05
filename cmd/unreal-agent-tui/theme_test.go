package main

import (
	"encoding/json/v2"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestBuiltInThemes(t *testing.T) {
	entries, err := themeFiles.ReadDir("themes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".json")
		t.Run(name, func(t *testing.T) {
			palette, err := loadTheme(name)
			if err != nil {
				t.Fatal(err)
			}
			if name == "turbo-vision" {
				return
			}
			if palette.Delimiter != "" {
				t.Fatal("legacy theme gained pane frames")
			}
			for _, pair := range [][2]string{
				{palette.SurfaceForeground, palette.Foreground}, {palette.SurfaceMuted, palette.Muted},
				{palette.SurfaceHint, palette.Hint}, {palette.SurfaceAccent, palette.Accent},
				{palette.SurfaceKey, palette.Hint}, {palette.StatusForeground, palette.Muted},
				{palette.StatusAccent, palette.Accent}, {palette.StatusError, palette.Error},
				{palette.UserForeground, palette.Foreground}, {palette.SelectionForeground, palette.Foreground},
				{palette.CodeBackground, palette.Surface},
			} {
				if pair[0] != pair[1] {
					t.Fatalf("legacy theme color changed: got %q, want %q", pair[0], pair[1])
				}
			}
		})
	}
}

func TestThemeRejectsInvalidPanelColor(t *testing.T) {
	data, err := themeFiles.ReadFile("themes/turbo-vision.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "invalid.json")
	var colors map[string]string
	if err := json.Unmarshal(data, &colors); err != nil {
		t.Fatal(err)
	}
	colors["surface_foreground"] = "blue"
	data, err = json.Marshal(colors)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTheme(path); err == nil || !strings.Contains(err.Error(), "surface_foreground") {
		t.Fatalf("invalid panel color error = %v", err)
	}
}

func TestLegacyThemeColors(t *testing.T) {
	data, err := themeFiles.ReadFile("themes/lite.json")
	if err != nil {
		t.Fatal(err)
	}
	var colors map[string]string
	if err := json.Unmarshal(data, &colors); err != nil {
		t.Fatal(err)
	}
	colors["status_active"] = "#405473"
	colors["pending_dim"] = "#705016"
	colors["pending_bright"] = "#ffe28a"
	data, err = json.Marshal(colors)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTheme(path); err != nil {
		t.Fatalf("existing custom theme no longer loads: %v", err)
	}
}

func TestTurboVisionRendering(t *testing.T) {
	opts, err := parseOptions([]string{"-model", "preview", "-theme", "turbo-vision"}, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", opts)
	if m.Init() != nil {
		t.Fatal("idle UI schedules background updates")
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(model)
	m.append("You", "User message")
	m.append("Agent", "## Agent heading\n\nAgent response with `inline code`.\n\n```go\nreturn 42\n```")
	m.tools = []toolCard{{name: "Bash", summary: "go test ./...", status: "Completed"}}
	m.composer.SetValue("Composer text")
	content := m.View().Content
	for _, check := range []struct{ text, foreground, background string }{
		{"User message", "#0000aa", "#00aaaa"},
		{"Agent heading", "#ffff55", "#0000aa"},
		{"Agent response with", "#ffffff", "#0000aa"},
		{"inline code", "#ffff55", "#0000aa"},
		{"return", "#ffff55", "#0000aa"},
		{"Tools", "#ffff55", "#0000aa"},
		{"Bash", "#ffffff", "#0000aa"},
		{"Idle", "#55ffff", "#0000aa"},
		{"Composer text", "#ffffff", "#0000aa"},
		{"Enter", "#ffff55", "#0000aa"},
		{"╔", "#55ffff", "#0000aa"},
	} {
		assertRenderedColors(t, content, check.text, check.foreground, check.background)
	}
	m.toolsFocused = true
	assertRenderedColors(t, m.View().Content, "Bash", "#ffffff", "#000000")
	m.detailsOpen = true
	m.refreshDetails()
	assertRenderedColors(t, m.View().Content, "Bash", "#ffff55", "#0000aa")
	m.detailsOpen = false
	m.toolsFocused = false
	m.tools[0].status = awaiting
	assertRenderedColors(t, m.View().Content, "●", "#ffff55", "#0000aa")
	updated, _ = m.Update(sessionstore.Item{Data: session.Turn{ID: "working"}})
	m = updated.(model)
	assertRenderedColors(t, m.View().Content, "Working", "#ffff55", "#0000aa")
	m.ended, m.runError = true, "Preview failure"
	assertRenderedColors(t, m.View().Content, "Run failed", "#ff5555", "#0000aa")
}

func TestFramedThemeLayout(t *testing.T) {
	palette, err := loadTheme("turbo-vision")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{20, 8}, {40, 12}, {80, 24}, {120, 40}, {190, 26}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			for _, mode := range []string{"conversation", "tools", "details", "collapsed"} {
				t.Run(mode, func(t *testing.T) {
					m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
					m.width, m.height = size[0], size[1]
					m.tools = []toolCard{{name: "Bash", status: "Completed", output: "Tool result"}}
					m.toolsFocused = mode == "tools" || mode == "details"
					m.detailsOpen = mode == "details"
					m.toolsCollapsed = mode == "collapsed"
					m.composer.SetValue("Draft")
					m.resize()
					m.refreshDetails()
					view := m.View()
					lines := strings.Split(ansi.Strip(view.Content), "\n")
					x, _ := m.padding()
					if len(lines) != m.height {
						t.Fatalf("rendered height = %d, want %d", len(lines), m.height)
					}
					for row, line := range lines {
						if width := ansi.StringWidth(line); width != m.width {
							t.Fatalf("row %d width = %d, want %d", row, width, m.width)
						}
						if border := strings.IndexAny(line, "╔║╚"); border >= 0 && ansi.StringWidth(line[:border]) != x {
							t.Fatalf("row %d left border is not aligned at column %d: %q", row, x, line)
						}
					}
					if !strings.Contains(view.Content, "║") || !strings.Contains(view.Content, "╚") {
						t.Fatal("pane borders missing")
					}
					if view.Cursor != nil {
						cursor := view.Cursor
						if cursor.Y >= len(lines) || ansi.Cut(lines[cursor.Y], cursor.X-5, cursor.X) != "Draft" {
							t.Fatalf("composer cursor does not follow text: %+v", cursor)
						}
					}
					m.composer.SetValue("First line\nDraft")
					m.resize()
					view = m.View()
					lines = strings.Split(ansi.Strip(view.Content), "\n")
					if len(lines) != m.height {
						t.Fatalf("multiline composer height = %d, want %d", len(lines), m.height)
					}
					if cursor := view.Cursor; cursor != nil &&
						(cursor.Y >= len(lines) || ansi.Cut(lines[cursor.Y], cursor.X-5, cursor.X) != "Draft") {
						t.Fatalf("multiline composer cursor does not follow text: %+v", cursor)
					}
					if m.mouseViewport(tea.Mouse{X: x, Y: 1}) != nil {
						t.Fatal("left border captures scrolling")
					}
				})
			}
		})
	}
}

func assertRenderedColors(t *testing.T, content, text, foreground, background string) {
	t.Helper()
	canvas := lipgloss.NewCanvas(lipgloss.Width(content), lipgloss.Height(content)).Compose(lipgloss.NewLayer(content))
	for row, line := range strings.Split(ansi.Strip(content), "\n") {
		before, _, found := strings.Cut(line, text)
		if !found {
			continue
		}
		column := ansi.StringWidth(before)
		for i := range ansi.StringWidth(text) {
			cell := canvas.CellAt(column+i, row)
			for name, pair := range map[string][2]color.Color{
				"foreground": {cell.Style.Fg, lipgloss.Color(foreground)},
				"background": {cell.Style.Bg, lipgloss.Color(background)},
			} {
				if pair[0] == nil {
					t.Fatalf("%q cell %d has no %s", text, i, name)
				}
				r, g, b, a := pair[0].RGBA()
				wr, wg, wb, wa := pair[1].RGBA()
				if r != wr || g != wg || b != wb || a != wa {
					t.Fatalf("%q cell %d %s = %v, want %v", text, i, name, pair[0], pair[1])
				}
			}
		}
		return
	}
	t.Fatalf("%q missing from rendered UI", text)
}
