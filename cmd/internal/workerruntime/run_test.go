package workerruntime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

var testIdentity = workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: "test"}

type fakeClient struct {
	calls     chan llm.Request
	responses chan llm.Response
	entered   atomic.Int64
	active    atomic.Int64
	closed    atomic.Bool
	onCancel  func()
	onClose   func() error
}

func newFakeClient() *fakeClient {
	return &fakeClient{calls: make(chan llm.Request, 16), responses: make(chan llm.Response, 16)}
}
func (c *fakeClient) Respond(ctx context.Context, request llm.Request, _ llm.RequestOptions) (llm.Response, error) {
	c.entered.Add(1)
	c.active.Add(1)
	defer c.active.Add(-1)
	c.calls <- request
	select {
	case response := <-c.responses:
		return response, nil
	case <-ctx.Done():
		if c.onCancel != nil {
			c.onCancel()
		}
		return llm.Response{}, ctx.Err()
	}
}
func (c *fakeClient) Close() error {
	c.closed.Store(true)
	if c.onClose != nil {
		return c.onClose()
	}
	return nil
}
func fakeProvider(client *fakeClient) []providers.Provider {
	return []providers.Provider{{Name: "fake", NewClient: func(_, _ string, _ int, _ func(string) string) (providers.Client, error) { return client, nil }}}
}
func bootstrap(t *testing.T) workerproto.Bootstrap {
	t.Helper()
	root := t.TempDir()
	// Resolve platform-owned aliases explicitly in both test paths and runtime.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return workerproto.Bootstrap{SessionID: "session", Directory: filepath.Join(root, "sessions"), Workspace: root, Provider: "fake", Model: llm.Model{ID: "chosen", CompactionThreshold: 123456, ReasoningEffort: llm.ReasoningEffortHigh}, MaxAttempts: 3, Shell: "/bin/sh", SystemPrompt: "Selected system prompt", Subagents: true}
}

type workerTest struct {
	input     *io.PipeWriter
	output    *io.PipeReader
	events    chan workerproto.Event
	decodeErr chan error
	done      chan error
	cancel    context.CancelFunc
}

func startWorker(t *testing.T, b workerproto.Bootstrap, client *fakeClient, wrap func(io.Writer) io.Writer) *workerTest {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	input, inputWriter := io.Pipe()
	output, outputWriter := io.Pipe()
	writer := io.Writer(outputWriter)
	if wrap != nil {
		writer = wrap(writer)
	}
	w := &workerTest{input: inputWriter, output: output, events: make(chan workerproto.Event, 128), decodeErr: make(chan error, 1), done: make(chan error, 1), cancel: cancel}
	go func() {
		scanner := workerproto.Scanner(output)
		for scanner.Scan() {
			var event workerproto.Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				w.decodeErr <- err
				return
			}
			w.events <- event
		}
		if err := scanner.Err(); err != nil {
			w.decodeErr <- err
		}
		close(w.events)
	}()
	go func() {
		w.done <- run(ctx, input, writer, io.Discard, func(string) string { return "" }, testIdentity, fakeProvider(client))
		_ = outputWriter.Close()
	}()
	t.Cleanup(func() { cancel(); _ = inputWriter.Close(); _ = output.Close() })
	w.event(t, "hello")
	w.send(t, workerproto.Request{Type: "start", ID: "start", Protocol: workerproto.Version, Bootstrap: &b})
	return w
}
func (w *workerTest) send(t *testing.T, r workerproto.Request) {
	t.Helper()
	if err := workerproto.Write(w.input, r); err != nil {
		t.Fatal(err)
	}
}
func (w *workerTest) event(t *testing.T, kind string) workerproto.Event {
	t.Helper()
	for {
		select {
		case e, ok := <-w.events:
			if !ok {
				t.Fatalf("output closed waiting for %s", kind)
			}
			if e.Type == "error" {
				t.Fatalf("worker error: %s", e.Error)
			}
			if e.Type == kind {
				return e
			}
		case err := <-w.decodeErr:
			t.Fatalf("decode event: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for %s", kind)
		}
	}
}
func (w *workerTest) finish(t *testing.T) {
	t.Helper()
	select {
	case err := <-w.done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not join")
	}
}

