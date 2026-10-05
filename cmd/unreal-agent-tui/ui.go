package main

import (
	"context"
	"encoding/json/v2"
	"math"
	"slices"
	"strings"
	"time"
	"uuid"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/stopwatch"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type runEnded struct{ err error }

const mascot = `⠀⠀⠀⠀⠀⢀⣤⡶⠟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⣀⣤⣴⣶⠿⠟⠛⠛⠛⠻⠶⣦⣄⠀⠀⠀⠀⠀⠀
⠀⠉⣽⡟⠁⠀⠀⠀⠀⠀⠀⠀⠀⠙⢷⣄⠀⠀⠀⠀
⠀⣼⠏⠀⠀⢀⣀⠀⢠⣤⡀⠀⠀⠀⠀⢻⡆⠀⠀⠀
⢸⡟⠀⠀⠀⠘⠿⠇⠘⠟⠁⠀⠀⠀⠀⠈⣿⠀⠀⠀
⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⡀⠀⢀⣿⠀⠀⠀
⢿⡇⠀⠀⢶⣄⡀⠀⠀⠀⣀⣼⠟⠀⠀⣼⠇⠀⠀⠀
⠘⣿⡄⠀⠀⠉⠛⠻⠿⠛⠋⠁⠀⢠⣾⠏⠀⠀⠀⠀
⠀⠈⠻⣦⣄⡀⠀⠀⠀⠀⢀⣠⣶⠟⠁⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠉⠛⠻⠿⠿⠟⠛⠉⠀⠀⠀⠀⠀⠀⠀⠀`

var (
	sendKey      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "send"))
	focusKey     = key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "tools"))
	quitKey      = key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "exit"))
	copyKey      = key.NewBinding(key.WithKeys("ctrl+y", "super+c", "ctrl+н", "super+с"), key.WithHelp("Ctrl+Y", "copy"))
	newlineKey   = key.NewBinding(key.WithKeys("shift+enter", "ctrl+j"), key.WithHelp("Shift+Enter/Ctrl+J", "newline"))
	scrollKey    = key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("PgUp/PgDn", "scroll"))
	selectKey    = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "tools"))
	backKey      = key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "tools"))
	mouseKey     = key.NewBinding(key.WithKeys("mouse left"), key.WithHelp("Drag", "select"))
	reasoningKey = key.NewBinding(key.WithKeys("f3"), key.WithHelp("F3", "hide reasoning"))
)

type submitted struct {
	text string
	err  error
}
type callKey struct {
	turn session.TurnID
	call string
}

type message struct {
	label, text, rendered string
	width                 int
}

type model struct {
	registry                            tool.Registry
	ctx                                 context.Context
	inputs                              inbox.Writer
	composer                            textarea.Model
	files                               filePicker
	conversation                        viewport.Model
	details                             viewport.Model
	detailsOpen                         bool
	width, height                       int
	runError                            string
	workspace, directory, configuration string
	lines                               []message
	theme                               theme
	tools                               []toolCard
	selected                            int
	toolsFocused                        bool
	toolsCollapsed                      bool
	selection                           textSelection
	dragComposer                        bool
	hideReasoning                       bool
	workTimer                           stopwatch.Model
	responding, sending, ended          bool
	turn                                session.TurnID
}

func newModel(ctx context.Context, inputs inbox.Writer, registry tool.Registry, workspace, directory string, opts options) model {
	composer := textarea.New()
	composer.Prompt = ""
	composer.Placeholder = "Send a message · @ to find files"
	composer.ShowLineNumbers = false
	composer.SetVirtualCursor(false)
	composer.DynamicHeight = true
	composer.MinHeight = 1
	composer.MaxContentHeight = math.MaxInt
	composer.CharLimit = 0
	composer.MaxWidth = 0
	composer.SetStyles(opts.theme.composerStyles())
	composer.Focus()
	conversation := viewport.New(viewport.WithWidth(80), viewport.WithHeight(17))
	conversation.FillHeight = true
	conversation.MouseWheelDelta = 1
	m := model{ctx: ctx, inputs: inputs, registry: registry, composer: composer, conversation: conversation,
		width: 80, height: 24, details: viewport.New(), theme: opts.theme,
		workspace: singleLine(workspace), directory: singleLine(directory),
		configuration: singleLine(strings.Join([]string{opts.provider, opts.model, opts.effort}, " · ")),
	}
	m.files.root = workspace
	m.workTimer = stopwatch.New(stopwatch.WithInterval(time.Second))
	m.details.FillHeight = true
	m.details.MouseWheelDelta = 1
	m.resize()
	return m
}

