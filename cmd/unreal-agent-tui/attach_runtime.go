package main

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

// The attach frontend selects no executable and owns no worker. Its existing
// optional reload/build interfaces delegate only to the authenticated host.
type attachedRuntimeWriter struct {
	drainEnabled bool
	pendingMu    sync.Mutex
	pendingID    string
	*attachclient.Client
	generation atomic.Uint64
	enabled    bool
}

func attachedWriter(c *attachclient.Client, s attachclient.Snapshot) inbox.Writer {
	w := &attachedRuntimeWriter{Client: c}
	for _, capability := range s.Hello.Capabilities {
		if capability == attachproto.CapabilityRuntimeDrain {
			w.drainEnabled = true
		}
		if capability == attachproto.CapabilityRuntime {
			w.enabled = true
		}
	}
	if s.Header.Status != nil {
		w.generation.Store(s.Header.Status.Generation)
	}
	return w
}
func (w *attachedRuntimeWriter) advance(g uint64) {
	for old := w.generation.Load(); g > old; old = w.generation.Load() {
		if w.generation.CompareAndSwap(old, g) {
			break
		}
	}
}
func (w *attachedRuntimeWriter) Reload(ctx context.Context, candidate []string) error {
	if len(candidate) != 0 {
		return errors.New("attach frontend cannot select a runtime executable")
	}
	_, err := w.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: w.generation.Load()})
	return err
}
func runtimeWorkerStatus(s attachproto.RuntimeStatus) workerhost.Status {
	v := workerhost.Status{Generation: s.Generation, Build: s.RunningBuild, InstalledBuild: s.CandidateBuild, ReloadState: s.State, Message: s.Message, Stopped: s.State == "stopped"}
	if s.Error != "" {
		v.Err = errors.New(s.Error)
	}
	return v
}
func (w *attachedRuntimeWriter) CheckInstalled(ctx context.Context) (workerhost.Status, error) {
	s, err := w.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "check"})
	return runtimeWorkerStatus(s), err
}
func (m *model) advanceAttachedRuntime(g uint64) {
	if w, ok := m.inputs.(*attachedRuntimeWriter); ok {
		w.advance(g)
	}
}

func (w *attachedRuntimeWriter) trackRuntime(e attachproto.Event) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()
	if e.Runtime == nil {
		return
	}
	if !e.Runtime.Terminal() {
		w.pendingID = e.ID
	} else if w.pendingID == e.ID {
		w.pendingID = ""
	}
}
func (m *model) startAttachedUpgrade(cancelOnly bool) tea.Cmd {
	w, ok := m.inputs.(*attachedRuntimeWriter)
	if !ok || !w.drainEnabled || !m.attachConnected {
		m.append("Upgrade", "Natural-drain replacement is not supported by this connected host.")
		return nil
	}
	if cancelOnly {
		w.pendingMu.Lock()
		id := w.pendingID
		w.pendingMu.Unlock()
		if id == "" {
			m.append("Upgrade", "No current runtime request to cancel.")
			return nil
		}
		ctx := m.ctx
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			err := w.CancelRuntime(ctx, id)
			return attachedRuntimeCancelFinished{err: err}
		}
	}
	if m.reloading {
		return nil
	}
	m.reloading = true
	m.reloadError = ""
	ctx := m.ctx
	generation := w.generation.Load()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 55*time.Second)
		defer cancel()
		_, err := w.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "drain", ExpectedGeneration: generation})
		return reloadFinished{err: err}
	}
}

type attachedRuntimeCancelFinished struct{ err error }
