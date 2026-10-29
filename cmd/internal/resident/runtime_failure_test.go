package resident

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func TestRuntimeCandidateFailureAfterRetirementRecoversDurableInput(t *testing.T) {
	root, err := testtmp.MkdirTemp("u34.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("UA_RESIDENT_FAILURE_TEST_ROOT", root)
	path := func(name string) string { return filepath.Join(root, name) }
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(path(name), []byte("joined"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	wait := func(what string, f func() bool) {
		t.Helper()
		for !f() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", what, ctx.Err())
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	exists := func(name string) bool { _, e := os.Stat(path(name)); return e == nil }
	for _, d := range []string{"run", "state"} {
		if err := os.Mkdir(path(d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"failure-recovery\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"m\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"recovered once\"}]}]}}\n\n")
	}))
	t.Cleanup(provider.Close)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	config := Config{RuntimeReplacement: true, RuntimeDirectory: path("run"), Worker: workerhost.Config{Command: []string{exe, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "lifecycle", Directory: path("state"), Workspace: root, Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh"}}}
	var orderingFailure atomic.Bool
	config.Worker.OnStatus = func(s workerhost.Status) {
		if s.Message == "Old worker exited; starting replacement" {
			if !exists("retired") || !exists("allow-exit") || exists("candidate-attempt") {
				orderingFailure.Store(true)
			}
			if e := os.WriteFile(path("host-wait"), []byte("joined"), 0600); e != nil {
				orderingFailure.Store(true)
			}
		}
	}
	start := func(expectFailure bool) (Info, func()) {
		t.Helper()
		serveCtx, serveCancel := context.WithCancel(context.Background())
		ready, done := make(chan Info, 1), make(chan error, 1)
		go func() { done <- Serve(serveCtx, config, func(i Info) { ready <- i }) }()
		joined := false
		stop := func() {
			// Release the owned process BEFORE cancellation, including failure paths.
			_ = os.WriteFile(path("allow-exit"), nil, 0600)
			serveCancel()
			if joined {
				return
			}
			joined = true
			select {
			case e := <-done:
				if expectFailure {
					if e == nil || !errors.Is(e, workerhost.ErrStopped) {
						t.Errorf("failed resident actual join: %v", e)
					}
				} else if e != nil {
					t.Errorf("healthy resident actual join: %v", e)
				}
			case <-time.After(10 * time.Second):
				t.Error("resident actual join unconfirmed")
			}
		}
		t.Cleanup(stop)
		select {
		case i := <-ready:
			return i, stop
		case e := <-done:
			joined = true
			serveCancel()
			t.Fatalf("startup: %v", e)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		panic("unreachable")
	}
	dial := func() (*attachclient.Client, func()) {
		t.Helper()
		c := lifecycleDial(t, ctx, path("run"), path("state"))
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			for {
				if _, e := c.Next(); e != nil {
					return
				}
			}
		}()
		stop := func() {
			_ = c.Close()
			select {
			case <-readDone:
			case <-time.After(5 * time.Second):
				t.Error("client reader join unconfirmed")
			}
		}
		t.Cleanup(stop)
		return c, stop
	}
	first, stopFirst := start(true)
	c, closeFirst := dial()
	gen := c.Snapshot().Header.Status.Generation
	if gen != 1 {
		t.Fatalf("initial generation=%d", gen)
	}
	write("startup-fail")
	type result struct {
		s   attachproto.RuntimeStatus
		err error
	}
	replaced := make(chan result, 1)
	go func() {
		s, e := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: gen})
		replaced <- result{s, e}
	}()
	wait("old Run joined", func() bool { return exists("retired") })
	pidBytes, err := os.ReadFile(path("retired"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("old process not held after Run returned: %v", err)
	}
	if exists("candidate-attempt") || exists("host-wait") {
		t.Fatal("candidate launched before actual old exit")
	}
	input := inbox.Input{ID: "private-fenced-stable", Kind: inbox.InputExternal, Payload: []byte(`"recover accepted input"`)}
	if err := c.Submit(ctx, input); err != nil {
		t.Fatalf("durable fenced admission: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("fenced input reached provider")
	}
	if exists("candidate-attempt") {
		t.Fatal("candidate bypassed old-process exit gate")
	}
	write("allow-exit")
	var r result
	select {
	case r = <-replaced:
	case <-ctx.Done():
		t.Fatal("replacement did not settle")
	}
	var typed *attachclient.RuntimeError
	if !errors.As(r.err, &typed) || r.s.State != "stopped" || r.s.State == "completed" {
		t.Fatalf("post-retirement failure: %+v %v", r.s, r.err)
	}
	receipt, err := os.ReadFile(path("candidate-attempt"))
	if err != nil || string(receipt) != "host-wait=joined" || orderingFailure.Load() {
		t.Fatalf("candidate/Host Wait ordering receipt=%q err=%v bad=%v", receipt, err, orderingFailure.Load())
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("old actual process still present: %v", err)
	}
	again, e := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: 1})
	if !errors.As(e, &typed) || again.State != "failed" || again.Generation != 0 || !strings.Contains(e.Error(), "stale runtime generation") {
		t.Fatalf("latched failure bypassed: %+v %v", again, e)
	}
	if calls.Load() != 0 {
		t.Fatal("automatic restart/replay after candidate failure")
	}
	closeFirst()
	stopFirst()
	journal, err := ingress.Open(filepath.Join(path("state"), "ingress"), "lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	pending := journal.Pending()
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != input.ID || string(pending[0].Payload) != string(input.Payload) {
		t.Fatalf("durable accepted input lost: %+v", pending)
	}
	if err := os.Remove(path("startup-fail")); err != nil {
		t.Fatal(err)
	}
	second, stopSecond := start(false)
	if second.Incarnation == first.Incarnation {
		t.Fatal("fresh host reused incarnation")
	}
	healthy, closeHealthy := dial()
	store, err := localfile.New(path("state"))
	if err != nil {
		t.Fatal(err)
	}
	canonicalCount := func() int {
		page, e := store.Items(ctx, "lifecycle", 0, 100)
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, item := range page.Items {
			if in, ok := item.Data.(inbox.Input); ok && in.ID == input.ID {
				n++
			}
		}
		return n
	}
	wait("canonical recovery", func() bool { return canonicalCount() == 1 && calls.Load() == 1 })
	if err := healthy.Submit(ctx, input); err != nil {
		t.Fatalf("stable body retry: %v", err)
	}
	// Check after actual worker/host joins, not just a provider-entry observation.
	closeHealthy()
	stopSecond()
	if canonicalCount() != 1 || calls.Load() != 1 {
		t.Fatalf("recovery duplicated canonical input/provider: count=%d calls=%d", canonicalCount(), calls.Load())
	}
	journal, err = ingress.Open(filepath.Join(path("state"), "ingress"), "lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	pending = journal.Pending()
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("journal Pending not cleared: %+v", pending)
	}
	if strings.TrimSpace(string(receipt)) != "host-wait=joined" {
		t.Fatal("bad ordering receipt")
	}
}
