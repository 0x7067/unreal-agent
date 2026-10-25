package workerhost

// Adversarial tests for the worker reload/handoff protocol.
//
// Every test here starts its host through the real start() (the function behind
// Start: frontend lease, durable ingress, stage/probe, launch, consume, dispatch).
// advHost only falls back to building those exact fields by hand when start()
// fails with the sandbox-specific lease error described in advHost -- in that
// environment start() cannot run at all, including for the pre-existing tests in
// this package.
//
// The fake peer is a POSIX shell script (advWorkerScript). The shared fixture in
// host_test.go cannot express a slow quiesce, a slow item, a bad probe, or a
// malformed frame.

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

const advWorkerScript = `#!/bin/sh
# Protocol-only test peer. $1 is worker-probe or worker (the host rewrites the
# last argument of the configured command). It owns no session store: its log is
# its durable history, so replay/dedup/ack behave like the real worker.
mode=$1
hello='{"Type":"hello","Identity":{"Protocol":1,"SessionVersion":2,"Build":"adversarial-fake"}}'
log=$FAKE_WORKER_LOG
received=$FAKE_WORKER_RECEIVED
seq=0

item_for() {
	printf '{"ID":"%s","Kind":"external","Payload":"body"}' "$1"
}

emit_item() {
	printf '{"Type":"item","Item":{"Sequence":%s,"RecordedAt":"2024-01-01T00:00:00Z","Kind":"input","Data":%s}}\n' "$1" "$2"
}

replay() {
	if [ -f "$log" ]; then
		while IFS= read -r id; do
			seq=$((seq + 1))
			emit_item "$seq" "$(item_for "$id")"
		done < "$log"
	fi
}

await_gate() {
	: > "$1.seen"
	n=0
	while [ ! -f "$1" ]; do
		n=$((n + 1))
		[ "$n" -gt 20000 ] && exit 9
		sleep 0.005
	done
}

probe() {
	case "$FAKE_PROBE" in
	bad-identity)
		printf '%s\n' '{"Type":"hello","Identity":{"Protocol":99,"SessionVersion":2,"Build":"adversarial-fake"}}'
		;;
	extra-frame)
		printf '%s\n' "$hello"
		printf '%s\n' '{"Type":"ready"}'
		;;
	silent)
		:
		;;
	garbage)
		printf '%s\n' 'not-json'
		;;
	*)
		printf '%s\n' "$hello"
		;;
	esac
}

worker() {
	case "$FAKE_WORKER" in
	duplicate-hello)
		printf '%s\n' "$hello"
		printf '%s\n' "$hello"
		exit 0
		;;
	ready-before-hello)
		printf '%s\n' '{"Type":"ready"}'
		exit 0
		;;
	malformed-frame)
		printf '%s\n' "$hello"
		printf '%s\n' '{"Type":"item"'
		exit 0
		;;
	truncated-frame)
		printf '%s\n' "$hello"
		printf '%s' '{"Type":"item"'
		exit 0
		;;
	esac
	printf '%s\n' "$hello"
	IFS= read -r start || exit 0
	replay
	printf '%s\n' '{"Type":"ready"}'
	while IFS= read -r line; do
		case "$line" in
		*'"Type":"input"'*)
			id=$(printf '%s' "$line" | sed -n 's/.*"ID": *"\([^"]*\)".*/\1/p')
			data=$(printf '%s' "$line" | sed 's/^.*"Input"://; s/}$//')
			printf '%s\n' "$id" >> "$received"
			if [ -f "$log" ] && grep -qxF -- "$id" "$log"; then
				continue
			fi
			if [ -n "$FAKE_ITEM_GATE" ]; then
				await_gate "$FAKE_ITEM_GATE"
			fi
			printf '%s\n' "$id" >> "$log"
			seq=$((seq + 1))
			emit_item "$seq" "$data"
			;;
		*'"quiesce"'*)
			qid=$(printf '%s' "$line" | sed -n 's/.*"ID": *"\([^"]*\)".*/\1/p')
			if [ -n "$FAKE_QUIESCE_GATE" ]; then
				await_gate "$FAKE_QUIESCE_GATE"
			fi
			printf '{"Type":"quiesced","ID":"%s"}\n' "$qid"
			exit 0
			;;
		esac
	done
	exit 0
}

case "$mode" in
worker-probe) probe ;;
worker) worker ;;
*) exit 64 ;;
esac
`

