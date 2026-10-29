package resident

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

// A raw watcher has deliberately no hidden reader. Only the provider is fake;
// transport, host, worker, ingress, session store and Bash executor are real.
func TestAdversarialStalledWatcherRealWorker(t *testing.T) {
	// Overall budget covers stalled-client publication plus all malformed-client
	// handshakes under race instrumentation. Per-attack deadlines stay at 2s.
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	root, err := testtmp.MkdirTemp("uad-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	runtime, state := filepath.Join(root, "r"), filepath.Join(root, "s")
	for _, p := range []string{runtime, state} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	const sid session.ID = "adversarial-session"
	seed := inbox.Input{ID: "seed", Kind: inbox.InputExternal, Payload: []byte(`"seed question"`)}
	release := func() {
		if err := os.WriteFile(filepath.Join(root, "release"), nil, 0600); err != nil {
			t.Error(err)
		}
	}
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, io.LimitReader(r.Body, 16<<20)); err != nil {
			return
		}
		n := calls.Add(1)
		output := []map[string]any{{"id": fmt.Sprint("answer-", n), "type": "message", "role": "assistant", "status": "completed", "content": []map[string]any{{"type": "output_text", "text": "adversarial canonical answer"}}}}
		if n == 1 {
			args, _ := json.Marshal(map[string]any{"command": "printf x >> effect; printf partial-output; touch started; while [ ! -f release ]; do sleep .02; done; printf final-output"})
			output = []map[string]any{{"id": "seed-function", "type": "function_call", "call_id": "seed-tool", "name": "Bash", "arguments": string(args), "status": "completed"}}
		}
		data, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprint("response-", n), "status": "completed", "output": output, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", data)
	}))
	t.Cleanup(provider.Close)
	var mu sync.Mutex
	admitted := map[inbox.ID]inbox.Input{}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hostCtx, stop := context.WithCancel(ctx)
	ready, done := make(chan Info, 1), make(chan error, 1)
	go func() {
		done <- Serve(hostCtx, Config{RuntimeDirectory: runtime, Worker: workerhost.Config{Command: []string{exe, "worker"}, ShutdownGrace: 2 * time.Second, Bootstrap: workerproto.Bootstrap{SessionID: sid, Directory: state, Workspace: root, Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh"}, OnAdmission: func(in inbox.Input) { mu.Lock(); admitted[in.ID] = in; mu.Unlock() }}}, func(i Info) { ready <- i })
	}()
	t.Cleanup(func() {
		release()
		stop()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("host shutdown: %v", err)
			} else {
				t.Log("resident Serve joined (including worker Close), not merely socket EOF")
			}
		case <-time.After(6 * time.Second):
			t.Error("host join unconfirmed")
		}
	})
	var info Info
	select {
	case info = <-ready:
	case err := <-done:
		t.Fatalf("startup: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	dial := func(t *testing.T) (*net.UnixConn, *bufio.Scanner, attachproto.Event, attachproto.Event) {
		t.Helper()
		var c *net.UnixConn
		var hello, header attachproto.Event
		for {
			var err error
			c, err = attachproto.Dial(ctx, runtime, state, sid)
			if err != nil {
				t.Fatal(err)
			}
			deadline, _ := ctx.Deadline()
			c.SetDeadline(deadline)
			if err = attachproto.Write(c, attachproto.Request{Type: "attach", ID: "attach", Protocol: attachproto.Version, SessionID: sid}); err != nil {
				t.Fatal(err)
			}
			s := attachproto.Scanner(c)
			hello, err = attachproto.ReadEvent(s)
			if err != nil {
				c.Close()
				t.Fatal(err)
			}
			if hello.Code == "busy" {
				c.Close()
				select {
				case <-time.After(time.Millisecond):
					continue
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			if hello.Type != "hello" {
				t.Fatalf("hello: %+v", hello)
			}
			for {
				e, err := attachproto.ReadEvent(s)
				if err != nil {
					t.Fatal(err)
				}
				if e.Type == "snapshot" {
					header = e
				}
				if e.Type == "snapshot_end" {
					return c, s, hello, header
				}
			}
		}
	}
	c, scanner, hello, header := dial(t)
	t.Cleanup(func() { c.Close() })
	c.SetReadBuffer(1024)
	request := func(id string, in inbox.Input) attachproto.Request {
		return attachproto.Request{Type: "submit", ID: id, Protocol: attachproto.Version, SessionID: sid, Connection: hello.Connection, Input: &in}
	}
	if err := attachproto.Write(c, request("seed-request", seed)); err != nil {
		t.Fatal(err)
	}
	for {
		e, err := attachproto.ReadEvent(scanner)
		if err != nil {
			t.Fatal(err)
		}
		if e.Type == "accepted" && e.InputID == seed.ID {
			break
		}
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "started")); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Bash not started", ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	// The marker follows partial stdout. Public attach exposes committed history,
	// not partial operation output: no assertion that this stdout was streamed.
	if other, err := attachclient.Dial(ctx, runtime, state, sid); err == nil {
		other.Close()
		t.Fatal("competing watcher admitted")
	} else if !strings.Contains(err.Error(), "busy") {
		t.Fatal(err)
	}
	// Stop reading c permanently. Large external requests exercise actual pending
	// publication, bounded queue and socket backpressure. The sender is joined.
	sent := make(chan error, 1)
	go func() {
		for i := 0; i < 48; i++ {
			payload, _ := json.Marshal(fmt.Sprintf("batch-%02d-", i) + strings.Repeat("q", 240<<10))
			in := inbox.Input{ID: inbox.ID(fmt.Sprintf("batch-%02d", i)), Kind: inbox.InputExternal, Payload: payload}
			if err := attachproto.Write(c, request(fmt.Sprint("request-", i), in)); err != nil {
				sent <- err
				return
			}
		}
		sent <- nil
	}()
	var healthy *attachclient.Client
	for {
		healthy, err = attachclient.Dial(ctx, runtime, state, sid)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "busy") {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("stalled watcher not fenced", ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	select {
	case <-sent:
	case <-ctx.Done():
		t.Fatal("batch sender did not join", ctx.Err())
	}
	snap := healthy.Snapshot()
	if snap.Hello.HostPID != info.HostPID || snap.Hello.Incarnation != hello.Incarnation || snap.Header.Status.Generation != header.Status.Generation {
		t.Fatal("host/worker identity changed")
	}
	readerDone := faultReader(t, healthy)
	if err := healthy.Submit(ctx, seed); err != nil {
		t.Fatal("proven accepted seed retry:", err)
	}
	if err := healthy.Submit(ctx, inbox.Input{ID: "healthy-new", Kind: inbox.InputExternal, Payload: []byte(`"new healthy input while Bash is blocked"`)}); err != nil {
		t.Fatal("healthy new admission while tool runs:", err)
	}
	mu.Lock()
	accepted := make(map[inbox.ID]inbox.Input, len(admitted))
	for k, v := range admitted {
		accepted[k] = v
	}
	mu.Unlock()
	if len(accepted) < 2 {
		t.Fatal("pressure admitted no queued input")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "effect")); string(b) != "x" {
		t.Fatalf("seed effect before release %q", b)
	}
	release()
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	for {
		counts := map[inbox.ID]int{}
		answers, toolCalls, toolResults := 0, 0, 0
		var after sessionstore.Sequence
		for {
			page, err := store.Items(ctx, sid, after, 128)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range page.Items {
				switch d := item.Data.(type) {
				case inbox.Input:
					if d.Kind == inbox.InputExternal {
						counts[d.ID]++
						if want, ok := accepted[d.ID]; ok && !bytes.Equal(want.Payload, d.Payload) {
							t.Fatal("accepted payload changed")
						}
					}
				case sessionstore.ModelResponse:
					for _, v := range d.Response.Output {
						if call, ok := v.Data.(llm.ToolCall); ok && call.CallID == "seed-tool" {
							toolCalls++
						}
						if v.Type == llm.ItemMessage {
							answers++
						}
					}
				case sessionstore.ToolCallStatus:
					encoded, _ := json.Marshal(d)
					if bytes.Contains(encoded, []byte("final-output")) {
						toolResults++
					}
				}
			}
			if !page.More {
				break
			}
			after = page.NextAfter
		}
		complete := answers > 0 && toolCalls == 1 && toolResults > 0
		for id := range accepted {
			complete = complete && counts[id] == 1
		}
		if counts[seed.ID] > 1 {
			t.Fatal("duplicate canonical seed")
		}
		if complete {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("canonical convergence counts=%v answers=%d calls=%d results=%d: %v", counts, answers, toolCalls, toolResults, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "effect")); string(b) != "x" {
		t.Fatalf("duplicate Bash effect %q", b)
	}
	healthy.Close()
	select {
	case <-readerDone:
	case <-ctx.Done():
		t.Fatal("healthy reader not joined")
	}
	// Every attack uses a real same-UID socket and production decoder, with a
	// valid attach first so rejection is not merely a failed handshake. None may
	// reach the ingress observer. Cross-UID SO_PEERCRED needs another UID fixture.
	for _, attack := range []string{"protocol", "session", "token", "after", "control", "shutdown", "oversized_payload", "oversized_frame", "partial_frame"} {
		t.Run(attack, func(t *testing.T) {
			conn, scan, greeting, _ := dial(t)
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			bad := attachproto.Request{Type: "submit", ID: "attack-request", Protocol: attachproto.Version, SessionID: sid, Connection: greeting.Connection, Input: &inbox.Input{ID: inbox.ID("rejected-" + attack), Kind: inbox.InputExternal, Payload: []byte(`"must not be admitted"`)}}
			switch attack {
			case "protocol":
				bad.Protocol++
			case "session":
				bad.SessionID = "wrong-session"
			case "token":
				bad.Connection = "forged-token"
			case "after":
				bad.After = 1
			case "control":
				bad.Input.Kind = inbox.InputControl
				bad.Input.Payload = []byte(`{"Mode":"stop_hard"}`)
			case "shutdown":
				bad.Type = "shutdown"
			case "oversized_payload":
				bad.Input.Payload = append(append([]byte{'"'}, bytes.Repeat([]byte{'x'}, attachproto.MaxPayload)...), '"')
			}
			switch attack {
			case "oversized_frame":
				_, _ = conn.Write(append(bytes.Repeat([]byte{'x'}, attachproto.MaxFrame+1), '\n'))
			case "partial_frame":
				raw, _ := attachproto.Encode(bad)
				_, _ = conn.Write(raw[:len(raw)-1])
				if err := conn.CloseWrite(); err != nil {
					t.Fatal(err)
				}
			default:
				if err := attachproto.Write(conn, bad); err != nil {
					t.Fatal(err)
				}
			}
			// An explicit close is required. A timeout would not demonstrate rejection.
			for {
				_, err := attachproto.ReadEvent(scan)
				if err != nil {
					if e, ok := err.(net.Error); ok && e.Timeout() {
						t.Fatalf("attack not rejected: %v", err)
					}
					break
				}
			}
			mu.Lock()
			_, mutated := admitted[bad.Input.ID]
			mu.Unlock()
			if mutated {
				t.Fatal("rejected request mutated durable ingress")
			}
		})
	}
	t.Logf("same-UID real transport; %d accepted identities preserved; provider requests=%d (batching intentionally unconstrained)", len(accepted), calls.Load())
}

// Exact accounting at both configured limits, independently summing encoded
// frames. No socket writer can drain these fixtures or hide retained bytes.
func TestAdversarialMeasuredPublicationBounds(t *testing.T) {
	for _, large := range []bool{false, true} {
		name := "frames"
		if large {
			name = "bytes"
		}
		t.Run(name, func(t *testing.T) {
			p := &publisher{}
			c := &client{queue: make(chan []byte, attachproto.MaxQueuedFrames), stopped: make(chan struct{})}
			e := attachproto.Event{Type: "pending", Protocol: attachproto.Version, SessionID: "bounds"}
			if large {
				e.Pending = []inbox.Input{{ID: "large", Kind: inbox.InputExternal, Payload: append(append([]byte{'"'}, bytes.Repeat([]byte{'x'}, 250<<10)...), '"')}}
			}
			frame, err := attachproto.Encode(e)
			if err != nil {
				t.Fatal(err)
			}
			limit := attachproto.MaxQueuedFrames
			if n := attachproto.MaxQueuedBytes / len(frame); n < limit {
				limit = n
			}
			for i := 0; i < limit; i++ {
				p.enqueue(c, e)
				if len(c.queue) != i+1 || c.bytes != (i+1)*len(frame) {
					t.Fatalf("after %d: frames=%d bytes=%d frame-size=%d", i+1, len(c.queue), c.bytes, len(frame))
				}
			}
			measured := 0
			for i := 0; i < limit; i++ {
				f := <-c.queue
				measured += len(f)
				c.queue <- f
			}
			if measured != c.bytes {
				t.Fatalf("measured/accounted bytes=%d/%d", measured, c.bytes)
			}
			if !large && len(c.queue) != 256 {
				t.Fatal("frame boundary not reached")
			}
			if large && (c.bytes > 8<<20 || c.bytes+len(frame) <= 8<<20) {
				t.Fatal("byte boundary not reached")
			}
			t.Logf("before overflow frames=%d bytes=%d next-frame=%d", len(c.queue), measured, len(frame))
			p.enqueue(c, e)
			select {
			case <-c.stopped:
			default:
				t.Fatal("overflow did not fence peer")
			}
			if c.bytes != 0 || len(c.queue) != 0 {
				t.Fatal("fenced peer retained queue")
			}
		})
	}
}
