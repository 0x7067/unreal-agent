package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"
	"time"
	"uuid"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
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

const revealDuration = 250 * time.Millisecond
const revealGlyphs = "01/<>_#"

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
	sendKey    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "send"))
	focusKey   = key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "tools"))
	quitKey    = key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "exit"))
	newlineKey = key.NewBinding(key.WithKeys("shift+enter", "ctrl+j"), key.WithHelp("Shift+Enter/Ctrl+J", "newline"))
	scrollKey  = key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("PgUp/PgDn", "scroll"))
	selectKey  = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "tools"))
	backKey    = key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "tools"))
	mouseKey   = key.NewBinding(key.WithKeys("f2"), key.WithHelp("F2", "select text"))
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
	row, height           int
	created               time.Time
}

type model struct {
	registry                            tool.Registry
	ctx                                 context.Context
	inputs                              inbox.Writer
	composer                            textarea.Model
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
	selectText                          bool
	spinner                             spinner.Model
	workTimer                           stopwatch.Model
	mascotTop                           int
	entranceStarted                     time.Time
	animationRunning                    bool
	animationFrame                      int
	toolPulseFrame                      int
	statusColors                        []color.Color
	toolColors                          []color.Color
	userColors, userTextColors          []color.Color
	responding, sending, ended          bool
	turn                                session.TurnID
}

func newModel(ctx context.Context, inputs inbox.Writer, registry tool.Registry, workspace, directory string, opts options) model {
	composer := textarea.New()
	composer.Prompt = ""
	composer.Placeholder = "Send a message, including while tools are active"
	composer.ShowLineNumbers = false
	composer.SetVirtualCursor(false)
	composer.DynamicHeight = true
	composer.MinHeight = 1
	composer.MaxContentHeight = math.MaxInt
	composer.CharLimit = 0
	composer.MaxWidth = 0
	composer.SetStyles(defaultTheme().composerStyles())
	composer.Focus()
	conversation := viewport.New(viewport.WithWidth(80), viewport.WithHeight(17))
	conversation.FillHeight = true
	loading := spinner.Spinner{Frames: []string{"●"}, FPS: time.Second / 30}
	m := model{ctx: ctx, inputs: inputs, registry: registry, composer: composer, conversation: conversation,
		width: 80, height: 24, spinner: spinner.New(spinner.WithSpinner(loading)), details: viewport.New(),
		theme: defaultTheme(), animationRunning: true,
		workspace: singleLine(workspace), directory: singleLine(directory),
		configuration: singleLine(strings.Join([]string{opts.provider, opts.model, opts.effort}, " · ")),
	}
	m.statusColors = lipgloss.Blend1D(36, lipgloss.Color(m.theme.Status), lipgloss.Color(m.theme.StatusActive), lipgloss.Color(m.theme.Status))
	m.workTimer = stopwatch.New(stopwatch.WithInterval(time.Second))
	m.toolColors = lipgloss.Blend1D(24, lipgloss.Color(m.theme.PendingBright), lipgloss.Color(m.theme.PendingDim), lipgloss.Color(m.theme.PendingBright))
	m.userColors = lipgloss.Blend1D(12, lipgloss.Color(m.theme.Background), lipgloss.Color(m.theme.User))
	m.userTextColors = lipgloss.Blend1D(12, lipgloss.Color(m.theme.Background), lipgloss.Color(m.theme.Foreground))
	m.details.SoftWrap, m.details.FillHeight = true, true
	m.resize()
	return m
}

func (m model) Init() tea.Cmd { return tea.Batch(m.composer.Focus(), m.spinner.Tick) }

func (m *model) mouseViewport(msg tea.MouseWheelMsg) *viewport.Model {
	x, _ := m.padding()
	if m.width < 20 || m.height < 8 || msg.X < x || msg.X >= m.width || msg.Y < 1 {
		return nil
	}
	if m.detailsOpen {
		if msg.Y <= m.details.Height() {
			return &m.details
		}
	} else if (m.width >= 90 || !m.toolsFocused) && msg.X < x+m.conversation.Width() && msg.Y <= m.conversation.Height() {
		return &m.conversation
	}
	return nil
}

