package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"
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
	composer                   textarea.Model
	conversation               viewport.Model
	width, height              int
	header                     string
	lines                      []string
	active                     map[callKey]string
	registry                   tool.Registry
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
		width: 80, height: 24, active: make(map[callKey]string),
		header: "Unreal Agent Lite · " + opts.model + " · " + opts.effort,
	}
	m.append("Workspace", workspace)
	m.append("Run files", directory)
	m.resize()
	return m
}

func (m model) Init() tea.Cmd { return m.composer.Focus() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
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
		if msg.err != nil {
			m.append("Run failed", msg.err.Error())
		}
	case tea.KeyPressMsg:
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
		for _, item := range data.Response.Output {
			switch value := item.Data.(type) {
			case llm.Message:
				m.append("Agent", value.Text)
			case llm.ToolCall:
				m.active[callKey{data.TurnID, value.CallID}] = value.Name
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
		key := callKey{data.TurnID, data.CallID}
		name, pending := m.active[key]
		if !pending {
			return
		}
		translator, ok := m.registry.Resolve(name)
		if !ok {
			delete(m.active, key)
			m.append("Tool failed", "Unknown tool: "+name)
			return
		}
		result, err := translator.TranslateResult(data.CallID, data.Status, data.Operations)
		active, failure := false, ""
		if err != nil {
			failure = err.Error()
		} else {
			active, failure = resultState(result)
		}
		if !active {
			delete(m.active, key)
		}
		if failure != "" {
			m.append("Tool failed", failure)
		}
	}
}

func (m *model) resize() {
	m.composer.SetWidth(max(1, m.width))
	m.composer.SetHeight(min(3, max(1, m.height-3)))
	m.conversation.SetWidth(max(1, m.width))
	m.conversation.SetHeight(max(0, m.height-m.composer.Height()-3))
}

func (m model) View() tea.View {
	status := "Idle"
	if m.responding {
		status = "Agent responding"
	}
	if m.sending {
		status = "Sending message"
	}
	status += fmt.Sprintf(" · %d tools awaiting results", len(m.active))
	if m.ended {
		status = "Run ended · Ctrl+C to exit"
	}
	fit := func(s string) string { return ansi.Truncate(terminaltext.Clean(s), max(1, m.width), "…") }
	header := lipgloss.NewStyle().Bold(true).Render(fit(m.header))
	parts := []string{header}
	if m.conversation.Height() > 0 {
		parts = append(parts, m.conversation.View())
	}
	parts = append(parts, fit(status), m.composer.View(), fit("Enter send · Ctrl+J newline · PgUp/PgDn scroll · Ctrl+End latest · Ctrl+C stop & exit"))
	view := tea.NewView(strings.Join(parts, "\n"))
	view.AltScreen = true
	view.WindowTitle = "Unreal Agent Lite"
	view.Cursor = m.composer.Cursor()
	if view.Cursor != nil {
		view.Cursor.Y += 2 + m.conversation.Height()
	}
	return view
}
