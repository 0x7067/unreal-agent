package workerhost

import (
	"encoding/json/v2"
	"fmt"
	"io"
	"sync"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
)

// eventMailbox separates pipe draining from ordered acknowledgment/callback work.
// The decoder owns admission and finish; consume is the only reader. Cancellation
// does not discard already decoded output. A stalled callback may grow this
// volatile FIFO: bounded memory cannot also guarantee an indefinitely unblocked
// producer. No journal durability or event-handler completion is implied by push.
type eventMailbox struct {
	mu       sync.Mutex
	queue    []workerproto.Event
	wake     chan struct{}
	finished bool
	err      error
}

func newEventMailbox() *eventMailbox { return &eventMailbox{wake: make(chan struct{}, 1)} }

func (q *eventMailbox) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *eventMailbox) push(event workerproto.Event) {
	q.mu.Lock()
	q.queue = append(q.queue, event)
	q.mu.Unlock()
	q.signal()
}

func (q *eventMailbox) finish(err error) {
	q.mu.Lock()
	q.finished, q.err = true, err
	q.mu.Unlock()
	q.signal()
}

func (q *eventMailbox) next() (workerproto.Event, error, bool) {
	for {
		q.mu.Lock()
		if len(q.queue) != 0 {
			event := q.queue[0]
			q.queue[0] = workerproto.Event{} // Release frame payloads after delivery.
			q.queue = q.queue[1:]
			if len(q.queue) == 0 {
				q.queue = nil
			}
			q.mu.Unlock()
			return event, nil, true
		}
		finished, err := q.finished, q.err
		q.mu.Unlock()
		if finished {
			return workerproto.Event{}, err, false
		}
		<-q.wake
	}
}

// decode never invokes callbacks, acquires host.mu, or touches the journal.
// scanDone is independent of ordered delivery, which p.done additionally joins.
func (p *process) decode(output io.ReadCloser, queue *eventMailbox) {
	defer close(p.scanDone)
	defer output.Close()
	scanner := workerproto.Scanner(output)
	for scanner.Scan() {
		var event workerproto.Event
		if err := json.Unmarshal(scanner.Bytes(), &event, json.RejectUnknownMembers(true)); err != nil {
			queue.finish(fmt.Errorf("decode worker event: %w", err))
			return
		}
		queue.push(event)
	}
	queue.finish(scanner.Err())
}