func (m model) Init() tea.Cmd { return m.composer.Focus() }

func (m *model) mouseViewport(msg tea.Mouse) *viewport.Model {
	x, _ := m.padding()
	if m.theme.Delimiter != "" {
		x++
	}
	if m.width < 20 || m.height < 8 || msg.X < x || msg.X >= m.width || msg.Y < 1 {
		return nil
	}
	if m.detailsOpen {
		if msg.X < x+m.details.Width() && msg.Y <= m.details.Height() {
			return &m.details
		}
	} else if (m.width >= 90 || !m.toolsFocused) && msg.X < x+m.conversation.Width() && msg.Y <= m.conversation.Height() {
		return &m.conversation
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case fileIndexReady:
		if !m.files.open || msg.id != m.files.scanID {
			return m, nil
		}
		m.files.scanning = false
		m.files.index, m.files.err = msg.index, msg.err
		cmd = m.searchFiles()
	case fileMatchesReady:
		if !m.files.open || msg.id != m.files.searchID {
			return m, nil
		}
		m.files.searching, m.files.matches = false, msg.matches
	case stopwatch.TickMsg, stopwatch.StartStopMsg:
		if m.ended || !m.responding {
			return m, nil
		}
		m.workTimer, cmd = m.workTimer.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			follow := m.followConversation()
			m.selection = textSelection{}
			m.dragComposer = false
			m.width, m.height = msg.Width, msg.Height
			m.resize()
			if follow {
				m.conversation.GotoBottom()
			}
		}
	case sessionstore.Item:
		previousTurn := m.turn
		m.apply(msg)
		if m.responding && m.turn != previousTurn {
			m.workTimer = stopwatch.New(stopwatch.WithInterval(time.Second))
			cmd = m.workTimer.Start()
		} else if !m.responding {
			m.workTimer, _ = m.workTimer.Update(m.workTimer.Stop()())
		}
	case submitted:
		m.sending = false
		if msg.err != nil {
			m.append("Send failed", msg.err.Error())
			m.composer.SetValue(strings.TrimSpace(msg.text + "\n" + m.composer.Value()))
		}
	case runEnded:
		m.ended, m.responding = true, false
		m.workTimer, _ = m.workTimer.Update(m.workTimer.Stop()())
		m.refreshDetails()
		if msg.err != nil {
			m.runError = singleLine(msg.err.Error())
			m.append("Run failed", msg.err.Error())
		}
	case tea.PasteMsg:
		m.selection = textSelection{}
		if !m.toolsFocused {
			m.composer, cmd = m.composer.Update(msg)
		}
	case tea.MouseWheelMsg:
		m.scrollWheel(msg)
		return m, nil
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			m.beginSelection(msg.Mouse())
		}
		return m, nil
	case tea.MouseMotionMsg:
		m.extendSelection(msg.Mouse(), true)
		return m, nil
	case tea.MouseReleaseMsg:
		m.endSelection(msg.Mouse())
		return m, nil
	case tea.KeyPressMsg:
		if key.Matches(msg, copyKey) {
			if !m.hasSelection() {
				return m, nil
			}
			return m, copyText(m.selectedText())
		}
		if msg.String() == "esc" && len(m.selection.lines) > 0 {
			m.selection = textSelection{}
			return m, nil
		}
		if m.filePickerKey(msg) {
			return m, nil
		}
		if key.Matches(msg, reasoningKey) {
			follow := m.followConversation()
			m.selection = textSelection{}
			m.hideReasoning = !m.hideReasoning
			m.renderConversation()
			if follow {
				m.conversation.GotoBottom()
			}
			return m, nil
		}
		if msg.String() == "shift+tab" {
			m.selection = textSelection{}
			if m.width < 90 {
				m.toolsCollapsed = m.toolsFocused || m.detailsOpen
			} else {
				m.toolsCollapsed = !m.toolsCollapsed
			}
			m.detailsOpen = false
			m.toolsFocused = !m.toolsCollapsed && m.width < 90
			if m.toolsFocused {
				m.selected = 0
				m.toolsCollapsed = false
				m.composer.Blur()
			} else {
				cmd = m.composer.Focus()
			}
			cmd = tea.Batch(cmd, m.syncFilePicker())
			m.resize()
			return m, cmd
		}
		if m.detailsOpen && !key.Matches(msg, quitKey) {
			switch {
			case key.Matches(msg, backKey):
				m.selection = textSelection{}
				m.detailsOpen = false
			case key.Matches(msg, focusKey):
				m.selection = textSelection{}
				m.detailsOpen, m.toolsFocused = false, false
				cmd = m.composer.Focus()
			case msg.String() == "ctrl+end":
				m.details.GotoBottom()
			default:
				m.details, cmd = m.details.Update(msg)
			}
			cmd = tea.Batch(cmd, m.syncFilePicker())
			m.resize()
			return m, cmd
		}
		if key.Matches(msg, focusKey) || key.Matches(msg, backKey) && m.toolsFocused {
			m.selection = textSelection{}
			m.toolsFocused = !m.toolsFocused
			if m.toolsFocused {
				m.selected = 0
				m.toolsCollapsed = false
				m.composer.Blur()
			} else {
				cmd = m.composer.Focus()
			}
			cmd = tea.Batch(cmd, m.syncFilePicker())
			m.resize()
			return m, cmd
		}
		if m.toolsFocused && !key.Matches(msg, quitKey) {
			switch {
			case key.Matches(msg, sendKey):
				if len(m.tools) > 0 {
					m.selection = textSelection{}
					m.detailsOpen = true
					m.refreshDetails()
					m.details.GotoTop()
				}
			case key.Matches(msg, selectKey, scrollKey):
				step := 1
				if key.Matches(msg, scrollKey) {
					step = max(1, m.conversation.Height()-1)
				}
				if msg.String() == "up" || msg.String() == "pgup" {
					step = -step
				}
				m.selected = max(0, min(len(m.tools)-1, m.selected+step))
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, quitKey):
			m.closeFilePicker()
			return m, tea.Quit
		case key.Matches(msg, sendKey):
			text := m.composer.Value()
			if m.ended || m.sending || strings.TrimSpace(text) == "" {
				break
			}
			m.composer.Reset()
			m.sending = true
			cmd = func() tea.Msg {
				payload, err := json.Marshal(text)
				if err == nil {
					err = m.inputs.Submit(m.ctx, inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputExternal, Payload: payload})
				}
				return submitted{text: text, err: err}
			}
		case key.Matches(msg, newlineKey):
			m.composer.InsertString("\n")
		case key.Matches(msg, scrollKey):
			m.conversation, cmd = m.conversation.Update(msg)
		case msg.String() == "ctrl+end":
			m.conversation.GotoBottom()
		default:
			if msg.Mod.Contains(tea.ModSuper) {
				return m, nil
			}
			m.selection = textSelection{}
			m.composer, cmd = m.composer.Update(msg)
		}
	default:
		m.composer, cmd = m.composer.Update(msg)
	}
	cmd = tea.Batch(cmd, m.syncFilePicker())
	m.resize()
	return m, cmd
}