// Bubble Tea calls View even after a no-op Update; discard wheel events at the edges before rendering.
func filterMouseWheel(current tea.Model, msg tea.Msg) tea.Msg {
	wheel, ok := msg.(tea.MouseWheelMsg)
	if !ok {
		return msg
	}
	m, ok := current.(model)
	if !ok {
		return msg
	}
	target := m.mouseViewport(wheel)
	if target == nil {
		return nil
	}
	if !wheel.Mod.Contains(tea.ModShift) &&
		(wheel.Button == tea.MouseWheelDown && target.AtBottom() || wheel.Button == tea.MouseWheelUp && target.AtTop()) {
		return nil
	}
	return msg
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg, tea.MouseWheelMsg:
		m.entranceStarted = time.Time{}
	}
	switch msg := msg.(type) {
	case stopwatch.TickMsg, stopwatch.StartStopMsg:
		if m.ended || !m.responding {
			return m, nil
		}
		m.workTimer, cmd = m.workTimer.Update(msg)
		return m, cmd
	case spinner.TickMsg:
		if !m.needsAnimation() {
			m.animationRunning = false
			return m, nil
		}
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			m.animationFrame = (m.animationFrame + 1) % len(m.statusColors)
			m.toolPulseFrame = (m.toolPulseFrame + 1) % len(m.toolColors)
		}
		return m, cmd
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			follow := m.conversation.AtBottom()
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
		if !m.toolsFocused {
			m.composer, cmd = m.composer.Update(msg)
		}
	case tea.MouseWheelMsg:
		if m.selectText {
			return m, nil
		}
		if target := m.mouseViewport(msg); target != nil {
			*target, cmd = target.Update(msg)
		}
		return m, cmd
	case tea.KeyPressMsg:
		if key.Matches(msg, mouseKey) {
			m.selectText = !m.selectText
			return m, nil
		}
		if msg.String() == "shift+tab" {
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
			m.resize()
			return m, cmd
		}
		if m.detailsOpen && !key.Matches(msg, quitKey) {
			switch {
			case key.Matches(msg, backKey):
				m.detailsOpen = false
			case key.Matches(msg, focusKey):
				m.detailsOpen, m.toolsFocused = false, false
				cmd = m.composer.Focus()
			case msg.String() == "ctrl+end":
				m.details.GotoBottom()
			default:
				m.details, cmd = m.details.Update(msg)
			}
			m.resize()
			return m, cmd
		}
		if key.Matches(msg, focusKey) || key.Matches(msg, backKey) && m.toolsFocused {
			m.toolsFocused = !m.toolsFocused
			if m.toolsFocused {
				m.selected = 0
				m.toolsCollapsed = false
				m.composer.Blur()
			} else {
				cmd = m.composer.Focus()
			}
			m.resize()
			return m, cmd
		}
		if m.toolsFocused && !key.Matches(msg, quitKey) {
			switch {
			case key.Matches(msg, sendKey):
				if len(m.tools) > 0 {
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
			m.composer, cmd = m.composer.Update(msg)
		}
	default:
		m.composer, cmd = m.composer.Update(msg)
	}
	m.resize()
	if !m.animationRunning && m.needsAnimation() {
		m.animationRunning = true
		cmd = tea.Batch(cmd, m.spinner.Tick)
	}
	return m, cmd
}

func (m model) needsAnimation() bool {
	now := time.Now()
	if !m.ended && (m.responding || m.sending || now.Sub(m.entranceStarted) < entranceDuration ||
		len(m.lines) > 0 && now.Sub(m.lines[len(m.lines)-1].created) < revealDuration) {
		return true
	}
	return slices.ContainsFunc(m.tools, func(card toolCard) bool {
		return now.Sub(card.completed) < toolStatusLinger ||
			!m.ended && (card.status == awaiting || now.Sub(card.created) < revealDuration)
	})
}

