package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeShutdownHost blocks Close until Kill, like a worker whose tools ignore cancellation.
type fakeShutdownHost struct {
	mu     sync.Mutex
	killed chan struct{}
	kills  int
}

func newFakeShutdownHost() *fakeShutdownHost { return &fakeShutdownHost{killed: make(chan struct{})} }

func (h *fakeShutdownHost) Close(ctx context.Context) error {
	select {
	case <-h.killed:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (h *fakeShutdownHost) Kill() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.kills++
	if h.kills == 1 {
		close(h.killed)
	}
	return nil
}

func noPendingSubmissions(context.Context) error { return nil }

func TestShutdownSecondSignalKillsStuckWorker(t *testing.T) {
	t.Parallel()
	host := newFakeShutdownHost()
	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt
	var stderr bytes.Buffer
	start := time.Now()
	err := shutdown(host, noPendingSubmissions, signals, &stderr, shutdownTiming{timeout: 10 * time.Second, noticeAfter: 10 * time.Second, forcedTimeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("shutdown after forced kill: %v", err)
	}
	if host.kills != 1 {
		t.Fatalf("kills = %d, want 1", host.kills)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("forced exit still waited for the graceful stop")
	}
	if !strings.Contains(stderr.String(), "Forcing exit") {
		t.Fatalf("stderr = %q, want forced-exit notice", stderr.String())
	}
}

func TestShutdownForcedJoinIsBounded(t *testing.T) {
	t.Parallel()
	// Kill reports success, but the worker never finishes: the join must still end.
	host := &fakeShutdownHost{killed: make(chan struct{})}
	host.kills = 1 // close(killed) is skipped; Close blocks until the context ends
	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt
	var stderr bytes.Buffer
	err := shutdown(host, noPendingSubmissions, signals, &stderr, shutdownTiming{timeout: 10 * time.Second, noticeAfter: 10 * time.Second, forcedTimeout: 50 * time.Millisecond})
	if !errors.Is(err, errForcedExit) {
		t.Fatalf("err = %v, want forced-exit cause", err)
	}
}

func TestShutdownReportsSlowStop(t *testing.T) {
	t.Parallel()
	host := newFakeShutdownHost()
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = host.Kill() // stands in for the worker eventually exiting on its own
	}()
	var stderr bytes.Buffer
	err := shutdown(host, noPendingSubmissions, nil, &stderr, shutdownTiming{timeout: 10 * time.Second, noticeAfter: 20 * time.Millisecond, forcedTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "press Ctrl+C again") {
		t.Fatalf("stderr = %q, want slow-stop notice", stderr.String())
	}
}

func TestShutdownQuietWhenFast(t *testing.T) {
	t.Parallel()
	host := newFakeShutdownHost()
	_ = host.Kill()
	var stderr bytes.Buffer
	waited := false
	err := shutdown(host, func(context.Context) error { waited = true; return nil }, nil, &stderr, defaultShutdownTiming)
	if err != nil || !waited {
		t.Fatalf("err = %v, waited = %v", err, waited)
	}
	if stderr.Len() != 0 {
		t.Fatalf("fast shutdown printed %q", stderr.String())
	}
}
