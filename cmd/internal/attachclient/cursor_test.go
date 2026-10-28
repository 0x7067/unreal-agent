package attachclient

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestCursorClientDeltaAndOldHostFallback(t *testing.T) {
	for _, old := range []bool{false, true} {
		t.Run(map[bool]string{true: "strict_old_host", false: "delta"}[old], func(t *testing.T) {
			root, err := os.MkdirTemp("/tmp", "uac-cur-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			state := filepath.Join(root, "state")
			if err = os.Mkdir(state, 0700); err != nil {
				t.Fatal(err)
			}
			ep, err := attachproto.Listen(root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			defer ep.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
			defer cancel()
			base := attachproto.Cursor{Incarnation: "host", Revision: 8, Head: 7}
			done := make(chan error, 1)
			go func() {
				handle := func(conn *net.UnixConn) error {
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
					scanner := attachproto.Scanner(conn)
					r, err := attachproto.ReadRequest(scanner)
					if err != nil {
						return err
					}
					if r.Cursor == nil || *r.Cursor != base {
						return errors.New("missing requested checkpoint")
					}
					if old {
						return nil
					}
					return cursorSnapshot(conn, &base)
				}
				conn, err := ep.Listener.AcceptUnix()
				if err != nil {
					done <- err
					return
				}
				if err = handle(conn); err != nil {
					done <- err
					return
				}
				if old {
					conn, err = ep.Listener.AcceptUnix()
					if err != nil {
						done <- err
						return
					}
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
					r, err := attachproto.ReadRequest(attachproto.Scanner(conn))
					if err != nil {
						done <- err
						return
					}
					if r.Cursor != nil {
						done <- errors.New("extension retained during old-host fallback")
						return
					}
					err = cursorSnapshot(conn, nil)
				}
				done <- err
			}()
			c, err := DialFrom(ctx, root, state, "test", &base)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			snapshot := c.Snapshot()
			if old {
				if snapshot.Header.Base != nil || snapshot.Header.Code != "cursor_unsupported" || len(snapshot.Items) != 8 {
					t.Fatal("old host fallback incomplete", snapshot.Header)
				}
			} else if snapshot.Header.Base == nil || *snapshot.Header.Base != base || len(snapshot.Items) != 1 || snapshot.Items[0].Sequence != 8 {
				t.Fatal("delta incomplete", snapshot)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("fixture not joined")
			}
		})
	}
}
func cursorSnapshot(conn *net.UnixConn, base *attachproto.Cursor) error {
	hello := attachproto.Event{Type: "hello", ID: "attach", Protocol: 1, SessionID: "test", Connection: "new", Incarnation: "host", Revision: 10, Head: 8, Capabilities: []string{"fresh_snapshot", "history_watch", "submit_external"}}
	if err := attachproto.Write(conn, hello); err != nil {
		return err
	}
	header := hello
	header.Type = "snapshot"
	header.Status = &attachproto.Status{Generation: 1}
	header.Model = &llm.Model{ID: "fake"}
	header.Base = base
	if err := attachproto.Write(conn, header); err != nil {
		return err
	}
	after := sessionstore.Sequence(0)
	if base != nil {
		after = base.Head
	}
	for seq := after + 1; seq <= 8; seq++ {
		payload, _ := json.Marshal("input")
		e := hello
		e.Type = "snapshot_item"
		e.Item = &sessionstore.Item{Sequence: seq, Kind: sessionstore.ItemInput, Data: inbox.Input{ID: inbox.ID(string(rune('a' + seq))), Kind: inbox.InputExternal, Payload: payload}}
		if err := attachproto.Write(conn, e); err != nil {
			return err
		}
	}
	end := hello
	end.Type = "snapshot_end"
	return attachproto.Write(conn, end)
}