func (m *model) append(label, text string) {
	text = terminaltext.Clean(text)
	if label == "Agent" && strings.TrimSpace(text) == "" {
		return
	}
	follow := m.conversation.AtBottom()
	m.lines = append(m.lines, message{label: terminaltext.Clean(label), text: text, created: time.Now()})
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
		lineCount := len(m.lines)
		if data.TurnID == m.turn {
			m.responding = false
		}
		for _, output := range data.Response.Output {
			switch value := output.Data.(type) {
			case llm.Message:
				m.append("Agent", value.Text)
			case llm.ToolCall:
				card := m.card(callKey{data.TurnID, value.CallID})
				card.name, card.arguments = value.Name, value.Arguments
				card.summary = toolSummary(value.Name, value.Arguments)
			}
		}
		if data.Response.Failure != nil {
			m.append("Model failed", data.Response.Failure.Message)
		} else if data.Response.Stop == llm.StopRefused && len(m.lines) == lineCount {
			m.append("Agent", "Model refused the request.")
		} else if data.Response.Stop == llm.StopMaxOutputTokens {
			m.append("Response truncated", "Model reached its output limit.")
		}
	case sessionstore.ToolCallStatus:
		card := m.card(callKey{data.TurnID, data.CallID})
		wasAwaiting := card.status == awaiting
		result := m.readToolResult(card, data)
		if wasAwaiting && card.status != awaiting {
			card.completed = time.Now()
		}
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
	m.mascotTop = 0
	if offset > 0 {
		m.mascotTop = max(0, (lipgloss.Height(info)-lipgloss.Height(logo)+1)/2)
		content = lipgloss.JoinHorizontal(lipgloss.Center, lipgloss.NewStyle().Width(offset).Render(logo), info)
	}
	intro := renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).Width(width).Padding(0, 1, 0, 2), content)
	blocks := []string{intro}
	row := lipgloss.Height(intro) + 1
	renderer, renderErr := glamour.NewTermRenderer(glamour.WithStyles(m.theme.markdownStyles()), glamour.WithWordWrap(max(1, width-3)))
	for i := range m.lines {
		entry := &m.lines[i]
		color := m.theme.Muted
		style := textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).Width(width).Padding(0, 1, 0, 2)
		switch entry.label {
		case "You":
			style = style.Background(lipgloss.Color(m.theme.User)).Padding(1, 1, 1, 2)
		case "Agent":
		case "Run failed", "Model failed", "Send failed":
			color = m.theme.Error
		default:
			style = style.Foreground(lipgloss.Color(m.theme.Muted))
		}
		content := entry.text
		if entry.label == "Agent" && renderErr == nil {
			if entry.width != width {
				entry.rendered = entry.text
				if rendered, err := renderer.Render(entry.text); err == nil {
					entry.rendered = strings.Trim(terminaltext.CleanStyled(rendered), "\n")
				}
				entry.width = width
			}
			content = entry.rendered
		}
		if entry.label != "You" && entry.label != "Agent" {
			content = textStyle(color).Bold(true).Render(entry.label) + "\n" + content
		}
		block := renderSurface(style, content)
		entry.row, entry.height = row, lipgloss.Height(block)
		row += entry.height + 1
		blocks = append(blocks, block)
	}
	m.conversation.SetContent(strings.Join(blocks, "\n\n"))
}