func (m *model) append(label, text string) {
	text = terminaltext.Clean(text)
	if (label == "Agent" || label == "Reasoning" || label == "Provider") && strings.TrimSpace(text) == "" {
		return
	}
	follow := m.followConversation()
	m.lines = append(m.lines, message{label: terminaltext.Clean(label), text: text})
	m.renderConversation()
	if follow {
		m.conversation.GotoBottom()
	}
}

func (m *model) apply(item sessionstore.Item) {
	switch data := item.Data.(type) {
	case inbox.Input:
		if data.Kind == inbox.InputExternal {
			var text string
			if json.Unmarshal(data.Payload, &text) == nil {
				m.append("You", text)
			}
		}
	case session.Turn:
		m.turn, m.responding = data.ID, true
	case sessionstore.ModelResponse:
		answered := false
		if data.TurnID == m.turn {
			m.responding = false
		}
		for _, output := range data.Response.Output {
			switch value := output.Data.(type) {
			case llm.Message:
				before := len(m.lines)
				m.append("Agent", value.Text)
				answered = answered || len(m.lines) > before
			case llm.ProviderItem:
				if value.Display != nil {
					label := "Provider"
					if value.Display.Kind == llm.ProviderDisplayReasoning {
						label = "Reasoning"
					}
					m.append(label, value.Display.Text)
				}
			case llm.ToolCall:
				card := m.card(callKey{data.TurnID, value.CallID})
				card.name, card.arguments = value.Name, value.Arguments
				card.summary = toolSummary(value.Name, value.Arguments)
			}
		}
		if data.Response.Failure != nil {
			m.append("Model failed", data.Response.Failure.Message)
		} else if data.Response.Stop == llm.StopRefused && !answered {
			m.append("Agent", "Model refused the request.")
		} else if data.Response.Stop == llm.StopMaxOutputTokens {
			m.append("Response truncated", "Model reached its output limit.")
		}
	case sessionstore.ToolCallStatus:
		card := m.card(callKey{data.TurnID, data.CallID})
		result := m.readToolResult(card, data)
		card.output, card.paths = resultText(result)
		if card.output == "" {
			card.output = boundedDetail(card.failure, 32000)
		}
		if m.detailsOpen && card.key == m.tools[m.selected].key {
			m.refreshDetails()
		}
	}
}

