package main

import (
	"bytes"
	"charm.land/bubbles/v2/spinner"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

type attachReconnected struct {
	epoch  uint64
	client *attachclient.Client
	err    error
}

func (m *model) canRetainAttached(text string) bool {
	payload, err := json.Marshal(text)
	if err != nil || len(payload) > attachproto.MaxPayload || len(m.attachOutbox) >= 64 {
		return false
	}
	size := len(payload)
	for _, in := range m.attachOutbox {
		size += len(in.Payload)
	}
	return size <= 8<<20
}
func (m *model) retainAttached(in inbox.Input) {
	if m.attachOutbox == nil {
		m.attachOutbox = make(map[inbox.ID]inbox.Input)
	}
	if _, exists := m.attachOutbox[in.ID]; exists {
		return
	}
	in.Payload = in.Payload.Clone()
	m.attachOutbox[in.ID] = in
	// Compact order only when adding, retaining FIFO without an immortal ID list.
	order := m.attachOrder[:0]
	for _, id := range m.attachOrder {
		if _, ok := m.attachOutbox[id]; ok {
			order = append(order, id)
		}
	}
	m.attachOrder = append(order, in.ID)
}
func (m *model) retryAttached(id string) tea.Cmd {
	if !m.attachConnected {
		m.append("Retry", "Disconnected; press F5 or /reconnect first. Nothing resent.")
		return nil
	}
	if id == "" {
		for _, candidate := range m.attachOrder {
			if _, ok := m.attachOutbox[candidate]; ok {
				id = string(candidate)
				break
			}
		}
	}
	in, ok := m.attachOutbox[inbox.ID(id)]
	if !ok {
		m.append("Retry", "No retained unconfirmed input for this ID (it may already be persisted). Nothing sent.")
		return nil
	}
	if _, sending := m.pendingSubmissions[in.ID]; sending {
		m.append("Retry", "This input already has a submission in flight. Nothing resent.")
		return nil
	}
	var text string
	if json.Unmarshal(in.Payload, &text) != nil || in.Kind != inbox.InputExternal {
		m.append("Retry", "Invalid retained external input; nothing sent.")
		return nil
	}
	if m.pendingSubmissions == nil {
		m.pendingSubmissions = make(map[inbox.ID]struct{})
	}
	m.pendingSubmissions[in.ID] = struct{}{}
	m.sending = true
	if index := m.submissionIndex(in.ID); index >= 0 && m.lines[index].submission != submissionQueued {
		m.setSubmission(index, submissionSending)
	}
	m.composer.Reset()
	if m.submissionQueue == nil {
		m.submissionQueue = newSubmissionQueue(m.ctx)
	}
	result := m.submissionQueue.enqueue(in, text, m.inputs, nil)
	return func() tea.Msg { return <-result }
}
func (m *model) startAttachReconnect() tea.Cmd {
	if m.attachDial == nil || m.attachReconnecting {
		return nil
	}
	m.attachReconnecting = true
	m.attachConnected = false
	m.attachEpoch++
	epoch, dial, ctx, old := m.attachEpoch, m.attachDial, m.ctx, m.inputs
	var cursor *attachproto.Cursor
	if m.attachCursor.Incarnation != "" {
		copy := m.attachCursor
		cursor = &copy
	}
	if strings.TrimSpace(m.composer.Value()) == "/reconnect" {
		m.composer.Reset()
	}
	return func() tea.Msg {
		if c, ok := old.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		for {
			c, err := dial(ctx, cursor)
			if err == nil {
				return attachReconnected{epoch: epoch, client: c}
			}
			if !strings.Contains(err.Error(), "busy") && !errors.Is(err, io.EOF) {
				return attachReconnected{epoch: epoch, err: err}
			}
			select {
			case <-attempt.Done():
				return attachReconnected{epoch: epoch, err: attempt.Err()}
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
}
func (m *model) finishAttachReconnect(msg attachReconnected) (tea.Model, tea.Cmd) {
	if msg.epoch != m.attachEpoch {
		if msg.client != nil {
			_ = msg.client.Close()
		}
		return *m, nil
	}
	m.attachReconnecting = false
	if msg.err != nil {
		m.append("Reconnect failed", msg.err.Error())
		return *m, attachSaveTimer(m.attachEpoch)
	}
	snapshot := msg.client.Snapshot()
	view := m.captureAttachView()
	fresh := *m
	if snapshot.Header.Base == nil {
		fresh = newAttachedModel(m.ctx, msg.client, snapshot, options{theme: m.theme, sessionID: m.sessionID})
	} else {
		fresh.inputs = attachedWriter(msg.client, snapshot)
		fresh.attachConnected = true
		fresh.attachPendingAvailable = snapshot.Header.PendingAvailable
		for _, item := range snapshot.Items {
			fresh.apply(item)
		}
		for _, in := range snapshot.Pending {
			fresh.applyAttachedPending(in)
		}
		fresh.applyWorkerStatus(attachedStatus(snapshot.Header.Status))
		fresh.workSpinner = spinner.New()
	}
	fresh.attachCursor = attachproto.Cursor{Incarnation: snapshot.Hello.Incarnation, Revision: snapshot.Header.Revision, Head: snapshot.Header.Head}
	fresh.attachNamespace = m.attachNamespace
	fresh.attachSave = m.attachSave
	fresh.attachSavedHash = m.attachSavedHash
	fresh.attachEpoch = m.attachEpoch
	fresh.attachDial = m.attachDial
	fresh.attachOutbox = m.attachOutbox
	fresh.attachOrder = m.attachOrder
	fresh.submissionQueue = m.submissionQueue
	fresh.pendingSubmissions = m.pendingSubmissions
	fresh.sending = len(fresh.pendingSubmissions) > 0
	fresh.width, fresh.height = m.width, m.height
	fresh.resize()
	fresh.composer.SetValue(m.composer.Value())
	life := m.attachedLifecycle()
	fresh.workerEvents = &attachEvents{client: msg.client, epoch: fresh.attachEpoch, lifecycle: life}
	fresh.reconcileAttachedLifecycleSnapshot(snapshot)
	for _, id := range m.attachOrder {
		in, ok := m.attachOutbox[id]
		if !ok {
			continue
		}
		if durable, ok := fresh.attachPending[id]; ok && !bytes.Equal(durable.Payload, in.Payload) {
			delete(fresh.attachOutbox, id)
			fresh.append("Retry disabled", "Retained payload conflicts with durable host input "+string(id)+"; nothing resent.")
			continue
		}
		index := fresh.submissionIndex(id)
		if index >= 0 && fresh.lines[index].submission == submissionPersisted {
			delete(fresh.attachOutbox, id)
			continue
		}
		if index < 0 {
			fresh.applyInput(in)
			index = fresh.submissionIndex(id)
			if old := m.submissionIndex(id); old >= 0 {
				fresh.setSubmission(index, m.lines[old].submission)
			}
		}
	}
	fresh.restoreAttachView(view)
	mode := "cursor delta"
	if snapshot.Header.Base == nil {
		mode = "fresh snapshot (" + snapshot.Header.Code + ")"
	}
	fresh.append("Attach", mode+" · "+fmt.Sprintf("Reconnected at revision %d / history %d. Retry retained inputs explicitly with /retry [ID]; no input was sent by reconnect.", snapshot.Header.Revision, snapshot.Header.Head))
	return fresh, fresh.Init()
}
