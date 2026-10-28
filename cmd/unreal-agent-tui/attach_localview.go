package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json/v2"
	"fmt"
	"image"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

type attachSaveTick struct{ epoch uint64 }

func attachSaveTimer(epoch uint64) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return attachSaveTick{epoch} })
}
func toolViewID(key callKey) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(string(key.turn)+"\x00"+key.call)))
}
func selectionOffset(lines []string, p image.Point) int {
	n := 0
	for i, line := range lines {
		if i == p.Y {
			return n + min(max(0, p.X), ansi.StringWidth(line))
		}
		n += ansi.StringWidth(line) + 1
	}
	return n
}
func selectionPoint(lines []string, n int) image.Point {
	for i, line := range lines {
		width := ansi.StringWidth(line)
		if n <= width {
			return image.Pt(n, i)
		}
		n -= width + 1
	}
	if len(lines) > 0 {
		return image.Pt(ansi.StringWidth(lines[len(lines)-1]), len(lines)-1)
	}
	return image.Point{}
}
func (m *model) captureAttachView() attachLocalState {
	state := attachLocalState{Version: attachLocalStateVersion, Namespace: m.attachNamespace, Draft: m.composer.Value(), ConversationOffset: m.conversation.YOffset(), ToolsOffset: m.toolsOffset, DetailsOffset: m.details.YOffset(), ToolsFocused: m.toolsFocused, ToolsCollapsed: m.toolsCollapsed}
	state.ConversationAnchor = scrollAnchor(m.conversation.GetContent(), state.ConversationOffset)
	if m.selected >= 0 && m.selected < len(m.tools) {
		state.SelectedCallID = toolViewID(m.tools[m.selected].key)
		if m.detailsOpen {
			state.DetailsCallID = state.SelectedCallID
		}
	}
	if len(m.selection.lines) > 0 {
		state.SelectionPane = "conversation"
		if m.selection.details {
			state.SelectionPane = "details"
		}
		state.SelectionStart = selectionOffset(m.selection.lines, m.selection.anchor)
		state.SelectionEnd = selectionOffset(m.selection.lines, m.selection.head)
	}
	for _, id := range m.attachOrder {
		if in, ok := m.attachOutbox[id]; ok {
			var text string
			if json.Unmarshal(in.Payload, &text) == nil {
				state.Retained = append(state.Retained, attachLocalInput{ID: string(id), Text: text})
			}
		}
	}
	return state
}
func (m *model) saveAttachView(state attachLocalState) error {
	if m.attachSave == nil {
		return nil
	}
	data, err := json.Marshal(state, json.Deterministic(true))
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	if hash == m.attachSavedHash {
		return nil
	}
	if err = m.attachSave(state); err != nil {
		return err
	}
	m.attachSavedHash = hash
	m.attachSaveError = ""
	return nil
}
func (m *model) checkpointAttachView() error { return m.saveAttachView(m.captureAttachView()) }
func (m *model) restoreRetainedAttached(state attachLocalState) {
	for _, local := range state.Retained {
		id := inbox.ID(local.ID)
		index := m.submissionIndex(id)
		if index >= 0 && m.lines[index].submission == submissionPersisted {
			continue
		}
		payload, _ := json.Marshal(local.Text)
		in := inbox.Input{ID: id, Kind: inbox.InputExternal, Payload: payload}
		if durable, ok := m.attachPending[id]; ok && !bytes.Equal(durable.Payload, payload) {
			m.append("Retry disabled", "Local retained payload conflicts with durable host input "+string(id)+"; nothing resent.")
			continue
		}
		m.retainAttached(in)
		if index < 0 {
			m.applyInput(in)
			m.setSubmission(m.submissionIndex(id), submissionUncertain)
		}
	}
}
func (m *model) restoreAttachView(state attachLocalState) {
	m.toolsFocused = state.ToolsFocused
	m.toolsCollapsed = state.ToolsCollapsed
	for i, card := range m.tools {
		if toolViewID(card.key) == state.SelectedCallID {
			m.selected = i
			break
		}
	}
	m.detailsOpen = false
	if state.DetailsCallID != "" {
		for i, card := range m.tools {
			if toolViewID(card.key) == state.DetailsCallID {
				m.selected = i
				m.detailsOpen = true
				m.toolsFocused = true
				m.toolsCollapsed = false
				break
			}
		}
	}
	if m.toolsFocused {
		m.composer.Blur()
	}
	m.toolsOffset = min(max(0, state.ToolsOffset), max(0, len(m.tools)-1))
	m.resize()
	m.refreshDetails()
	offset := state.ConversationOffset
	if state.ConversationAnchor != "" {
		lines := strings.Split(ansi.Strip(m.conversation.GetContent()), "\n")
		for i := range lines {
			if scrollAnchorLines(lines, i) == state.ConversationAnchor {
				offset = i
				break
			}
		}
	}
	m.conversation.SetYOffset(offset)
	m.details.SetYOffset(state.DetailsOffset)
	if state.SelectionPane == "conversation" || state.SelectionPane == "details" && m.detailsOpen {
		m.selection = textSelection{details: state.SelectionPane == "details"}
		m.selection.capture(*m.selectionViewport())
		m.selection.anchor = selectionPoint(m.selection.lines, state.SelectionStart)
		m.selection.head = selectionPoint(m.selection.lines, state.SelectionEnd)
	}
}

func scrollAnchor(content string, offset int) string {
	return scrollAnchorLines(strings.Split(ansi.Strip(content), "\n"), offset)
}
func scrollAnchorLines(lines []string, offset int) string {
	if offset < 0 || offset >= len(lines) {
		return ""
	}
	end := min(len(lines), offset+8)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(lines[offset:end], "\n"))))
}
