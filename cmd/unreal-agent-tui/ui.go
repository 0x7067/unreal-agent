package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"
	"time"
	"uuid"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
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
type tick time.Time

func tickCommand() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return tick(now) })
}

type submitted struct {
	text string
	err  error
}
type callKey struct {
	turn session.TurnID
	call string
}

type model struct {
	ctx                        context.Context
	inputs                     inbox.Writer
	registry                   tool.Registry
	composer                   textarea.Model
	conversation               viewport.Model
	width, height              int
	header                     string
	lines                      []string
	tools                      []toolCard
	selected                   int
	toolsFocused               bool
	now                        time.Time
	responding, sending, ended bool
	turn                       session.TurnID
}

func newModel(ctx context.Context, inputs inbox.Writer, registry tool.Registry, opts options, workspace, directory string) model {
	composer := textarea.New()
	composer.Prompt = "> "
	composer.Placeholder = "Send a message, including while tools are active"
	composer.ShowLineNumbers = false
	composer.SetVirtualCursor(false)
	composer.SetHeight(3)
	composer.CharLimit = 0
	composer.MaxWidth = 0
	composer.Focus()
	conversation := viewport.New(viewport.WithWidth(80), viewport.WithHeight(17))
	conversation.SoftWrap, conversation.FillHeight = true, true
	m := model{ctx: ctx, inputs: inputs, registry: registry, composer: composer, conversation: conversation,
		width: 80, height: 24, now: time.Now(),
		header: "Unreal Agent Lite · " + opts.model + " · " + opts.effort,
	}
	m.append("Workspace", workspace)
	m.append("Run files", directory)
	m.resize()
	return m
}

func (m model) Init() tea.Cmd { return tea.Batch(m.composer.Focus(), tickCommand()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tick:
		if m.ended {
			break
		}
		m.now = time.Time(msg)
		cmd = tickCommand()
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
		m.apply(msg)
	case submitted:
		m.sending = false
		if msg.err != nil {
			m.append("Send failed", msg.err.Error())
			m.composer.SetValue(strings.TrimSpace(msg.text + "\n" + m.composer.Value()))
		}
	case runEnded:
		m.ended, m.responding = true, false
		m.now = time.Now()
		if msg.err != nil {
			m.append("Run failed", msg.err.Error())
		}
	case tea.PasteMsg:
		if !m.toolsFocused {
			m.composer, cmd = m.composer.Update(msg)
		}
	case tea.KeyPressMsg:
		if msg.String() == "tab" || msg.String() == "esc" && m.toolsFocused {
			m.toolsFocused = !m.toolsFocused
			if m.toolsFocused {
				m.composer.Blur()
			} else {
				cmd = m.composer.Focus()
			}
			m.resize()
			return m, cmd
		}
		if m.toolsFocused && msg.String() != "ctrl+c" {
			switch msg.String() {
			case "up":
				m.selected = max(0, m.selected-1)
			case "down":
				m.selected = min(max(0, len(m.tools)-1), m.selected+1)
			case "pgup":
				m.selected = max(0, m.selected-max(1, (m.conversation.Height()-1)/3))
			case "pgdown":
				m.selected = min(max(0, len(m.tools)-1), m.selected+max(1, (m.conversation.Height()-1)/3))
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
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
		case "shift+enter", "ctrl+j":
			m.composer.InsertString("\n")
		case "pgup", "pgdown":
			m.conversation, cmd = m.conversation.Update(msg)
		case "ctrl+end":
			m.conversation.GotoBottom()
		default:
			m.composer, cmd = m.composer.Update(msg)
		}
	default:
		m.composer, cmd = m.composer.Update(msg)
	}
	return m, cmd
}

func (m *model) append(label, text string) {
	text = terminaltext.Clean(text)
	if label == "Agent" && strings.TrimSpace(text) == "" {
		return
	}
	follow := m.conversation.AtBottom()
	m.lines = append(m.lines, terminaltext.Clean(label)+"\n"+text)
	m.conversation.SetContent(strings.Join(m.lines, "\n\n"))
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
				card.started = item.RecordedAt
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
		m.readToolResult(card, data)
		if card.status != awaiting && card.finished.IsZero() {
			card.finished = item.RecordedAt
		}

	}
}

func (m *model) resize() {
	m.composer.SetWidth(max(1, m.width))
	m.composer.SetHeight(min(3, max(1, m.height-3)))
	width := m.width
	if m.width >= 90 {
		width -= m.sidebarWidth() + 1
	}
	m.conversation.SetWidth(max(1, width))
	m.conversation.SetHeight(max(0, m.height-m.composer.Height()-3))
}

func (m model) sidebarWidth() int { return min(42, m.width/3) }

func (m model) View() tea.View {
	if m.width < 20 || m.height < 8 {
		v := tea.NewView(ansi.Truncate("Resize terminal", max(1, m.width), "…"))
		v.AltScreen = true
		return v
	}
	status := "Idle"
	if m.responding {
		status = "Agent responding"
	}
	if m.sending {
		status = "Sending message"
	}
	active, failed := m.toolCounts()
	status += fmt.Sprintf(" · %d tools awaiting results", active)
	if failed > 0 {
		status += fmt.Sprintf(" · %d failed", failed)
	}
	if m.ended {
		status = "Run ended · Ctrl+C to exit"
	}
	fit := func(s string) string { return ansi.Truncate(terminaltext.Clean(s), max(1, m.width), "…") }
	header := lipgloss.NewStyle().Bold(true).Render(fit(m.header))
	parts := []string{header}
	if m.conversation.Height() > 0 {
		body := m.conversation.View()
		if m.width >= 90 {
			side := lipgloss.NewStyle().BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).Render(m.toolPane(m.sidebarWidth(), m.conversation.Height()))
			body = lipgloss.JoinHorizontal(lipgloss.Top, body, side)
		} else if m.toolsFocused {
			body = m.toolPane(m.width, m.conversation.Height())
		}
		parts = append(parts, body)
	}
	parts = append(parts, fit(status), m.composer.View(), fit("Enter send · Ctrl+J newline · Tab tools · PgUp/PgDn scroll · Ctrl+C exit"))
	view := tea.NewView(strings.Join(parts, "\n"))
	view.AltScreen = true
	view.WindowTitle = "Unreal Agent Lite"
	if !m.toolsFocused {
		view.Cursor = m.composer.Cursor()
	}
	if view.Cursor != nil {
		view.Cursor.Y += 2 + m.conversation.Height()
	}
	return view
}
