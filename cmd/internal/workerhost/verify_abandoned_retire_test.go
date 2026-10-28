package workerhost

// Real-runtime cancellation coverage: a transparent relay holds the ordered
// drain frame after the host admitted retirement but before the actual worker
// sees it. Everything else (history, cleanup receipt, process exit) is real.

import (
	"context"
	"errors"
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

// Canceling a wait cannot undo retirement or abandon eventual replacement.
func TestVerifyAbandonedRetireCompletesReplacement(t *testing.T) {
	verifyAbandonedWait(t, false)
}
func verifyAbandonedWait(t *testing.T, deadline bool) {
	gate := filepath.Join(t.TempDir(), "drain-gate")
	h, records := verifyFixture(t, gate)
	before, pid := generation(h)
	var ctx context.Context
	var cancel context.CancelFunc
	if deadline {
		ctx, cancel = context.WithTimeout(t.Context(), 5*time.Second)
	} else {
		ctx, cancel = context.WithCancel(t.Context())
	}
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- h.Reload(ctx, nil) }()
	verifyWaitFile(t, gate+".seen", "relay to hold admitted drain frame")
	if !deadline {
		cancel()
	}
	err := advReceiveError(t, result)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("client wait = %v", err)
	}
	if !h.ReloadPending() {
		t.Fatal("client cancellation abandoned host completion")
	}
	input := inbox.Input{ID: "pending-input", Kind: inbox.InputExternal, Payload: []byte(`"pending"`)}
	if err := h.Submit(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	for _, item := range verifyItems(records) {
		if value, ok := item.Data.(inbox.Input); ok && value.ID == input.ID {
			t.Fatal("post-cutoff input reached old worker")
		}
	}
	if err := os.WriteFile(gate, []byte("release\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.AwaitReload(t.Context()); err != nil {
		t.Fatal(err)
	}
	waitInput(t, records, string(input.ID))
	if after, next := generation(h); after != before+1 || next == 0 || next == pid {
		t.Fatal("host did not complete exactly one replacement")
	}
	count := 0
	for _, item := range verifyItems(records) {
		if value, ok := item.Data.(inbox.Input); ok && value.ID == input.ID {
			count++
		}
	}
	if count != 1 || len(h.journal.Pending()) != 0 {
		t.Fatalf("queued input count=%d pending=%+v", count, h.journal.Pending())
	}
	if last := verifyLast(records); last.Stopped || last.Err != nil || last.Message != "Worker ready" {
		t.Fatalf("host result status %s", verifyStatusLine(last))
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

// Client deadline expiry is likewise only wait cancellation, not drain authority.
func TestVerifyReloadDeadlineStillCompletesReplacement(t *testing.T) {
	verifyAbandonedWait(t, true)
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
	return request.Type == "drain"
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
