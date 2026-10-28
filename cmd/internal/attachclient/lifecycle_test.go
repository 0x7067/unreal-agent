package attachclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func lifecycleGreeting(conn *net.UnixConn, capabilities ...string) error {
	if _, err := attachproto.ReadRequest(attachproto.Scanner(conn)); err != nil {
		return err
	}
	for _, kind := range []string{"hello", "snapshot", "snapshot_end"} {
		e := attachproto.Event{Type: kind, ID: "attach", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", Capabilities: append([]string{"fresh_snapshot", "history_watch", "submit_external"}, capabilities...)}
		if kind == "snapshot" {
			e.Model = &llm.Model{}
			e.Status = &attachproto.Status{Generation: 7}
		}
		if err := attachproto.Write(conn, e); err != nil {
			return err
		}
	}
	return nil
}
func lifecycleInput(mode string) inbox.Input {
	parameters := `{"Version":1,"Scope":"host/7"}`
	if mode == "abort_foreground" {
		parameters = `{"Version":1,"Scope":"host/7","TargetTurnID":"turn"}`
	}
	return inbox.Input{ID: inbox.ID(mode), Kind: inbox.InputControl, Payload: []byte(`{"Mode":"` + mode + `","Parameters":` + parameters + `}`)}
}
func lifecycleReply(r attachproto.Request) attachproto.Event {
	return attachproto.Event{Type: "accepted", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", ID: r.ID, InputID: r.Input.ID}
}

func TestLifecycleRejectsBeforeRPC(t *testing.T) {
	for _, tc := range []struct {
		name  string
		caps  []string
		input inbox.Input
	}{
		{"missing abort capability", nil, lifecycleInput("abort_foreground")},
		{"wrong capability", []string{attachproto.CapabilityForegroundAbort}, lifecycleInput("shutdown_session")},
		{"wrong scope", []string{attachproto.CapabilitySessionShutdown}, inbox.Input{ID: "id", Kind: inbox.InputControl, Payload: []byte(`{"Mode":"shutdown_session","Parameters":{"Version":1,"Scope":"host/6"}}`)}},
		{"injected receipt", []string{attachproto.CapabilitySessionShutdown}, inbox.Input{ID: "id", Kind: inbox.InputControl, Payload: []byte(`{"Mode":"execution_drained","Parameters":{"Version":1,"Scope":"host/7"}}`)}},
		{"injected field", []string{attachproto.CapabilityForegroundAbort}, inbox.Input{ID: "id", Kind: inbox.InputControl, Payload: []byte(`{"Mode":"abort_foreground","Parameters":{"Version":1,"Scope":"host/7","TargetTurnID":"turn","Injected":true}}`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := make(chan error, 1)
			root, state, done := fakeHost(t, func(conn *net.UnixConn) {
				if err := lifecycleGreeting(conn, tc.caps...); err != nil {
					observed <- err
					return
				}
				_, err := attachproto.ReadRequest(attachproto.Scanner(conn))
				observed <- err
			})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			c, err := Dial(ctx, root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			if err = c.SubmitLifecycle(ctx, tc.input); err == nil {
				t.Error("accepted invalid or unsupported intent")
			}
			c.Close()
			if err = <-observed; !errors.Is(err, io.EOF) {
				t.Fatalf("unexpected RPC: %v", err)
			}
			<-done
		})
	}
}

func TestLifecycleLostAckRetainsStableInputAndFreshCorrelation(t *testing.T) {
	original := lifecycleInput("abort_foreground")
	original.Payload = append(original.Payload[:len(original.Payload):len(original.Payload)], ' ') // retain exact caller bytes, not re-encoding
	var previousID string
	for _, lost := range []bool{true, false} {
		observed := make(chan attachproto.Request, 1)
		root, state, done := fakeHost(t, func(conn *net.UnixConn) {
			if lifecycleGreeting(conn, attachproto.CapabilityForegroundAbort) != nil {
				return
			}
			r, err := attachproto.ReadRequest(attachproto.Scanner(conn))
			if err != nil {
				return
			}
			observed <- r
			if !lost {
				_ = attachproto.Write(conn, lifecycleReply(r))
				_, _ = io.Copy(io.Discard, conn)
			}
		})
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		c, err := Dial(ctx, root, state, "test")
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		next := make(chan error, 1)
		go func() { _, err := c.Next(); next <- err }()
		before := original.Payload.Clone()
		err = c.SubmitLifecycle(ctx, original)
		if lost && !errors.Is(err, ErrUncertain) {
			t.Fatalf("lost ack: %v", err)
		}
		if !lost && err != nil {
			t.Fatal(err)
		}
		r := <-observed
		if r.Type != "lifecycle" || r.ID == "" || r.ID == previousID || r.ID == string(original.ID) || r.Input == nil || (r.Input.ID != original.ID || r.Input.Kind != original.Kind || !bytes.Equal(bytes.TrimSpace(r.Input.Payload), bytes.TrimSpace(original.Payload))) {
			t.Fatalf("unstable input or correlation: %+v", r)
		}
		if !reflect.DeepEqual(original.Payload, before) {
			t.Fatal("caller input mutated")
		}
		previousID = r.ID
		c.Close()
		<-next
		<-done
		cancel()
	}
}

func TestLifecycleConcurrentSubmitUsesOneNextOwner(t *testing.T) {
	requests := make(chan attachproto.Request, 2)
	root, state, done := fakeHost(t, func(conn *net.UnixConn) {
		if lifecycleGreeting(conn, attachproto.CapabilitySessionShutdown) != nil {
			return
		}
		scanner := attachproto.Scanner(conn)
		for range 2 {
			r, err := attachproto.ReadRequest(scanner)
			if err != nil {
				return
			}
			requests <- r
			if attachproto.Write(conn, lifecycleReply(r)) != nil {
				return
			}
		}
		_, _ = io.Copy(io.Discard, conn)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	c, err := Dial(ctx, root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	next := make(chan error, 1)
	go func() {
		for range 2 {
			e, err := c.Next()
			if err != nil {
				next <- err
				return
			}
			if e.Type != "accepted" {
				next <- errors.New("unexpected event")
				return
			}
		}
		next <- nil
	}()
	results := make(chan error, 2)
	go func() { results <- c.SubmitLifecycle(ctx, lifecycleInput("shutdown_session")) }()
	go func() {
		results <- c.Submit(ctx, inbox.Input{ID: "external", Kind: inbox.InputExternal, Payload: []byte(`"text"`)})
	}()
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if err := <-next; err != nil {
		t.Fatal(err)
	}
	a, b := <-requests, <-requests
	if a.Type == b.Type || a.ID == b.ID {
		t.Fatalf("not distinct serialized RPCs: %+v %+v", a, b)
	}
	// Ordinary Submit must never admit controls, even when lifecycle is enabled.
	if c.Submit(ctx, lifecycleInput("shutdown_session")) == nil {
		t.Fatal("ordinary submit admitted control")
	}
	c.Close()
	<-done
}
