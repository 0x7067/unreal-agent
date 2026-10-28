package main

// Explicit attached controls have a separate, bounded local intent journal.
// Admission is not an execution result. Only matching canonical outcomes settle
// these intents; neither EOF nor a missing socket proves cleanup.
import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

const attachControlLimit = 32

type attachLocalControl struct {
	ID           string
	Scope        string
	TargetTurnID string
	Input        inbox.Input
}
type attachLifecycleState struct {
	scope      string
	caps       map[string]bool
	generation uint64
	turn       string
	controls   []attachLocalControl
	inflight   map[string]bool
	outcomes   map[string]string
	matched    map[string]bool
	directory  string
	sessionID  session.ID
}

func boundedLifecycleID(s string, limit int) bool {
	if s == "" || len(s) > limit || !utf8.ValidString(s) || strings.TrimSpace(s) != s {
		return false
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func validateAttachLocalControls(state attachLocalState) error {
	if len(state.Controls) > attachControlLimit || state.Version == 1 && len(state.Controls) != 0 {
		return errors.New("invalid local lifecycle journal version/count")
	}
	seen := map[string]bool{}
	for _, c := range state.Controls {
		id, err := uuid.Parse(c.ID)
		if err != nil || id.String() != c.ID || len(c.ID) > 128 || seen[c.ID] {
			return errors.New("invalid/duplicate lifecycle UUID")
		}
		seen[c.ID] = true
		if string(c.Input.ID) != c.ID || !boundedLifecycleID(c.Scope, 256) {
			return errors.New("invalid lifecycle identity/scope")
		}
		if err := attachproto.ValidateLifecycle(c.Input, c.Scope); err != nil {
			return err
		}
		msg, err := c.Input.DecodeControlMessage()
		if err != nil {
			return err
		}
		switch msg.Mode {
		case inbox.AbortForeground:
			p := msg.Parameters.(inbox.ForegroundAbort)
			if !boundedLifecycleID(c.TargetTurnID, 128) || p.TargetTurnID != c.TargetTurnID {
				return errors.New("invalid lifecycle original target")
			}
		case inbox.ShutdownSession:
			if c.TargetTurnID != "" {
				return errors.New("shutdown cannot have a turn target")
			}
		}
	}
	return nil
}
func newAttachLifecycle(snapshot attachclient.Snapshot) *attachLifecycleState {
	l := &attachLifecycleState{caps: map[string]bool{}, inflight: map[string]bool{}, outcomes: map[string]string{}, matched: map[string]bool{}}
	l.bind(snapshot)
	return l
}
func (l *attachLifecycleState) bind(s attachclient.Snapshot) {
	l.caps = map[string]bool{}
	for _, c := range s.Hello.Capabilities {
		l.caps[c] = true
	}
	l.scope = ""
	l.generation = 0
	if s.Header.Status != nil {
		l.generation = s.Header.Status.Generation
		l.scope = fmt.Sprintf("%s/%d", s.Hello.Incarnation, l.generation)
	}
}
func (m *model) attachedLifecycle() *attachLifecycleState {
	if events, ok := m.workerEvents.(*attachEvents); ok {
		return events.lifecycle
	}
	return nil
}
func (m *model) reconcileAttachedLifecycleSnapshot(s attachclient.Snapshot) {
	l := m.attachedLifecycle()
	if l == nil {
		return
	}
	l.bind(s)
	l.inflight = map[string]bool{}
	if s.Header.Base == nil {
		l.matched = map[string]bool{}
		l.outcomes = map[string]string{}
		l.turn = ""
	}
	for _, item := range s.Items {
		m.observeAttachedLifecycle(item)
	}
	for _, c := range l.controls {
		if l.outcomes[c.ID] == "" {
			m.append("Lifecycle", c.ID+" · outcome unknown; retained original scope/target. Use /retry-control ID explicitly; never /retry.")
		}
	}
}
func (m *model) observeAttachedLifecycle(item sessionstore.Item) {
	l := m.attachedLifecycle()
	if l == nil {
		return
	}
	switch d := item.Data.(type) {
	case sessionstore.Fork:
		l.turn = ""
	case session.Turn:
		l.turn = string(d.ID)
	// Keep the latest canonical turn through its tool phase. The host checks
	// the exact target against actual current execution before canceling anything.
	case inbox.Input:
		if d.Kind != inbox.InputControl {
			return
		}
		msg, err := d.DecodeControlMessage()
		if err != nil {
			return
		}
		for _, c := range l.controls {
			if d.ID == c.Input.ID && d.Kind == c.Input.Kind && bytes.Equal(d.Payload, c.Input.Payload) {
				l.matched[c.ID] = true
			}
			if !l.matched[c.ID] {
				continue
			}
			var outcome string
			switch msg.Mode {
			case inbox.ForegroundAbortFinished:
				p := msg.Parameters.(inbox.ForegroundAbortResult)
				if string(p.InputID) == c.ID && p.TargetTurnID == c.TargetTurnID && c.TargetTurnID != "" {
					outcome = p.Status
					if p.Error != "" {
						outcome += " · " + p.Error
					}
				}
			case inbox.SessionShutdownFinished:
				p := msg.Parameters.(inbox.SessionShutdownResult)
				if string(p.InputID) == c.ID && p.Scope == c.Scope && c.TargetTurnID == "" {
					outcome = p.Phase
					if p.Error != "" {
						outcome += " · " + p.Error
					}
				}
			}
			if outcome != "" && l.outcomes[c.ID] != outcome {
				l.outcomes[c.ID] = outcome
				m.append("Lifecycle canonical", c.ID+" · "+outcome)
			}
		}
	}
}
func (m *model) startAttachedLifecycle(text string) (bool, tea.Cmd) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false, nil
	}
	if fields[0] != "/abort" && fields[0] != "/shutdown" && fields[0] != "/retry-control" {
		return false, nil
	}
	if !m.attached {
		return false, nil
	}
	refuse := func(reason string) (bool, tea.Cmd) {
		m.append("Lifecycle refused", reason+"; nothing sent.")
		return true, nil
	}
	l := m.attachedLifecycle()
	if l == nil {
		return refuse("Lifecycle state unavailable")
	}
	if fields[0] == "/retry-control" && len(fields) == 2 {
		for _, c := range l.controls {
			if c.ID == fields[1] {
				if !m.attachConnected {
					return true, m.reconcileAttachedLifecycleOffline()
				}
				return true, m.sendAttachedLifecycle(c)
			}
		}
		return refuse("No original retained control for this ID")
	}
	if len(fields) != 1 || fields[0] == "/retry-control" {
		return refuse("Use /abort, /shutdown, or /retry-control ID")
	}
	if !m.attachConnected {
		return refuse("Reconnect first")
	}
	if len(l.controls) >= attachControlLimit {
		return refuse("Local control journal full; choose a separate client-state directory after reconciling old intents")
	}
	mode, capability := inbox.AbortForeground, attachproto.CapabilityForegroundAbort
	if fields[0] == "/shutdown" {
		mode, capability = inbox.ShutdownSession, attachproto.CapabilitySessionShutdown
	}
	if !l.caps[capability] {
		return refuse("Host does not advertise " + capability)
	}
	if l.scope == "" || l.generation != m.workerGeneration {
		return refuse("Host generation changed; reconnect to capture a fresh scope")
	}
	c := attachLocalControl{ID: uuid.NewString(), Scope: l.scope}
	var params any = inbox.ShutdownSessionRequest{Version: 1, Scope: c.Scope}
	if mode == inbox.AbortForeground {
		if !boundedLifecycleID(l.turn, 128) {
			return refuse("No actual canonical turn target")
		}
		c.TargetTurnID = l.turn
		params = inbox.ForegroundAbort{Version: 1, Scope: c.Scope, TargetTurnID: c.TargetTurnID}
	}
	payload, err := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: params})
	if err != nil {
		return refuse(err.Error())
	}
	c.Input = inbox.Input{ID: inbox.ID(c.ID), Kind: inbox.InputControl, Payload: payload}
	if m.attachSave == nil {
		return refuse("Durable client-local state unavailable")
	}
	l.controls = append(l.controls, c)
	state := m.captureAttachView()
	state.Controls = append([]attachLocalControl(nil), l.controls...)
	state.Draft = ""
	if err := m.saveAttachView(state); err != nil {
		l.controls = l.controls[:len(l.controls)-1]
		return refuse("Cannot persist original control identity: " + err.Error())
	}
	m.composer.Reset()
	return true, m.sendAttachedLifecycle(c)
}
func (m *model) sendAttachedLifecycle(c attachLocalControl) tea.Cmd {
	l := m.attachedLifecycle()
	if l == nil {
		return nil
	}
	refuse := func(s string) tea.Cmd {
		m.append("Lifecycle refused", c.ID+" · "+s+"; original control not rebound, nothing sent.")
		return nil
	}
	if l.inflight[c.ID] {
		return refuse("admission already in flight")
	}
	if l.outcomes[c.ID] != "" && !strings.HasPrefix(l.outcomes[c.ID], "uncertain") {
		return refuse("canonical outcome already observed")
	}
	if !m.attachConnected || l.scope != c.Scope || l.generation != m.workerGeneration {
		return refuse("original scope is stale; reconnect/reconcile only")
	}
	msg, err := c.Input.DecodeControlMessage()
	if err != nil {
		return refuse(err.Error())
	}
	cap := attachproto.CapabilitySessionShutdown
	if msg.Mode == inbox.AbortForeground {
		cap = attachproto.CapabilityForegroundAbort
		if l.turn != c.TargetTurnID {
			return refuse("original turn is no longer current")
		}
	}
	if !l.caps[cap] {
		return refuse("host capability unavailable")
	}
	if err := attachproto.ValidateLifecycle(c.Input, l.scope); err != nil {
		return refuse(err.Error())
	}
	client, ok := m.inputs.(interface {
		SubmitLifecycle(context.Context, inbox.Input) error
	})
	if !ok {
		return refuse("lifecycle admission unavailable")
	}
	l.inflight[c.ID] = true
	ctx, epoch := m.ctx, m.attachEpoch
	return func() tea.Msg {
		attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		err := client.SubmitLifecycle(attempt, c.Input)
		event := attachproto.Event{Type: "local_lifecycle_admission", InputID: c.Input.ID, Code: "accepted"}
		if err != nil {
			event.Code = "uncertain"
			var reply *attachclient.ReplyError
			if errors.As(err, &reply) && !errors.Is(err, attachclient.ErrUncertain) {
				event.Code = "refused"
			}
			event.Error = err.Error()
		}
		return workerEvent{message: attachedEvent{event}, epoch: epoch, local: true}
	}
}
func (m *model) applyAttachedLifecycleEvent(e attachproto.Event) bool {
	l := m.attachedLifecycle()
	if l == nil {
		return false
	}
	if e.Type == "status" && e.Status != nil && e.Status.Generation != l.generation {
		m.append("Lifecycle", "Host generation changed; reconnect before any lifecycle control.")
	}
	if e.Type != "local_lifecycle_admission" {
		return false
	}
	delete(l.inflight, string(e.InputID))
	if e.Code == "accepted" {
		m.append("Lifecycle admission", string(e.InputID)+" · accepted; execution/cleanup still unknown, awaiting canonical outcome.")
	} else if e.Code == "refused" {
		m.append("Lifecycle refused", string(e.InputID)+" · "+e.Error+"; host rejected admission, no cleanup claim.")
	} else {
		m.append("Lifecycle uncertain", string(e.InputID)+" · "+e.Error+"; admission/outcome unconfirmed. Retry only original ID/scope/target with /retry-control ID.")
	}
	return true
}
func (m *model) reconcileAttachedLifecycleOffline() tea.Cmd {
	l := m.attachedLifecycle()
	if l == nil {
		return nil
	}
	// Open only the already-known private state namespace and exact session file.
	// No New store, mkdir, lease, metadata mutation or provider/autostart occurs.
	dir, err := openAttachLocalDirectory(l.directory, false)
	if err != nil {
		m.append("Lifecycle unknown", "Read-only reconciliation refused: "+err.Error())
		return nil
	}
	defer dir.Close()
	f, err := openAttachLifecycleFile(dir, string(l.sessionID)+".session.jsonl")
	if err != nil {
		m.append("Lifecycle unknown", "Read-only reconciliation unavailable: "+err.Error())
		return nil
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Second)
	defer cancel()
	h, err := localfile.ReadLifecycle(ctx, f, l.sessionID)
	if err != nil {
		m.append("Lifecycle unknown", "Canonical reconciliation failed: "+err.Error())
		return nil
	}
	for _, c := range l.controls {
		// Match the original typed intent as well as its outcome, not just an ID.
		msg, _ := c.Input.DecodeControlMessage()
		if p, ok := h.Aborts[c.Input.ID]; ok && msg.Mode == inbox.AbortForeground && p == msg.Parameters.(inbox.ForegroundAbort) {
			if result, ok := h.AbortResults[c.Input.ID]; ok {
				l.matched[c.ID] = true
				m.observeAttachedLifecycle(sessionstore.Item{Data: lifecycleResultInput(inbox.ForegroundAbortFinished, result)})
			}
		}
		if p, ok := h.Shutdowns[c.Input.ID]; ok && msg.Mode == inbox.ShutdownSession && p == msg.Parameters.(inbox.ShutdownSessionRequest) {
			if result, ok := h.ShutdownResults[c.Input.ID]; ok {
				l.matched[c.ID] = true
				m.observeAttachedLifecycle(sessionstore.Item{Data: lifecycleResultInput(inbox.SessionShutdownFinished, result)})
			}
		}
		if l.outcomes[c.ID] == "" {
			m.append("Lifecycle unknown", c.ID+" · no matching canonical outcome; EOF/socket loss is not cleanup proof.")
		}
	}
	return nil
}
func lifecycleResultInput(mode inbox.ControlMode, p any) inbox.Input {
	b, _ := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: p})
	return inbox.Input{Kind: inbox.InputControl, Payload: b}
}