func (m model) conversationView(now time.Time) string {
	view := m.conversation.View()
	if m.ended || m.detailsOpen || m.width < 90 && m.toolsFocused {
		return view
	}
	width, height := m.conversation.Width(), m.conversation.Height()
	var canvas *lipgloss.Canvas
	for _, entry := range slices.Backward(m.lines) {
		elapsed := now.Sub(entry.created)
		if elapsed >= revealDuration {
			break
		}
		top := entry.row - m.conversation.YOffset()
		if entry.label != "You" && entry.label != "Agent" || top >= height || top+entry.height <= 0 {
			continue
		}
		if canvas == nil {
			canvas = lipgloss.NewCanvas(width, height).Compose(lipgloss.NewLayer(view))
		}
		front := float64(max(0, elapsed)) / float64(revealDuration) * 1.15
		for row := max(0, top); row < min(height, top+entry.height); row++ {
			for column := range width {
				position := (float64(column)/float64(max(1, width-1)) + float64(row-top)/float64(max(1, entry.height-1))) / 2
				cell := canvas.CellAt(column, row)
				if entry.label == "Agent" {
					if front <= position {
						cell.Style.Fg = cell.Style.Bg
						if cell.Style.Fg == nil {
							cell.Style.Fg = lipgloss.Color(m.theme.Background)
						}
					}
					continue
				}
				shade := max(0, min(len(m.userColors)-1, int((front-position)/0.15*float64(len(m.userColors)-1))))
				cell.Style.Bg = m.userColors[shade]
				cell.Style.Fg = m.userTextColors[shade]
			}
		}
	}
	if canvas != nil {
		return canvas.Render()
	}
	return view
}

func (m *model) resize() {
	follow := m.conversation.AtBottom()
	x, y := m.padding()
	width := max(1, m.width-x)
	m.composer.MaxHeight = max(1, min(8, m.height/3, m.height-7-2*y))
	m.composer.SetWidth(max(1, m.width-4))
	// Refresh wrapped content before the textarea repositions its cursor.
	m.composer, _ = m.composer.Update(nil)
	chatWidth := width - 1
	if m.width >= 90 && !m.toolsCollapsed {
		chatWidth -= m.sidebarWidth()
	}
	if chatWidth = max(1, chatWidth); chatWidth != m.conversation.Width() {
		m.conversation.SetWidth(chatWidth)
		m.renderConversation()
	}
	m.conversation.SetHeight(max(0, m.height-m.composer.Height()-4-2*y))
	if detailWidth := max(1, width-1); detailWidth != m.details.Width() {
		m.details.SetWidth(detailWidth)
		m.refreshDetails()
	}
	m.details.SetHeight(m.conversation.Height() + 1)
	if follow {
		m.conversation.GotoBottom()
	}
}

func (m model) sidebarWidth() int { return min(42, m.width/3) }

func (m model) heading(label, hint, background string, width int) string {
	innerWidth := max(1, width-4)
	hint = ansi.Truncate(hint, innerWidth, "…")
	labelWidth := innerWidth
	if hint != "" {
		labelWidth = max(0, innerWidth-ansi.StringWidth(hint)-1)
	}
	label = ansi.Truncate(label, labelWidth, "…")
	gap := strings.Repeat(" ", max(0, innerWidth-ansi.StringWidth(label)-ansi.StringWidth(hint)))
	return renderSurface(textStyle(m.theme.Accent).Background(lipgloss.Color(background)).Width(width).Padding(0, 2),
		textStyle(m.theme.Accent).Bold(true).Render(label)+gap+textStyle(m.theme.Hint).Render(hint))
}

