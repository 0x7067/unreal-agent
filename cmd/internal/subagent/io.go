package subagent

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// Limits apply to this private runner JSONL protocol, not workerproto or the
// one-shot runner's separate input parser. EOF-terminated final lines remain
// supported. An oversized frame fails explicitly, never as a truncated message.
const MaxSubagentFrameBytes = 8 << 20
const maxInputFrames = 32
const maxDeliveryBytes = 16 << 20

type inputFrame struct {
	mu       sync.Mutex
	data     []byte
	id       inbox.ID
	barrier  string
	canceled bool
}
type inputWriter struct {
	ctx        context.Context
	cancel     context.CancelFunc
	queue      chan *inputFrame
	mu         sync.Mutex
	bytes      int
	stdin      io.WriteCloser
	supervisor *Supervisor
	session    string
	generation int
	done       chan struct{}
}

func (s *Supervisor) newInputWriter(ctx context.Context, stdin io.WriteCloser, session string, generation int) *inputWriter {
	ctx, cancel := context.WithCancel(ctx)
	w := &inputWriter{ctx: ctx, cancel: cancel, queue: make(chan *inputFrame, maxInputFrames), stdin: stdin, supervisor: s, session: session, generation: generation, done: make(chan struct{})}
	s.readers.Add(1)
	go w.loop()
	return w
}
func (w *inputWriter) enqueue(data []byte, id inbox.ID) (*inputFrame, error) {
	if len(data) > MaxSubagentFrameBytes {
		return nil, errors.New("subagent input exceeds 8 MiB frame limit")
	}
	f := &inputFrame{data: append(append([]byte(nil), data...), '\n'), id: id, barrier: "queued"}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctx.Err() != nil {
		return nil, w.ctx.Err()
	}
	if w.bytes+len(f.data) > MaxSubagentFrameBytes {
		return nil, errors.New("subagent input byte capacity exhausted")
	}
	select {
	case w.queue <- f:
		w.bytes += len(f.data)
		return f, nil
	default:
		return nil, errors.New("subagent input frame capacity exhausted")
	}
}
func (w *inputWriter) stop() { w.cancel(); _ = w.stdin.Close() }
func (w *inputWriter) loop() {
	defer w.supervisor.readers.Done()
	defer close(w.done)
	for {
		select {
		case <-w.ctx.Done():
			return
		case f := <-w.queue:
			f.mu.Lock()
			skip := f.canceled
			if !skip {
				f.barrier = "write_started"
			}
			f.mu.Unlock()
			if !skip {
				w.supervisor.activity.input(w.session, w.generation, string(f.id), "write_started")
				n, err := w.stdin.Write(f.data)
				if err == nil && n != len(f.data) {
					err = io.ErrShortWrite
				}
				f.mu.Lock()
				if err == nil {
					f.barrier = "written"
				} else {
					f.barrier = "write_failed"
				}
				barrier := f.barrier
				f.mu.Unlock()
				w.supervisor.activity.input(w.session, w.generation, string(f.id), barrier)
				if err != nil {
					w.stop()
					return
				} // Never append another frame after a partial write.
			}
			w.mu.Lock()
			w.bytes -= len(f.data)
			w.mu.Unlock()
		}
	}
}
func boundedLine(r *bufio.Reader) ([]byte, error) {
	var result []byte
	for {
		piece, err := r.ReadSlice('\n')
		if len(result)+len(piece) > MaxSubagentFrameBytes {
			return nil, errors.New("subagent stdout exceeds 8 MiB JSONL frame limit")
		}
		result = append(result, piece...)
		if err != bufio.ErrBufferFull {
			return result, err
		}
	}
}

type deliveryJob struct {
	target     *child
	generation int
	inputs     []inbox.Input
	bytes      int
}

