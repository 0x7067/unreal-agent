package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/filesearch"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestMentionAtCursor(t *testing.T) {
	for _, test := range []struct {
		value              string
		line, column       int
		query              string
		start, end, cursor int
		active             bool
	}{
		{"@", 0, 1, "", 0, 1, 1, true},
		{"fix @main.go please", 0, 7, "ma", 4, 12, 7, true},
		{"first\nλ @café", 1, 7, "café", 8, 13, 13, true},
		{`read @"docs/a b.md" please`, 0, 15, "docs/a b", 5, 19, 15, true},
		{`@"a\"b`, 0, 6, `a"b`, 0, 6, 6, true},
		{"email@example.com", 0, 17, "", 0, 0, 0, false},
		{"@main.go next", 0, 13, "", 0, 0, 0, false},
		{"@file\nnext", 1, 4, "", 0, 0, 0, false},
		{`@"a b" `, 0, 6, "", 0, 0, 0, false},
		{"text", 0, 4, "", 0, 0, 0, false},
	} {
		t.Run(fmt.Sprintf("%s/%d", test.value, test.column), func(t *testing.T) {
			mention, active := mentionAt(test.value, test.line, test.column)
			want := fileMention{start: test.start, end: test.end, cursor: test.cursor, query: test.query}
			if active != test.active || mention != want {
				t.Fatalf("mention = %+v, %v; want %+v, %v", mention, active, want, test.active)
			}
		})
	}
}

func TestInsertFileMentionPreservesDraft(t *testing.T) {
	for _, test := range []struct {
		value, path, want string
		line, column      int
		cursor            int
	}{
		{"fix @ma please", "src/main.go", "fix src/main.go please", 0, 7, 16},
		{"first\nλ @caf after\nlast", "café/Éclair.txt", "first\nλ café/Éclair.txt after\nlast", 1, 5, 24},
		{`read @"docs/a b.md" please`, "docs/with spaces.md", `read "docs/with spaces.md" please`, 0, 14, 27},
		{"@f", `a"b\c.txt`, `"a\"b\\c.txt" `, 0, 2, 14},
	} {
		t.Run(test.path, func(t *testing.T) {
			mention, ok := mentionAt(test.value, test.line, test.column)
			if !ok {
				t.Fatal("mention missing")
			}
			value, cursor := insertFileMention(test.value, mention, test.path)
			if value != test.want || cursor != test.cursor {
				t.Fatalf("insert = %q, %d; want %q, %d", value, cursor, test.want, test.cursor)
			}
		})
	}
}

func fileTestModel(t *testing.T, root, palette string) model {
	t.Helper()
	theme, err := loadTheme(palette)
	if err != nil {
		t.Fatal(err)
	}
	return newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), root, "sessions", options{theme: theme})
}

func applyFileCommands(m model, cmd tea.Cmd) model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, next := range batch {
			m = applyFileCommands(m, next)
		}
		return m
	}
	updated, next := m.Update(msg)
	return applyFileCommands(updated.(model), next)
}

func fileKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func TestFilePickerKeyboardAndSubmission(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", "/nonexistent-shell")
	root := t.TempDir()
	for _, name := range []string{"alpha.go", "beta.go", "docs with spaces.md"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := fileTestModel(t, root, "turbo-vision")
	inputs, err := inbox.New(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	m.inputs = inputs
	updated, cmd := m.Update(tea.KeyPressMsg{Code: '@', Text: "@"})
	m = applyFileCommands(updated.(model), cmd)
	if !m.files.open || len(m.files.matches) != 3 || m.files.matches[0].Path != "beta.go" {
		t.Fatalf("picker = %+v", m.files)
	}
	updated, cmd = m.Update(fileKey(tea.KeyDown))
	m = applyFileCommands(updated.(model), cmd)
	if m.files.selected != 1 {
		t.Fatalf("selected = %d", m.files.selected)
	}
	updated, cmd = m.Update(fileKey(tea.KeyEnter))
	m = updated.(model)
	if cmd != nil || m.files.open || m.sending || m.composer.Value() != "alpha.go " {
		t.Fatalf("accept: value=%q open=%v sending=%v cmd=%v", m.composer.Value(), m.files.open, m.sending, cmd)
	}
	updated, cmd = m.Update(fileKey(tea.KeyEnter))
	m = applyFileCommands(updated.(model), cmd)
	if m.sending || m.composer.Value() != "" {
		t.Fatal("second Enter did not send")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var input inbox.Input
	select {
	case input = <-inputs.Output():
	case <-ctx.Done():
		t.Fatal("submitted input missing")
	}
	var text string
	if err := json.Unmarshal(input.Payload, &text); err != nil || text != "alpha.go " {
		t.Fatalf("submitted text = %q, %v", text, err)
	}
	updated, cmd = m.Update(tea.PasteMsg{Content: "@dws"})
	m = applyFileCommands(updated.(model), cmd)
	updated, cmd = m.Update(fileKey(tea.KeyTab))
	m = applyFileCommands(updated.(model), cmd)
	if m.composer.Value() != `"docs with spaces.md" ` || m.toolsFocused {
		t.Fatalf("Tab acceptance = %q, tools=%v", m.composer.Value(), m.toolsFocused)
	}
}

func TestFilePickerStaleResultsAndDismissal(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), "turbo-vision")
	m.composer.SetValue("@a")
	scan := m.syncFilePicker()
	scanID := m.files.scanID
	m.composer.SetValue("@b")
	if cmd := m.syncFilePicker(); cmd != nil {
		t.Fatal("query edit rescanned workspace")
	}
	updated, search := m.Update(fileIndexReady{id: scanID, index: filesearch.NewIndex([]string{"alpha", "beta"})})
	m = updated.(model)
	searchID := m.files.searchID
	m.composer.SetValue("@a")
	latest := m.syncFilePicker()
	updated, _ = m.Update(search())
	m = updated.(model)
	if !m.files.searching || len(m.files.matches) != 0 || m.files.searchID == searchID {
		t.Fatal("stale search replaced current results")
	}
	m = applyFileCommands(m, latest)
	if len(m.files.matches) != 2 || m.files.matches[0].Path != "alpha" {
		t.Fatalf("latest matches = %v", m.files.matches)
	}
	updated, cmd := m.Update(fileKey(tea.KeyEsc))
	m = updated.(model)
	if cmd != nil || m.files.open || m.composer.Value() != "@a" || m.syncFilePicker() != nil {
		t.Fatal("Esc did not leave the draft with the picker dismissed")
	}
	updated, _ = m.Update(scan())
	m = updated.(model)
	if m.files.open || m.files.scanning {
		t.Fatal("stale index reopened dismissed picker")
	}
	updated, cmd = m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = applyFileCommands(updated.(model), cmd)
	if !m.files.open {
		t.Fatal("editing a dismissed query did not reopen the picker")
	}
}

func TestPickerLoadingAndNoMatchesDoNotSend(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), "turbo-vision")
	m.composer.SetValue("@missing")
	scan := m.syncFilePicker()
	for _, code := range []rune{tea.KeyEnter, tea.KeyTab, tea.KeyUp, tea.KeyDown} {
		updated, cmd := m.Update(fileKey(code))
		m = updated.(model)
		if cmd != nil || m.sending || m.toolsFocused || m.composer.Value() != "@missing" {
			t.Fatal("loading picker intercepted input incorrectly")
		}
	}
	m = applyFileCommands(m, scan)
	if !strings.Contains(m.View().Content, "No matching files") {
		t.Fatal("empty state missing")
	}
	updated, cmd := m.Update(fileKey(tea.KeyEnter))
	if cmd != nil || updated.(model).sending {
		t.Fatal("empty picker sent the message")
	}
}

