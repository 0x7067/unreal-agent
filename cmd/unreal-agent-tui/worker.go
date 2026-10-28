package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// Optional: existing writers and test doubles need only implement Submit.
type workerReloader interface {
	Reload(context.Context, []string) error
}

type reloadFinished struct{ err error }
type workerEvent struct {
	local   bool // synthetic local completion must not start a second socket reader
	message tea.Msg
	epoch   uint64
}
type workerEventSource interface{ next() tea.Msg }

// Callbacks can run synchronously during Start, before Bubble Tea is running.
// This mailbox never calls Program.Send and preserves replay/status order. Only
// its immutable pointer, not a mutable UI model, is captured by commands.
type workerEvents struct {
	ctx   context.Context
	mu    sync.Mutex
	queue []tea.Msg
	wake  chan struct{}
}

func newWorkerEvents(ctx context.Context) *workerEvents {
	return &workerEvents{ctx: ctx, wake: make(chan struct{}, 1)}
}

func (e *workerEvents) add(msg tea.Msg) {
	e.mu.Lock()
	e.queue = append(e.queue, msg)
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *workerEvents) item(item sessionstore.Item)     { e.add(item) }
func (e *workerEvents) status(status workerhost.Status) { e.add(status) }

func (e *workerEvents) next() tea.Msg {
	for {
		e.mu.Lock()
		if len(e.queue) > 0 {
			msg := e.queue[0]
			e.queue[0] = nil
			e.queue = e.queue[1:]
			e.mu.Unlock()
			return workerEvent{message: msg}
		}
		e.mu.Unlock()
		select {
		case <-e.wake:
		case <-e.ctx.Done():
			return workerEvent{}
		}
	}
}

func (m *model) startReload() tea.Cmd {
	if m.reloading {
		return nil
	}
	m.reloading = true
	m.reloadError = ""
	ctx, reloader := m.ctx, m.inputs
	return func() tea.Msg {
		r, ok := reloader.(workerReloader)
		if !ok {
			return reloadFinished{err: errors.New("this input writer does not support worker reload")}
		}
		ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		return reloadFinished{err: r.Reload(ctx, nil)}
	}
}

func (m *model) applyWorkerStatus(status workerhost.Status) {
	if status.Generation != 0 && status.Generation < m.workerGeneration {
		return
	}
	if status.Generation > m.workerGeneration {
		m.workerGeneration = status.Generation
	}
	if status.InstalledBuild != "" {
		m.installedBuild = singleLine(status.InstalledBuild)
	}
	if status.ReloadState != "" {
		m.reloadState = singleLine(status.ReloadState)
	}
	if status.Build != "" {
		m.workerBuild = singleLine(status.Build)
	}
	m.workerStatus = singleLine(status.Message)
	m.workerStopped = status.Stopped
	m.workerStatusError = status.Err != nil
	if status.Err != nil {
		m.workerStatus += " · " + singleLine(status.Err.Error())
	}
	// Reload is not a run end. Ingress acknowledgments remain unconfirmed until
	// item events prove session persistence, even while a worker is replaced.
	// Metadata updates must not steal the user's transcript/detail scroll.
	offset := m.conversation.YOffset()
	m.renderConversation()
	m.conversation.SetYOffset(offset)
}

// workerLabel is presentation-only: the full build metadata and executable
// fingerprint remain unchanged in the model and host.
func workerLabel(generation uint64, build string) string {
	metadata := build
	fingerprint := ""
	if i := strings.LastIndex(build, " ["); i >= 0 && strings.HasSuffix(build, "]") {
		metadata = build[:i]
		fingerprint = build[i+2 : len(build)-1]
	}
	version, _, _ := strings.Cut(metadata, "+")
	label := fmt.Sprintf("#%d · %s", generation, version)
	if fingerprint != "" {
		label += " · " + fingerprint[:min(8, len(fingerprint))]
	}
	return label
}

func frontendLabel() string {
	label := version
	if commit != "unknown" {
		label += " · " + commit[:min(12, len(commit))]
	}
	return label
}
