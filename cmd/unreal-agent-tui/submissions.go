package main

import (
	"encoding/json/v2"
	"strings"
	"uuid"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

// Submit acknowledges ingress (durable for workerhost, in-memory for legacy
// writers), not session persistence. Only a typed session-store item is proof
// of session persistence; neither acknowledgment implies model delivery.
type submissionState uint8

const (
	submissionPersisted submissionState = iota
	submissionSending
	submissionQueued
	submissionUnconfirmed
	submissionFailed
)

func (s submissionState) status() string {
	switch s {
	case submissionSending:
		return "Submitting · not yet persisted"
	case submissionQueued:
		return "Queued · not yet persisted"
	case submissionUnconfirmed:
		return "Run ended · persistence unconfirmed"
	case submissionFailed:
		return "Send failed · not persisted"
	default:
		return ""
	}
}

func (m *model) startSubmission(text string) tea.Cmd {
	// Defense in depth for callers other than the Enter handler: this local
	// command must never become model input, durable ingress, or session history.
	if trimmed := strings.TrimSpace(text); trimmed == "/agents" || strings.HasPrefix(trimmed, "/agents ") {
		return m.startAgentCommand(trimmed)
	}
	if strings.TrimSpace(text) == "/builds" {
		return m.startBuildInspection()
	}
	if strings.TrimSpace(text) == "/reload" {
		return m.startReload()
	}
	id := inbox.ID(uuid.New().String())
	m.composer.Reset()
	if m.pendingSubmissions == nil {
		m.pendingSubmissions = make(map[inbox.ID]struct{})
	}
	m.pendingSubmissions[id] = struct{}{}
	m.sending = len(m.pendingSubmissions) > 0
	// Explicit user action reveals the local echo, unlike background appends.
	m.selection = textSelection{}
	m.dragComposer = false
	m.lines = append(m.lines, message{label: "You", text: terminaltext.Clean(text), inputID: id, submission: submissionSending})
	m.renderConversation()
	m.conversation.GotoBottom()
	// Admission order is fixed here on the UI goroutine, before Bubble Tea
	// schedules any result waiter. Snapshot the writer too: tests and callers
	// may replace m.inputs while earlier records are still pending.
	kind := inbox.InputExternal
	var body any = text
	if strings.TrimSpace(text) == "/compact" {
		kind = inbox.InputControl
		body = inbox.ControlMessage{Mode: inbox.Compact}
	}
	payload, err := json.Marshal(body)
	if m.submissionQueue == nil {
		m.submissionQueue = newSubmissionQueue(m.ctx)
	}
	result := m.submissionQueue.enqueue(inbox.Input{ID: id, Kind: kind, Payload: payload}, text, m.inputs, err)
	return func() tea.Msg { return <-result }
}

func (m *model) submissionIndex(id inbox.ID) int {
	if id != "" {
		for i := range m.lines {
			if m.lines[i].inputID == id {
				return i
			}
		}
	}
	return -1
}

func (m *model) setSubmission(index int, state submissionState) {
	if m.lines[index].submission == state {
		return
	}
	follow := m.followConversation()
	m.lines[index].submission, m.lines[index].width = state, 0
	m.renderConversation()
	if follow {
		m.conversation.GotoBottom()
	}
}

func (m *model) finishSubmission(msg submitted) {
	delete(m.pendingSubmissions, msg.id)
	m.sending = len(m.pendingSubmissions) > 0
	index := m.submissionIndex(msg.id)
	if index < 0 {
		return
	}
	// Persistence may be observed before a result, but must never be
	// downgraded by a late success or error for that ID.
	if m.lines[index].submission == submissionPersisted || m.lines[index].submission == submissionFailed {
		return
	}
	if msg.err != nil {
		m.setSubmission(index, submissionFailed)
		m.append("Send failed", msg.err.Error())
		m.composer.SetValue(strings.TrimSpace(msg.text + "\n" + m.composer.Value()))
	} else if !m.ended {
		m.setSubmission(index, submissionQueued)
	}
}

func (m *model) endSubmissions() {
	clear(m.pendingSubmissions)
	m.sending = false
	for i := range m.lines {
		if m.lines[i].submission == submissionSending || m.lines[i].submission == submissionQueued {
			m.setSubmission(i, submissionUnconfirmed)
		}
	}
}

func (m *model) applyInput(input inbox.Input) {
	if input.Kind != inbox.InputExternal && input.Kind != inbox.InputControl {
		return
	}
	if index := m.submissionIndex(input.ID); index >= 0 {
		m.setSubmission(index, submissionPersisted)
		return
	}
	if input.Kind != inbox.InputExternal {
		return
	}
	var text string
	if json.Unmarshal(input.Payload, &text) != nil {
		return
	}
	label := "You"
	if strings.HasPrefix(text, subagentMessagePrefix) {
		label = "Subagent"
	}
	before := len(m.lines)
	m.append(label, text)
	// append may filter an empty entry. Never correlate a skipped input with
	// an earlier message (or index an empty transcript).
	if len(m.lines) > before {
		m.lines[before].inputID = input.ID
	}
}