func (m model) padding() (horizontal, vertical int) {
	if m.height < 14 {
		return 0, 0
	}
	return 1, 1
}

func (m model) introTextOffset() int {
	offset := lipgloss.Width(mascot) + 2
	if m.conversation.Width()-3-offset < 24 {
		return 0
	}
	return offset
}

func (m *model) renderConversation() {
	width := m.conversation.Width()
	offset := m.introTextOffset()
	textWidth := max(1, width-3-offset)
	paths := m.labelValue(textWidth, "Workspace", m.workspace) + "\n" + m.labelValue(textWidth, "Run Files", m.directory)
	title := textStyle(m.theme.Accent).Bold(true).Render("Unreal Agent") +
		textStyle(m.theme.Muted).Render(" · "+m.configuration)
	logo := textStyle(m.theme.Accent).MaxWidth(max(1, width-3)).Render(mascot)
	info := lipgloss.NewStyle().Width(textWidth).Render(title + "\n\n" + paths)
	content := logo + "\n\n" + info
	if offset > 0 {
		content = lipgloss.JoinHorizontal(lipgloss.Center, lipgloss.NewStyle().Width(offset).Render(logo), info)
	}
	intro := renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).Width(width).Padding(0, 1, 0, 2), content)
	blocks := []string{intro}
	renderer, renderErr := glamour.NewTermRenderer(glamour.WithStyles(m.theme.markdownStyles()), glamour.WithWordWrap(max(1, width-3)), glamour.WithChromaFormatter("terminal16m"))
	reasoningStyle := m.theme.markdownStyles()
	reasoningStyle.Document.Color = &m.theme.Hint
	reasoningRenderer, reasoningErr := glamour.NewTermRenderer(glamour.WithStyles(reasoningStyle), glamour.WithWordWrap(max(1, width-5)), glamour.WithChromaFormatter("terminal16m"))
	for i := range m.lines {
		entry := &m.lines[i]
		if m.hideReasoning && entry.label == "Reasoning" {
			continue
		}
		color := m.theme.Muted
		style := textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).Width(width).Padding(0, 1, 0, 2)
		switch entry.label {
		case "You":
			style = style.Foreground(lipgloss.Color(m.theme.UserForeground)).Background(lipgloss.Color(m.theme.User)).Padding(1, 1, 1, 2)
		case "Agent":
		case "Run failed", "Model failed", "Send failed", "Skill error":
			color = m.theme.Error
		default:
			style = style.Foreground(lipgloss.Color(m.theme.Muted))
		}
		content := entry.text
		markdown, err := renderer, renderErr
		if entry.label == "Reasoning" {
			markdown, err = reasoningRenderer, reasoningErr
		}
		if (entry.label == "Agent" || entry.label == "Reasoning") && err == nil {
			if entry.width != width {
				entry.rendered = entry.text
				if rendered, err := markdown.Render(entry.text); err == nil {
					entry.rendered = strings.Trim(terminaltext.CleanStyled(rendered), "\n")
				}
				entry.width = width
			}
			content = entry.rendered
		}
		if entry.label != "You" && entry.label != "Agent" && entry.label != "Reasoning" {
			content = textStyle(color).Bold(true).Render(entry.label) + "\n" + content
		}
		if entry.label == "Reasoning" {
			style = style.Foreground(lipgloss.Color(m.theme.Hint))
			content = lipgloss.JoinHorizontal(lipgloss.Top, textStyle(m.theme.Hint).Render("• "), content)
		}
		block := renderSurface(style, content)
		if entry.label == "Reasoning" && i > 0 && m.lines[i-1].label == "Reasoning" {
			blocks[len(blocks)-1] += "\n" + block
		} else {
			if entry.label == "Reasoning" {
				block = renderSurface(style.Bold(true), "Thinking") + "\n" + block
			}
			blocks = append(blocks, block)
		}
	}
	m.conversation.SetContent(strings.Join(blocks, "\n\n"))
	m.syncSelection(false)
}

