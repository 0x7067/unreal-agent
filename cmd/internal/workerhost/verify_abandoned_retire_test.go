package workerhost

// Real-runtime regression coverage for abandoned retirement in
// cmd/internal/workerhost/host.go.
//
// Construction path: the real Start()/start() (frontend lease, durable ingress,
// stage/probe of a private executable copy, launch, consume, dispatch) on a
// normal filesystem. The worker is the real workerruntime: the host's direct
// child is a tiny relay that exec()s the repository's own worker fixture
// (host_test.go TestMain "worker" mode with UA_HOST_TEST_REAL=1) and forwards
// every workerproto frame unchanged. The relay exists for exactly one reason:
// workerruntime answers quiesce synchronously inside serve() using
// coordinator.TryQuiesce on an idle loop, so there is no injectable point at
// which the real worker can be made to answer late. The relay holds the single
// quiesce frame after the host wrote it and before the worker saw it, which is
// the cancellation window under test; everything else - hello, ready, replay, item,
// quiesced, exit status - is the real worker's.
//
// Assertions use only observable state: the Status sequence delivered to
// OnStatus, the return value of Submit, delivered session items, and Done().
// generation() (host_test.go) is used only as a settle barrier, never as an
// assertion about the status behavior.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// Cancellation abandons the Reload caller, not the admitted quiesce. A later
// clean exit must publish stopped while keeping the durable host recoverable.
func TestVerifyAbandonedRetireReportsStopped(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "quiesce-gate")
	h, records := verifyFixture(t, gate)
	if status := verifyLast(records); status.Message != "Worker ready" || status.Stopped {
		t.Fatalf("fixture did not reach a ready worker: %s", verifyStatusLine(status))
	}

	// The caller owns this context; canceling it must not cancel the host
	// lifetime, which derives from the Start context only.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- h.Reload(ctx, nil) }()
	// The relay has read the quiesce frame the host wrote and has not passed it
	// to the worker: canceling now is exactly "canceled after the frame was
	// written, before any reply exists".
	verifyWaitFile(t, gate+".seen", "the peer to admit the quiesce request")
	cancel()
	var reloadErr error
	select {
	case reloadErr = <-result:
	case <-time.After(30 * time.Second):
		t.Fatal("Reload did not return after its context was canceled")
	}
	if reloadErr == nil || !strings.Contains(reloadErr.Error(), "quiesce outcome unconfirmed; no replacement started") {
		t.Fatalf("Reload = %v, want the abandoned-quiesce error", reloadErr)
	}

	// Release the held frame: the real worker answers the abandoned request and
	// retires on its own.
	if err := os.WriteFile(gate, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := strings.TrimSpace(string(verifyWaitFile(t, gate+".exited", "the real worker to exit"))); code != "0" {
		t.Fatalf("real worker exit status %q, want a clean 0", code)
	}
	// Wait until the host has finished consuming the retirement before asserting.
	verifyWait(t, "the host to retire the process", func() bool {
		_, pid := generation(h)
		return pid == 0
	})

	statuses := verifyStatuses(records)
	last := verifyLast(records)
	ready := statuses[0]
	if !last.Stopped || last.Err != nil || last.Generation != ready.Generation || last.Build != ready.Build {
		t.Fatalf("last status after clean retirement = %s; statuses: %s", verifyStatusLine(last), verifyStatusLines(statuses))
	}
	advAssertHostOpen(t, h)

	// Submit is durable admission, not a worker-liveness promise. The input
	// remains pending until an explicit Reload recovers the safe stopped host.
	input := inbox.Input{ID: "pending-input", Kind: inbox.InputExternal, Payload: []byte(`"pending"`)}
	if err := h.Submit(t.Context(), input); err != nil {
		t.Fatalf("Submit after clean retirement: %v", err)
	}
	if pending := h.journal.Pending(); len(pending) != 1 || pending[0].ID != input.ID {
		t.Fatalf("pending durable input = %+v", pending)
	}
	for _, item := range verifyItems(records) {
		if value, ok := item.Data.(inbox.Input); ok && value.ID == input.ID {
			t.Fatal("input delivered without a worker")
		}
	}
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatalf("recover after clean retirement: %v", err)
	}
	waitInput(t, records, string(input.ID))
	count := 0
	for _, item := range verifyItems(records) {
		if value, ok := item.Data.(inbox.Input); ok && value.ID == input.ID {
			count++
		}
	}
	if count != 1 || len(h.journal.Pending()) != 0 {
		t.Fatalf("recovered input count=%d pending=%+v", count, h.journal.Pending())
	}
	if status := verifyLast(records); status.Stopped || status.Err != nil || status.Generation <= ready.Generation || status.Message != "Worker ready" {
		t.Fatalf("last recovery status = %s", verifyStatusLine(status))
	}
	advAssertHostOpen(t, h)
}