func TestFilePickerLayoutAndCursor(t *testing.T) {
	for _, palette := range []string{"turbo-vision", "lite"} {
		for _, size := range [][2]int{{20, 8}, {40, 12}, {80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s/%dx%d", palette, size[0], size[1]), func(t *testing.T) {
				m := fileTestModel(t, "workspace", palette)
				m.width, m.height = size[0], size[1]
				m.composer.SetValue("First line\n@draft")
				m.files.open = true
				for i := range 8 {
					m.files.matches = append(m.files.matches, filesearch.Match{Path: fmt.Sprintf("long/path/to/a/file%d.go", i)})
				}
				m.files.selected = 7
				m.resize()
				view := m.View()
				if m.filePickerHeight() == 0 {
					if !strings.Contains(view.Content, "Resize") || view.Cursor != nil {
						t.Fatal("small terminal has no resize hint")
					}
					return
				}
				lines := strings.Split(ansi.Strip(view.Content), "\n")
				if len(lines) != m.height {
					t.Fatalf("height = %d, want %d", len(lines), m.height)
				}
				for row, line := range lines {
					if width := ansi.StringWidth(line); width != m.width {
						t.Fatalf("row %d width = %d, want %d", row, width, m.width)
					}
				}
				if cursor := view.Cursor; cursor == nil || cursor.Y >= len(lines) || ansi.Cut(lines[cursor.Y], cursor.X-6, cursor.X) != "@draft" {
					t.Fatalf("cursor not after draft: %+v", cursor)
				}
				if m.filePickerHeight() > 0 && !strings.Contains(view.Content, "Files") {
					t.Fatal("picker is missing")
				}
				if !strings.Contains(ansi.Strip(view.Content), "file7.go") {
					t.Fatal("selected filename is hidden")
				}
			})
		}
	}
}

func TestFileInsertionAtWrappedMultilineCursor(t *testing.T) {
	m := fileTestModel(t, "workspace", "turbo-vision")
	m.width = 24
	m.composer.SetValue("long first line that wraps\nλ @caf suffix\nlast")
	m.composer.MoveToBegin()
	m.composer.CursorEnd()
	m.composer.CursorDown()
	m.composer.SetCursorColumn(6)
	m.resize()
	mention, ok := mentionAt(m.composer.Value(), m.composer.Line(), m.composer.Column())
	if !ok {
		t.Fatal("mention missing")
	}
	m.files.open, m.files.mention = true, mention
	m.files.matches = []filesearch.Match{{Path: "café/Éclair.txt"}}
	m.acceptFile()
	if m.composer.Value() != "long first line that wraps\nλ café/Éclair.txt suffix\nlast" || m.composer.Line() != 1 || m.composer.Column() != 18 {
		t.Fatalf("composer = %q at %d:%d", m.composer.Value(), m.composer.Line(), m.composer.Column())
	}
}

func TestFilePickerCancelsWhenLeavingComposer(t *testing.T) {
	m := fileTestModel(t, t.TempDir(), "turbo-vision")
	m.composer.SetValue("@")
	scan := m.syncFilePicker()
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = updated.(model)
	if m.files.open || !m.toolsFocused {
		t.Fatal("tool focus left picker open")
	}
	if result := scan().(fileIndexReady); result.err != context.Canceled {
		t.Fatalf("scan not canceled: %v", result.err)
	}
}

func TestFilePickerRefreshesAfterAcceptance(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "before.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := fileTestModel(t, root, "turbo-vision")
	updated, cmd := m.Update(tea.PasteMsg{Content: "@before"})
	m = applyFileCommands(updated.(model), cmd)
	updated, cmd = m.Update(fileKey(tea.KeyEnter))
	m = applyFileCommands(updated.(model), cmd)
	if m.files.open {
		t.Fatal("acceptance did not close picker")
	}
	if err := os.WriteFile(filepath.Join(root, "after.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	updated, cmd = m.Update(tea.PasteMsg{Content: "@after"})
	m = applyFileCommands(updated.(model), cmd)
	if len(m.files.matches) != 1 || m.files.matches[0].Path != "after.go" {
		t.Fatalf("reopened picker has stale files: %v", m.files.matches)
	}
}

func TestFilePickerColors(t *testing.T) {
	m := fileTestModel(t, "workspace", "turbo-vision")
	m.composer.SetValue("@fi")
	m.files.open = true
	m.files.matches = []filesearch.Match{{Path: "chosen.go"}, {Path: "files.go", Positions: []int{0, 1}}}
	m.resize()
	view := m.View()
	assertRenderedColors(t, view.Content, "chosen.go", "#ffffff", "#000000")
	assertRenderedColors(t, view.Content, "fi", "#ffff55", "#0000aa")
}
