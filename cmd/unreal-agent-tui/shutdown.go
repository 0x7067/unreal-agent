package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type shutdownHost interface {
	Close(context.Context) error
	Kill() error
}

type shutdownTiming struct {
	// timeout bounds the graceful stop. It leaves headroom beyond the host's 20s
	// graceful/escalation bound so the frontend does not exit at the same
	// instant the worker is forcibly killed.
	timeout time.Duration
	// noticeAfter is how long a silent stop may take before the user is told
	// what is happening and how to force it.
	noticeAfter time.Duration
	// forcedTimeout bounds the join after the worker group was killed.
	forcedTimeout time.Duration
}

var errForcedExit = errors.New("forced exit: the worker did not stop after being killed")

var defaultShutdownTiming = shutdownTiming{timeout: 30 * time.Second, noticeAfter: time.Second, forcedTimeout: 3 * time.Second}

// shutdown stops the worker host and joins pending submissions. Close runs
// first: the host cancels and joins durable admissions, including an active
// Submit that cannot finish from caller cancellation alone. A slow stop is
// reported on stderr, and one signal (a second Ctrl+C) kills the worker group
// instead of waiting out tools or subagents that ignore cancellation.
func shutdown(host shutdownHost, wait func(context.Context) error, signals <-chan os.Signal, stderr io.Writer, timing shutdownTiming) error {
	ctx, cancel := context.WithTimeoutCause(context.Background(), timing.timeout, errors.New("worker shutdown timed out"))
	defer cancel()
	forcedCtx, force := context.WithCancelCause(ctx)
	defer force(nil)
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		notice := time.NewTimer(timing.noticeAfter)
		defer notice.Stop()
		for {
			select {
			case <-done:
				return
			case <-notice.C:
				_, _ = fmt.Fprintln(stderr, "Stopping the worker and its tools; press Ctrl+C again to force exit.")
			case <-signals:
				_, _ = fmt.Fprintln(stderr, "Forcing exit: killing the worker; its running shell commands may be left behind.")
				if err := host.Kill(); err != nil {
					_, _ = fmt.Fprintln(stderr, "kill worker:", err)
				}
				forced := time.AfterFunc(timing.forcedTimeout, func() { force(errForcedExit) })
				defer forced.Stop()
				<-done
				return
			}
		}
	}()
	err := errors.Join(host.Close(forcedCtx), wait(forcedCtx))
	// Join the reporter so no notice is written after the caller moves on.
	close(done)
	<-finished
	return err
}
