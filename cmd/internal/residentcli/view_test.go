package residentcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func frame(kind string) attachproto.Event {
	return attachproto.Event{Type: kind, Protocol: attachproto.Version, SessionID: "test", Incarnation: "host", Connection: "client", Revision: 7, Head: 1}
}
func validSnapshot(t *testing.T) *attachproto.View {
	t.Helper()
	v := attachproto.NewView(frame("hello"))
	header := frame("snapshot")
	header.Status = &attachproto.Status{Generation: 1}
	header.Model = &llm.Model{ID: "fake"}
	item := frame("snapshot_item")
	item.Item = &sessionstore.Item{Sequence: 1}
	for _, e := range []attachproto.Event{header, item, frame("snapshot_end")} {
		if err := v.Accept(e); err != nil {
			t.Fatal(err)
		}
	}
	return v
}
func TestViewRejectsInvalidSnapshotAndLiveState(t *testing.T) {
	for _, kind := range []string{"snapshot_end", "snapshot_item", "pending", "status", "unknown"} {
		v := attachproto.NewView(frame("hello"))
		if err := v.Accept(frame(kind)); err == nil {
			t.Fatal("accepted incomplete snapshot", kind)
		}
	}
	for _, test := range []struct {
		name   string
		change func(*attachproto.Event)
	}{
		{"protocol", func(e *attachproto.Event) { e.Protocol++ }},
		{"session", func(e *attachproto.Event) { e.SessionID = "other" }},
		{"incarnation", func(e *attachproto.Event) { e.Incarnation = "replacement" }},
		{"connection", func(e *attachproto.Event) { e.Connection = "stale" }},
		{"revision_gap", func(e *attachproto.Event) { e.Revision++ }},
		{"head_gap", func(e *attachproto.Event) { e.Head++ }},
		{"sequence_gap", func(e *attachproto.Event) { e.Item.Sequence++ }},
		{"missing_item", func(e *attachproto.Event) { e.Item = nil }},
		{"unknown_type", func(e *attachproto.Event) { e.Type = "unknown" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			v := validSnapshot(t)
			e := frame("item")
			e.Revision = 8
			e.Head = 2
			e.Item = &sessionstore.Item{Sequence: 2}
			test.change(&e)
			if err := v.Accept(e); err == nil {
				t.Fatal("accepted invalid state")
			}
		})
	}
	v := validSnapshot(t)
	e := frame("status")
	e.Revision = 8
	e.Status = &attachproto.Status{Generation: 1}
	if err := v.Accept(e); err != nil {
		t.Fatal(err)
	}
	e = frame("accepted")
	e.Revision = 8
	if err := v.Accept(e); err != nil {
		t.Fatal("reply advanced cursor", err)
	}
	if err := v.Accept(frame("snapshot_end")); err == nil {
		t.Fatal("accepted duplicate snapshot")
	}
}

func TestLostSubmissionReplyIsExplicitlyUncertain(t *testing.T) {
	root, err := testtmp.MkdirTemp("uak-")
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
	done := make(chan error, 1)
	go func() {
		conn, err := endpoint.Listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		scanner := attachproto.Scanner(conn)
		_, err = attachproto.ReadRequest(scanner)
		if err != nil {
			done <- err
			return
		}
		for _, kind := range []string{"hello", "snapshot", "snapshot_end"} {
			e := frame(kind)
			e.Head = 0
			e.Revision = 0
			if kind == "snapshot" {
				e.Model = &llm.Model{ID: "fake"}
				e.Status = &attachproto.Status{Generation: 1}
			}
			if err := attachproto.Write(conn, e); err != nil {
				done <- err
				return
			}
		}
		r, err := attachproto.ReadRequest(scanner)
		if err == nil && (r.Type != "submit" || r.Input == nil || r.Input.ID != "stable-unknown") {
			err = context.Canceled
		}
		done <- err
		// It may have been durably admitted: lose the reply without saying that
		// it was rejected. The client must retain the stable identity for retry.
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var output bytes.Buffer
	err = attach(ctx, root, state, "test", true, "stable-unknown", "message", &output)
	if err == nil || !strings.Contains(err.Error(), "uncertain") || !strings.Contains(err.Error(), "stable-unknown") {
		t.Fatal("lost reply not explicitly uncertain", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("fake host did not join")
	}
}