func TestProbeHelloOnly(t *testing.T) {
	var output bytes.Buffer
	if err := Probe(&output, testIdentity); err != nil {
		t.Fatal(err)
	}
	scanner := workerproto.Scanner(&output)
	if !scanner.Scan() {
		t.Fatal("no hello")
	}
	var event workerproto.Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "hello" || !reflect.DeepEqual(event.Identity, &testIdentity) || event.Item != nil {
		t.Fatalf("event = %#v", event)
	}
	if scanner.Scan() {
		t.Fatal("probe emitted more than hello")
	}
	if err := Probe(io.Discard, workerproto.Identity{}); !errors.Is(err, ErrProtocol) {
		t.Fatalf("invalid identity: %v", err)
	}
}

// checkWriter observes a complete protocol frame at the actual write boundary.
// The final event cannot reach the frontend until lease and provider cleanup.
type checkWriter struct {
	io.Writer
	check func(workerproto.Event)
}

func (w checkWriter) Write(p []byte) (int, error) {
	var e workerproto.Event
	if json.Unmarshal(bytes.TrimSpace(p), &e) == nil {
		w.check(e)
	}
	return w.Writer.Write(p)
}

func TestResidentConfigOnlySettingsAndLeaseBeforeQuiesced(t *testing.T) {
	b := bootstrap(t)
	client := newFakeClient()
	var checkErr error
	w := startWorker(t, b, client, func(output io.Writer) io.Writer {
		return checkWriter{output, func(e workerproto.Event) {
			if e.Type != "quiesced" {
				return
			}
			if !client.closed.Load() || client.active.Load() != 0 {
				checkErr = errors.New("provider not joined/closed before quiesced")
				return
			}
			lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
			if err != nil {
				checkErr = err
				return
			}
			checkErr = lease.Close()
		}}
	})
	initial := w.event(t, "item")
	if initial.Item == nil || initial.Item.Kind != sessionstore.ItemInput || initial.Item.Sequence != 1 {
		t.Fatalf("initial=%#v", initial)
	}
	input, ok := initial.Item.Data.(inbox.Input)
	if !ok || input.Kind != inbox.InputControl {
		t.Fatalf("initial settings data=%#v", initial.Item.Data)
	}
	settings, err := input.DecodeControlMessage()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Mode != inbox.UpdateSettings || settings.Parameters.(inbox.Settings).Model != b.Model.ID {
		t.Fatalf("settings=%#v", settings)
	}
	ready := w.event(t, "ready")
	if ready.ID != "start" {
		t.Fatalf("ready=%#v", ready)
	}
	if client.entered.Load() != 0 {
		t.Fatal("configuration woke the model")
	}
	lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
	if lease != nil {
		_ = lease.Close()
	}
	if !errors.Is(err, localfile.ErrSessionLocked) {
		t.Fatalf("worker does not hold lease: %v", err)
	}
	w.send(t, workerproto.Request{Type: "quiesce", ID: "retire"})
	if e := w.event(t, "quiesced"); e.ID != "retire" {
		t.Fatalf("quiesced=%#v", e)
	}
	w.finish(t)
	if checkErr != nil {
		t.Fatal(checkErr)
	}
	if client.entered.Load() != 0 {
		t.Fatal("idle worker called model")
	}
}

