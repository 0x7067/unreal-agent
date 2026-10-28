package residentcli

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/resident"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

// The thin executable fixture runs the production command dispatcher, headless
// host, client, worker, and provider adapter in separate real processes.
func TestMain(m *testing.M) {
	testtmp.Resolve()
	if Handles(os.Args[1:]) {
		os.Exit(Main(os.Args[1:]))
	}
	os.Exit(m.Run())
}

type lockedOutput struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *lockedOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

type cliProcess struct {
	cmd     *exec.Cmd
	frames  chan []byte
	done    chan struct{}
	discard chan struct{}
	stderr  lockedOutput
	result  error
	once    sync.Once
}

func startCLI(t *testing.T, env []string, args ...string) *cliProcess {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := &cliProcess{cmd: exec.Command(binary, args...), frames: make(chan []byte, 64), done: make(chan struct{}), discard: make(chan struct{})}
	p.cmd.Env = append(os.Environ(), env...)
	p.cmd.Stderr = &p.stderr
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		scanner := attachproto.Scanner(stdout)
		for scanner.Scan() {
			frame := bytes.Clone(scanner.Bytes())
			select {
			case p.frames <- frame:
			case <-p.discard:
			}
		}
		scanErr := scanner.Err()
		p.result = errors.Join(p.cmd.Wait(), scanErr)
		close(p.frames)
		close(p.done)
	}()
	t.Cleanup(func() { p.stop(t) })
	return p
}
func (p *cliProcess) stop(t *testing.T) {
	t.Helper()
	p.once.Do(func() {
		close(p.discard)
		_ = p.cmd.Process.Signal(os.Interrupt)
		timer := time.NewTimer(25 * time.Second)
		defer timer.Stop()
		select {
		case <-p.done:
		case <-timer.C:
			t.Error("CLI did not join after explicit interrupt; killing owned process")
			_ = p.cmd.Process.Kill()
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				t.Error("CLI reap still unconfirmed")
			}
		}
	})
}
func (p *cliProcess) frame(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	select {
	case frame, open := <-p.frames:
		if !open {
			t.Fatalf("CLI exited: %v %s", p.result, p.stderr.String())
		}
		return frame
	case <-ctx.Done():
		t.Fatal("CLI frame deadline:", ctx.Err(), p.stderr.String())
	}
	return nil
}
func (p *cliProcess) event(t *testing.T, ctx context.Context) attachproto.Event {
	t.Helper()
	var event attachproto.Event
	if err := json.Unmarshal(p.frame(t, ctx), &event, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	return event
}
func snapshot(t *testing.T, ctx context.Context, p *cliProcess) (attachproto.Event, attachproto.Event, []sessionstore.Item) {
	t.Helper()
	hello := p.event(t, ctx)
	if hello.Type != "hello" || hello.Connection == "" || hello.Incarnation == "" {
		t.Fatalf("bad hello: %+v", hello)
	}
	var header attachproto.Event
	var items []sessionstore.Item
	for {
		e := p.event(t, ctx)
		if e.Type == "snapshot" {
			header = e
		}
		if e.Item != nil {
			items = append(items, *e.Item)
		}
		if e.Type == "snapshot_end" {
			if sessionstore.Sequence(len(items)) != e.Head || e.Revision != header.Revision {
				t.Fatal("inconsistent snapshot marker")
			}
			return hello, header, items
		}
	}
}

func TestRealHeadlessHostClientDisconnectReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 65*time.Second)
	defer cancel()
	root, err := os.MkdirTemp("/tmp", "ua-m11-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	state, workspace := filepath.Join(root, "state"), filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	const sid session.ID = "headless-test"
	const inputID = "stable-headless-input"
	const text = "one real headless input"
	const answer = "the durable session outlived its client"
	var calls, interrupted atomic.Int64
	started, release := make(chan []byte, 1), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		id := calls.Add(1)
		if id == 1 {
			started <- body
			select {
			case <-release:
			case <-r.Context().Done():
				interrupted.Add(1)
				return
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		response := map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprintf("response-%d", id), "status": "completed", "output": []any{map[string]any{"id": "answer", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": answer}}}}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}}
		data, _ := json.Marshal(response)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}))
	t.Cleanup(server.Close)
	env := []string{"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Join(root, "config"), "OPENAI_API_KEY=fake-loopback-only", "UNREAL_HARNESS_LLM_API_KEY=fake-loopback-only"}
	hostArgs := []string{"host", "-runtime-directory", root, "-session-directory", state, "-workspace", workspace, "-session", string(sid), "-provider", "openai", "-base-url", server.URL, "-model", "fake"}
	host := startCLI(t, env, hostArgs...)
	var ready struct {
		Type string
		resident.Info
	}
	if err := json.Unmarshal(host.frame(t, ctx), &ready); err != nil {
		t.Fatal(err)
	}
	if ready.Type != "host_ready" || ready.HostPID != host.cmd.Process.Pid || ready.Endpoint == "" {
		t.Fatalf("invalid readiness: %+v", ready)
	}
	endpointBefore, err := os.Lstat(ready.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	attachArgs := []string{"attach", "-runtime-directory", root, "-session-directory", state, "-session", string(sid)}
	client := startCLI(t, env, append(append([]string{}, attachArgs...), "-submit", "-input-id", inputID, "-message", text)...)
	hello, firstSnapshot, _ := snapshot(t, ctx, client)
	if hello.Incarnation != ready.Incarnation || hello.HostPID != ready.HostPID || firstSnapshot.Status == nil {
		t.Fatal("client attached to different owner")
	}
	select {
	case body := <-started:
		if bytes.Count(body, []byte(text)) != 1 {
			t.Fatalf("input duplicated in provider request: %s", body)
		}
	case <-ctx.Done():
		t.Fatal("provider not reached:", ctx.Err())
	}
	for {
		e := client.event(t, ctx)
		if e.Type == "accepted" {
			if e.InputID != inputID {
				t.Fatal("wrong accepted identity")
			}
			break
		}
	}
	// Another authenticated peer is refused without disturbing the active one.
	busy, err := attachproto.Dial(ctx, root, state, sid)
	if err != nil {
		t.Fatal(err)
	}
	_ = busy.SetDeadline(time.Now().Add(5 * time.Second))
	if err := attachproto.Write(busy, attachproto.Request{Type: "attach", ID: "busy", Protocol: attachproto.Version, SessionID: sid}); err != nil {
		busy.Close()
		t.Fatal(err)
	}
	busyScanner := attachproto.Scanner(busy)
	refusal, err := attachproto.ReadEvent(busyScanner)
	if err != nil || refusal.Type != "error" || refusal.Code != "busy" {
		busy.Close()
		t.Fatal("second client not explicitly refused", refusal, err)
	}
	if _, err := attachproto.ReadEvent(busyScanner); !errors.Is(err, io.EOF) {
		busy.Close()
		t.Fatal("busy peer handler did not drain", err)
	}
	_ = busy.Close()
	// A second headless process must fail BEFORE replacing/unlinking the endpoint.
	competitor := startCLI(t, env, hostArgs...)
	select {
	case <-competitor.done:
	case <-ctx.Done():
		t.Fatal("competing host did not refuse")
	}
	if competitor.result == nil || !strings.Contains(competitor.stderr.String(), "locked") {
		t.Fatalf("competing host unexpectedly succeeded: %v %s", competitor.result, competitor.stderr.String())
	}
	after, err := os.Lstat(ready.Endpoint)
	if err != nil || !os.SameFile(endpointBefore, after) {
		t.Fatal("competitor replaced endpoint")
	}
	assertLeased := func() {
		t.Helper()
		for _, id := range []session.ID{sid, session.ID("host-" + string(sid))} {
			lease, err := localfile.AcquireLease(state, id)
			if err == nil {
				lease.Close()
				t.Fatalf("owner released %s during detach", id)
			}
			if !errors.Is(err, localfile.ErrSessionLocked) {
				t.Fatal(err)
			}
		}
	}
	assertLeased()
	// Terminate only the client process while the model is still blocked.
	client.stop(t)
	assertLeased()
	if interrupted.Load() != 0 {
		t.Fatal("client disconnection canceled provider request")
	}
	// A partial unauthenticated protocol frame is a client-local failure.
	broken, err := attachproto.Dial(ctx, root, state, sid)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = broken.Write([]byte(`{"Type":"attach"`))
	_ = broken.CloseWrite()
	_ = broken.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.Copy(io.Discard, broken); err != nil {
		t.Fatal("partial handshake did not drain", err)
	}
	_ = broken.Close()
	// The separate client process has no provider/bootstrap flags and does not
	// load even deliberately invalid client-side provider environment settings.
	clientEnv := append(append([]string{}, env...), "UNREAL_HARNESS_LLM_BASE_URL=http://127.0.0.1:1", "OPENAI_API_KEY=not-a-host-credential")
	second := startCLI(t, clientEnv, attachArgs...)
	hello2, snap2, prefix := snapshot(t, ctx, second)
	if hello2.HostPID != hello.HostPID || hello2.Incarnation != hello.Incarnation || snap2.Status == nil || snap2.Status.Generation != firstSnapshot.Status.Generation || snap2.Status.Stopped {
		t.Fatal("reconnect replaced/stopped worker or host")
	}
	if len(prefix) != 4 {
		t.Fatalf("blocked turn snapshot has %d items, want settings/input/request-charge/turn", len(prefix))
	}
	settings, ok := prefix[0].Data.(inbox.Input)
	if !ok {
		t.Fatal("snapshot missing settings")
	}
	setting, err := settings.DecodeControlMessage()
	if err != nil || setting.Mode != inbox.UpdateSettings {
		t.Fatal("snapshot settings changed")
	}
	external, ok := prefix[1].Data.(inbox.Input)
	if !ok || external.Kind != inbox.InputExternal {
		t.Fatal("snapshot missing external input")
	}
	charged, ok := prefix[2].Data.(inbox.Input)
	if !ok {
		t.Fatal("snapshot missing request charge")
	}
	chargeControl, err := charged.DecodeControlMessage()
	if err != nil || chargeControl.Mode != inbox.ModelRequest {
		t.Fatalf("snapshot charge=%+v %v", chargeControl, err)
	}
	request := chargeControl.Parameters.(inbox.ModelRequestRecord)
	turn, ok := prefix[3].Data.(session.Turn)
	if !ok || request.Version != 1 || request.Action != "started" || request.TurnID != string(turn.ID) || request.TurnType != string(turn.Type) || request.RequestID == "" || request.Attempt != 1 || request.Limit != 2 || request.Inputs != 1 || request.ExternalInputs != 1 {
		t.Fatalf("snapshot has invalid charge-before-turn: %+v %+v", request, turn)
	}
	assertLeased()
	if interrupted.Load() != 0 || calls.Load() != 1 {
		t.Fatal("failed connection or reconnect repeated/canceled provider work")
	}
	releaseOnce.Do(func() { close(release) })
	items := append([]sessionstore.Item(nil), prefix...)
	for {
		e := second.event(t, ctx)
		if e.Item != nil {
			if e.Item.Sequence != sessionstore.Sequence(len(items)+1) {
				t.Fatal("live history discontinuity")
			}
			items = append(items, *e.Item)
		}
		if e.Item != nil && e.Item.Kind == sessionstore.ItemModelResponse {
			encoded, err := json.Marshal(e.Item.Data)
			if err != nil || !bytes.Contains(encoded, []byte(answer)) {
				t.Fatalf("committed response lacks expected answer: %s %v", encoded, err)
			}
			break
		}
	}
	second.stop(t)
	host.stop(t)
	if host.result != nil {
		t.Fatalf("host graceful stop: %v %s", host.result, host.stderr.String())
	}
	lease, err := localfile.AcquireLease(state, sid)
	if err != nil {
		t.Fatal("worker did not release session lease:", err)
	}
	defer lease.Close()
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Items(ctx, sid, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != len(items)+2 || !reflect.DeepEqual(items, page.Items[:len(items)]) {
		t.Fatalf("client canonical prefix differs: client=%d store=%d", len(items), len(page.Items))
	}
	// Legacy host stop follows detach, records its stop control, then the new
	// strong runtime's independently joined execution-drain receipt. Neither
	// tail item is a model turn or an update missed while the client was live.
	shutdown, ok := page.Items[len(items)].Data.(inbox.Input)
	if !ok || shutdown.Kind != inbox.InputControl {
		t.Fatal("unexpected post-detach record")
	}
	control, err := shutdown.DecodeControlMessage()
	if err != nil || control.Mode != inbox.StopHard || control.Reason != "worker shutdown" {
		t.Fatal("unexpected shutdown suffix", control, err)
	}
	drain, ok := page.Items[len(items)+1].Data.(inbox.Input)
	if !ok || drain.Kind != inbox.InputControl {
		t.Fatal("missing canonical cleanup tail")
	}
	receipt, err := drain.DecodeControlMessage()
	if err != nil || receipt.Mode != inbox.ExecutionDrained {
		t.Fatalf("unexpected cleanup tail: %+v %v", receipt, err)
	}
	if receipt.Parameters.(inbox.ExecutionDrainReceipt).Scope == "" {
		t.Fatal("unscoped cleanup tail")
	}

	if calls.Load() != 1 || interrupted.Load() != 0 {
		t.Fatalf("requests/interrupted=%d/%d, want 1/0", calls.Load(), interrupted.Load())
	}
	if _, err := os.Lstat(ready.Endpoint); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned endpoint not removed on stop: %v", err)
	}
	t.Logf("host pid=%d incarnation=%s worker generation=%d survived separate-client death/reconnect; one input and one committed answer", hello.HostPID, hello.Incarnation, firstSnapshot.Status.Generation)
}

func TestAttachAndProbeDoNotLoadProviderConfiguration(t *testing.T) {
	for _, args := range [][]string{{"attach", "-h"}, {"host", "-h"}, {"worker-probe"}, {"attach", "-provider", "untrusted"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Run(t.Context(), args, func(name string) string { t.Fatalf("unexpected config/environment read %s", name); return "" }, nil, &out, &stderr)
			if args[0] == "worker-probe" && err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, args := range [][]string{nil, {"-p", "hello"}, {"other"}} {
		if Handles(args) {
			t.Fatal("one-shot runner was hijacked")
		}
	}
}