// A reservation is obtained by the reader before actor admission. Saturation
// backpressures only readers, never control admission or process reaping.
func (s *Supervisor) reserveDelivery(ctx context.Context, n int) error {
	if n > maxDeliveryBytes {
		return errors.New("subagent delivery frame capacity exhausted")
	}
	for {
		s.deliveryMu.Lock()
		if s.deliveryBytes+n <= maxDeliveryBytes {
			s.deliveryBytes += n
			// A reservation remains owned through queued and active Submit.
			s.deliveryMu.Unlock()
			s.changeWork(func() { s.workDeliveries++ })
			return nil
		}
		changed := s.deliveryChanged
		s.deliveryMu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
func (s *Supervisor) releaseDelivery(n int) {
	s.deliveryMu.Lock()
	s.deliveryBytes -= n
	close(s.deliveryChanged)
	s.deliveryChanged = make(chan struct{})
	s.deliveryMu.Unlock()
	s.changeWork(func() { s.workDeliveries-- })
}
func (s *Supervisor) deliveryLoop() {
	defer s.readers.Done()
	for {
		select {
		case job := <-s.deliveries:
			failed := false
			for _, input := range job.inputs {
				if s.config.Inbox.Submit(s.ctx, input) != nil {
					failed = true
				}
			}
			s.releaseDelivery(job.bytes)
			<-s.deliverySlots
			_ = s.post(func(state *state) {
				c := job.target
				if c.generation != job.generation {
					return
				}
				c.parentPending--
				if failed {
					s.activity.change(c.sessionID, c.generation, func(a *ChildActivity) { a.DeliveryError = true })
					c.failure = "parent inbox admission failed"
				}
				state.maybeExit(c)
			})
		case <-s.workCtx.Done():
			return
		}
	}
}

func (s *state) maybeExit(c *child) {
	if c.processDone && c.outputDone && c.parentPending == 0 {
		err := c.exitErr
		if err == nil && c.failure != "" {
			err = errors.New(c.failure)
		}
		s.exited(c, c.generation, err)
	}
}

// prepJob retains an immutable operation/remote value. All full-output artifact
// work uses the existing UpdateRemoteJob contract off the supervisor actor.
type prepJob struct {
	sequence uint64
	message  bool
	value    operation.Operation
	remote   operation.RemoteJobState
	status   operation.Status
	done     func(*state, operation.Operation)
	bytes    int
}

func (s *Supervisor) prepareLoop(queue <-chan prepJob) {
	defer s.readers.Done()
	for {
		select {
		case job := <-queue:
			prepare := s.config.prepare
			if prepare == nil {
				prepare = func(v operation.Operation, r operation.RemoteJobState, status operation.Status) (operation.Step, error) {
					return operation.UpdateRemoteJob(v, r, status)
				}
			}
			step, err := prepare(job.value, job.remote, job.status)
			_ = s.post(func(state *state) { state.prepared(prepResult{job: job, step: step, err: err}) })
		case <-s.workCtx.Done():
			return
		}
	}
}
func (s *state) prepare(value operation.Operation, remote operation.RemoteJobState, status operation.Status, done func(*state, operation.Operation)) (operation.Operation, bool) {
	if len(s.pendingFinals) != 0 && !s.retryingFinal {
		return value, false
	}
	// Pure serialization supplies actor-local state, not a durability ack.
	encoded, err := json.Marshal(remote)
	if err != nil {
		return value, false
	}
	tentative := value
	tentative.State = encoded
	tentative.Status = status
	size := len(encoded) + len(value.State)
	current := s.supervisor
	current.prepMu.Lock()
	defer current.prepMu.Unlock()
	if size > 32<<20 || current.prepBytes+size > 32<<20 {
		return value, false
	}
	queue := current.runPrep
	if planType(value) == "subagent.message" {
		queue = current.messagePrep
	}
	message := planType(value) == "subagent.message"
	sequence := s.runIssued + 1
	if message {
		sequence = s.messageIssued + 1
	}
	job := prepJob{value: value, remote: remote, status: status, done: done, bytes: size, sequence: sequence, message: message}
	select {
	case queue <- job:
		if message {
			s.messageIssued++
		} else {
			s.runIssued++
		}
		current.prepBytes += size
		s.pendingPreps++
		return tentative, true
	default:
		return value, false
	}
}

// now is kept here with input barrier code to make timestamps observation-based.
func cancellationTime(c *ChildActivity) {
	c.CancellationRequested = true
	if c.CancellationRequestedAt.IsZero() {
		c.CancellationRequestedAt = time.Now()
	}
}

type terminalRetry struct {
	value  operation.Operation
	remote operation.RemoteJobState
	status operation.Status
	done   func(*state, operation.Operation)
}

func (s *state) retryFinals() {
	s.retryingFinal = true
	defer func() { s.retryingFinal = false }()
	for len(s.pendingFinals) > 0 {
		job := s.pendingFinals[0]
		if _, ok := s.prepare(job.value, job.remote, job.status, job.done); !ok {
			return
		}
		s.pendingFinals[0] = terminalRetry{}
		s.pendingFinals = s.pendingFinals[1:]
	}
}

type prepResult struct {
	job  prepJob
	step operation.Step
	err  error
}

func (s *state) prepared(result prepResult) {
	ready := s.runPrepared
	if result.job.message {
		ready = s.messagePrepared
	}
	if ready == nil {
		ready = make(map[uint64]prepResult)
		if result.job.message {
			s.messagePrepared = ready
		} else {
			s.runPrepared = ready
		}
	}
	ready[result.job.sequence] = result
	for {
		next := s.runPublished + 1
		if result.job.message {
			next = s.messagePublished + 1
		}
		current, ok := ready[next]
		if !ok {
			break
		}
		delete(ready, next)
		if result.job.message {
			s.messagePublished = next
		} else {
			s.runPublished = next
		}
		s.pendingPreps--
		s.supervisor.prepMu.Lock()
		s.supervisor.prepBytes -= current.job.bytes
		s.supervisor.prepMu.Unlock()
		if current.err != nil || current.step.Operation == nil {
			s.supervisor.activity.preparationFailed(current.job.value.ID)
			continue
		}
		s.enqueue(*current.step.Operation)
		if current.job.done != nil {
			current.job.done(s, *current.step.Operation)
		}
	}
	s.retryFinals()
}
