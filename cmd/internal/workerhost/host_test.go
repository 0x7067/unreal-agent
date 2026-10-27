package workerhost

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "agent-runner" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		code := agentrunner.RunMain(ctx, os.Args[2:], os.Getenv, os.Environ, os.Stdin, os.Stdout, os.Stderr, agentrunner.Config{Name: "workerhost-fixture", Providers: providers.Default(), ParseRequest: agentrunner.ParseRequest})
		stop()
		os.Exit(code)
	}

	if len(os.Args) > 1 {
		mode := os.Args[len(os.Args)-1]
		if mode == "worker-probe" || mode == "worker" {
			if err := helper(mode); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(20)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}
func helper(mode string) error {
	if os.Getenv("UA_HOST_TEST_REAL") == "1" {
		identity := workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: "real-runtime-fixture"}
		if mode == "worker-probe" {
			return workerruntime.Probe(os.Stdout, identity)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err := workerruntime.Run(ctx, os.Stdin, os.Stdout, os.Stderr, os.Getenv, identity)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	identity := workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: "host-fixture"}
	if mode == "worker-probe" && os.Getenv("UA_HOST_TEST_BAD_PROBE") == "1" {
		identity.Protocol++
	}
	if mode == "worker-probe" {
		if gate := os.Getenv("UA_HOST_TEST_PROBE_GATE"); gate != "" {
			if err := os.WriteFile(gate+".ready", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
				return err
			}
			for {
				if _, err := os.Stat(gate); err == nil {
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
	}
	if err := workerproto.Write(os.Stdout, workerproto.Event{Type: "hello", Identity: &identity}); err != nil {
		return err
	}
	if mode == "worker-probe" {
		return nil
	}
	scanner := workerproto.Scanner(os.Stdin)
	if !scanner.Scan() {
		return io.ErrUnexpectedEOF
	}
	var request workerproto.Request
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
		return err
	}
	if request.Type != "start" || request.Bootstrap == nil {
		return errors.New("missing start")
	}
	b := request.Bootstrap
	lease, err := localfile.AcquireLease(b.Directory, b.SessionID)
	if err != nil {
		return err
	}
	defer lease.Close()
	store, err := localfile.New(b.Directory)
	if err != nil {
		return err
	}
	_, err = store.Resume(context.Background(), b.SessionID)
	if errors.Is(err, os.ErrNotExist) {
		_, err = store.Create(context.Background(), b.SessionID)
	}
	if err != nil {
		return err
	}
	if os.Getenv("UA_HOST_TEST_FAIL_AFTER_WRITE") == "1" {
		if err := store.AppendInput(context.Background(), b.SessionID, inbox.Input{ID: "candidate-startup-write", Kind: inbox.InputExternal, Payload: []byte(`"candidate mutation"`)}); err != nil {
			return err
		}
		return errors.New("injected replacement failure after session write")
	}
	seen := map[inbox.ID]bool{}
	after := sessionstore.BeforeFirst
	for {
		page, err := store.Items(context.Background(), b.SessionID, after, 100)
		if err != nil {
			return err
		}
		for _, item := range page.Items {
			if input, ok := item.Data.(inbox.Input); ok {
				seen[input.ID] = true
			}
			if err = workerproto.Write(os.Stdout, workerproto.Event{Type: "item", Item: &item}); err != nil {
				return err
			}
		}
		if !page.More {
			break
		}
		after = page.NextAfter
	}
	return helperRequests(scanner, store, lease, b, seen)
}
func helperRequests(scanner *bufio.Scanner, store *localfile.Store, lease *localfile.Lease, b *workerproto.Bootstrap, seen map[inbox.ID]bool) error {
	var outputErr error
	store.AddObserver(func(_ session.ID, item sessionstore.Item) {
		outputErr = workerproto.Write(os.Stdout, workerproto.Event{Type: "item", Item: &item})
	})
	if os.Getenv("UA_HOST_TEST_STUBBORN") == "1" {
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		if err := workerproto.Write(os.Stdout, workerproto.Event{Type: "ready"}); err != nil {
			return err
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	if err := workerproto.Write(os.Stdout, workerproto.Event{Type: "ready"}); err != nil {
		return err
	}
	busy := false
	for scanner.Scan() {
		var request workerproto.Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			return err
		}
		switch request.Type {
		case "activity_snapshot":
			if os.Getenv("UA_HOST_TEST_SLOW_SNAPSHOT") == "1" {
				time.Sleep(100 * time.Millisecond)
			}
			if err := workerproto.Write(os.Stdout, workerproto.Event{Type: "activity_snapshot", ID: request.ID}); err != nil {
				return err
			}
		case "input":
			if request.Input == nil {
				return errors.New("missing input")
			}
			input := *request.Input
			if seen[input.ID] {
				continue
			}
			var text string
			_ = json.Unmarshal(input.Payload, &text)
			if text == "crash-before-persist" {
				marker := filepath.Join(b.Directory, "crash-once")
				if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
					_ = os.WriteFile(marker, nil, 0600)
					os.Exit(42)
				}
			}
			if text == "close-output" {
				signal.Ignore(os.Interrupt, syscall.SIGTERM)
				_ = os.Stdout.Close()
				for {
					time.Sleep(time.Hour)
				}
			}
			if text == "busy" {
				busy = true
			}
			if text == "idle" {
				busy = false
			}
			if err := store.AppendInput(context.Background(), b.SessionID, input); err != nil {
				return err
			}
			if outputErr != nil {
				return outputErr
			}
			seen[input.ID] = true
		case "quiesce":
			if busy {
				if err := workerproto.Write(os.Stdout, workerproto.Event{Type: "busy", ID: request.ID, Error: "fixture model active"}); err != nil {
					return err
				}
				continue
			}
			if err := lease.Close(); err != nil {
				return err
			}
			return workerproto.Write(os.Stdout, workerproto.Event{Type: "quiesced", ID: request.ID})
		case "stop":
			return nil
		default:
			return errors.New("unknown helper request")
		}
	}
	return scanner.Err()
}

type recorder struct {
	mu       sync.Mutex
	items    []sessionstore.Item
	statuses []Status
	changed  chan struct{}
}

func (r *recorder) item(item sessionstore.Item) {
	r.mu.Lock()
	r.items = append(r.items, item)
	r.mu.Unlock()
	select {
	case r.changed <- struct{}{}:
	default:
	}
}
func (r *recorder) status(status Status) {
	r.mu.Lock()
	r.statuses = append(r.statuses, status)
	r.mu.Unlock()
	select {
	case r.changed <- struct{}{}:
	default:
	}
}
func testHost(t *testing.T, dir string) (*Host, *recorder, Config) {
	t.Helper()
	return testHostWith(t, dir, nil, nil)
}
func testHostWith(t *testing.T, dir string, configure func(*Config), wrapJournal func(admissionJournal) admissionJournal) (*Host, *recorder, Config) {
	t.Helper()
	if dir == "" {
		var err error
		dir, err = filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := &recorder{changed: make(chan struct{}, 1)}
	cfg := Config{Command: []string{exe, "-test.run=^$", "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "fixture-session", Directory: dir, Workspace: dir}, OnItem: r.item, OnStatus: r.status, Stderr: os.Stderr}
	if configure != nil {
		configure(&cfg)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	h, err := start(ctx, cfg, wrapJournal)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := h.Close(ctx); err != nil {
			t.Errorf("close host: %v", err)
		}
	})
	return h, r, cfg
}
func send(t *testing.T, h *Host, id, text string) {
	t.Helper()
	payload, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Submit(t.Context(), inbox.Input{ID: inbox.ID(id), Kind: inbox.InputExternal, Payload: payload}); err != nil {
		t.Fatal(err)
	}
}
func waitInput(t *testing.T, r *recorder, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		r.mu.Lock()
		found := false
		for _, item := range r.items {
			if input, ok := item.Data.(inbox.Input); ok && input.ID == inbox.ID(id) {
				found = true
			}
		}
		r.mu.Unlock()
		if found {
			return
		}
		select {
		case <-r.changed:
		case <-ctx.Done():
			t.Fatalf("input %s not observed", id)
		}
	}
}
func generation(h *Host) (uint64, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	pid := 0
	if h.current != nil {
		pid = h.current.command.Process.Pid
	}
	return h.generation, pid
}
func TestIdleReloadRetainsSessionAndDeduplicatesReplay(t *testing.T) {
	h, r, _ := testHost(t, "")
	send(t, h, "first", "hello")
	waitInput(t, r, "first")
	before, pid := generation(h)
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	after, nextPID := generation(h)
	if after != before+1 || pid == nextPID || nextPID == 0 {
		t.Fatalf("generation/PID unchanged: %d/%d -> %d/%d", before, pid, after, nextPID)
	}
	send(t, h, "first", "hello")
	send(t, h, "second", "world")
	waitInput(t, r, "second")
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) != 2 {
		t.Fatalf("replay duplicated UI history: %d items", len(r.items))
	}
	if pending := h.journal.Pending(); len(pending) != 0 {
		t.Fatalf("unacknowledged inputs: %+v", pending)
	}
}
func TestBusyAndBadCandidateLeaveWorkerUntouched(t *testing.T) {
	h, r, _ := testHost(t, "")
	send(t, h, "work", "busy")
	waitInput(t, r, "work")
	before, pid := generation(h)
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy reload: %v", err)
	}
	if _, next := generation(h); next != pid {
		t.Fatal("busy reload replaced worker")
	}
	if err := h.Reload(t.Context(), []string{"/nonexistent/worker", "worker"}); err == nil {
		t.Fatal("missing candidate accepted")
	}
	t.Setenv("UA_HOST_TEST_BAD_PROBE", "1")
	if err := h.Reload(t.Context(), nil); err == nil {
		t.Fatal("incompatible protocol accepted")
	}
	t.Setenv("UA_HOST_TEST_BAD_PROBE", "0")
	if current, next := generation(h); current != before || next != pid {
		t.Fatal("preflight failure changed current worker")
	}
	send(t, h, "finish", "idle")
	waitInput(t, r, "finish")
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
}
func TestDurableInputSurvivesWorkerDeathBeforePersistence(t *testing.T) {
	h, _, cfg := testHost(t, "")
	send(t, h, "survivor", "crash-before-persist")
	select {
	case err := <-h.Done():
		if err == nil {
			t.Fatal("crash reported clean exit")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("worker death not reported")
	}
	if pending := h.journal.Pending(); len(pending) != 1 || pending[0].ID != "survivor" {
		t.Fatalf("lost acknowledged admission: %+v", pending)
	}
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrStopped) {
		t.Fatalf("arbitrary crash automatically restarted: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	h2, _, _ := testHost(t, cfg.Bootstrap.Directory)
	send(t, h2, "after", "alive")
	// New Start automatically retries the journal, preserving its original ID.
	deadline := time.After(10 * time.Second)
	for len(h2.journal.Pending()) != 0 {
		select {
		case <-time.After(time.Millisecond):
		case <-deadline:
			t.Fatal("durable retry was not acknowledged")
		}
	}
	store, err := localfile.New(cfg.Bootstrap.Directory)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Items(t.Context(), cfg.Bootstrap.SessionID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range page.Items {
		if input, ok := item.Data.(inbox.Input); ok && input.ID == "survivor" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("survivor persisted %d times", count)
	}
}
func TestWireNormalizationAndConflictingIdentity(t *testing.T) {
	h, r, _ := testHost(t, "")
	input := inbox.Input{ID: "normalized", Kind: inbox.InputExternal, Payload: []byte(`  "hello"  `)}
	if err := h.Submit(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	waitInput(t, r, "normalized")
	input.Payload = []byte(`"hello"`)
	if err := h.Submit(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	input.Payload = []byte(`"different"`)
	if err := h.Submit(t.Context(), input); !errors.Is(err, ingress.ErrConflict) {
		t.Fatalf("conflict = %v", err)
	}
}
func TestConcurrentHostOwnershipFailsClosed(t *testing.T) {
	h, _, cfg := testHost(t, "")
	if duplicate, err := Start(t.Context(), cfg); err == nil {
		_ = duplicate.Close(t.Context())
		t.Fatal("two parent journal owners")
	}
	if _, err := localfile.AcquireLease(cfg.Bootstrap.Directory, cfg.Bootstrap.SessionID); !errors.Is(err, localfile.ErrSessionLocked) {
		t.Fatalf("live worker lease: %v", err)
	}
	if err := h.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	lease, err := localfile.AcquireLease(cfg.Bootstrap.Directory, cfg.Bootstrap.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("gate not reached: %s", path)
		}
	}
}
func TestProbeDoesNotStopAdmissionAndRepeatedReloadCoalesces(t *testing.T) {
	h, r, _ := testHost(t, "")
	gate := filepath.Join(h.config.Bootstrap.Directory, "probe-gate")
	t.Setenv("UA_HOST_TEST_PROBE_GATE", gate)
	result := make(chan error, 1)
	go func() { result <- h.Reload(t.Context(), nil) }()
	waitFile(t, gate+".ready")
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("concurrent reload = %v", err)
	}
	// Preflight happens with the old worker fully serving inputs.
	send(t, h, "during-probe", "hello")
	waitInput(t, r, "during-probe")
	if err := os.WriteFile(gate, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	send(t, h, "after-probe", "world")
	waitInput(t, r, "after-probe")
}
func TestDeliveryFenceRespectsCancellationWithoutStoppingWorker(t *testing.T) {
	h, r, _ := testHost(t, "")
	if err := h.lockDelivery(t.Context()); err != nil {
		t.Fatal(err)
	}
	before, pid := generation(h)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- h.Reload(ctx, nil) }()
	// Probe or delivery acquisition may time out, but never retire the worker.
	if err := <-result; err == nil {
		t.Fatal("canceled reload succeeded")
	}
	h.unlockDelivery()
	if after, next := generation(h); after != before || next != pid {
		t.Fatal("timed out fence changed worker")
	}
	send(t, h, "still-live", "hello")
	waitInput(t, r, "still-live")
}
func TestInputAdmittedWhileDeliveryFrozenSurvivesReplacement(t *testing.T) {
	h, r, _ := testHost(t, "")
	if err := h.lockDelivery(t.Context()); err != nil {
		t.Fatal(err)
	}
	send(t, h, "held", "hello")
	if pending := h.journal.Pending(); len(pending) != 1 {
		t.Fatal("held input was not durably admitted")
	}
	h.unlockDelivery()
	h.wake()
	waitInput(t, r, "held")
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) != 1 {
		t.Fatal("held input duplicated after reload")
	}
}

func TestCloseJoinsAnInFlightCandidateProbe(t *testing.T) {
	h, _, _ := testHost(t, "")
	gate := filepath.Join(h.config.Bootstrap.Directory, "closing-probe")
	t.Setenv("UA_HOST_TEST_PROBE_GATE", gate)
	result := make(chan error, 1)
	go func() { result <- h.Reload(t.Context(), nil) }()
	waitFile(t, gate+".ready")
	data, err := os.ReadFile(gate + ".ready")
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("candidate probe still exists after Close: %v", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled reload succeeded")
		}
	case <-ctx.Done():
		t.Fatal("reload not joined")
	}
}
func TestReplacementFailureAfterWriteNeverDowngrades(t *testing.T) {
	h, _, _ := testHost(t, "")
	_, oldPID := generation(h)
	t.Setenv("UA_HOST_TEST_FAIL_AFTER_WRITE", "1")
	if err := h.Reload(t.Context(), nil); err == nil {
		t.Fatal("injected startup failure succeeded")
	}
	h.mu.Lock()
	p := h.current
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if p != nil {
		select {
		case <-p.done:
		case <-ctx.Done():
			t.Fatal("failed replacement did not exit")
		}
	}
	if err := syscall.Kill(oldPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("old worker resurrected/remains: %v", err)
	}
	generation, _ := generation(h)
	if generation != 2 {
		t.Fatalf("unexpected fallback launch, generation %d", generation)
	}
	store, err := localfile.New(h.config.Bootstrap.Directory)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Items(t.Context(), h.config.Bootstrap.SessionID, sessionstore.BeforeFirst, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range page.Items {
		if input, ok := item.Data.(inbox.Input); ok && input.ID == "candidate-startup-write" {
			found = true
		}
	}
	if !found {
		t.Fatal("candidate did not mutate history before failure")
	}
}

func TestCloseEscalatesAndJoinsStubbornWorker(t *testing.T) {
	t.Setenv("UA_HOST_TEST_STUBBORN", "1")
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h, err := Start(t.Context(), Config{
		Command:       []string{executable, "-test.run=^$", "worker"},
		Bootstrap:     workerproto.Bootstrap{SessionID: "stubborn", Directory: directory, Workspace: directory},
		ShutdownGrace: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	_, pid := generation(h)
	// A separately parented fixture member lets this test reap it explicitly
	// while verifying that fallback kills the whole worker-owned process group.
	member := exec.Command("/bin/sleep", "30")
	member.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: pid}
	if err := member.Start(); err != nil {
		t.Fatal(err)
	}
	memberDone := make(chan error, 1)
	go func() { memberDone <- member.Wait() }()
	t.Cleanup(func() { _ = member.Process.Kill(); <-memberDone })
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("worker not reaped: %v", err)
	}
	select {
	case err := <-memberDone:
		memberDone <- err
		if err == nil || member.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Fatalf("group member not forcibly stopped: %v (%v)", err, member.ProcessState)
		}
	case <-time.After(time.Second):
		t.Fatal("fallback left worker group member alive")
	}
	for _, id := range []session.ID{"stubborn", "host-stubborn"} {
		lease, err := localfile.AcquireLease(directory, id)
		if err != nil {
			t.Fatalf("lease %s retained: %v", id, err)
		}
		_ = lease.Close()
	}
}

func TestWorkerClosedOutputCancelsAndJoins(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h, err := Start(t.Context(), Config{
		Command:       []string{executable, "-test.run=^$", "worker"},
		Bootstrap:     workerproto.Bootstrap{SessionID: "closed-output", Directory: directory, Workspace: directory},
		ShutdownGrace: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	_, pid := generation(h)
	send(t, h, "unpersisted", "close-output")
	select {
	case err := <-h.Done():
		if err == nil {
			t.Fatal("unexpected EOF reported success")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("EOF left worker alive/Wait blocked")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("worker not reaped: %v", err)
	}
	if len(h.journal.Pending()) != 1 {
		t.Fatal("unconfirmed input lost")
	}
	if err := h.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	lease, err := localfile.AcquireLease(directory, "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
}

func TestKillEndsCloseBeforeGraceExpires(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("UA_HOST_TEST_STUBBORN", "1")
	h, err := Start(t.Context(), Config{
		Command:       []string{executable, "-test.run=^$", "worker"},
		Bootstrap:     workerproto.Bootstrap{SessionID: "killed", Directory: directory, Workspace: directory},
		ShutdownGrace: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	_, pid := generation(h)
	closed := make(chan error, 1)
	go func() { closed <- h.Close(context.Background()) }()
	// The worker ignores Interrupt; without Kill, Close would wait out the grace.
	time.Sleep(100 * time.Millisecond)
	if err := h.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not finish after Kill")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("worker not reaped: %v", err)
	}
	lease, err := localfile.AcquireLease(directory, "killed")
	if err != nil {
		t.Fatalf("lease retained: %v", err)
	}
	_ = lease.Close()
}

func TestInstalledProbeDoesNotReloadRunningWorker(t *testing.T) {
	h, _, _ := testHost(t, "")
	generationBefore, pidBefore := generation(h)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	status, err := h.CheckInstalled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Build == "" || status.InstalledBuild == "" || status.ReloadState != "running worker matches installed candidate" {
		t.Fatalf("incomplete build evidence: %+v", status)
	}
	after, pidAfter := generation(h)
	if after != generationBefore || pidAfter != pidBefore {
		t.Fatal("inspection reloaded worker")
	}
}

func TestLiveSnapshotQueryRoundtripAndCanceledReplyDoesNotRetireWorker(t *testing.T) {
	t.Setenv("UA_HOST_TEST_SLOW_SNAPSHOT", "1")
	h, r, _ := testHost(t, "")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	_, gen, err := h.queryActivity(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || gen != 1 {
		t.Fatalf("deadline outcome: generation=%d error=%v", gen, err)
	}
	ctx, cancel = context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	event, gen, err := h.queryActivity(ctx)
	if err != nil || event.Type != "activity_snapshot" || event.ID == "" || gen != 1 {
		t.Fatalf("query: %+v generation=%d error=%v", event, gen, err)
	}
	r.mu.Lock()
	items := len(r.items)
	r.mu.Unlock()
	if items != 0 {
		t.Fatalf("live query persisted %d items", items)
	}
	h.mu.Lock()
	p := h.current
	failure := h.failure
	h.mu.Unlock()
	if p == nil || failure != nil {
		t.Fatalf("late canceled query retired healthy worker: %v", failure)
	}
	p.mu.Lock()
	waiting := len(p.waiting)
	p.mu.Unlock()
	if waiting != 0 {
		t.Fatalf("leaked %d query waiters", waiting)
	}
}
