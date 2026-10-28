package resident

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func testPublisher() *publisher {
	return &publisher{info: Info{SessionID: "test", Incarnation: "host"}, available: true, seen: map[inbox.ID]bool{}}
}
func testClient() *client {
	return &client{token: "client", queue: make(chan []byte, attachproto.MaxQueuedFrames), stopped: make(chan struct{})}
}

func TestPublisherAdmissionPersistenceAndRetryCapacity(t *testing.T) {
	p := testPublisher()
	in := inbox.Input{ID: "accepted", Kind: inbox.InputExternal, Payload: []byte(`"message"`)}
	p.admission(in)
	in.Payload[1] = 'X'
	if len(p.pending) != 1 || string(p.pending[0].Payload) != `"message"` {
		t.Fatal("admission mirror aliases caller")
	}
	p.item(sessionstore.Item{Sequence: 1, Kind: sessionstore.ItemInput, Data: p.pending[0]})
	if len(p.pending) != 0 || p.pendingBytes != 0 || !p.seen[in.ID] {
		t.Fatal("persistence did not reconcile pending")
	}
	p.admission(in)
	if len(p.pending) != 0 {
		t.Fatal("late admission callback resurrected persisted input")
	}
	for i := 0; i < 64; i++ {
		p.admission(inbox.Input{ID: inbox.ID(fmt.Sprintf("p-%d", i)), Kind: inbox.InputExternal, Payload: []byte(`"queued"`)})
	}
	if p.canAdmit(inbox.Input{ID: "new"}) || !p.canAdmit(inbox.Input{ID: "p-0"}) || !p.canAdmit(inbox.Input{ID: "accepted"}) {
		t.Fatal("capacity failed to distinguish new input from accepted retry")
	}
	p.admission(inbox.Input{ID: "recovered-overflow", Kind: inbox.InputExternal, Payload: []byte(`"queued"`)})
	if p.available || len(p.pending) != 64 || p.canAdmit(inbox.Input{ID: "new"}) {
		t.Fatal("recovery overflow must remain unavailable, bounded and mutation-fenced")
	}
	p.statusChanged(workerhost.Status{Err: errors.New("uncertain")})
	if p.available {
		t.Fatal("status error exposed complete pending state")
	}
}

func TestPublisherOverflowInvalidatesAndClearsOnlyClient(t *testing.T) {
	p := testPublisher()
	c := testClient()
	p.active = c
	for range attachproto.MaxQueuedFrames {
		p.admission(inbox.Input{ID: inbox.ID(fmt.Sprintf("q-%d", p.revision)), Kind: inbox.InputExternal, Payload: []byte(`"message"`)})
	}
	// Admission mirror has its own limit; drive status events independently.
	for range attachproto.MaxQueuedFrames + 1 {
		p.statusChanged(workerhost.Status{Generation: 1})
	}
	select {
	case <-c.stopped:
	default:
		t.Fatal("queue overflow did not invalidate subscription")
	}
	if len(c.queue) != 0 || c.bytes != 0 {
		t.Fatal("invalidated queue retained payloads")
	}
	revision := p.revision
	p.statusChanged(workerhost.Status{Generation: 1})
	if p.revision != revision+1 || len(c.queue) != 0 || c.bytes != 0 {
		t.Fatal("stopped client blocked further publication or retained frames")
	}
}

func TestServerRejectsUnsafeRuntimeBeforeWorkerStartup(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "must-not-exist")
	err := Serve(context.Background(), Config{RuntimeDirectory: root, Worker: workerhost.Config{Command: []string{"/must/not/be/executed", "worker"}, Bootstrap: workerproto.Bootstrap{Directory: state, Workspace: root, SessionID: "test"}}}, nil)
	if err == nil {
		t.Fatal("unsafe runtime accepted")
	}
	if _, err := os.Stat(state); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid transport started worker/session ownership", err)
	}
}

func TestRejectedOrStaleRequestCannotReachHostMutation(t *testing.T) {
	base := func() attachproto.Request {
		return attachproto.Request{Type: "submit", ID: "request", Protocol: attachproto.Version, SessionID: "test", Connection: "client", Input: &inbox.Input{ID: "input", Kind: inbox.InputExternal, Payload: []byte(`"message"`)}}
	}
	for _, test := range []struct {
		name   string
		change func(*attachproto.Request)
	}{
		{"version", func(r *attachproto.Request) { r.Protocol++ }},
		{"session", func(r *attachproto.Request) { r.SessionID = "different" }},
		{"connection", func(r *attachproto.Request) { r.Connection = "stale" }},
		{"cursor", func(r *attachproto.Request) { r.After = 1 }},
		{"control", func(r *attachproto.Request) {
			r.Input.Kind = inbox.InputControl
			r.Input.Payload = []byte(`{"Mode":"stop_hard"}`)
		}},
		{"shutdown", func(r *attachproto.Request) { r.Type = "shutdown" }},
		{"missing_correlation", func(r *attachproto.Request) { r.ID = "" }},
		{"missing_input", func(r *attachproto.Request) { r.Input = nil }},
		{"large_payload", func(r *attachproto.Request) {
			r.Input.Payload = append(append([]byte{'"'}, bytes.Repeat([]byte{'x'}, attachproto.MaxPayload)...), '"')
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := testPublisher()
			c := testClient()
			p.active = c
			r := base()
			test.change(&r)
			frame, err := attachproto.Encode(r)
			if err != nil {
				t.Fatal(err)
			}
			readRequests(t.Context(), p, nil, c, attachproto.Scanner(bytes.NewReader(frame)))
			if p.revision != 0 || len(p.pending) != 0 {
				t.Fatal("rejected request mutated publisher")
			}
		})
	}
	for _, stopped := range []bool{false, true} {
		p := testPublisher()
		c := testClient()
		if stopped {
			c.stop()
			p.active = c
		}
		frame, err := attachproto.Encode(base())
		if err != nil {
			t.Fatal(err)
		}
		readRequests(t.Context(), p, nil, c, attachproto.Scanner(bytes.NewReader(frame)))
	}
}