func (m *model) resize() {
	follow := m.followConversation()
	x, y := m.padding()
	width := max(1, m.width-x)
	inputPadding := 2 * y
	inputWidth := m.width
	if m.theme.Delimiter != "" {
		inputPadding = 2
		inputWidth = width
	}
	m.composer.MaxHeight = max(1, min(8, m.height/3, m.height-7-inputPadding))
	m.composer.SetWidth(max(1, inputWidth-4))
	// Refresh wrapped content before the textarea repositions its cursor.
	m.composer, _ = m.composer.Update(nil)
	chatWidth := width - 1
	if m.theme.Delimiter != "" {
		chatWidth--
	}
	if m.width >= 90 && !m.toolsCollapsed {
		chatWidth -= m.sidebarWidth()
	}
	if chatWidth = max(1, chatWidth); chatWidth != m.conversation.Width() {
		m.selection = textSelection{}
		m.conversation.SetWidth(chatWidth)
		m.renderConversation()
	}
	m.conversation.SetHeight(max(0, m.height-m.composer.Height()-4-inputPadding-m.filePickerHeight()))
	detailWidth := width - 1
	if m.theme.Delimiter != "" {
		detailWidth--
	}
	if detailWidth = max(1, detailWidth); detailWidth != m.details.Width() {
		m.selection = textSelection{}
		m.details.SetWidth(detailWidth)
		m.refreshDetails()
	}
	detailHeight := m.conversation.Height() + 1
	if m.theme.Delimiter != "" {
		detailHeight--
	}
	m.details.SetHeight(detailHeight)
	if follow {
		m.conversation.GotoBottom()
	}
}

func (m model) sidebarWidth() int { return min(42, m.width/3) }