func (m model) View() tea.View {
	if m.width < 20 || m.height < 8 {
		v := tea.NewView(ansi.Truncate("Resize terminal", max(1, m.width), "…"))
		v.AltScreen = true
		return v
	}
	x, y := m.padding()
	width := m.width - x
	fit := func(s string) string { return ansi.Truncate(s, width-2, "…") }
	pending := slices.ContainsFunc(m.tools, func(card toolCard) bool { return card.status == awaiting })
	status, color := "Idle", m.theme.Muted
	if pending {
		status = "Waiting for tools"
	}
	if m.responding {
		status, color = fmt.Sprintf("Working%-3s · %s", strings.Repeat(".", 1+m.animationFrame/3%3), m.workTimer.View()), m.theme.Accent
	}
	if m.sending {
		status, color = "Sending message", m.theme.Accent
	}
	status = textStyle(color).Render(status)
	if m.ended {
		status = textStyle(m.theme.Muted).Render("Run ended · Ctrl+C to exit")
		if m.runError != "" {
			status = textStyle(m.theme.Error).Render("Run failed · " + m.runError)
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
	header := m.heading("", hint, m.theme.Background, chatWidth)
	pane := textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Background)).Padding(0, 1)
	body := header + "\n" + renderSurface(pane.PaddingLeft(0).PaddingBottom(1), m.conversationView(time.Now()))
	if m.detailsOpen {
		body = strings.Repeat(" ", width) + "\n" + renderSurface(pane.PaddingLeft(0), m.details.View())
	} else if m.width >= 90 && !m.toolsCollapsed {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, m.toolPane(m.sidebarWidth(), m.conversation.Height()+2))
	} else if m.toolsFocused {
		body = m.toolPane(width, m.conversation.Height()+2)
	}
	statusWidth := m.width - x - 2
	indicators := m.toolIndicators(statusWidth / 2)
	status = ansi.Truncate(status, statusWidth-ansi.StringWidth(indicators)-1, "…")
	status += strings.Repeat(" ", statusWidth-ansi.StringWidth(status)-ansi.StringWidth(indicators)) + indicators
	statusRow := renderSurface(textStyle(m.theme.Muted).Background(lipgloss.Color(m.theme.Status)).
		Width(m.width).Padding(0, 1, 0, x+1), ansi.Truncate(status, m.width-x-2, "…"))
	if !m.ended && (m.responding || m.sending || pending) {
		canvas := lipgloss.NewCanvas(m.width, 1).Compose(lipgloss.NewLayer(statusRow))
		for column := range m.width {
			phase := (column*len(m.statusColors)/m.width - m.animationFrame + len(m.statusColors)) % len(m.statusColors)
			canvas.CellAt(column, 0).Style.Bg = m.statusColors[phase]
		}
		statusRow = canvas.Render()
	}
	leftInset := lipgloss.NewStyle().Background(lipgloss.Color(m.theme.Background)).PaddingLeft(x)
	parts := []string{leftInset.Render(body), statusRow}
	input := renderSurface(textStyle(m.theme.Foreground).Background(lipgloss.Color(m.theme.Surface)).Width(m.width).Padding(y, 2), m.composer.View())
	parts = append(parts, input,
		renderSurface(textStyle(m.theme.Hint).Background(lipgloss.Color(m.theme.Surface)).Width(m.width).
			Padding(0, 1, 0, x+1), fit(m.helpView(width-2))))
	view := tea.NewView(strings.Join(parts, "\n"))
	view.BackgroundColor = lipgloss.Color(m.theme.Surface)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	if m.selectText {
		view.MouseMode = tea.MouseModeNone
	}
	view.WindowTitle = "Unreal Agent Lite"
	if !m.toolsFocused {
		view.Cursor = m.composer.Cursor()
	}
	if view.Cursor != nil {
		view.Cursor.X += 2
		view.Cursor.Y += 3 + y + m.conversation.Height()
	}
	return m.entranceView(view, time.Now())
}

func (m model) helpView(width int) string {
	enter, tab := sendKey, focusKey
	mouse := mouseKey
	if m.selectText {
		mouse.SetHelp("F2", "mouse scroll")
	}
	bindings := []key.Binding{enter, tab, mouse, quitKey, newlineKey, scrollKey}
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
	muted := textStyle(m.theme.Hint)
	hints.Styles.ShortKey, hints.Styles.ShortDesc = muted, muted
	hints.Styles.ShortSeparator, hints.Styles.Ellipsis = muted, muted
	return hints.ShortHelpView(bindings)
}