// advHost starts a host through the real start() and returns the recorder that
// observes items and statuses.
//
// Context lifetime contract: start() derives the Host's entire
// lifetime from the ctx it is given, so advHost must never cancel that ctx while
// returning. It is bounded by a generous timeout and released from t.Cleanup,
// which runs LIFO: the host is closed first, then the lifetime is released.
//
// Fallback: in some sandboxes the root directory is owned by neither root nor the
// current user, so every localfile.AcquireLease fails (lease_unix.go:37) and even
// the pre-existing tests in this package cannot start a host. Only in that case
// advHost builds the same Host fields start() initializes, minus the frontend
// lease, and says so loudly. A successful start() is asserted to be the real path
// (frontend lease present), so fallback state can never hide inside it.
func advHost(t *testing.T, configure func(*Config)) (*Host, *recorder, Config) {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(directory, "fake-worker.sh")
	if err = os.WriteFile(script, []byte(advWorkerScript), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_WORKER_LOG", filepath.Join(directory, "worker.log"))
	t.Setenv("FAKE_WORKER_RECEIVED", filepath.Join(directory, "received.log"))
	r := &recorder{changed: make(chan struct{}, 1)}
	// The ingress journal is opened by exactly one owner of this directory:
	// start() on a normal machine, or the fallback below. Opening it here first
	// would make start()'s own ingress.Open fail with ErrLocked.
	cfg := Config{
		Command:       []string{"/bin/sh", script, "worker"},
		Bootstrap:     workerproto.Bootstrap{SessionID: "adversarial-session", Directory: directory, Workspace: directory},
		OnItem:        r.item,
		OnStatus:      r.status,
		Stderr:        io.Discard,
		ShutdownGrace: 2 * time.Second,
	}
	if configure != nil {
		configure(&cfg)
	}
	if len(cfg.Candidate) == 0 {
		cfg.Candidate = append([]string(nil), cfg.Command...)
	}
	// Registered first so it runs last: the host is closed before this releases
	// the lifetime the host is running on.
	lifetime, releaseLifetime := context.WithTimeout(t.Context(), 90*time.Second)
	t.Cleanup(releaseLifetime)
	h, err := start(lifetime, cfg, nil)
	if err == nil {
		// start() succeeded: no fallback state may exist in this host.
		if h.parentLease == nil {
			t.Fatal("start() returned a host without a frontend lease: the fallback built this host")
		}
		advCloseHost(t, h)
		advAssertLiveHost(t, h)
		return h, r, cfg
	}
	if !strings.Contains(err.Error(), "session lease ancestor must be owned by root or current user") {
		t.Fatalf("start: %v", err)
	}
	t.Logf("FIXTURE FALLBACK: start() cannot acquire a frontend lease here (%v); building the same Host fields by hand. This run does NOT exercise start()", err)
	h = advFallbackHost(t, lifetime, releaseLifetime, cfg)
	advCloseHost(t, h)
	advAssertLiveHost(t, h)
	return h, r, cfg
}

// advCloseHost joins the host. It is registered after releaseLifetime, so LIFO
// order closes the host before its lifetime is released.
func advCloseHost(t *testing.T, h *Host) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := h.Close(ctx); err != nil {
			t.Errorf("close host: %v", err)
		}
	})
}

// advAssertLiveHost rejects a host whose lifetime was already canceled, which is
// exactly the failure mode a deferred cancel() in this helper would produce.
func advAssertLiveHost(t *testing.T, h *Host) {
	t.Helper()
	if err := context.Cause(h.ctx); err != nil {
		t.Fatalf("host lifetime canceled before the test could use it: %v", err)
	}
	if _, _, _, failure, live := advState(h); !live || failure != nil {
		t.Fatalf("no live worker after the fixture built the host: live=%v failure=%v", live, failure)
	}
}

