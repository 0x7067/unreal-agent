package resident

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
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
)

// faultProxy owns one connection in each direction. All framing passes through
// the production bounded strict decoders; accepted is never forwarded. The cut
// is causal: a decoded submit, confirmed journal pending, or canonical input.
func faultProxy(t *testing.T, ctx context.Context, public, backend, state string, sid session.ID, id inbox.ID, cut string) <-chan error {
	t.Helper()
	ep, err := attachproto.Listen(public, state, sid)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { _ = ep.Close() })
	t.Cleanup(func() {
		stop()
		_ = ep.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("proxy did not join")
		}
	})
	go func() {
		defer close(done)
		front, err := ep.Listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		defer front.Close()
		deadline, _ := ctx.Deadline()
		_ = front.SetDeadline(deadline)
		frontScanner := attachproto.Scanner(front)
		hello, err := attachproto.ReadRequest(frontScanner)
		if err != nil {
			done <- err
			return
		}
		// Closing the old client's reader is not a server detach receipt.
		// Retry only explicit pre-admission busy responses, on fresh backend
		// sockets, without dropping/recreating the fault proxy's frontend.
		// Once admitted, every frame/cut below is still relayed exactly once.
		var back *net.UnixConn
		var backScanner *bufio.Scanner
		for {
			candidate, e := attachproto.Dial(ctx, backend, state, sid)
			if e != nil {
				done <- e
				return
			}
			_ = candidate.SetDeadline(deadline)
			stopCandidate := context.AfterFunc(ctx, func() { _ = candidate.Close() })
			if e = attachproto.Write(candidate, hello); e != nil {
				stopCandidate()
				_ = candidate.Close()
				done <- e
				return
			}
			scanner := attachproto.Scanner(candidate)
			greeting, e := attachproto.ReadEvent(scanner)
			if e != nil {
				stopCandidate()
				_ = candidate.Close()
				done <- e
				return
			}
			if greeting.Type == "error" && greeting.Code == "busy" {
				stopCandidate()
				_ = candidate.Close()
				select {
				case <-ctx.Done():
					done <- ctx.Err()
					return
				case <-time.After(10 * time.Millisecond):
					continue
				}
			}
			stopCandidate()
			if e = attachproto.Write(front, greeting); e != nil {
				_ = candidate.Close()
				done <- e
				return
			}
			back, backScanner = candidate, scanner
			break
		}
		defer back.Close()
		stopIO := context.AfterFunc(ctx, func() { _ = front.Close(); _ = back.Close() })
		defer stopIO()

		results := make(chan error, 2)
		var owners sync.WaitGroup
		owners.Add(2)
		var once sync.Once
		closeBoth := func() { once.Do(func() { _ = front.Close(); _ = back.Close() }) }
		go func() {
			defer owners.Done()
			scanner := frontScanner
			for {
				r, err := attachproto.ReadRequest(scanner)
				if err != nil {
					results <- err
					closeBoth()
					return
				}
				if r.Type == "submit" && cut == "before_admission" {
					results <- nil
					closeBoth()
					return
				}
				if err := attachproto.Write(back, r); err != nil {
					results <- err
					closeBoth()
					return
				}
			}
		}()
		go func() {
			defer owners.Done()
			scanner := backScanner
			for {
				e, err := attachproto.ReadEvent(scanner)
				if err != nil {
					results <- err
					closeBoth()
					return
				}
				matched := false
				if cut == "journal_before_ack" && e.Type == "pending" {
					for _, in := range e.Pending {
						matched = matched || in.ID == id
					}
				}
				if cut == "persisted_before_ack" && e.Type == "item" && e.Item != nil {
					in, ok := e.Item.Data.(inbox.Input)
					matched = ok && in.ID == id
				}
				if matched {
					results <- nil
					closeBoth()
					return
				}
				if e.Type == "accepted" {
					continue
				}
				if err := attachproto.Write(front, e); err != nil {
					results <- err
					closeBoth()
					return
				}
			}
		}()
		a, b := <-results, <-results
		owners.Wait()
		// One nil is the witnessed cut, the other owner exits due to socket close.
		if a != nil && b != nil {
			done <- errors.Join(a, b)
		} else {
			done <- nil
		}
	}()
	return done
}

