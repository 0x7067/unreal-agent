package workerhost

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
)

func TestSnapshotCancellationInterruptsBlockedTransportWithoutWriterGoroutine(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if err := writer.SetWriteDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Skipf("nonpollable pipe: %v", err)
	}
	// Deliberately fill the pipe without a reader. The diagnostic frame now blocks
	// on transport, independent of child/model behavior.
	if _, err := writer.Write(make([]byte, 1<<20)); err == nil {
		t.Fatal("fixture pipe did not fill")
	}
	if err := writer.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	var retired atomic.Bool
	p := &process{input: writer, generation: 1, cancel: func() { retired.Store(true) }, waiting: make(map[string]chan workerproto.Event), done: make(chan struct{})}
	p.accepting.Store(true) // a ready worker
	h := &Host{ctx: context.Background(), query: make(chan struct{}, 1), delivery: make(chan struct{}, 1), current: p}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stop := time.AfterFunc(20*time.Millisecond, cancel)
	defer stop.Stop()
	started := time.Now()
	_, _, err = h.queryActivity(ctx)
	if !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
		t.Fatalf("blocked transport ignored cancellation: elapsed=%s error=%v", time.Since(started), err)
	}
	if !retired.Load() {
		t.Fatal("uncertain partial transport was reused")
	}
	p.mu.Lock()
	waiting := len(p.waiting)
	p.mu.Unlock()
	if waiting != 0 || len(h.query) != 0 || len(h.delivery) != 0 {
		t.Fatal("query retained waiter, slot, or transport lock")
	}
}
