package workerruntime

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
)

var (
	ErrProtocol = errors.New("worker protocol error")
	ErrOutput   = errors.New("worker output error")
)

// emitter owns the pipe. A failed write poisons it permanently: neither error
// reporting nor concurrent persistence observers can append a partial frame.
type emitter struct {
	mu     sync.Mutex
	output io.Writer
	err    error
	cancel context.CancelFunc
}

func (e *emitter) write(event workerproto.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err != nil {
		return e.err
	}
	if err := workerproto.Write(e.output, event); err != nil {
		e.err = fmt.Errorf("%w: %w", ErrOutput, err)
		e.cancel()
	}
	return e.err
}
func (e *emitter) failure() error { e.mu.Lock(); defer e.mu.Unlock(); return e.err }

type frame struct {
	request workerproto.Request
	err     error
}

// The pump reads only on demand. In particular it cannot be blocked reading the
// *next* request while a quiesce request is retiring this worker.
type inputPump struct {
	input  io.Reader
	demand chan struct{}
	frames chan frame
	stop   chan struct{}
	done   chan struct{}
}

func newInputPump(input io.Reader) *inputPump {
	p := &inputPump{input: input, demand: make(chan struct{}), frames: make(chan frame), stop: make(chan struct{}), done: make(chan struct{})}
	go p.run()
	return p
}
func (p *inputPump) run() {
	defer close(p.done)
	scanner := workerproto.Scanner(p.input)
	for {
		select {
		case <-p.stop:
			return
		case <-p.demand:
		}
		var f frame
		if !scanner.Scan() {
			f.err = scanner.Err()
			if f.err == nil {
				f.err = io.EOF
			} else {
				f.err = fmt.Errorf("%w: read frame: %w", ErrProtocol, f.err)
			}
		} else if err := json.Unmarshal(scanner.Bytes(), &f.request, json.RejectUnknownMembers(true)); err != nil {
			f.err = fmt.Errorf("%w: decode request: %w", ErrProtocol, err)
		}
		select {
		case <-p.stop:
			return
		case p.frames <- f:
		}
		if f.err != nil {
			return
		}
	}
}
func (p *inputPump) next(ctx context.Context, ended <-chan struct{}) (workerproto.Request, error) {
	select {
	case <-ctx.Done():
		return workerproto.Request{}, context.Cause(ctx)
	case <-ended:
		return workerproto.Request{}, errCoordinatorEnded
	case p.demand <- struct{}{}:
	}
	select {
	case <-ctx.Done():
		return workerproto.Request{}, context.Cause(ctx)
	case <-ended:
		return workerproto.Request{}, errCoordinatorEnded
	case f := <-p.frames:
		return f.request, f.err
	}
}
func (p *inputPump) join() {
	close(p.stop)
	// Do not close a caller's stdin descriptor. Pollable os.File pipes support
	// deadlines; io.PipeReader is an explicitly transferred in-process read end.
	// Other readers must be unblocked by their owner (see Run's contract).
	switch input := p.input.(type) {
	case *os.File:
		if input.SetReadDeadline(time.Now()) == nil {
			defer input.SetReadDeadline(time.Time{}) // restore ordinary blocking reads
		}
	case *io.PipeReader:
		_ = input.CloseWithError(context.Canceled)
	}
	<-p.done
}

var errCoordinatorEnded = errors.New("coordinator ended")
