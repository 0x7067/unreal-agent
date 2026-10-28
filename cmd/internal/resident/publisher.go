// Package resident publishes a foreground worker host over local attach transport.
package resident

import (
	"sync"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

type publisher struct {
	mu                              sync.Mutex
	runtimeEnabled                  bool
	runtimeStatus                   workerhost.Status
	info                            Info
	revision                        uint64
	head                            sessionstore.Sequence
	pending                         []inbox.Input
	pendingBytes                    int
	available                       bool
	seen                            map[inbox.ID]bool
	status                          attachproto.Status
	model                           llm.Model
	active                          *client
	checkpoints                     [256]attachproto.Cursor
	checkpointNext, checkpointCount int
}

func (p *publisher) event(kind string) attachproto.Event {
	return attachproto.Event{Type: kind, Protocol: attachproto.Version, SessionID: p.info.SessionID, Incarnation: p.info.Incarnation, Revision: p.revision, Head: p.head, PendingAvailable: p.available}
}

// All publication and queue accounting happen under mu. No callback does socket I/O.
func (p *publisher) publish(e attachproto.Event) {
	// Retain metadata only, including revisions published without a client.
	p.checkpoints[p.checkpointNext] = attachproto.Cursor{Incarnation: p.info.Incarnation, Revision: e.Revision, Head: e.Head}
	p.checkpointNext = (p.checkpointNext + 1) % len(p.checkpoints)
	if p.checkpointCount < len(p.checkpoints) {
		p.checkpointCount++
	}
	if p.active != nil {
		e.Connection = p.active.token
		p.enqueue(p.active, e)
	}
}

// stopClient invalidates admission and releases queued frames under mu. A writer
// that already dequeued a frame owns that frame's byte-accounting decrement.
func (p *publisher) stopClient(c *client) {
	c.stop()
	for {
		select {
		case frame := <-c.queue:
			c.bytes -= len(frame)
		default:
			return
		}
	}
}
func (p *publisher) enqueue(c *client, e attachproto.Event) {
	select {
	case <-c.stopped:
		p.stopClient(c)
		return
	default:
	}
	frame, err := attachproto.Encode(e)
	if err != nil || c.bytes+len(frame) > attachproto.MaxQueuedBytes {
		p.stopClient(c)
		return
	}
	select {
	case c.queue <- frame:
		c.bytes += len(frame)
	default:
		p.stopClient(c)
	}
}
func (p *publisher) admission(in inbox.Input) {
	in.Payload = in.Payload.Clone()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen[in.ID] {
		return
	}
	for _, old := range p.pending {
		if old.ID == in.ID {
			return
		}
	}
	// Bound only this publication mirror, never the canonical ingress journal.
	// Recovery overflow permanently marks it incomplete until host restart.
	if !p.available {
		return
	}
	if len(p.pending) >= 64 || p.pendingBytes+len(in.Payload) > 8<<20 {
		p.available = false
		p.revision++
		p.publish(p.event("pending_unavailable"))
		return
	}
	p.pending = append(p.pending, in)
	p.pendingBytes += len(in.Payload)
	p.revision++
	e := p.event("pending")
	e.Pending = []inbox.Input{in}
	p.publish(e)
}
func (p *publisher) item(item sessionstore.Item) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if item.Sequence <= p.head {
		return
	}
	p.head = item.Sequence
	if in, ok := item.Data.(inbox.Input); ok {
		p.seen[in.ID] = true
		for i, old := range p.pending {
			if old.ID == in.ID {
				p.pendingBytes -= len(old.Payload)
				p.pending = append(p.pending[:i], p.pending[i+1:]...)
				break
			}
		}
		if in.Kind == inbox.InputControl {
			if control, err := in.DecodeControlMessage(); err == nil && control.Mode == inbox.UpdateSettings {
				if s, ok := control.Parameters.(inbox.Settings); ok {
					if s.Model != "" {
						p.model.ID = s.Model
					}
					if s.CompactionThreshold != nil {
						p.model.CompactionThreshold = *s.CompactionThreshold
					}
					if s.MaxOutputTokens != nil {
						v := *s.MaxOutputTokens
						p.model.MaxOutputTokens = &v
					}
					if s.ReasoningEffort != "" {
						p.model.ReasoningEffort = s.ReasoningEffort
					}
				}
			}
		}
	}
	p.revision++
	e := p.event("item")
	e.Item = &item
	m := p.model
	e.Model = &m
	p.publish(e)
}
func (p *publisher) statusChanged(s workerhost.Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runtimeStatus = s
	p.status = attachproto.Status{Generation: s.Generation, Build: s.Build, Stopped: s.Stopped}
	if s.Err != nil {
		p.status.Error = s.Err.Error()
		p.available = false
	}
	p.revision++
	e := p.event("status")
	v := p.status
	e.Status = &v
	p.publish(e)
}
func (p *publisher) uncertain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.available = false
	p.revision++
	p.publish(p.event("pending_unavailable"))
}

// canAdmit is called under mu. Capacity applies to new identities, not retries
// whose acceptance already exists. The canonical journal still checks conflicts
// and uncertain durability; this mirror cannot authorize a changed payload.
func (p *publisher) canAdmit(in inbox.Input) bool {
	if p.seen[in.ID] {
		return true
	}
	for _, old := range p.pending {
		if old.ID == in.ID {
			return true
		}
	}
	return p.available && len(p.pending) < 64 && p.pendingBytes+len(in.Payload) <= 8<<20
}

// cursorBase is called with mu held. Intermediate pending/status revisions are
// coalesced into the captured complete snapshot, never replayed from an event log.
func (p *publisher) cursorBase(c *attachproto.Cursor) (*attachproto.Cursor, string) {
	if c == nil {
		return nil, ""
	}
	if c.Incarnation != p.info.Incarnation {
		return nil, "cursor_incarnation"
	}
	if c.Revision > p.revision || c.Head > p.head {
		return nil, "cursor_invalid"
	}
	if c.Revision == p.revision {
		if c.Head != p.head {
			return nil, "cursor_invalid"
		}
		copy := *c
		return &copy, ""
	}
	for i := 0; i < p.checkpointCount; i++ {
		old := p.checkpoints[i]
		if old.Revision == c.Revision {
			if old != *c {
				return nil, "cursor_invalid"
			}
			copy := *c
			return &copy, ""
		}
	}
	return nil, "cursor_expired"
}