// advFallbackHost mirrors the field initialization in start()
// for environments where the frontend lease cannot be acquired.
func advFallbackHost(t *testing.T, lifetime context.Context, cancel context.CancelFunc, cfg Config) *Host {
	t.Helper()
	root := filepath.Join(cfg.Bootstrap.Directory, "ingress")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	journal, err := ingress.Open(root, session.ID(cfg.Bootstrap.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	h := &Host{
		config:         cfg,
		ctx:            lifetime,
		cancel:         cancel,
		journal:        journal,
		done:           make(chan error, 1),
		kick:           make(chan struct{}, 1),
		dispatcherDone: make(chan struct{}),
		delivery:       make(chan struct{}, 1),
		admissionDone:  make(chan struct{}),
	}
	// launchCtx bounds only the stage()/launch() handshake: launch() derives the
	// process context from h.ctx, so canceling it here cannot end the
	// host.
	launchCtx, stop := context.WithTimeout(lifetime, 20*time.Second)
	defer stop()
	stage, err := h.stage(launchCtx, cfg.Command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.launch(launchCtx, stage); err != nil {
		t.Fatal(err)
	}
	go h.dispatch()
	h.wake()
	return h
}

func advStageDir(cfg Config) string {
	return filepath.Join(cfg.Bootstrap.Directory, "workers", string(cfg.Bootstrap.SessionID))
}

func advStageFiles(t *testing.T, cfg Config) []string {
	t.Helper()
	entries, err := os.ReadDir(advStageDir(cfg))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func advLogPath(cfg Config, name string) string {
	return filepath.Join(cfg.Bootstrap.Directory, name)
}

func advCounts(t *testing.T, path string) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return counts
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			counts[line]++
		}
	}
	return counts
}

// advUntil is a bounded rendezvous. Callers always follow it with an explicit
// assertion about what must not have happened.
func advUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func advState(h *Host) (pid int, generation uint64, stopped bool, failure error, hasCurrent bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current != nil {
		hasCurrent = true
		pid = h.current.command.Process.Pid
	}
	return pid, h.generation, h.stopped, h.failure, hasCurrent
}

func advStatuses(r *recorder) []Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Status(nil), r.statuses...)
}

func advItemCounts(r *recorder) map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := make(map[string]int)
	for _, item := range r.items {
		if input, ok := item.Data.(inbox.Input); ok {
			counts[string(input.ID)]++
		}
	}
	return counts
}

func advBody(t *testing.T) inbox.Input {
	t.Helper()
	payload, err := json.Marshal("body")
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{Kind: inbox.InputExternal, Payload: payload}
}

func advSubmit(t *testing.T, h *Host, id string) error {
	t.Helper()
	input := advBody(t)
	input.ID = inbox.ID(id)
	return h.Submit(t.Context(), input)
}

// A canceled Reload leaves the admitted quiesce request live. Its eventual clean
// retirement must publish stopped without ending durable admission or recovery.
func TestCanceledReloadReportsLateRetirement(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "quiesce-gate")
	t.Setenv("FAKE_QUIESCE_GATE", gate)
	h, r, _ := advHost(t, nil)
	h.mu.Lock()
	p := h.current
	h.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- h.Reload(ctx, nil) }()
	waitFile(t, gate+".seen")
	cancel()
	if err := advReceiveError(t, result); !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "quiesce outcome unconfirmed") {
		t.Fatalf("Reload before reply = %v", err)
	}
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, p.done, "late retirement did not complete")
	advAssertCleanRetirement(t, h, r, p)
	advAssertRetirementRecovery(t, h, r, p, "late-input")
}

