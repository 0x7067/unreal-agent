package main

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/filesearch"
)

type fileMention struct {
	start, end, cursor int
	query              string
}

func mentionAt(value string, line, column int) (fileMention, bool) {
	lines := strings.Split(value, "\n")
	if line < 0 || line >= len(lines) {
		return fileMention{}, false
	}
	runes := []rune(lines[line])
	column = max(0, min(column, len(runes)))
	offset := 0
	for _, before := range lines[:line] {
		offset += len([]rune(before)) + 1
	}
	for start := column - 1; start >= 0; start-- {
		if runes[start] != '@' || start > 0 && !unicode.IsSpace(runes[start-1]) {
			continue
		}
		queryStart, end := start+1, start+1
		quoted := queryStart < len(runes) && runes[queryStart] == '"'
		if quoted {
			queryStart++
			end = queryStart
			for end < len(runes) && runes[end] != '"' {
				if runes[end] == '\\' && end+1 < len(runes) {
					end++
				}
				end++
			}
			if column < queryStart || column > end {
				return fileMention{}, false
			}
		} else {
			for end < len(runes) && !unicode.IsSpace(runes[end]) {
				end++
			}
			if column > end {
				return fileMention{}, false
			}
		}
		query := string(runes[queryStart:column])
		if quoted {
			if decoded, err := strconv.Unquote(`"` + query + `"`); err == nil {
				query = decoded
			}
			if end < len(runes) {
				end++
			}
		}
		return fileMention{start: offset + start, end: offset + end, cursor: offset + column, query: query}, true
	}
	return fileMention{}, false
}

