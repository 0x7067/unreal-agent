package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

type uncertainInputWriter struct{ input inbox.Input }

func (w *uncertainInputWriter) Submit(_ context.Context, in inbox.Input) error {
	w.input = in
	w.input.Payload = in.Payload.Clone()
	return attachclient.ErrUncertain
}
func TestAttachReconnectRetainsIdentityExplicitRetryAndOneEcho(t *testing.T) {
	root, err := testtmp.MkdirTemp("ur13-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	endpoint, err := attachproto.Listen(root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	var requests atomic.Int64
	peerDone := make(chan error, 1)
	go func() {
		conn, err := endpoint.Listener.AcceptUnix()
		if err != nil {
			peerDone <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
		scanner := attachproto.Scanner(conn)
		_, err = attachproto.ReadRequest(scanner)
		if err != nil {
			peerDone <- err
			return
		}
		frame := func(kind string) attachproto.Event {
			return attachproto.Event{Type: kind, Protocol: 1, SessionID: "test", Incarnation: "host", Connection: "fresh", ID: "attach", Capabilities: []string{"fresh_snapshot", "history_watch", "submit_external"}}
		}
		for _, kind := range []string{"hello", "snapshot", "snapshot_end"} {
			e := frame(kind)
			if kind == "snapshot" {
				e.Model = &llm.Model{ID: "fake"}
				e.Status = &attachproto.Status{Generation: 1}
			}
			if err = attachproto.Write(conn, e); err != nil {
				peerDone <- err
				return
			}
		}
		r, err := attachproto.ReadRequest(scanner)
		if err != nil {
			peerDone <- err
			return
		}
		requests.Add(1)
		pending := frame("pending")
		pending.Revision = 1
		pending.Pending = []inbox.Input{*r.Input}
		pending.PendingAvailable = true
		item := frame("item")
		item.Revision = 2
		item.Head = 1
		item.Item = &sessionstore.Item{Sequence: 1, Kind: sessionstore.ItemInput, Data: *r.Input}
		ack := frame("accepted")
		ack.Revision = 2
		ack.Head = 1
		ack.ID = r.ID
		ack.InputID = r.Input.ID
		for _, e := range []attachproto.Event{pending, pending, item, item, ack} {
			if err = attachproto.Write(conn, e); err != nil {
				peerDone <- err
				return
			}
		}
		_, err = attachproto.ReadRequest(scanner)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		peerDone <- err
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	writer := &uncertainInputWriter{}
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	m.ctx = ctx
	m.inputs = writer
	m.attached = true
	m.attachConnected = true
	m.attachEpoch = 1
	cmd := m.startSubmission("original payload")
	m.finishSubmission(cmd().(submitted))
	id := writer.input.ID
	original := writer.input.Payload.Clone()
	if id == "" || m.lines[m.submissionIndex(id)].submission != submissionUncertain {
		t.Fatal("uncertainty did not retain logical submission")
	}
	m.attachDial = func(ctx context.Context, _ *attachproto.Cursor) (*attachclient.Client, error) {
		return attachclient.Dial(ctx, root, state, "test")
	}
	reconnect := m.startAttachReconnect()
	reply := reconnect().(attachReconnected)
	if reply.err != nil {
		t.Fatal(reply.err)
	}
	updated, _ := m.finishAttachReconnect(reply)
	m = updated.(model)
	defer reply.client.Close()
	defer func() { m.submissionQueue.Cancel(); reply.client.Close(); _ = m.submissionQueue.Wait(ctx) }()
	if requests.Load() != 0 || !bytes.Equal(m.attachOutbox[id].Payload, original) || m.submissionIndex(id) < 0 {
		t.Fatal("reconnect replayed or lost retained identity/payload")
	}
	// A stale old-source event must not create an input or schedule a second reader.
	stale := sessionstore.Item{Sequence: 99, Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "stale", Kind: inbox.InputExternal, Payload: []byte(`"stale"`)}}
	unchanged, next := m.Update(workerEvent{message: stale, epoch: 1})
	m = unchanged.(model)
	if next != nil || m.submissionIndex("stale") >= 0 {
		t.Fatal("stale connection mutated fresh view")
	}
	events := make(chan attachproto.Event, 3)
	readDone := make(chan error, 1)
	go func() {
		for range 3 {
			e, err := reply.client.Next()
			if err != nil {
				readDone <- err
				return
			}
			events <- e
		}
		readDone <- nil
	}()
	retry := m.retryAttached(string(id))
	if retry == nil {
		t.Fatal("retained retry unavailable")
	}
	result := retry().(submitted)
	for range 3 {
		select {
		case e := <-events:
			if e.Item != nil {
				m.apply(*e.Item)
			} else {
				m.applyAttachedEvent(e)
			}
		case <-ctx.Done():
			t.Fatal("retry stream deadline")
		}
	}
	m.finishSubmission(result)
	if result.err != nil || result.id != id || requests.Load() != 1 {
		t.Fatal("retry failed or changed identity", result)
	}
	count := 0
	for _, line := range m.lines {
		if line.inputID == id {
			count++
			if line.submission != submissionPersisted {
				t.Fatal("canonical persistence downgraded")
			}
		}
	}
	if count != 1 || len(m.attachOutbox) != 0 {
		t.Fatal("duplicate echo or persisted input still retryable", count, len(m.attachOutbox))
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	reply.client.Close()
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("peer not drained")
	}
	var text string
	_ = json.Unmarshal(original, &text)
	if text != "original payload" {
		t.Fatal("payload changed")
	}
}

func TestAttachAcknowledgmentStatesAndCapacityRejection(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{true: "accepted", false: "capacity_rejected"}[accepted], func(t *testing.T) {
			m := toolsModel(t, defaultTheme, 120, 40, 0)
			m.attached = true
			m.attachConnected = true
			in := inbox.Input{ID: "original-id", Kind: inbox.InputExternal, Payload: []byte(`"one echo"`)}
			m.retainAttached(in)
			m.applyInput(in)
			index := m.submissionIndex(in.ID)
			m.setSubmission(index, submissionSending)
			var err error
			if !accepted {
				err = &attachclient.ReplyError{Code: "not_accepted", InputID: in.ID, Message: "capacity full"}
			}
			m.finishSubmission(submitted{id: in.ID, text: "one echo", err: err})
			if accepted && m.lines[index].submission != submissionQueued {
				t.Fatal("ack not accepted")
			}
			if !accepted && m.lines[index].submission != submissionFailed {
				t.Fatal("capacity rejection displayed as acceptance")
			}
			if !accepted && m.composer.Value() != "" {
				t.Fatal("rejection invited accidental new-ID resubmit")
			}
			if accepted {
				m.apply(sessionstore.Item{Sequence: 1, Kind: sessionstore.ItemInput, Data: in})
				m.finishSubmission(submitted{id: in.ID, err: attachclient.ErrUncertain})
				if m.lines[index].submission != submissionPersisted || len(m.attachOutbox) != 0 {
					t.Fatal("late uncertain reply downgraded canonical persistence")
				}
			}
		})
	}
}