// TestVerifyAbandonedRetireHarnessControl is the positive control for the same
// fixture: an ordinary reload through the same relay publishes the handoff
// status and the ready status, and inputs reach the worker on both sides of it.
func TestVerifyAbandonedRetireHarnessControl(t *testing.T) {
	h, records := verifyFixture(t, "")
	if status := verifyLast(records); status.Message != "Worker ready" || status.Stopped {
		t.Fatalf("fixture did not reach a ready worker: %s", verifyStatusLine(status))
	}
	send(t, h, "control-first", "first")
	waitInput(t, records, "control-first")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := h.Reload(ctx, nil); err != nil {
		t.Fatalf("plain reload through the same relay failed: %v", err)
	}
	handoff, ready := false, false
	for _, status := range verifyStatuses(records) {
		if status.Stopped && status.Message == "Old worker exited; starting replacement" {
			handoff = true
		}
		if status.Message == "Worker ready" && status.Generation > 1 {
			ready = true
		}
	}
	if !handoff || !ready {
		t.Fatalf("control reload did not publish handoff+ready statuses: %s", verifyStatusLines(verifyStatuses(records)))
	}
	if last := verifyLast(records); last.Message != "Worker ready" || last.Stopped {
		t.Fatalf("control reload left status %s", verifyStatusLine(last))
	}
	send(t, h, "control-second", "second")
	waitInput(t, records, "control-second")
}

// An explicitly requested Reload can recover a safely stopped worker and
// deliver inputs durably admitted after its caller abandoned retirement.
func TestVerifyAbandonedRetireInputRecoveredByReload(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "quiesce-gate")
	h, records := verifyFixture(t, gate)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- h.Reload(ctx, nil) }()
	verifyWaitFile(t, gate+".seen", "the peer to admit the quiesce request")
	cancel()
	if err := advReceiveError(t, result); err == nil || !strings.Contains(err.Error(), "quiesce outcome unconfirmed") {
		t.Fatalf("canceled Reload = %v, want an unconfirmed-quiesce error", err)
	}
	if err := os.WriteFile(gate, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	verifyWaitFile(t, gate+".exited", "the real worker to exit")
	verifyWait(t, "the host to retire the process", func() bool {
		_, pid := generation(h)
		return pid == 0
	})
	if err := h.Submit(context.Background(), inbox.Input{ID: "late-input", Kind: inbox.InputExternal, Payload: []byte(`"late"`)}); err != nil {
		t.Fatalf("Submit after the abandoned retirement = %v", err)
	}
	// Recovery requires a proven safe retirement, rather than an arbitrary crash.
	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reloadCancel()
	if err := h.Reload(reloadCtx, nil); err != nil {
		t.Fatalf("second reload after the abandoned retirement failed: %v", err)
	}
	waitInput(t, records, "late-input")
}

// verifyFixture starts a real Host whose worker is the real workerruntime behind
// the frame relay. An empty gate leaves the relay transparent.
func verifyFixture(t *testing.T, gate string) (*Host, *recorder) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	peer := buildVerifyPeer(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		completed(w, 1, assistant(1, "verification fixture response"))
	}))
	t.Cleanup(server.Close)
	t.Setenv("UA_VERIFY_REAL_WORKER", executable)
	t.Setenv("UA_VERIFY_QUIESCE_GATE", gate)
	h, records, _ := realHostWith(t, server, false, func(config *Config) {
		config.Command = []string{peer, "worker"}
	})
	// The host validates the probed identity against the running worker's own
	// hello, so this build string is proof that the real workerruntime is on the
	// other side of the relay (host_test.go helper, UA_HOST_TEST_REAL branch).
	if status := verifyLast(records); !strings.Contains(status.Build, "real-runtime-fixture") {
		t.Fatalf("fixture worker is not the real workerruntime: %s", verifyStatusLine(status))
	}
	return h, records
}

// buildVerifyPeer compiles the relay into a private directory. It is stdlib-only
// so it builds outside the module (it needs the go command on PATH; GOCACHE and
// GOTMPDIR default to /tmp when unset), and it imports nothing from the repository:
// every frame it forwards is the real worker's own workerproto JSONL.
func buildVerifyPeer(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	source := filepath.Join(directory, "peer.go")
	if err := os.WriteFile(source, []byte(verifyPeerProgram), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "verify-peer")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, source)
	build.Env = verifyBuildEnv()
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build protocol relay: %v\n%s", err, output)
	}
	return binary
}

func verifyBuildEnv() []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); name == "GOFLAGS" {
			continue
		}
		env = append(env, entry)
	}
	if os.Getenv("GOCACHE") == "" {
		env = append(env, "GOCACHE=/tmp/gocache")
	}
	if os.Getenv("GOTMPDIR") == "" {
		env = append(env, "GOTMPDIR=/tmp")
	}
	return append(env, "GOFLAGS=")
}