func (m model) heading(label, hint, background, foreground, hintColor string, width int) string {
	innerWidth := max(1, width-4)
	hint = ansi.Truncate(hint, innerWidth, "…")
	labelWidth := innerWidth
	if hint != "" {
		labelWidth = max(0, innerWidth-ansi.StringWidth(hint)-1)
	}
	label = ansi.Truncate(label, labelWidth, "…")
	gap := strings.Repeat(" ", max(0, innerWidth-ansi.StringWidth(label)-ansi.StringWidth(hint)))
	return renderSurface(textStyle(foreground).Background(lipgloss.Color(background)).Width(width).Padding(0, 2),
		textStyle(foreground).Bold(true).Render(label)+gap+textStyle(hintColor).Render(hint))
}

func (m model) framedPane(label, hint, content, background string, width int) string {
	innerWidth := max(0, width-2)
	label = " " + ansi.Truncate(label, max(0, innerWidth-2), "…") + " "
	hint = ansi.Truncate(hint, max(0, innerWidth-ansi.StringWidth(label)-2), "…")
	if hint != "" {
		hint = " " + hint + " "
	}
	border := textStyle(m.theme.Delimiter).Background(lipgloss.Color(background))
	rule := strings.Repeat("═", max(0, innerWidth-ansi.StringWidth(label)-ansi.StringWidth(hint)))
	top := border.Render("╔") + textStyle(m.theme.Accent).Background(lipgloss.Color(background)).Render(label) +
		border.Render(rule+hint+"╗")
	rows := []string{top}
	inside := renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(background)).Width(innerWidth), content)
	for line := range strings.SplitSeq(inside, "\n") {
		rows = append(rows, border.Render("║")+line+border.Render("║"))
	}
	rows = append(rows, border.Render("╚"+strings.Repeat("═", innerWidth)+"╝"))
	return strings.Join(rows, "\n")
}