func TestResumeReplaysSettingsWithoutBootstrapOverride(t *testing.T) {
	b := bootstrap(t)
	first := startWorker(t, b, newFakeClient(), nil)
	original := first.event(t, "item")
	first.event(t, "ready")
	first.send(t, workerproto.Request{Type: "quiesce", ID: "first"})
	first.event(t, "quiesced")
	first.finish(t)
	b.Model = llm.Model{ID: "wrong-default", CompactionThreshold: 5, ReasoningEffort: llm.ReasoningEffortLow}
	client := newFakeClient()
	second := startWorker(t, b, client, nil)
	replay := second.event(t, "item")
	second.event(t, "ready")
	if !reflect.DeepEqual(original.Item, replay.Item) {
		t.Fatalf("replay differs:\n%#v\n%#v", original.Item, replay.Item)
	}
	payload, _ := json.Marshal("hello")
	second.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "prompt", Kind: inbox.InputExternal, Payload: payload}})
	var request llm.Request
	select {
	case request = <-client.calls:
	case <-time.After(10 * time.Second):
		t.Fatal("no model request")
	}
	if request.Model.ID != "chosen" || request.Model.CompactionThreshold != 123456 || request.Model.ReasoningEffort != llm.ReasoningEffortHigh {
		t.Fatalf("persisted settings lost: %#v", request.Model)
	}
	if len(request.Input) == 0 || !strings.HasSuffix(request.Input[0].Data.(llm.Message).Text, b.SystemPrompt) {
		t.Fatalf("system prompt lost: %#v", request.Input)
	}
	names := map[string]bool{}
	for _, tool := range request.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"Bash", "ViewImage", "TaskGraph", "Agent", "SendMessage"} {
		if !names[name] {
			t.Errorf("missing tool %s: %#v", name, names)
		}
	}
	second.send(t, workerproto.Request{Type: "quiesce", ID: "busy"})
	if e := second.event(t, "busy"); e.ID != "busy" {
		t.Fatalf("busy=%#v", e)
	}
	if client.active.Load() != 1 {
		t.Fatal("busy quiesce canceled model")
	}
	client.responses <- llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "done"}}}}
	for {
		e := second.event(t, "item")
		if e.Item.Kind == sessionstore.ItemModelResponse {
			break
		}
	}
	second.send(t, workerproto.Request{Type: "quiesce", ID: "second"})
	second.event(t, "quiesced")
	second.finish(t)
	store, err := localfile.New(b.Directory)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Items(t.Context(), b.SessionID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	settingsCount := 0
	for _, item := range page.Items {
		if item.Kind == sessionstore.ItemInput {
			input := item.Data.(inbox.Input)
			if input.Kind == inbox.InputControl {
				settingsCount++
			}
		}
	}
	if settingsCount != 1 {
		t.Fatalf("settings inputs=%d, want initial input only", settingsCount)
	}
}

type switchWriter struct {
	output io.Writer
	fail   atomic.Bool
}

func (w *switchWriter) Write(p []byte) (int, error) {
	if w.fail.Load() {
		return 0, io.ErrClosedPipe
	}
	return w.output.Write(p)
}

func TestShutdownCancelsAndJoinsModelBeforeReleasingLease(t *testing.T) {
	for _, mode := range []string{"stop", "eof", "cancel", "output"} {
		t.Run(mode, func(t *testing.T) {
			b := bootstrap(t)
			client := newFakeClient()
			canceled := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			client.onCancel = func() { once.Do(func() { close(canceled) }); <-release }
			failedOutput := &switchWriter{}
			w := startWorker(t, b, client, func(output io.Writer) io.Writer {
				failedOutput.output = output
				return failedOutput
			})
			w.event(t, "ready")
			payload, _ := json.Marshal("work")
			w.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "work", Kind: inbox.InputExternal, Payload: payload}})
			select {
			case <-client.calls:
			case <-time.After(10 * time.Second):
				t.Fatal("model not started")
			}
			switch mode {
			case "stop":
				w.send(t, workerproto.Request{Type: "stop"})
			case "eof":
				if err := w.input.Close(); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				w.cancel()
			case "output":
				failedOutput.fail.Store(true)
				w.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "break-output", Kind: inbox.InputExternal, Payload: payload}})
			}
			select {
			case <-canceled:
			case <-time.After(10 * time.Second):
				t.Fatal("model not canceled")
			}
			lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
			if lease != nil {
				_ = lease.Close()
			}
			if !errors.Is(err, localfile.ErrSessionLocked) {
				t.Fatalf("lease released before model joined: %v", err)
			}
			if client.closed.Load() {
				t.Fatal("provider closed before model joined")
			}
			close(release)
			select {
			case err := <-w.done:
				if mode == "cancel" {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("Run=%v", err)
					}
				} else if mode == "output" {
					if !errors.Is(err, ErrOutput) {
						t.Fatalf("Run=%v", err)
					}
				} else if err != nil {
					t.Fatalf("Run=%v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Run did not join")
			}
			lease, err = localfile.AcquireLease(b.Directory, b.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			_ = lease.Close()
			for e := range w.events {
				if e.Type == "quiesced" {
					t.Fatal("stop falsely acknowledged quiesce")
				}
			}
		})
	}
}

