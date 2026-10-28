package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/xdgpath"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

type attachOptions struct {
	runtime, state, local string
	ui                    options
}

func parseAttachOptions(args []string, getenv func(string) string, out io.Writer) (attachOptions, error) {
	var opts attachOptions
	var palette string
	flags := flag.NewFlagSet("unreal-agent-tui attach", flag.ContinueOnError)
	flags.SetOutput(out)
	flags.StringVar(&opts.runtime, "runtime-directory", "", "existing private runtime directory; defaults to XDG_RUNTIME_DIR")
	flags.StringVar(&opts.local, "client-state-directory", "", "private client-local draft/retry/view directory; defaults to XDG state (not session history)")
	flags.StringVar(&opts.state, "session-directory", "", "existing session directory; defaults to XDG state")
	flags.StringVar(&opts.ui.sessionID, "session", "", "required hosted session ID")
	flags.StringVar(&palette, "theme", defaultTheme, "client-local palette name or JSON path")
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() != 0 || opts.ui.sessionID == "" {
		return opts, errors.New("attach requires a session ID and no positional arguments")
	}
	if opts.runtime == "" {
		opts.runtime = getenv("XDG_RUNTIME_DIR")
	}
	if opts.state == "" {
		var err error
		opts.state, err = xdgpath.SessionDirectory(getenv)
		if err != nil {
			return opts, err
		}
	}
	if opts.local == "" {
		opts.local = filepath.Join(filepath.Dir(opts.state), "clients")
	}
	if !filepath.IsAbs(opts.local) {
		return opts, errors.New("client-state-directory must be absolute")
	}
	if !filepath.IsAbs(opts.runtime) || !filepath.IsAbs(opts.state) {
		return opts, errors.New("attach runtime/session directories must be absolute")
	}
	var err error
	opts.ui.theme, err = loadTheme(palette)
	return opts, err
}

type attachedEvent struct{ event attachproto.Event }
type attachDisconnected struct{ err error }
type attachEvents struct {
	client    *attachclient.Client
	ended     bool
	epoch     uint64
	lifecycle *attachLifecycleState
}