func (m model) View() tea.View {
	if m.width < 20 || m.height < 8 || m.files.open && m.filePickerHeight() == 0 {
		hint := "Resize terminal"
		if m.files.open {
			hint = "Resize · Esc cancel"
		}
		v := tea.NewView(ansi.Truncate(hint, max(1, m.width), "…"))
		v.AltScreen = true
		v.BackgroundColor, v.ForegroundColor = lipgloss.Color(m.theme.Background), lipgloss.Color(m.theme.Foreground)
		return v
	}
	x, y := m.padding()
	width := m.width - x
	fit := func(s string) string { return ansi.Truncate(s, width-2, "…") }
	statusWidth := m.width - x - 2
	toolsVisible := !m.detailsOpen && (m.width >= 90 && !m.toolsCollapsed || m.toolsFocused)
	indicators := ""
	if !toolsVisible {
		indicators = m.toolIndicators(statusWidth / 2)
	}
	pending := slices.ContainsFunc(m.tools, func(card toolCard) bool { return card.status == awaiting })
	status, color := "Idle", m.theme.StatusForeground
	if pending {
		status = "Waiting for tools"
	}
	if m.responding {
		status, color = "Working · "+m.workTimer.View(), m.theme.StatusAccent
	}
	if m.sending {
		status, color = "Sending message", m.theme.StatusAccent
	}
	status = textStyle(color).Render(status)
	if m.ended {
		status = textStyle(m.theme.StatusForeground).Render("Run ended · Ctrl+C to exit")
		if m.runError != "" {
			status = textStyle(m.theme.StatusError).Render("Run failed · " + m.runError)
		}
	}
	chatWidth := width
	if m.width >= 90 && !m.toolsCollapsed {
		chatWidth -= m.sidebarWidth()
	}
	hint := ""
	if m.toolsCollapsed || m.width < 90 {
		hint = "Tools · Shift+Tab ‹"
	}
	header := m.heading("", hint, m.theme.Background, m.theme.Accent, m.theme.Hint, chatWidth)
	pane := textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).Padding(0, 1)
	conversation := m.selection.render(m.conversation, false, m.theme)
	body := header + "\n" + renderSurface(pane.PaddingLeft(0).PaddingBottom(1), conversation)
	if m.theme.Delimiter != "" {
		body = m.framedPane("Conversation", hint, conversation, m.theme.Background, chatWidth)
	}
	if m.detailsOpen {
		details := m.selection.render(m.details, true, m.theme)
		body = strings.Repeat(" ", width) + "\n" + renderSurface(pane.PaddingLeft(0), details)
		if m.theme.Delimiter != "" {
			body = m.framedPane("Tool details", "Esc back · Tab input", details, m.theme.Background, width)
		}
	} else if m.width >= 90 && !m.toolsCollapsed {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, m.toolPane(m.sidebarWidth(), m.conversation.Height()+2))
	} else if m.toolsFocused {
		body = m.toolPane(width, m.conversation.Height()+2)
	}
	status = ansi.Truncate(status, statusWidth-ansi.StringWidth(indicators)-1, "…")
	status += strings.Repeat(" ", statusWidth-ansi.StringWidth(status)-ansi.StringWidth(indicators)) + indicators
	statusRow := renderSurface(textStyle(m.theme.StatusForeground).Background(lipgloss.Color(m.theme.Status)).
		Width(m.width).Padding(0, 1, 0, x+1), ansi.Truncate(status, m.width-x-2, "…"))
	leftInset := lipgloss.NewStyle().Background(lipgloss.Color(m.theme.Background)).PaddingLeft(x)
	parts := []string{leftInset.Render(body), statusRow}
	if picker := m.filePickerView(width); picker != "" {
		parts = append(parts, leftInset.Render(picker))
	}
	input := renderSurface(textStyle(m.theme.SurfaceForeground).Background(lipgloss.Color(m.theme.Surface)).Width(m.width).Padding(y, 2), m.composer.View())
	if m.theme.Delimiter != "" {
		input = leftInset.Render(m.framedPane("Message", "", renderSurface(textStyle(m.theme.SurfaceForeground).
			Background(lipgloss.Color(m.theme.Surface)).Width(width-2).Padding(0, 1), m.composer.View()), m.theme.Surface, width))
	}
	parts = append(parts, input,
		renderSurface(textStyle(m.theme.SurfaceHint).Background(lipgloss.Color(m.theme.Surface)).Width(m.width).
			Padding(0, 1, 0, x+1), fit(m.helpView(width-2))))
	view := tea.NewView(strings.Join(parts, "\n"))
	view.BackgroundColor = lipgloss.Color(m.theme.Surface)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.KeyboardEnhancements.ReportAlternateKeys = true
	view.WindowTitle = "Unreal Agent"
	if !m.toolsFocused {
		view.Cursor = m.composer.Cursor()
	}
	if view.Cursor != nil {
		origin := m.composerOrigin()
		view.Cursor.X += origin.X
		view.Cursor.Y += origin.Y
	}
	return view
}

func (m model) helpView(width int) string {
	enter, tab := sendKey, focusKey
	mouse := mouseKey
	if m.hasSelection() {
		mouse = copyKey
	}
	reasoning := reasoningKey
	if m.hideReasoning {
		reasoning.SetHelp("F3", "show reasoning")
	}
	bindings := []key.Binding{enter, tab, mouse, reasoning, quitKey, newlineKey, scrollKey}
	if m.files.open {
		selectFile := key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "files"))
		accept := key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("Enter/Tab", "insert"))
		cancel := key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "cancel"))
		bindings = []key.Binding{selectFile, accept, cancel, quitKey}
	}
	if m.toolsFocused {
		enter.SetHelp("Enter", "view")
		tab.SetHelp("Tab", "input")
		bindings = []key.Binding{selectKey, enter, tab, mouse, quitKey}
	}
	if m.detailsOpen {
		bindings = []key.Binding{scrollKey, backKey, tab, mouse, quitKey}
	}
	hints := help.New()
	hints.SetWidth(width)
	hints.ShortSeparator = " · "
	muted := textStyle(m.theme.SurfaceHint)
	hints.Styles.ShortKey, hints.Styles.ShortDesc = textStyle(m.theme.SurfaceKey), muted
	hints.Styles.ShortSeparator, hints.Styles.Ellipsis = muted, muted
	return hints.ShortHelpView(bindings)
}