func insertFileMention(value string, mention fileMention, path string) (string, int) {
	if strings.ContainsFunc(path, unicode.IsSpace) || strings.ContainsAny(path, `"\`) {
		path = strconv.Quote(path)
	}
	runes := []rune(value)
	suffix := runes[mention.end:]
	if len(suffix) > 0 && suffix[0] == ' ' {
		suffix = suffix[1:]
	}
	prefix := string(runes[:mention.start]) + path + " "
	return prefix + string(suffix), len([]rune(prefix))
}

type filePicker struct {
	root                string
	open                bool
	mention, dismissed  fileMention
	index               filesearch.Index
	matches             []filesearch.Match
	selected            int
	scanning, searching bool
	scanID, searchID    uint64
	scanCancel          context.CancelFunc
	searchCancel        context.CancelFunc
	err                 error
}

type fileIndexReady struct {
	id    uint64
	index filesearch.Index
	err   error
}

type fileMatchesReady struct {
	id      uint64
	matches []filesearch.Match
}

func (m *model) closeFilePicker() {
	f := &m.files
	if f.scanCancel != nil {
		f.scanCancel()
	}
	if f.searchCancel != nil {
		f.searchCancel()
	}
	f.open, f.scanning, f.searching = false, false, false
	f.index, f.matches = filesearch.Index{}, nil
	f.scanID++
	f.searchID++
}

func (m *model) syncFilePicker() tea.Cmd {
	m.syncCommandPicker()
	if m.attached {
		return nil
	}
	mention, active := mentionAt(m.composer.Value(), m.composer.Line(), m.composer.Column())
	f := &m.files
	if !active || mention == f.dismissed || m.composer.HasSelection() || m.toolsFocused || m.detailsOpen || m.ended {
		if f.open {
			m.closeFilePicker()
		}
		return nil
	}
	if f.open && mention == f.mention {
		return nil
	}
	f.mention = mention
	f.matches, f.selected = nil, 0
	if !f.open {
		f.open, f.scanning, f.err = true, true, nil
		f.scanID++
		id, root := f.scanID, f.root
		ctx, cancel := context.WithCancel(m.ctx)
		f.scanCancel = cancel
		return func() tea.Msg {
			defer cancel()
			index, err := filesearch.Scan(ctx, root)
			return fileIndexReady{id: id, index: index, err: err}
		}
	}
	return m.searchFiles()
}

func (m *model) searchFiles() tea.Cmd {
	f := &m.files
	f.searchID++
	if f.searchCancel != nil {
		f.searchCancel()
	}
	f.matches, f.selected = nil, 0
	if f.scanning {
		return nil
	}
	f.searching = true
	id, index, query := f.searchID, f.index, f.mention.query
	ctx, cancel := context.WithCancel(m.ctx)
	f.searchCancel = cancel
	return func() tea.Msg {
		defer cancel()
		matches, _ := index.Search(ctx, query, 8)
		return fileMatchesReady{id: id, matches: matches}
	}
}

func (m *model) acceptFile() {
	f := &m.files
	if len(f.matches) == 0 || m.filePickerHeight() == 0 || m.width < 20 || m.height < 8 {
		return
	}
	value, cursor := insertFileMention(m.composer.Value(), f.mention, f.matches[f.selected].Path)
	m.composer.SetValue(value)
	m.composer.MoveToBegin()
	prefix := string([]rune(value)[:cursor])
	line := strings.Count(prefix, "\n")
	for m.composer.Line() < line {
		m.composer.CursorEnd()
		m.composer.CursorDown()
	}
	last := prefix[strings.LastIndexByte(prefix, '\n')+1:]
	m.composer.SetCursorColumn(len([]rune(last)))
	m.closeFilePicker()
}

func (m *model) filePickerKey(msg tea.KeyPressMsg) bool {
	if !m.files.open {
		return false
	}
	switch msg.String() {
	case "up", "down", "ctrl+p", "ctrl+n", "pgup", "pgdown":
		if count := len(m.files.matches); count > 0 {
			step := 1
			if msg.String() == "pgup" || msg.String() == "pgdown" {
				step = max(1, m.filePickerHeight()-2)
			}
			if msg.String() == "up" || msg.String() == "ctrl+p" || msg.String() == "pgup" {
				step = -step
			}
			m.files.selected = (m.files.selected + step%count + count) % count
		}
	case "enter", "tab":
		m.acceptFile()
	case "esc":
		m.files.dismissed = m.files.mention
		m.closeFilePicker()
	default:
		return false
	}
	m.resize()
	return true
}

func (m model) filePickerHeight() int { return m.pickerHeight(m.files.open, len(m.files.matches)) }

func (m model) pickerHeight(open bool, matches int) int {
	if !open {
		return 0
	}
	_, y := m.padding()
	padding, overhead := 2*y, 1
	if m.theme.Delimiter != "" {
		padding, overhead = 2, 2
	}
	rows := max(1, matches)
	height := min(rows+overhead, max(0, m.height-m.composer.Height()-5-padding))
	if height <= overhead {
		return 0
	}
	return height
}

func (m model) filePickerView(width int) string {
	hint := fmt.Sprintf("%d results", len(m.files.matches))
	if m.files.err != nil {
		hint = "Some paths unavailable"
	}
	status := "No matching files"
	if m.files.scanning {
		status = "Scanning workspace…"
	} else if m.files.searching {
		status = "Searching…"
	}
	return m.pickerPane("Files", hint, status, m.filePickerHeight(), width, len(m.files.matches), m.files.selected, func(i int, selected bool, labelWidth int) string {
		match := m.files.matches[i]
		label := match.Path
		if !selected {
			var highlighted strings.Builder
			for j, r := range []rune(match.Path) {
				if slices.Contains(match.Positions, j) {
					highlighted.WriteString(textStyle(m.theme.SurfaceAccent).Bold(true).Render(string(r)))
				} else {
					highlighted.WriteRune(r)
				}
			}
			label = highlighted.String()
		}
		if width := ansi.StringWidth(label); width > labelWidth {
			label = ansi.TruncateLeft(label, width-labelWidth+1, "…")
		}
		return label
	})
}

// pickerPane renders the window of rows around the selection above the
// composer. label renders row i in at most labelWidth cells.
func (m model) pickerPane(title, hint, empty string, height, width, count, selected int, label func(i int, selected bool, labelWidth int) string) string {
	if height == 0 {
		return ""
	}
	overhead, innerWidth := 1, width
	if m.theme.Delimiter != "" {
		overhead, innerWidth = 2, width-2
	}
	rows := height - overhead
	start := max(0, min(selected-rows+1, count-rows))
	content := make([]string, 0, rows)
	for i := start; i < min(start+rows, count); i++ {
		style := textStyle(m.theme.SurfaceForeground).Background(lipgloss.Color(m.theme.Surface))
		marker := "  "
		if i == selected {
			marker = "› "
			style = style.Foreground(lipgloss.Color(m.theme.SelectionForeground)).Background(lipgloss.Color(m.theme.Selection))
		}
		content = append(content, renderSurface(style.Width(innerWidth).Padding(0, 1), marker+label(i, i == selected, max(1, innerWidth-4))))
	}
	if len(content) == 0 {
		content = append(content, renderSurface(textStyle(m.theme.SurfaceMuted).Background(lipgloss.Color(m.theme.Surface)).
			Width(innerWidth).Padding(0, 1), ansi.Truncate(empty, max(1, innerWidth-2), "…")))
	}
	for len(content) < rows {
		content = append(content, strings.Repeat(" ", innerWidth))
	}
	if overhead == 2 {
		return m.framedPane(title, hint, strings.Join(content, "\n"), m.theme.Surface, width)
	}
	return m.heading(title, hint, m.theme.Surface, m.theme.SurfaceAccent, m.theme.SurfaceHint, width) + "\n" + strings.Join(content, "\n")
}