func verifyStatuses(records *recorder) []Status {
	records.mu.Lock()
	defer records.mu.Unlock()
	return append([]Status(nil), records.statuses...)
}

func verifyItems(records *recorder) []sessionstore.Item {
	records.mu.Lock()
	defer records.mu.Unlock()
	return append([]sessionstore.Item(nil), records.items...)
}

func verifyLast(records *recorder) Status {
	statuses := verifyStatuses(records)
	if len(statuses) == 0 {
		return Status{}
	}
	return statuses[len(statuses)-1]
}

func verifyStatusLine(status Status) string {
	return fmt.Sprintf("{generation=%d build=%q message=%q stopped=%v err=%v}", status.Generation, status.Build, status.Message, status.Stopped, status.Err)
}

func verifyStatusLines(statuses []Status) string {
	lines := make([]string, 0, len(statuses))
	for _, status := range statuses {
		lines = append(lines, verifyStatusLine(status))
	}
	return "[" + strings.Join(lines, ", ") + "]"
}

func verifyWaitFile(t *testing.T, path, what string) []byte {
	t.Helper()
	data, ok := verifyAwait(t, what, func() ([]byte, bool) {
		value, err := os.ReadFile(path)
		return value, err == nil && len(value) > 0
	})
	if !ok {
		t.Fatalf("timed out waiting for %s (%s)", what, path)
	}
	return data
}

func verifyWait(t *testing.T, what string, done func() bool) {
	t.Helper()
	if _, ok := verifyAwait(t, what, func() (struct{}, bool) { return struct{}{}, done() }); !ok {
		t.Fatalf("timed out waiting for %s", what)
	}
}

func verifyAwait[T any](t *testing.T, what string, probe func() (T, bool)) (T, bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if value, ok := probe(); ok {
			return value, true
		}
		if time.Now().After(deadline) {
			var zero T
			return zero, false
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// verifyPeerProgram relays workerproto JSONL between the host and the real
// worker. In gated mode it holds a quiesce request until the gate file appears,
// publishing "<gate>.seen" when it has read the frame and "<gate>.exited" with
// the real worker's exit status once it has joined.
const verifyPeerProgram = `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func main() {
	real := os.Getenv("UA_VERIFY_REAL_WORKER")
	if real == "" {
		fail("UA_VERIFY_REAL_WORKER is unset")
	}
	mode := os.Args[len(os.Args)-1]
	if mode != "worker" {
		args := []string{real, "-test.run=^$", mode}
		if mode == "agent-runner" {
			args = append([]string{real, "agent-runner"}, os.Args[1:len(os.Args)-1]...)
		}
		fmt.Fprintf(os.Stderr, "verify relay pid=%d exec %q mode=%s\n", os.Getpid(), real, mode)
		if err := syscall.Exec(real, args, os.Environ()); err != nil {
			fail(err.Error())
		}
	}
	gate := os.Getenv("UA_VERIFY_QUIESCE_GATE")
	child := exec.Command(real, "-test.run=^$", "worker")
	child.Env = os.Environ()
	child.Stderr = os.Stderr
	input, err := child.StdinPipe()
	if err != nil {
		fail(err.Error())
	}
	output, err := child.StdoutPipe()
	if err != nil {
		fail(err.Error())
	}
	if err := child.Start(); err != nil {
		fail(err.Error())
	}
	fmt.Fprintf(os.Stderr, "verify relay pid=%d real worker pid=%d gate=%q\n", os.Getpid(), child.Process.Pid, gate)
	go relayToWorker(input, gate)
	reader := bufio.NewReader(output)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, writeErr := os.Stdout.Write(line); writeErr != nil {
				_ = child.Process.Kill()
				_ = child.Wait()
				fail(writeErr.Error())
			}
		}
		if readErr != nil {
			break
		}
	}
	code := 0
	if waitErr := child.Wait(); waitErr != nil {
		code = 1
		if exit, ok := waitErr.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
	}
	if gate != "" {
		_ = os.WriteFile(gate+".exited", []byte(fmt.Sprintf("%d", code)), 0o600)
	}
	os.Exit(code)
}

func relayToWorker(input io.WriteCloser, gate string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			if gate != "" && isQuiesce(line) {
				_ = os.WriteFile(gate+".seen", []byte("seen"), 0o600)
				if !waitForFile(gate, 30*time.Second) {
					fail("quiesce gate was never released")
				}
			}
			if _, writeErr := input.Write(line); writeErr != nil {
				_ = input.Close()
				return
			}
		}
		if readErr != nil {
			_ = input.Close()
			return
		}
	}
}

func isQuiesce(line []byte) bool {
	var request struct{ Type string }
	if json.Unmarshal(line, &request) != nil {
		return false
	}
	return request.Type == "quiesce"
}

func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "verify peer:", message)
	os.Exit(3)
}
`