func TestResumeRefusesUnknownOperationBeforeProviderSetup(t *testing.T) {
	b := bootstrap(t)
	store, err := localfile.New(b.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), b.SessionID); err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewShellSpec(operation.ShellInput{Command: "touch should-not-exist", Shell: "/bin/sh", Directory: b.Workspace}, filepath.Join(b.Directory, "operations"), operation.DefaultMaxOutputLength)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "unknown", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
	if err := store.AppendTurn(t.Context(), b.SessionID, session.Turn{ID: "turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendToolCallStatus(t.Context(), b.SessionID, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
		t.Fatal(err)
	}
	var input, output bytes.Buffer
	if err := workerproto.Write(&input, workerproto.Request{Type: "start", Protocol: workerproto.Version, Bootstrap: &b}); err != nil {
		t.Fatal(err)
	}
	var constructed bool
	available := []providers.Provider{{Name: "fake", NewClient: func(_, _ string, _ int, _ func(string) string) (providers.Client, error) {
		constructed = true
		return newFakeClient(), nil
	}}}
	err = run(t.Context(), &input, &output, io.Discard, func(string) string { t.Fatal("unexpected env access"); return "" }, testIdentity, available)
	if err == nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("Run = %v", err)
	}
	if constructed {
		t.Fatal("provider initialized for unsafe resume")
	}
	if _, err := os.Stat(filepath.Join(b.Workspace, "should-not-exist")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown command relaunched: %v", err)
	}
	lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
}

func TestInvalidProtocolDoesNotReadEnvironmentOrCreateSession(t *testing.T) {
	for _, request := range []workerproto.Request{{Type: "input"}, {Type: "start", Protocol: 99, Bootstrap: &workerproto.Bootstrap{}}, {Type: "start", Protocol: workerproto.Version, Bootstrap: &workerproto.Bootstrap{SessionID: "../bad", Directory: "relative", Workspace: "relative"}}} {
		var input, output bytes.Buffer
		_ = workerproto.Write(&input, request)
		err := Run(t.Context(), &input, &output, io.Discard, func(string) string { t.Fatal("unexpected environment access"); return "" }, testIdentity)
		if !errors.Is(err, ErrProtocol) {
			t.Fatalf("request=%#v, err=%v", request, err)
		}
	}
}

func TestCancelBlockedPipeReader(t *testing.T) {
	b := bootstrap(t)
	w := startWorker(t, b, newFakeClient(), nil)
	w.event(t, "ready")
	w.cancel()
	select {
	case err := <-w.done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run=%v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("blocked pipe input was not joined")
	}
	lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
}

type failWriter struct {
	writes int
	failAt int
}

