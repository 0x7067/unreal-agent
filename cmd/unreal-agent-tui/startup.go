package main

import (
	"context"
	"errors"
	"io"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning"
	"github.com/unreallabsai/unreal-agent/harness/settings"
)

const launchExamples = `Sign in with Codex, then run without flags:
  codex -c 'cli_auth_credentials_store="file"' login
  unreal-agent

Or choose a provider explicitly:
  OPENAI_API_KEY=YOUR_API_KEY unreal-agent -provider openai -model gpt-6.1-sol
  unreal-agent -provider ollama -model qwen3.8:27b`

type subscriptionDialog struct {
	opts          options
	models        []string
	focus         int
	width, height int
	use, accepted bool
}

const (
	subscriptionModel = iota
	subscriptionEffort
	subscriptionActions
)

func newSubscriptionDialog(opts options, configured settings.Settings) subscriptionDialog {
	models := []string{opts.model}
	for _, model := range configured.Providers["openai-codex"].Models {
		if !slices.Contains(models, model.ID) {
			models = append(models, model.ID)
		}
	}
	return subscriptionDialog{opts: opts, models: models, width: 80, height: 24, use: true}
}

func confirmSubscription(ctx context.Context, opts options, configured settings.Settings, input io.Reader, output io.Writer) (options, bool, error) {
	dialog := newSubscriptionDialog(opts, configured)
	final, err := tea.NewProgram(dialog, tea.WithContext(ctx), tea.WithoutSignalHandler(),
		tea.WithInput(input), tea.WithOutput(output)).Run()
	if ctx.Err() != nil || errors.Is(err, tea.ErrProgramKilled) {
		err = nil
	}
	if err != nil {
		return opts, false, err
	}
	accepted := false
	if final != nil && ctx.Err() == nil {
		chosen := final.(subscriptionDialog)
		opts, accepted = chosen.opts, chosen.accepted
	}
	return opts, accepted, nil
}

func (m subscriptionDialog) Init() tea.Cmd { return nil }

func (m subscriptionDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			m.width, m.height = msg.Width, msg.Height
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab", "down":
			m.focus = (m.focus + 1) % 3
		case "shift+tab", "up":
			m.focus = (m.focus + 2) % 3
		case "left", "right":
			step := 1
			if msg.String() == "left" {
				step = -1
			}
			switch m.focus {
			case subscriptionModel:
				index := slices.Index(m.models, m.opts.model)
				m.opts.model = m.models[(index+step+len(m.models))%len(m.models)]
			case subscriptionEffort:
				efforts := reasoning.Choices()
				index := slices.Index(efforts, m.opts.effort)
				m.opts.effort = efforts[(index+step+len(efforts))%len(efforts)]
			case subscriptionActions:
				m.use = msg.String() == "left"
			}
		case "y", "Y":
			m.accepted = true
			return m, tea.Quit
		case "enter":
			m.accepted = m.use
			return m, tea.Quit
		case "n", "N", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m subscriptionDialog) View() tea.View {
	palette := m.opts.theme
	innerWidth := max(1, min(64, m.width-4)-6)
	selector := func(label, value string, focused bool) string {
		style := textStyle(palette.SurfaceForeground).Background(lipgloss.Color(palette.Surface))
		if focused {
			style = style.Foreground(lipgloss.Color(palette.SelectionForeground)).Background(lipgloss.Color(palette.Selection))
		}
		value = ansi.Truncate("‹ "+singleLine(value)+" ›", max(1, innerWidth-11), "…")
		return label + renderSurface(style, value)
	}
	button := func(label string, selected bool) string {
		style := textStyle(palette.SurfaceForeground).Background(lipgloss.Color(palette.Surface)).Padding(0, 1)
		if selected && m.focus == subscriptionActions {
			style = style.Foreground(lipgloss.Color(palette.SelectionForeground)).Background(lipgloss.Color(palette.Selection))
		}
		return renderSurface(style, label)
	}
	content := textStyle(palette.SurfaceAccent).Bold(true).Render("Codex subscription available") +
		"\n\nUse your existing Codex login?\n\n" + selector("Model:     ", m.opts.model, m.focus == subscriptionModel) +
		"\n" + selector("Reasoning: ", m.opts.effort, m.focus == subscriptionEffort) +
		"\n\nYour choice will be remembered for future launches.\n\n" + button("Yes", m.use) + "  " + button("No", !m.use) +
		"\n\n" + textStyle(palette.SurfaceHint).Render("Tab field · ←/→ change · Enter confirm · Esc cancel")
	borderColor := palette.Delimiter
	if borderColor == "" {
		borderColor = palette.SurfaceHint
	}
	panel := renderSurface(textStyle(palette.SurfaceForeground).Background(lipgloss.Color(palette.Surface)).
		Width(max(1, min(64, m.width-4))).Padding(1, 2).Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(borderColor)).BorderBackground(lipgloss.Color(palette.Surface)), content)
	if lipgloss.Width(panel) > m.width || lipgloss.Height(panel) > m.height {
		panel = textStyle(palette.Foreground).Render("Resize terminal")
	}
	view := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(lipgloss.Color(palette.Background)))))
	view.AltScreen = true
	view.BackgroundColor = lipgloss.Color(palette.Background)
	view.WindowTitle = "Unreal Agent"
	return view
}