// The stopped callback is part of the handoff barrier. Exercise both a normal
// reload and cancellation after the reply has been received but before p.done.
func TestRetirementStatusCallbackFencesReplacement(t *testing.T) {
	for _, cancelReload := range []bool{false, true} {
		name := "normal-reload"
		if cancelReload {
			name = "canceled-after-reply"
		}
		t.Run(name, func(t *testing.T) {
			gate := filepath.Join(t.TempDir(), "quiesce-gate")
			t.Setenv("FAKE_QUIESCE_GATE", gate)
			entered, release := make(chan Status, 1), make(chan struct{})
			h, r, _ := advHost(t, func(config *Config) {
				original := config.OnStatus
				config.OnStatus = func(status Status) {
					if status.Generation == 1 && status.Stopped {
						entered <- status
						<-release
					}
					original(status)
				}
			})
			// Registered after host cleanup so failure also releases the callback.
			open := releaseGate(t, release)
			h.mu.Lock()
			p := h.current
			h.mu.Unlock()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- h.Reload(ctx, nil) }()
			waitFile(t, gate+".seen")
			// Capture the sole request's channel before the worker can reply. An
			// empty channel after callback entry proves Reload consumed the reply.
			p.mu.Lock()
			var reply chan workerproto.Event
			for _, waiter := range p.waiting {
				reply = waiter
			}
			p.mu.Unlock()
			if reply == nil {
				t.Fatal("no quiesce waiter")
			}
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			select {
			case status := <-entered:
				if !status.Stopped || status.Err != nil || status.Generation != p.generation || status.Build != p.build {
					t.Fatalf("retirement callback = %+v", status)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("retirement callback not entered")
			}
			advUntil(t, "Reload to receive the quiesced reply", func() bool { return len(reply) == 0 })
			select {
			case <-p.done:
				t.Fatal("p.done closed before retirement callback returned")
			default:
			}
			h.mu.Lock()
			current, generation := h.current, h.generation
			h.mu.Unlock()
			if current != p || generation != p.generation {
				t.Fatal("replacement crossed the blocked callback barrier")
			}
			select {
			case err := <-result:
				t.Fatalf("Reload returned before callback release: %v", err)
			default:
			}
			if cancelReload {
				cancel()
				if err := advReceiveError(t, result); !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "old worker shutdown incomplete") {
					t.Fatalf("Reload after reply = %v", err)
				}
			}
			if err := advSubmit(t, h, "callback-input"); err != nil {
				t.Fatalf("durable admission during retirement callback: %v", err)
			}
			open()
			awaitSignal(t, p.done, "retirement callback did not complete")
			if cancelReload {
				advAssertCleanRetirement(t, h, r, p)
				if err := h.Reload(t.Context(), nil); err != nil {
					t.Fatalf("recover after canceled reload: %v", err)
				}
			} else if err := advReceiveError(t, result); err != nil {
				t.Fatalf("normal reload: %v", err)
			}
			advAssertRecoveredInput(t, h, r, p, "callback-input")
		})
	}
}

func advAssertCleanRetirement(t *testing.T, h *Host, r *recorder, p *process) {
	t.Helper()
	_, generation, stopped, failure, live := advState(h)
	if live || !stopped || failure != nil || generation != p.generation {
		t.Fatalf("retired host: live=%v stopped=%v failure=%v generation=%d", live, stopped, failure, generation)
	}
	statuses := advStatuses(r)
	last := statuses[len(statuses)-1]
	if !last.Stopped || last.Err != nil || last.Generation != p.generation || last.Build != p.build {
		t.Fatalf("last retirement status = %+v", last)
	}
	advAssertHostOpen(t, h)
}

func advAssertHostOpen(t *testing.T, h *Host) {
	t.Helper()
	select {
	case err := <-h.Done():
		t.Fatalf("recoverable retirement ended host: %v", err)
	default:
	}
}

func advAssertRetirementRecovery(t *testing.T, h *Host, r *recorder, p *process, id string) {
	t.Helper()
	if err := advSubmit(t, h, id); err != nil {
		t.Fatalf("Submit after clean retirement: %v", err)
	}
	if pending := h.journal.Pending(); len(pending) != 1 || string(pending[0].ID) != id {
		t.Fatalf("durable admission after retirement = %+v", pending)
	}
	if got := advItemCounts(r)[id]; got != 0 {
		t.Fatalf("input delivered without a worker: %d", got)
	}
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatalf("recover after clean retirement: %v", err)
	}
	advAssertRecoveredInput(t, h, r, p, id)
}

func advAssertRecoveredInput(t *testing.T, h *Host, r *recorder, p *process, id string) {
	t.Helper()
	advUntil(t, "pending input to persist after recovery", func() bool { return advItemCounts(r)[id] > 0 })
	if got := advItemCounts(r)[id]; got != 1 {
		t.Fatalf("recovered input count = %d, want 1", got)
	}
	if pending := h.journal.Pending(); len(pending) != 0 {
		t.Fatalf("input still pending after recovery: %+v", pending)
	}
	statuses := advStatuses(r)
	last := statuses[len(statuses)-1]
	if last.Stopped || last.Err != nil || last.Generation <= p.generation || last.Message != "Worker ready" {
		t.Fatalf("last recovery status = %+v", last)
	}
	retirements := 0
	for _, status := range statuses {
		if status.Generation == p.generation && status.Stopped {
			retirements++
		}
	}
	if retirements != 1 {
		t.Fatalf("old generation retirement callbacks = %d, want 1", retirements)
	}
	advAssertHostOpen(t, h)
}