func (w *failWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes >= w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
func TestOutputFailureStopsAndReleasesLease(t *testing.T) {
	b := bootstrap(t)
	client := newFakeClient()
	var input bytes.Buffer
	_ = workerproto.Write(&input, workerproto.Request{Type: "start", Protocol: workerproto.Version, Bootstrap: &b})
	output := &failWriter{failAt: 3} // hello, settings item, then ready fails
	err := run(t.Context(), &input, output, io.Discard, func(string) string { return "" }, testIdentity, fakeProvider(client))
	if !errors.Is(err, ErrOutput) {
		t.Fatalf("Run=%v", err)
	}
	if output.writes != 3 {
		t.Fatalf("writes after failed frame: %d", output.writes)
	}
	if !client.closed.Load() {
		t.Fatal("provider not closed")
	}
	lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
}

func TestBlockedFilePipeCancellationDoesNotCloseCallerFD(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	// The pump owns deadlines, but not the caller's descriptor.
	pump := newInputPump(input)
	ctx, cancel := context.WithCancel(t.Context())
	next := make(chan error, 1)
	go func() { _, err := pump.next(ctx, nil); next <- err }()
	cancel()
	if err := <-next; !errors.Is(err, context.Canceled) {
		t.Fatalf("next=%v", err)
	}
	joined := make(chan struct{})
	go func() { pump.join(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(10 * time.Second):
		t.Fatal("file pipe pump did not join")
	}
	if _, err := input.Stat(); err != nil {
		t.Fatalf("caller FD was closed: %v", err)
	}
}

func TestMalformedAndOversizedFrames(t *testing.T) {
	for _, data := range []string{"{not json}\n", `{"Type":"start","Unexpected":true}` + "\n", strings.Repeat("x", workerproto.MaxFrame+2) + "\n"} {
		var output bytes.Buffer
		err := Run(t.Context(), strings.NewReader(data), &output, io.Discard, func(string) string { t.Fatal("environment read for invalid frame"); return "" }, testIdentity)
		if !errors.Is(err, ErrProtocol) {
			t.Fatalf("Run invalid frame: %v", err)
		}
	}
	if err := Probe(&failWriter{failAt: 1}, testIdentity); !errors.Is(err, ErrOutput) {
		t.Fatalf("Probe output failure: %v", err)
	}
}

func TestBusyGraphDoesNotRetireAndStopJoinsNestedShell(t *testing.T) {
	b := bootstrap(t)
	b.Subagents = false
	client := newFakeClient()
	w := startWorker(t, b, client, nil)
	w.event(t, "ready")
	payload, _ := json.Marshal("run graph")
	w.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "graph", Kind: inbox.InputExternal, Payload: payload}})
	select {
	case <-client.calls:
	case <-time.After(10 * time.Second):
		t.Fatal("model not started")
	}
	arguments := `{"action":"start","name":"work","tasks":[{"id":"shell","command":"touch started; sleep 30; touch escaped","reads":[],"writes":["started","escaped"],"acceptance":"verify candidate"}]}`
	client.responses <- llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "graph-call", Name: "TaskGraph", Arguments: arguments}}}}
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(b.Workspace, "started")); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("nested shell did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	w.send(t, workerproto.Request{Type: "quiesce", ID: "graph-busy"})
	if e := w.event(t, "busy"); e.ID != "graph-busy" {
		t.Fatalf("busy=%#v", e)
	}
	lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
	if lease != nil {
		_ = lease.Close()
	}
	if !errors.Is(err, localfile.ErrSessionLocked) {
		t.Fatalf("busy graph released lease: %v", err)
	}
	w.send(t, workerproto.Request{Type: "stop"})
	w.finish(t)
	if _, err := os.Stat(filepath.Join(b.Workspace, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("nested shell escaped cancellation: %v", err)
	}
	lease, err = localfile.AcquireLease(b.Directory, b.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
	for e := range w.events {
		if e.Type == "quiesced" {
			t.Fatal("stop falsely acknowledged quiescence")
		}
	}
}

func TestShutdownPersistsTerminalOperationsBeforeResume(t *testing.T) {
	for _, mode := range []string{"stop", "eof", "cancel", "output"} {
		for _, owner := range []string{"shell", "graph"} {
			t.Run(mode+"/"+owner, func(t *testing.T) {
				b := bootstrap(t)
				b.Subagents = false
				client := newFakeClient()
				failedOutput := &switchWriter{}
				w := startWorker(t, b, client, func(output io.Writer) io.Writer {
					failedOutput.output = output
					return failedOutput
				})
				w.event(t, "ready")
				payload, _ := json.Marshal("start owner")
				w.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "work", Kind: inbox.InputExternal, Payload: payload}})
				select {
				case <-client.calls:
				case <-time.After(10 * time.Second):
					t.Fatal("model not started")
				}
				name := "Bash"
				arguments := `{"command":"touch started; sleep 30; touch escaped"}`
				if owner == "graph" {
					name = "TaskGraph"
					arguments = `{"action":"start","name":"work","tasks":[{"id":"shell","command":"touch started; sleep 30; touch escaped","reads":[],"writes":["started","escaped"],"acceptance":"shutdown joins shell"}]}`
				}
				client.responses <- llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "owner", Name: name, Arguments: arguments}}}}
				deadline := time.After(10 * time.Second)
				for {
					if _, err := os.Stat(filepath.Join(b.Workspace, "started")); err == nil {
						break
					}
					select {
					case <-deadline:
						t.Fatal("owner did not start")
					case <-time.After(time.Millisecond):
					}
				}
				switch mode {
				case "stop":
					w.send(t, workerproto.Request{Type: "stop"})
				case "eof":
					if err := w.input.Close(); err != nil {
						t.Fatal(err)
					}
				case "cancel":
					w.cancel()
				case "output":
					failedOutput.fail.Store(true)
					w.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "break-output", Kind: inbox.InputExternal, Payload: payload}})
				}
				select {
				case err := <-w.done:
					if mode == "cancel" {
						if !errors.Is(err, context.Canceled) {
							t.Fatalf("Run=%v", err)
						}
					} else if mode == "output" {
						if !errors.Is(err, ErrOutput) {
							t.Fatalf("Run=%v", err)
						}
					} else if err != nil {
						t.Fatalf("Run=%v", err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("Run did not join")
				}
				if _, err := os.Stat(filepath.Join(b.Workspace, "escaped")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("owner escaped cancellation: %v", err)
				}
				store, err := localfile.New(b.Directory)
				if err != nil {
					t.Fatal(err)
				}
				resumed, err := store.Resume(t.Context(), b.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				for _, op := range resumed.Operations {
					if !terminal(op.Status) {
						t.Errorf("shutdown left unresolved operation: %+v", op)
					}
				}
				// Settled terminal operations disappear from Resume.Operations.
				// Also inspect the latest persisted tool-call snapshots so this
				// assertion cannot pass merely because the resume set is empty.
				latest := make(map[operation.ID]operation.Operation)
				after := sessionstore.BeforeFirst
				for {
					page, err := store.Items(t.Context(), b.SessionID, after, 100)
					if err != nil {
						t.Fatal(err)
					}
					for _, item := range page.Items {
						if status, ok := item.Data.(sessionstore.ToolCallStatus); ok {
							for _, op := range status.Operations {
								latest[op.ID] = op
							}
						}
					}
					if !page.More {
						break
					}
					after = page.NextAfter
				}
				if len(latest) == 0 {
					t.Fatal("no durable owner operations")
				}
				for _, op := range latest {
					if !terminal(op.Status) {
						t.Errorf("history left unresolved operation: %+v", op)
					}
				}
				restartedClient := newFakeClient()
				restarted := startWorker(t, b, restartedClient, nil)
				restarted.event(t, "ready")
				fresh, _ := json.Marshal("fresh-resume")
				restarted.send(t, workerproto.Request{Type: "input", Input: &inbox.Input{ID: "fresh-resume", Kind: inbox.InputExternal, Payload: fresh}})
				// A canceled tool's result may itself need a model acknowledgment
				// after restart. Prove the resident coordinator also accepts fresh
				// work, without mistaking that acknowledgment for tool replay.
				deadline = time.After(10 * time.Second)
				for {
					var request llm.Request
					select {
					case request = <-restartedClient.calls:
					case <-deadline:
						t.Fatal("restarted worker did not accept fresh input")
					}
					containsFresh := false
					for _, item := range request.Input {
						if message, ok := item.Data.(llm.Message); ok && strings.Contains(message.Text, "fresh-resume") {
							containsFresh = true
						}
					}
					restartedClient.responses <- llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "done"}}}}
					if containsFresh {
						break
					}
				}
				for {
					if event := restarted.event(t, "item"); event.Item.Kind == sessionstore.ItemModelResponse {
						break
					}
				}
				restarted.send(t, workerproto.Request{Type: "stop"})
				restarted.finish(t)
				if _, err := os.Stat(filepath.Join(b.Workspace, "escaped")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("restart replayed canceled owner: %v", err)
				}
			})
		}
	}
}
