package main

import (
	"encoding/json/v2"
	"errors"
	"strings"
	"uuid"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
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
	submissionUncertain
)

func (s submissionState) status() string {
	switch s {
	case submissionSending:
		return "Submitting · not yet persisted"
	case submissionQueued:
		return "Queued · not yet persisted"
	case submissionUnconfirmed:
		return "Run ended · persistence unconfirmed"
	case submissionUncertain:
		return "Delivery uncertain · retry same input ID and payload"
	case submissionFailed:
		return "Send failed · not persisted"
	default:
		return ""
	}
}

func (m *model) startSubmission(text string) tea.Cmd {
	if m.attached {
		if strings.TrimSpace(text) == "/upgrade" {
			return m.startAttachedUpgrade(false)
		}
		if strings.TrimSpace(text) == "/cancel-upgrade" {
			return m.startAttachedUpgrade(true)
		}
		if writer, ok := m.inputs.(*attachedRuntimeWriter); ok && writer.enabled && m.attachConnected {
			switch strings.TrimSpace(text) {
			case "/builds":
				return m.startBuildInspection()
			case "/reload":
				return m.startReload()
			}
		}
		if handled, cmd := m.startAttachedLifecycle(text); handled {
			return cmd
		}
		switch {
		case strings.TrimSpace(text) == "/reconnect":
			return m.startAttachReconnect()
		case strings.TrimSpace(text) == "/retry" || strings.HasPrefix(strings.TrimSpace(text), "/retry "):
			return m.retryAttached(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "/retry")))
		}
	}
	trimmed := strings.TrimSpace(text)
	interventionsAvailable := !m.attached
	if w, ok := m.inputs.(*attachedRuntimeWriter); ok {
		interventionsAvailable = m.attachConnected && w.interventionsEnabled
	}
	if interventionsAvailable {
		if trimmed == "/agents" || strings.HasPrefix(trimmed, "/agents ") {
			return m.startAgentCommand(trimmed)
		}
		if trimmed == "/graph" || strings.HasPrefix(trimmed, "/graph ") {
			return m.startGraphCommand(trimmed)
		}
	}
	if m.attached && (!m.attachConnected || strings.HasPrefix(trimmed, "/")) {
		m.append("Attach", "Host controls and disconnected submissions are unavailable in attach v1. Draft retained.")
		return nil
	}
	if strings.TrimSpace(text) == "/builds" {
		return m.startBuildInspection()
	}
	if strings.TrimSpace(text) == "/reload" {
		return m.startReload()
	}
	if m.attached && !m.canRetainAttached(text) {
		m.append("Send rejected", "Client retry buffer full or input too large; draft retained, nothing sent.")
		return nil
	}
	id := inbox.ID(uuid.New().String())
	kind := inbox.InputExternal
	var body any = text
	if strings.TrimSpace(text) == "/compact" {
		kind = inbox.InputControl
		body = inbox.ControlMessage{Mode: inbox.Compact}
	}
	payload, err := json.Marshal(body)
	if m.attached {
		if err != nil {
			m.append("Send rejected", err.Error()+"; draft retained, nothing sent.")
			return nil
		}
		m.retainAttached(inbox.Input{ID: id, Kind: kind, Payload: payload})
		state := m.captureAttachView()
		state.Draft = ""
		if saveErr := m.saveAttachView(state); saveErr != nil {
			delete(m.attachOutbox, id)
			m.append("Send rejected", "Could not save stable retry identity; draft retained, nothing sent: "+saveErr.Error())
			return nil
		}
	}
	m.composer.Reset()
	if m.pendingSubmissions == nil {
		m.pendingSubmissions = make(map[inbox.ID]struct{})
	}
	m.pendingSubmissions[id] = struct{}{}
	m.sending = true
	m.selection = textSelection{}
	m.dragComposer = false
	m.lines = append(m.lines, message{label: "You", text: terminaltext.Clean(text), inputID: id, submission: submissionSending})
	m.renderConversation()
	m.conversation.GotoBottom()
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
	if m.attached && m.lines[index].submission == submissionQueued && msg.err != nil {
		if !errors.Is(msg.err, attachclient.ErrUncertain) {
			m.append("Retry failed", msg.err.Error()+"; prior durable acceptance remains confirmed.")
		}
		return
	}
	if errors.Is(msg.err, attachclient.ErrUncertain) {
		m.setSubmission(index, submissionUncertain)
		m.append("Delivery uncertain", msg.err.Error()+"\nPress F5 to reconnect, then /retry this input ID. The original payload and identity are retained; do not send a new message to retry.")
	} else if msg.err != nil {
		m.setSubmission(index, submissionFailed)
		m.append("Send failed", msg.err.Error())
		if !m.attached {
			m.composer.SetValue(strings.TrimSpace(msg.text + "\n" + m.composer.Value()))
		}
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