// TestReloadRefusedWhileDeliveredInputIsUnacknowledged pins the check-then-act
// window of the pending-journal guard. The peer accepts the input
// and holds its item frame, so the host holds an input that is durably admitted,
// already written to the live pipe, and not yet persisted: exactly the state the
// guard must catch. The reload must be refused without disturbing the worker, and
// the candidate staged by that refused attempt must be discarded.
func TestReloadRefusedWhileDeliveredInputIsUnacknowledged(t *testing.T) {
	itemGate := filepath.Join(t.TempDir(), "item-gate")
	t.Setenv("FAKE_ITEM_GATE", itemGate)
	h, _, cfg := advHost(t, nil)
	pid, beforeGeneration, _, _, _ := advState(h)
	if err := advSubmit(t, h, "in-flight"); err != nil {
		t.Fatal(err)
	}
	// The worker read the input; its persistence frame is held open.
	waitFile(t, itemGate+".seen")
	if got := advCounts(t, advLogPath(cfg, "received.log")); got["in-flight"] != 1 {
		t.Fatalf("worker received = %v", got)
	}
	if pending := h.journal.Pending(); len(pending) != 1 || pending[0].ID != "in-flight" {
		t.Fatalf("pending = %#v", pending)
	}
	err := h.Reload(h.ctx, nil)
	if !errors.Is(err, ErrBusy) || !strings.Contains(err.Error(), "has not reached session persistence") {
		t.Fatalf("reload with unacknowledged delivery = %v", err)
	}
	afterPID, afterGeneration, _, failure, hasCurrent := advState(h)
	if !hasCurrent || afterPID != pid || afterGeneration != beforeGeneration || failure != nil {
		t.Fatalf("refused reload disturbed the worker: live=%v pid %d->%d generation %d->%d failure=%v", hasCurrent, pid, afterPID, beforeGeneration, afterGeneration, failure)
	}
	if stages := advStageFiles(t, cfg); len(stages) != 1 {
		t.Fatalf("refused reload leaked a staged candidate: %v", stages)
	}
	// Persistence lands; the identical request now succeeds.
	if err := os.WriteFile(itemGate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	advUntil(t, "the delivered input to be acknowledged", func() bool { return len(h.journal.Pending()) == 0 })
	if err := h.Reload(h.ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, after, _, _, _ := advState(h); after != beforeGeneration+1 {
		t.Fatalf("generation %d after handoff, want %d", after, beforeGeneration+1)
	}
}

// TestInputAdmittedDuringQuiesceReachesOnlyReplacement proves that the delivery
// fence plus the per-generation sent map (host.go:253-276, host.go:566-569) never
// leak a mid-handoff admission into the retiring generation and never lose it: it
// is delivered to the replacement exactly once.
func TestInputAdmittedDuringQuiesceReachesOnlyReplacement(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "quiesce-gate")
	t.Setenv("FAKE_QUIESCE_GATE", gate)
	h, _, cfg := advHost(t, nil)
	_, beforeGeneration, _, _, _ := advState(h)
	received := advLogPath(cfg, "received.log")
	result := make(chan error, 1)
	go func() { result <- h.Reload(h.ctx, nil) }()
	waitFile(t, gate+".seen")
	if err := advSubmit(t, h, "mid-handoff"); err != nil {
		t.Fatal(err)
	}
	if pending := h.journal.Pending(); len(pending) != 1 || pending[0].ID != "mid-handoff" {
		t.Fatalf("held input not durably admitted: %#v", pending)
	}
	if got := advCounts(t, received); len(got) != 0 {
		t.Fatalf("retiring worker received a fenced input: %v", got)
	}
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := advReceiveError(t, result); err != nil {
		t.Fatal(err)
	}
	advUntil(t, "the replacement to receive the held input", func() bool {
		return advCounts(t, received)["mid-handoff"] == 1
	})
	if _, generation, _, failure, _ := advState(h); generation != beforeGeneration+1 || failure != nil {
		t.Fatalf("generation %d failure %v", generation, failure)
	}
	advUntil(t, "the held input to be acknowledged", func() bool { return len(h.journal.Pending()) == 0 })
	if got := advCounts(t, received); got["mid-handoff"] != 1 {
		t.Fatalf("held input delivered %d times: %v", got["mid-handoff"], got)
	}
}

// TestRejectedProbeKeepsWorkerAndLeavesNoStagedArtifact covers every way the
// capability probe can fail: an incompatible identity, an unexpected extra frame,
// no identity at all, and a non-JSON frame (stage.go:120-166). In all of them the
// old worker must keep running, the staged candidate must be discarded, and a
// later handoff must still work.
func TestRejectedProbeKeepsWorkerAndLeavesNoStagedArtifact(t *testing.T) {
	for _, mode := range []string{"bad-identity", "extra-frame", "silent", "garbage"} {
		t.Run(mode, func(t *testing.T) {
			h, _, cfg := advHost(t, nil)
			if stages := advStageFiles(t, cfg); len(stages) != 1 {
				t.Fatalf("live stage files = %v", stages)
			}
			pid, beforeGeneration, _, _, _ := advState(h)
			t.Setenv("FAKE_PROBE", mode)
			err := h.Reload(h.ctx, nil)
			if err == nil {
				t.Fatalf("%s probe accepted", mode)
			}
			if !strings.Contains(err.Error(), "candidate rejected") {
				t.Fatalf("%s: unexpected error: %v", mode, err)
			}
			afterPID, afterGeneration, _, failure, hasCurrent := advState(h)
			if !hasCurrent || afterPID != pid || afterGeneration != beforeGeneration || failure != nil {
				t.Fatalf("%s probe changed the worker: live=%v pid %d->%d generation %d->%d failure=%v", mode, hasCurrent, pid, afterPID, beforeGeneration, afterGeneration, failure)
			}
			if err := syscall.Kill(pid, 0); err != nil {
				t.Fatalf("%s: old worker is gone: %v", mode, err)
			}
			if stages := advStageFiles(t, cfg); len(stages) != 1 {
				t.Fatalf("%s leaked staged artifacts: %v", mode, stages)
			}
			// The surviving worker is still a valid handoff source.
			t.Setenv("FAKE_PROBE", "")
			if err := h.Reload(h.ctx, nil); err != nil {
				t.Fatalf("%s: reload after rejected probe: %v", mode, err)
			}
			if stages := advStageFiles(t, cfg); len(stages) != 1 {
				t.Fatalf("%s: retired stage not discarded: %v", mode, stages)
			}
		})
	}
}

// TestMalformedWorkerFrameLatchesFailureWithoutReplacing shows that a replacement
// which misbehaves after the old worker retired fails closed: the host latches the
// failure, never downgrades, spawns no second process, discards
// every stage, and keeps the durable input pending.
func TestMalformedWorkerFrameLatchesFailureWithoutReplacing(t *testing.T) {
	for _, mode := range []string{"malformed-frame", "truncated-frame", "duplicate-hello", "ready-before-hello"} {
		t.Run(mode, func(t *testing.T) {
			h, r, cfg := advHost(t, nil)
			oldPID, beforeGeneration, _, _, _ := advState(h)
			t.Setenv("FAKE_WORKER", mode)
			err := h.Reload(h.ctx, nil)
			if err == nil || !strings.Contains(err.Error(), "replacement failed after retirement") {
				t.Fatalf("%s: replacement = %v", mode, err)
			}
			advUntil(t, "the failed replacement to be joined", func() bool {
				_, _, _, failure, hasCurrent := advState(h)
				return !hasCurrent && failure != nil
			})
			_, afterGeneration, _, failure, hasCurrent := advState(h)
			if hasCurrent || failure == nil {
				t.Fatalf("%s: live=%v failure=%v", mode, hasCurrent, failure)
			}
			// Exactly one replacement attempt: no downgrade and no automatic retry.
			if afterGeneration != beforeGeneration+1 {
				t.Fatalf("%s: generation %d after a failed replacement, want %d", mode, afterGeneration, beforeGeneration+1)
			}
			if err := syscall.Kill(oldPID, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("%s: retired worker still alive: %v", mode, err)
			}
			// The latched failure is checked before staging: no artifact, no launch.
			if err := h.Reload(h.ctx, nil); err == nil {
				t.Fatalf("%s: latched failure allowed a restart", mode)
			}
			if _, generation, _, _, _ := advState(h); generation != afterGeneration {
				t.Fatalf("%s: latched-failure reload launched generation %d", mode, generation)
			}
			if stages := advStageFiles(t, cfg); len(stages) != 0 {
				t.Fatalf("%s leaked staged artifacts: %v", mode, stages)
			}
			if err := advSubmit(t, h, "unpersisted"); err != nil {
				t.Fatalf("%s: durable admission rejected: %v", mode, err)
			}
			if pending := h.journal.Pending(); len(pending) != 1 || pending[0].ID != "unpersisted" {
				t.Fatalf("%s: durable input lost: %#v", mode, pending)
			}
			time.Sleep(20 * time.Millisecond)
			if counts := advItemCounts(r); len(counts) != 0 {
				t.Fatalf("%s: input delivered without a worker: %v", mode, counts)
			}
		})
	}
}

// TestConcurrentSubmitAndReloadDeliversEachInputExactlyOnce races continuous
// admission against three completed handoffs and asserts the end-to-end invariant:
// every durably admitted input reaches the worker exactly once across generation
// swaps, none is lost, each is acknowledged, and each reaches the UI exactly once
// (host.go:253-276, host.go:416-441, host.go:585-590).
func TestConcurrentSubmitAndReloadDeliversEachInputExactlyOnce(t *testing.T) {
	h, r, cfg := advHost(t, nil)
	const submitters = 4
	const perSubmitter = 15
	const handoffs = 3
	total := submitters * perSubmitter
	var wg sync.WaitGroup
	for submitter := range submitters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range perSubmitter {
				payload, err := json.Marshal("body")
				if err != nil {
					t.Error(err)
					return
				}
				input := inbox.Input{ID: inbox.ID(fmt.Sprintf("s%d-i%d", submitter, index)), Kind: inbox.InputExternal, Payload: payload}
				if err := h.Submit(t.Context(), input); err != nil {
					t.Errorf("submit %s: %v", input.ID, err)
					return
				}
			}
		}()
	}
	handoffResult := make(chan error, 1)
	go func() {
		completed := 0
		for attempt := 0; attempt < 800 && completed < handoffs; attempt++ {
			// ErrBusy is the documented conservative answer while a delivered
			// input has not reached persistence; retry the same request.
			if err := h.Reload(h.ctx, nil); err == nil {
				completed++
			} else if !errors.Is(err, ErrBusy) {
				handoffResult <- fmt.Errorf("handoff %d: %w", completed+1, err)
				return
			}
			time.Sleep(time.Millisecond)
		}
		if completed != handoffs {
			handoffResult <- fmt.Errorf("only %d/%d handoffs completed", completed, handoffs)
			return
		}
		handoffResult <- nil
	}()
	submissionsDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(submissionsDone)
	}()
	select {
	case <-submissionsDone:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent submissions did not finish")
	}
	if err := advReceiveError(t, handoffResult); err != nil {
		t.Fatal(err)
	}
	received := advLogPath(cfg, "received.log")
	advUntil(t, "every admitted input to be delivered", func() bool { return len(advCounts(t, received)) >= total })
	counts := advCounts(t, received)
	if len(counts) != total {
		t.Fatalf("delivered %d distinct inputs, want %d", len(counts), total)
	}
	for submitter := range submitters {
		for index := range perSubmitter {
			id := fmt.Sprintf("s%d-i%d", submitter, index)
			if counts[id] != 1 {
				t.Errorf("expected input %s delivered %d times, want 1", id, counts[id])
			}
		}
	}
	for id, count := range counts {
		if count != 1 {
			t.Errorf("input %s delivered %d times across generations", id, count)
		}
	}
	advUntil(t, "every admission to be acknowledged", func() bool { return len(h.journal.Pending()) == 0 })
	advUntil(t, "the UI to see every input", func() bool { return len(advItemCounts(r)) >= total })
	for id, count := range advItemCounts(r) {
		if count != 1 {
			t.Errorf("input %s surfaced %d times", id, count)
		}
	}
	if _, _, _, failure, hasCurrent := advState(h); failure != nil || !hasCurrent {
		t.Fatalf("host unhealthy after handoffs: failure=%v live=%v", failure, hasCurrent)
	}
}

// advReceiveError bounds asynchronous protocol rendezvous so a regression fails
// the individual test instead of waiting for the package-wide Go test timeout.
func advReceiveError(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for protocol operation")
		return nil
	}
}
