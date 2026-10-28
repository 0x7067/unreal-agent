package attachclient

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func fakeHost(t *testing.T, handle func(*net.UnixConn)) (string, string, <-chan struct{}) {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "uac-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
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
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := endpoint.Listener.AcceptUnix()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		handle(conn)
	}()
	t.Cleanup(func() {
		endpoint.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("fake host did not drain")
		}
	})
	return root, state, done
}
func greeting(conn *net.UnixConn) error {
	scanner := attachproto.Scanner(conn)
	if _, err := attachproto.ReadRequest(scanner); err != nil {
		return err
	}
	for _, kind := range []string{"hello", "snapshot", "snapshot_end"} {
		e := attachproto.Event{Type: kind, ID: "attach", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", Capabilities: []string{"fresh_snapshot", "history_watch", "submit_external"}}
		if kind == "snapshot" {
			e.Model = &llm.Model{ID: "fake"}
			e.Status = &attachproto.Status{Generation: 1}
		}
		if err := attachproto.Write(conn, e); err != nil {
			return err
		}
	}
	return nil
}
func TestConfirmedOrLostReplyAndCloseUnblocksSubmit(t *testing.T) {
	for _, mode := range []string{"accepted", "uncertain", "lost", "wrong_connection", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			root, state, done := fakeHost(t, func(conn *net.UnixConn) {
				if greeting(conn) != nil {
					return
				}
				r, err := attachproto.ReadRequest(attachproto.Scanner(conn))
				if err != nil {
					return
				}
				if mode == "lost" {
					return
				}
				if mode == "cancel" {
					_, _ = io.Copy(io.Discard, conn)
					return
				}
				e := attachproto.Event{Type: "accepted", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", ID: r.ID, InputID: r.Input.ID}
				if mode == "uncertain" {
					e.Type = "error"
					e.Code = "uncertain"
					e.Error = "sync outcome unknown"
				}
				if mode == "wrong_connection" {
					e.Connection = "stale"
				}
				_ = attachproto.Write(conn, e)
				_, _ = io.Copy(io.Discard, conn)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
			defer cancel()
			c, err := Dial(ctx, root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			next := make(chan error, 1)
			go func() { _, err := c.Next(); next <- err }()
			submitCtx, stopSubmit := context.WithCancel(ctx)
			defer stopSubmit()
			result := make(chan error, 1)
			go func() {
				result <- c.Submit(submitCtx, inbox.Input{ID: "stable", Kind: inbox.InputExternal, Payload: []byte(`"message"`)})
			}()
			if mode == "cancel" {
				stopSubmit()
			} // before write is permitted to be canceled, not uncertain.
			select {
			case err := <-result:
				if mode == "accepted" && err != nil {
					t.Fatal(err)
				}
				if mode != "accepted" && mode != "cancel" && (!errors.Is(err, ErrUncertain) || !strings.Contains(err.Error(), "stable")) {
					t.Fatal("lost/untrusted reply is not uncertain", err)
				}
				if mode == "cancel" && err == nil {
					t.Fatal("canceled send accepted without reply")
				}
			case <-ctx.Done():
				t.Fatal("Submit did not resolve")
			}
			c.Close()
			select {
			case <-next:
			case <-ctx.Done():
				t.Fatal("Next did not drain")
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("peer did not drain")
			}
		})
	}
}
func TestRejectPartialSnapshotAndControlInput(t *testing.T) {
	root, state, _ := fakeHost(t, func(conn *net.UnixConn) {
		_, _ = attachproto.ReadRequest(attachproto.Scanner(conn))
		_ = attachproto.Write(conn, attachproto.Event{Type: "hello", ID: "attach", Protocol: 1, SessionID: "test", Connection: "c", Incarnation: "h", Capabilities: []string{"fresh_snapshot", "history_watch", "submit_external"}})
	})
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	if c, err := Dial(ctx, root, state, "test"); err == nil {
		c.Close()
		t.Fatal("partial snapshot became attached")
	}
	// Submission rejects controls before consulting any socket or mutating a host.
	var c Client
	if err := c.Submit(ctx, inbox.Input{ID: "control", Kind: inbox.InputControl, Payload: []byte(`{"Mode":"StopHard"}`)}); err == nil {
		t.Fatal("control accepted")
	}
}
