package attachproto

import (
	"crypto/sha256"
	"encoding/json/v2"
	"errors"

	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// View validates a complete fresh snapshot before accepting its marker, then
// requires contiguous published state. Replies never advance the live cursor.
// ErrDuplicate identifies an exact previously validated live update. Callers
// may skip it; conflicting or evicted revisions still require resnapshotting.
var ErrDuplicate = errors.New("duplicate host update")

type eventFingerprint struct {
	revision uint64
	sum      [32]byte
	known    bool
}
type View struct {
	fingerprints        [256]eventFingerprint
	hello               Event
	requested           *Cursor
	header, items, live bool
	revision            uint64
	head, sequence      sessionstore.Sequence
}

// NewView starts validation after the caller has authenticated hello.
func NewView(hello Event) *View {
	return NewViewFrom(hello, nil)
}

// NewViewFrom validates a delta only against the exact requested checkpoint.
func NewViewFrom(hello Event, requested *Cursor) *View {
	v := &View{hello: hello, revision: hello.Revision, head: hello.Head}
	if requested != nil {
		copy := *requested
		v.requested = &copy
	}
	return v
}

// Accept validates the next snapshot or live event and advances the cursor.
func (v *View) Accept(e Event) error {
	bad := func() error { return errors.New("invalid or discontinuous host view; reconnect with a fresh snapshot") }
	if (e.Type == "runtime") != (e.Runtime != nil) {
		return bad()
	}
	if e.Runtime != nil && ValidateRuntimeStatus(*e.Runtime) != nil {
		return bad()
	}
	if e.Protocol != Version || e.SessionID != v.hello.SessionID || e.Connection != v.hello.Connection || e.Incarnation != v.hello.Incarnation {
		return bad()
	}
	if e.Base != nil && e.Type != "snapshot" {
		return bad()
	}
	if !v.live {
		if e.Revision != v.revision || e.Head != v.head {
			return bad()
		}
		switch e.Type {
		case "snapshot":
			if v.header || e.Item != nil || e.Status == nil || e.Model == nil {
				return bad()
			}
			if e.Base == nil && v.requested != nil {
				switch e.Code {
				case "cursor_expired", "cursor_invalid", "cursor_incarnation", "cursor_unsupported":
				default:
					return bad()
				}
			}
			if e.Base != nil {
				b := e.Base
				if v.requested == nil || *b != *v.requested || b.Incarnation != v.hello.Incarnation || b.Revision > v.revision || b.Head > v.head {
					return bad()
				}
				v.sequence = b.Head
			}
			v.header = true
		case "snapshot_pending":
			if !v.header || v.items || e.Item != nil || len(e.Pending) != 1 {
				return bad()
			}
		case "snapshot_item":
			if !v.header || e.Item == nil || e.Item.Sequence != v.sequence+1 || e.Item.Sequence > v.head {
				return bad()
			}
			v.items = true
			v.sequence = e.Item.Sequence
		case "snapshot_end":
			if !v.header || e.Item != nil || v.sequence != v.head {
				return bad()
			}
			v.live = true
		default:
			return bad()
		}
		return nil
	}
	switch e.Type {
	case "item", "pending", "status", "pending_unavailable":
		data, err := json.Marshal(e, json.Deterministic(true))
		if err != nil {
			return bad()
		}
		sum := sha256.Sum256(data)
		if e.Revision <= v.revision {
			old := v.fingerprints[e.Revision%256]
			if old.known && old.revision == e.Revision && old.sum == sum {
				return ErrDuplicate
			}
			return bad()
		}
		if e.Revision != v.revision+1 {
			return bad()
		}
		if e.Type == "item" {
			if e.Item == nil || e.Item.Sequence != v.sequence+1 || e.Head != e.Item.Sequence {
				return bad()
			}
			v.sequence = e.Item.Sequence
		} else if e.Item != nil || e.Head != v.sequence {
			return bad()
		}
		if e.Type == "pending" && len(e.Pending) != 1 || e.Type == "status" && e.Status == nil || e.Type == "pending_unavailable" && e.PendingAvailable {
			return bad()
		}
		v.fingerprints[e.Revision%256] = eventFingerprint{revision: e.Revision, sum: sum, known: true}
		v.revision = e.Revision
		v.head = e.Head
	case "accepted", "error", "runtime":
		if e.Item != nil || e.Revision != v.revision || e.Head != v.sequence {
			return bad()
		}
	default:
		return bad()
	}
	return nil
}