// Next must remain active while Submit waits. Closing a client explicitly joins
// its sole reader; no test goroutine is left borrowing its scanner.
func faultReader(t *testing.T, c *attachclient.Client) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := c.Next(); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = c.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("client reader did not join")
		}
	})
	return done
}

func TestSubmissionFaultStableIdentity(t *testing.T) {
	for _, cut := range []string{"before_admission", "journal_before_ack", "persisted_before_ack"} {
		for _, cursorMode := range []string{"fresh", "valid", "expired", "invalid"} {
			t.Run(cut+"/"+cursorMode, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 18*time.Second)
				defer cancel()
				root, err := os.MkdirTemp("/tmp", "ua-fault-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.RemoveAll(root) })
				root, err = filepath.EvalSymlinks(root)
				if err != nil {
					t.Fatal(err)
				}
				backend, public, state := filepath.Join(root, "b"), filepath.Join(root, "p"), filepath.Join(root, "s")
				for _, dir := range []string{backend, public, state} {
					if err := os.Mkdir(dir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				const sid session.ID = "fault-session"
				const id inbox.ID = "stable-input"
				const text = "exactly one durable question"
				const answer = "exactly one durable answer"
				payload, _ := json.Marshal(text)
				in := inbox.Input{ID: id, Kind: inbox.InputExternal, Payload: payload}
				release := make(chan struct{})
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				started := make(chan []byte, 4)
				var calls, interrupted atomic.Int64
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
					if err != nil {
						return
					}
					n := calls.Add(1)
					select {
					case started <- body:
					default:
					}
					select {
					case <-release:
					case <-r.Context().Done():
						interrupted.Add(1)
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					event := map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprintf("response-%d", n), "status": "completed", "output": []any{map[string]any{"id": "answer", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": answer}}}}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}}
					data, _ := json.Marshal(event)
					_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				}))
				t.Cleanup(provider.Close)
				t.Cleanup(unblock)
				exe, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				hostCtx, stopHost := context.WithCancel(ctx)
				ready, hostDone := make(chan Info, 1), make(chan error, 1)
				responsePersisted := make(chan struct{})
				var responseOnce sync.Once
				go func() {
					hostDone <- Serve(hostCtx, Config{RuntimeDirectory: backend, Worker: workerhost.Config{
						Command: []string{exe, "worker"}, ShutdownGrace: 3 * time.Second,
						Bootstrap: workerproto.Bootstrap{SessionID: sid, Directory: state, Workspace: root, Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh"},
						OnItem: func(item sessionstore.Item) {
							if item.Kind == sessionstore.ItemModelResponse {
								responseOnce.Do(func() { close(responsePersisted) })
							}
						},
					}}, func(info Info) { ready <- info })
				}()
				t.Cleanup(func() {
					unblock()
					stopHost()
					select {
					case err := <-hostDone:
						if err != nil {
							t.Errorf("host close: %v", err)
						}
					case <-time.After(8 * time.Second):
						t.Error("host did not join")
					}
				})
				select {
				case <-ready:
				case err := <-hostDone:
					t.Fatalf("host startup: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				proxyDone := faultProxy(t, ctx, public, backend, state, sid, id, cut)
				first, err := attachclient.Dial(ctx, public, state, sid)
				if err != nil {
					t.Fatal(err)
				}
				initial := first.Snapshot()
				cursor := &attachproto.Cursor{Incarnation: initial.Hello.Incarnation, Revision: initial.Header.Revision, Head: initial.Header.Head}
				switch cursorMode {
				case "fresh":
					cursor = nil
				case "expired":
					cursor.Revision = 0
					cursor.Head = 0
				case "invalid":
					cursor.Revision += 1 << 40
					cursor.Head += 1 << 40
				}
				firstReader := faultReader(t, first)
				if err := first.Submit(ctx, in); !errors.Is(err, attachclient.ErrUncertain) {
					t.Fatalf("fault reply = %v, want uncertain", err)
				}
				_ = first.Close()
				select {
				case <-firstReader:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				select {
				case err := <-proxyDone:
					if err != nil {
						t.Fatal("causal fault not reached:", err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				// A closed peer's handler may still be releasing its exclusive slot. Retry
				// only the explicit busy handshake, bounded by the test deadline.
				var retry *attachclient.Client
				for {
					retry, err = attachclient.DialFrom(ctx, backend, state, sid, cursor)
					if err == nil {
						break
					}
					if !strings.Contains(err.Error(), "busy") {
						t.Fatal(err)
					}
					select {
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					case <-time.After(time.Millisecond):
					}
				}
				resumed := retry.Snapshot()
				if cursorMode == "valid" {
					if resumed.Header.Base == nil || *resumed.Header.Base != *cursor {
						t.Fatal("valid cursor did not resume", resumed.Header)
					}
				} else if resumed.Header.Base != nil {
					t.Fatal("unexpected resume")
				}
				if cursorMode == "expired" && resumed.Header.Code != "cursor_expired" {
					t.Fatal("missing expired fallback", resumed.Header)
				}
				if cursorMode == "invalid" && resumed.Header.Code != "cursor_invalid" {
					t.Fatal("missing invalid fallback", resumed.Header)
				}
				base := sessionstore.Sequence(0)
				if resumed.Header.Base != nil {
					base = resumed.Header.Base.Head
				}
				for i, item := range resumed.Items {
					if item.Sequence != base+sessionstore.Sequence(i)+1 {
						t.Fatal("missing or duplicate delta item", item.Sequence)
					}
				}
				faultReader(t, retry)
				if err := retry.Submit(ctx, in); err != nil {
					t.Fatal("stable retry:", err)
				}
				changed := in
				changed.Payload, _ = json.Marshal("conflicting different payload")
				err = retry.Submit(ctx, changed)
				var rejection *attachclient.ReplyError
				if !errors.As(err, &rejection) || rejection.Code != "conflict" || rejection.InputID != id {
					t.Fatalf("conflict reply = %v", err)
				}
				select {
				case body := <-started:
					if bytes.Count(body, []byte(text)) != 1 {
						t.Fatalf("request input cardinality: %s", body)
					}
				case <-ctx.Done():
					t.Fatal("provider not reached", ctx.Err())
				}
				if calls.Load() != 1 || interrupted.Load() != 0 {
					t.Fatalf("before release requests/interrupted = %d/%d", calls.Load(), interrupted.Load())
				}
				unblock()
				select {
				case <-responsePersisted:
				case <-ctx.Done():
					t.Fatal("answer not persisted", ctx.Err())
				}
				store, err := localfile.New(state)
				if err != nil {
					t.Fatal(err)
				}
				var all []sessionstore.Item
				var after sessionstore.Sequence
				for {
					page, err := store.Items(ctx, sid, after, 128)
					if err != nil {
						t.Fatal(err)
					}
					all = append(all, page.Items...)
					if !page.More {
						break
					}
					if page.NextAfter <= after {
						t.Fatal("non-progressing canonical page")
					}
					after = page.NextAfter
				}
				inputs, turns, answers := 0, 0, 0
				for i, item := range all {
					if item.Sequence != sessionstore.Sequence(i+1) {
						t.Fatal("canonical sequence gap")
					}
					switch data := item.Data.(type) {
					case inbox.Input:
						if data.Kind == inbox.InputExternal {
							inputs++
							if data.ID != id || !bytes.Equal(data.Payload, in.Payload) {
								t.Fatalf("unexpected canonical input: %+v", data)
							}
						}
					case session.Turn:
						turns++
					case sessionstore.ModelResponse:
						answers++
						encoded, err := json.Marshal(data.Response)
						if err != nil || !bytes.Contains(encoded, []byte(answer)) {
							t.Fatalf("canonical answer: %s %v", encoded, err)
						}
					}
				}
				if inputs != 1 || turns != 1 || answers != 1 || calls.Load() != 1 || interrupted.Load() != 0 {
					t.Fatalf("inputs/turns/answers/requests/interrupted = %d/%d/%d/%d/%d", inputs, turns, answers, calls.Load(), interrupted.Load())
				}
			})
		}
	}
}