// Keep the original JSON bytes stable across retries and reloads.
func sameAttachedControl(a, b attachLocalControl) bool {
	return a.ID == b.ID && a.Scope == b.Scope && a.TargetTurnID == b.TargetTurnID && a.Input.ID == b.Input.ID && a.Input.Kind == b.Input.Kind && bytes.Equal(a.Input.Payload, b.Input.Payload)
}

func openAttachLifecycleFile(dir *os.File, name string) (*os.File, error) {
	if strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".session.jsonl") {
		return nil, errors.New("invalid original session filename")
	}
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err == nil && (st.Mode&unix.S_IFMT != unix.S_IFREG || st.Uid != uint32(os.Geteuid()) || st.Mode&07777 != 0600 || st.Nlink != 1 || st.Size > 64<<20) {
		err = errors.New("unsafe or oversized canonical lifecycle file")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// reconcileUnavailableAttach is a terminal read-only fallback, not offline
// attach or recovery. It considers only control identities already persisted in
// this exact private client namespace, never discovers other sessions.
func reconcileUnavailableAttach(ctx context.Context, opts attachOptions, out io.Writer) (bool, error) {
	endpoint, err := attachproto.EndpointPath(opts.runtime, opts.state, session.ID(opts.ui.sessionID))
	if err != nil {
		return false, err
	}
	namespace := strings.TrimSuffix(filepath.Base(endpoint), ".sock")
	// Do not create a client directory merely because dialing failed.
	dir, err := openAttachLocalDirectory(opts.local, false)
	if err != nil {
		return false, err
	}
	dir.Close()
	lease, err := lockAttachLocalState(opts.local, namespace)
	if err != nil {
		return false, err
	}
	defer lease.Close()
	saved, err := loadAttachLocalState(opts.local, namespace)
	if err != nil {
		return false, err
	}
	if len(saved.Controls) == 0 {
		return false, nil
	}
	dir, err = openAttachLocalDirectory(opts.state, false)
	if err != nil {
		return false, err
	}
	defer dir.Close()
	f, err := openAttachLifecycleFile(dir, opts.ui.sessionID+".session.jsonl")
	if err != nil {
		return false, err
	}
	defer f.Close()
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	history, err := localfile.ReadLifecycle(readCtx, f, session.ID(opts.ui.sessionID))
	if err != nil {
		return false, err
	}
	fmt.Fprintln(out, "Host unavailable; read-only canonical lifecycle reconciliation (no input submitted, no startup).")
	for _, c := range saved.Controls {
		outcome := "unknown; EOF/socket loss does not prove cleanup"
		msg, _ := c.Input.DecodeControlMessage()
		if p, ok := history.Aborts[c.Input.ID]; ok && msg.Mode == inbox.AbortForeground && p == msg.Parameters.(inbox.ForegroundAbort) {
			if r, ok := history.AbortResults[c.Input.ID]; ok {
				outcome = r.Status
				if r.Error != "" {
					outcome += " · " + r.Error
				}
			}
		}
		if p, ok := history.Shutdowns[c.Input.ID]; ok && msg.Mode == inbox.ShutdownSession && p == msg.Parameters.(inbox.ShutdownSessionRequest) {
			if r, ok := history.ShutdownResults[c.Input.ID]; ok {
				outcome = r.Phase
				if r.Error != "" {
					outcome += " · " + r.Error
				}
			}
		}
		fmt.Fprintf(out, "Lifecycle %s · %s\n", c.ID, terminaltext.Clean(outcome))
	}
	return true, nil
}
