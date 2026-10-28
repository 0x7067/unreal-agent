package resident

import (
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"testing"
)

func TestCursorCheckpointsBounded(t *testing.T) {
	p := &publisher{info: Info{Incarnation: "host"}}
	for i := 1; i <= 300; i++ {
		p.revision++
		p.publish(p.event("status"))
	}
	if p.checkpointCount != 256 {
		t.Fatalf("ring size %d", p.checkpointCount)
	}
	for _, tc := range []struct {
		c    attachproto.Cursor
		code string
	}{
		{attachproto.Cursor{Incarnation: "host", Revision: 300}, ""},
		{attachproto.Cursor{Incarnation: "host", Revision: 45}, ""},
		{attachproto.Cursor{Incarnation: "host", Revision: 44}, "cursor_expired"},
		{attachproto.Cursor{Incarnation: "other", Revision: 300}, "cursor_incarnation"},
		{attachproto.Cursor{Incarnation: "host", Revision: 301}, "cursor_invalid"},
		{attachproto.Cursor{Incarnation: "host", Revision: 300, Head: 1}, "cursor_invalid"},
	} {
		base, code := p.cursorBase(&tc.c)
		if code != tc.code || (base != nil) != (code == "") {
			t.Fatalf("%+v: base=%+v code=%q", tc.c, base, code)
		}
		if base != nil && *base != tc.c {
			t.Fatal("changed cursor")
		}
	}
	if base, code := p.cursorBase(nil); base != nil || code != "" {
		t.Fatal("fresh changed")
	}
}