func (e *attachEvents) next() tea.Msg {
	if e.ended {
		return workerEvent{}
	}
	event, err := e.client.Next()
	if err != nil {
		e.ended = true
		return workerEvent{message: attachDisconnected{err}, epoch: e.epoch}
	}
	return workerEvent{message: attachedEvent{event}, epoch: e.epoch}
}
func attachedStatus(s *attachproto.Status) workerhost.Status {
	status := workerhost.Status{Generation: s.Generation, Build: s.Build, Stopped: s.Stopped, Message: "Attached · Ctrl+C detaches"}
	if s.Error != "" {
		status.Err = errors.New(s.Error)
	}
	return status
}
func newAttachedModel(ctx context.Context, writer inbox.Writer, snapshot attachclient.Snapshot, opts options) model {
	opts.model = snapshot.Header.Model.ID
	opts.effort = string(snapshot.Header.Model.ReasoningEffort)
	opts.provider = "host"
	if c, ok := writer.(*attachclient.Client); ok {
		writer = attachedWriter(c, snapshot)
	}
	m := newModel(ctx, writer, newUIRegistry(true, false), "host workspace (file picker unavailable)", "host-owned session", opts)
	m.attached, m.attachConnected, m.attachPendingAvailable = true, true, snapshot.Header.PendingAvailable
	m.attachCursor = attachproto.Cursor{Incarnation: snapshot.Hello.Incarnation, Revision: snapshot.Header.Revision, Head: snapshot.Header.Head}
	m.attachEpoch = 1
	m.workerEvents = &attachEvents{lifecycle: newAttachLifecycle(snapshot)}
	m.attachOutbox = make(map[inbox.ID]inbox.Input)
	m.composer.Placeholder = "Send a message · Ctrl+C detaches"
	m.configuration = singleLine(fmt.Sprintf("attached · %s · host %d · worker #%d · committed tool history only", opts.model, snapshot.Hello.HostPID, snapshot.Header.Status.Generation))
	m.activityError = "Live operation checkpoints and agent activity unavailable in attach v1"
	for _, item := range snapshot.Items {
		m.apply(item)
	}
	for _, in := range snapshot.Pending {
		m.applyAttachedPending(in)
	}
	m.applyWorkerStatus(attachedStatus(snapshot.Header.Status))
	if !m.attachPendingAvailable {
		m.append("Attach", "Pending ingress view unavailable; listed queued inputs may be incomplete.")
	}
	if m.responding {
		m.workSpinner = spinner.New()
		m.workStarted = time.Now()
	}
	m.renderConversation()
	m.conversation.GotoBottom()
	return m
}
func (m *model) applyAttachedPending(in inbox.Input) {
	if in.Kind != inbox.InputExternal {
		return
	}
	if m.attachPending == nil {
		m.attachPending = make(map[inbox.ID]inbox.Input)
	}
	m.attachPending[in.ID] = in
	if local, ok := m.attachOutbox[in.ID]; ok && !bytes.Equal(local.Payload, in.Payload) {
		delete(m.attachOutbox, in.ID)
		m.append("Retry disabled", "Retained payload conflicts with durable host input "+string(in.ID)+"; host payload wins.")
	}
	if index := m.submissionIndex(in.ID); index >= 0 {
		if m.lines[index].submission != submissionPersisted {
			var text string
			if json.Unmarshal(in.Payload, &text) == nil {
				m.lines[index].text = terminaltext.Clean(text)
				m.lines[index].width = 0
			}
			m.setSubmission(index, submissionQueued)
		}
		return
	}
	m.applyInput(in)
	if index := m.submissionIndex(in.ID); index >= 0 {
		m.setSubmission(index, submissionQueued)
	}
}
func (m *model) applyAttachedEvent(event attachproto.Event) {
	if m.applyAttachedLifecycleEvent(event) {
		return
	}
	m.attachPendingAvailable = event.PendingAvailable
	switch event.Type {
	case "pending":
		for _, in := range event.Pending {
			m.applyAttachedPending(in)
		}
	case "status":
		if event.Status != nil {
			m.advanceAttachedRuntime(event.Status.Generation)
			m.applyWorkerStatus(attachedStatus(event.Status))
		}
	case "runtime":
		if w, ok := m.inputs.(*attachedRuntimeWriter); ok {
			w.trackRuntime(event)
		}
		if event.Runtime != nil {
			m.advanceAttachedRuntime(event.Runtime.Generation)
			m.applyWorkerStatus(runtimeWorkerStatus(*event.Runtime))
		}
	case "pending_unavailable":
		m.attachPendingAvailable = false
		m.append("Attach", "Pending ingress view unavailable; missing entries do not prove persistence or rejection.")
	}
}
func runAttached(ctx context.Context, args []string, getenv func(string) string, output io.Writer) (result error) {
	ctx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()
	opts, err := parseAttachOptions(args, getenv, output)
	if err != nil {
		return err
	}
	client, err := attachclient.Dial(ctx, opts.runtime, opts.state, session.ID(opts.ui.sessionID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			handled, reconcileErr := reconcileUnavailableAttach(ctx, opts, output)
			if handled && reconcileErr == nil {
				return nil
			}
			return errors.Join(err, reconcileErr)
		}
		return err
	}
	defer client.Close()
	endpoint, err := attachproto.EndpointPath(opts.runtime, opts.state, session.ID(opts.ui.sessionID))
	if err != nil {
		return err
	}
	namespace := strings.TrimSuffix(filepath.Base(endpoint), ".sock")
	localLease, err := lockAttachLocalState(opts.local, namespace)
	if err != nil {
		return err
	}
	defer localLease.Close()
	saved, err := loadAttachLocalState(opts.local, namespace)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local view state refused (host untouched; choose a new -client-state-directory or repair the snapshot): %w", err)
	}
	input, tty, err := tea.OpenTTY()
	if err != nil {
		return fmt.Errorf("open attach terminal: %w", err)
	}
	defer func() {
		_ = input.Close()
		if input != tty {
			_ = tty.Close()
		}
	}()
	submissions := newSubmissionQueue(ctx)
	defer func() {
		submissions.Cancel()
		_ = client.Close()
		wait, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = errors.Join(result, submissions.Wait(wait))
	}()
	ui := newAttachedModel(ctx, client, client.Snapshot(), opts.ui)
	ui.attachNamespace = namespace
	life := ui.attachedLifecycle()
	life.directory, life.sessionID = opts.state, session.ID(opts.ui.sessionID)
	life.controls = append([]attachLocalControl(nil), saved.Controls...)
	ui.attachSave = func(state attachLocalState) error {
		state.Version = attachLocalStateVersion
		state.Controls = append([]attachLocalControl(nil), life.controls...)
		return saveAttachLocalState(opts.local, state)
	}
	ui.reconcileAttachedLifecycleSnapshot(client.Snapshot())
	if saved.Version != 0 {
		ui.restoreRetainedAttached(saved)
		ui.composer.SetValue(saved.Draft)
		ui.attachRestore = &saved
	}
	ui.submissionQueue = submissions
	ui.workerEvents = &attachEvents{client: client, epoch: ui.attachEpoch, lifecycle: life}
	var clientsMu sync.Mutex
	currentClient := client
	clientOwnerClosed := false
	defer func() {
		cancelSession()
		clientsMu.Lock()
		clientOwnerClosed = true
		c := currentClient
		clientsMu.Unlock()
		_ = c.Close()
	}()
	ui.attachDial = func(next context.Context, cursor *attachproto.Cursor) (*attachclient.Client, error) {
		c, err := attachclient.DialFrom(next, opts.runtime, opts.state, session.ID(opts.ui.sessionID), cursor)
		if err == nil {
			clientsMu.Lock()
			if clientOwnerClosed {
				clientsMu.Unlock()
				_ = c.Close()
				return nil, io.ErrClosedPipe
			}
			currentClient = c
			clientsMu.Unlock()
		}
		return c, err
	}
	screen := &mouseScreen{model: ui}
	program := tea.NewProgram(screen, tea.WithContext(ctx), tea.WithoutSignalHandler(), tea.WithInput(input), tea.WithOutput(tty), tea.WithFilter(screen.filter))
	_, err = program.Run()
	result = errors.Join(result, screen.model.checkpointAttachView())
	if ctx.Err() != nil || errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return errors.Join(result, err)
}
