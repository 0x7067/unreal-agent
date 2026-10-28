package attachproto

import (
	"bytes"
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"testing"
)

func TestCursorViewDeltaAndFallback(t *testing.T) {
	hello := Event{Type: "hello", Protocol: Version, SessionID: "s", Connection: "c", Incarnation: "host", Revision: 4, Head: 2}
	requested := &Cursor{Incarnation: "host", Revision: 2, Head: 1}
	for _, delta := range []bool{false, true} {
		v := NewViewFrom(hello, requested)
		header := hello
		header.Type = "snapshot"
		header.Status = &Status{}
		header.Model = &llm.Model{}
		start := sessionstore.Sequence(1)
		if delta {
			header.Base = requested
			start = 2
		} else {
			header.Code = "cursor_expired"
		}
		if err := v.Accept(header); err != nil {
			t.Fatal(err)
		}
		for seq := start; seq <= 2; seq++ {
			e := hello
			e.Type = "snapshot_item"
			e.Item = &sessionstore.Item{Sequence: seq}
			if err := v.Accept(e); err != nil {
				t.Fatal(err)
			}
		}
		end := hello
		end.Type = "snapshot_end"
		if err := v.Accept(end); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCursorViewRejectsUnrequestedAndCloneAuthority(t *testing.T) {
	hello := Event{Protocol: Version, SessionID: "s", Connection: "c", Incarnation: "host", Revision: 4, Head: 2}
	requested := &Cursor{Incarnation: "host", Revision: 2, Head: 1}
	header := hello
	header.Type = "snapshot"
	header.Status = &Status{}
	header.Model = &llm.Model{}
	header.Base = requested
	if NewView(hello).Accept(header) == nil {
		t.Fatal("unrequested base accepted")
	}
	for _, bad := range []Cursor{{Incarnation: "other", Revision: 2, Head: 1}, {Incarnation: "host", Revision: 5, Head: 1}, {Incarnation: "host", Revision: 2, Head: 3}, {Incarnation: "host", Revision: 1, Head: 1}} {
		header.Base = &bad
		if NewViewFrom(hello, requested).Accept(header) == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	header.Base = requested
	v := NewViewFrom(hello, requested)
	if err := v.Accept(header); err != nil {
		t.Fatal(err)
	}
	clone := hello
	clone.Type = "snapshot_end"
	clone.Base = requested
	if v.Accept(clone) == nil {
		t.Fatal("clone base accepted")
	}
	frame, err := Encode(Request{Type: "attach"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(frame, []byte("Cursor")) {
		t.Fatal("fresh client not old-host compatible")
	}
	frame, err = Encode(hello)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(frame, []byte("Base")) {
		t.Fatal("fresh host not old-client compatible")
	}
}

func TestCursorExactDuplicateOnly(t *testing.T) {
	hello := Event{Protocol: Version, SessionID: "s", Incarnation: "i", Connection: "c"}
	v := NewView(hello)
	e := hello
	e.Type = "snapshot"
	e.Status = &Status{}
	e.Model = &llm.Model{}
	if err := v.Accept(e); err != nil {
		t.Fatal(err)
	}
	e.Type = "snapshot_end"
	if err := v.Accept(e); err != nil {
		t.Fatal(err)
	}
	e.Type = "status"
	e.Revision = 1
	if err := v.Accept(e); err != nil {
		t.Fatal(err)
	}
	if err := v.Accept(e); !errors.Is(err, ErrDuplicate) {
		t.Fatal("exact duplicate not identified", err)
	}
	changed := e
	changed.Code = "different"
	if err := v.Accept(changed); err == nil || errors.Is(err, ErrDuplicate) {
		t.Fatal("conflicting duplicate ignored", err)
	}
	for r := uint64(2); r <= 257; r++ {
		e.Revision = r
		if err := v.Accept(e); err != nil {
			t.Fatal(err)
		}
	}
	e.Revision = 1
	if err := v.Accept(e); err == nil || errors.Is(err, ErrDuplicate) {
		t.Fatal("evicted revision ignored", err)
	}
}
