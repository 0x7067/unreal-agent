//go:build linux

package workerhost

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"golang.org/x/sys/unix"
)

const lifecycleInputID = "lifecycle-input"

// This is a thin frontend-process fixture, not a terminal renderer. It owns the
// same real Host as the TUI, so killing it closes the actual worker pipes without
// running Host.Close. No production frontend or lifecycle behavior is changed.
func TestLifecycleFrontendProcess(t *testing.T) {
	encoded := os.Getenv("UA_LIFECYCLE_FRONTEND")
	if encoded == "" {
		return
	}
	var bootstrap workerproto.Bootstrap
	if err := json.Unmarshal([]byte(encoded), &bootstrap); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UA_HOST_TEST_REAL", "1")
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	t.Setenv("HOME", bootstrap.Directory)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(bootstrap.Directory, "config"))
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	signalCtx, stopSignals := signal.NotifyContext(t.Context(), os.Interrupt)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(signalCtx, 40*time.Second)
	defer cancel()
	h, err := Start(ctx, Config{Command: []string{executable, "-test.run=^$", "worker"}, Bootstrap: bootstrap, Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = h.Close(closeCtx)
	})
	_, pid := generation(h)
	if err := json.MarshalWrite(os.Stdout, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		t.Fatal(err)
	}
	// Do not admit work until the parent has pinned worker identity and installed
	// cleanup. EOF/interrupt is cooperative fixture cleanup, not the crash case.
	control := bufio.NewScanner(os.Stdin)
	if !control.Scan() {
		return
	}
	if control.Text() != "run" {
		t.Fatal("unexpected frontend fixture control")
	}
	send(t, h, lifecycleInputID, "start lifecycle work")
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatal("frontend fixture was not terminated by its parent")
	}
}

// pidfds pin the observed process identity and signal exit even for an orphan
// awaiting reaping. kill(pid, 0) alone would mistake a zombie for live execution.
func requireLifecyclePidfd(t *testing.T) {
	t.Helper()
	fd, err := unix.PidfdOpen(os.Getpid(), 0)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		t.Skipf("Linux pidfd observation unavailable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = unix.Close(fd)
}

func lifecycleProcess(t *testing.T, pid int) int {
	t.Helper()
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0)
		if !lifecycleExited(t, fd, 5*time.Second) {
			t.Error("fixture process did not exit during cleanup")
		}
		_ = unix.Close(fd)
	})
	return fd
}

func lifecycleExited(t *testing.T, fd int, wait time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		remaining := max(0, time.Until(deadline).Milliseconds())
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(remaining))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		return n != 0 && fds[0].Revents&unix.POLLIN != 0
	}
}

func lifecycleShell(t *testing.T, directory string) int {
	t.Helper()
	waitFile(t, filepath.Join(directory, "lifecycle-started"))
	data, err := os.ReadFile(filepath.Join(directory, "lifecycle-shell.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid fixture shell PID %q: %v", data, err)
	}
	return lifecycleProcess(t, pid)
}

func lifecycleServer(t *testing.T, shell bool) (*httptest.Server, *atomic.Int64, <-chan struct{}, <-chan struct{}) {
	t.Helper()
	var calls atomic.Int64
	started, canceled := make(chan struct{}, 8), make(chan struct{}, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		id := calls.Add(1)
		if shell && id == 1 {
			completed(w, id, toolCall(id, "Bash", map[string]any{"command": "printf x >> lifecycle-effect; echo $$ > lifecycle-shell.pid; touch lifecycle-started; read token < lifecycle-release"}))
			return
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
		select {
		case canceled <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(server.Close)
	return server, &calls, started, canceled
}

func lifecycleLease(t *testing.T, directory string, id session.ID, locked bool) {
	t.Helper()
	lease, err := localfile.AcquireLease(directory, id)
	if lease != nil {
		defer lease.Close()
	}
	if locked {
		if !errors.Is(err, localfile.ErrSessionLocked) {
			t.Fatalf("lease %s = %v, want locked", id, err)
		}
	} else if err != nil {
		t.Fatalf("lease %s not released: %v", id, err)
	}
}

func lifecycleResume(t *testing.T, directory string, id session.ID) sessionstore.ResumeState {
	t.Helper()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Resume(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func lifecycleModelHistory(t *testing.T, directory string, id session.ID, wantTurns int) {
	t.Helper()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	inputs, turns, responses := 0, 0, 0
	after := sessionstore.BeforeFirst
	for {
		page, err := store.Items(t.Context(), id, after, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if item.Kind == sessionstore.ItemInput {
				input := item.Data.(inbox.Input)
				if string(input.ID) == lifecycleInputID {
					inputs++
				}
			}
			if item.Kind == sessionstore.ItemTurn {
				turns++
			}
			if item.Kind == sessionstore.ItemModelResponse {
				responses++
			}
		}
		if !page.More {
			break
		}
		after = page.NextAfter
	}
	if inputs != 1 || turns != wantTurns || responses != 0 {
		t.Fatalf("interrupted model history: inputs/turns/responses = %d/%d/%d, want 1/%d/0", inputs, turns, responses, wantTurns)
	}
}

func TestRealFrontendProcessLossExposesInterruptedShell(t *testing.T) {
	requireLifecyclePidfd(t)
	for _, mode := range []string{"model", "shell"} {
		t.Run(mode, func(t *testing.T) {
			server, calls, started, canceled := lifecycleServer(t, mode == "shell")
			directory, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(filepath.Join(directory, "lifecycle-release"), 0600); err != nil {
				t.Fatal(err)
			}
			bootstrap := workerproto.Bootstrap{SessionID: "frontend-loss", Directory: directory, Workspace: directory, Provider: "openai", BaseURL: server.URL, MaxAttempts: 1, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, Shell: "/bin/sh"}
			encoded, err := json.Marshal(bootstrap)
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(executable, "-test.run=^TestLifecycleFrontendProcess$", "-test.timeout=45s")
			command.Env = append(os.Environ(), "UA_LIFECYCLE_FRONTEND="+string(encoded))
			command.Stderr = os.Stderr
			input, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			var cleanupOnce sync.Once
			cleanupFrontend := func() {
				cleanupOnce.Do(func() {
					_ = input.Close()
					_ = command.Process.Signal(os.Interrupt)
					select {
					case <-done:
						return
					case <-time.After(15 * time.Second):
						t.Error("frontend fixture did not shut down cooperatively")
					}
					_ = command.Process.Kill()
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						t.Error("frontend fixture process did not join")
					}
				})
			}
			t.Cleanup(cleanupFrontend)
			var workerPID int
			pipe, ok := output.(*os.File)
			if !ok {
				t.Fatal("expected an OS stdout pipe")
			}
			if err := pipe.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatal(err)
			}
			line, err := bufio.NewReader(output).ReadBytes('\n')
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(line, &workerPID); err != nil {
				t.Fatal(err)
			}
			worker := lifecycleProcess(t, workerPID)
			// On an assertion failure before shell discovery, Close runs with live
			// pipes before pidfd fallback kills the worker. No unknown shell is
			// orphaned by fixture cleanup. The initial cleanup also covers a bad
			// PID handshake (when no work has been admitted).
			t.Cleanup(cleanupFrontend)
			if _, err := fmt.Fprintln(input, "run"); err != nil {
				t.Fatal(err)
			}
			shell := -1
			if mode == "shell" {
				shell = lifecycleShell(t, directory)
			} else {
				awaitSignal(t, started, "frontend-owned model did not start")
			}
			lifecycleLease(t, directory, bootstrap.SessionID, true)
			lifecycleLease(t, directory, "host-"+bootstrap.SessionID, true)
			if err := command.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				done <- err // retain the joined result for cleanup
				if err == nil {
					t.Fatal("killed frontend exited successfully")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("frontend did not exit")
			}
			if !lifecycleExited(t, worker, 12*time.Second) {
				t.Fatal("frontend loss left the worker running")
			}
			if shell >= 0 && lifecycleExited(t, shell, 0) {
				t.Fatal("fixture expected separately owned shell to survive lost frontend pipes")
			}
			if mode == "model" {
				awaitSignal(t, canceled, "frontend loss did not cancel provider transport")
			}
			lifecycleLease(t, directory, bootstrap.SessionID, false)
			lifecycleLease(t, directory, "host-"+bootstrap.SessionID, false)
			state := lifecycleResume(t, directory, bootstrap.SessionID)
			if mode == "shell" {
				unresolved := false
				for _, op := range state.Operations {
					unresolved = unresolved || op.Type == operation.TypeShell && !lifecycleTerminal(op.Status)
				}
				if !unresolved {
					t.Fatal("frontend loss hid unresolved surviving shell")
				}
				t.Setenv("UA_HOST_TEST_REAL", "1")
				t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
				t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
				t.Setenv("HOME", directory)
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(directory, "config"))
				before := calls.Load()
				restarted, err := Start(t.Context(), Config{Command: []string{executable, "-test.run=^$", "worker"}, Bootstrap: bootstrap, Stderr: os.Stderr})
				if err == nil {
					_ = restarted.Close(t.Context())
					t.Fatal("frontend-loss shell was resumed")
				}
				if !strings.Contains(err.Error(), "unresolved status") || calls.Load() != before {
					t.Fatalf("resume refusal = %v; calls %d -> %d", err, before, calls.Load())
				}
				effect, err := os.ReadFile(filepath.Join(directory, "lifecycle-effect"))
				if err != nil || string(effect) != "x" || lifecycleExited(t, shell, 0) {
					t.Fatalf("surviving shell/effect changed: %q %v", effect, err)
				}
			} else {
				if len(state.Operations) != 0 {
					t.Fatalf("model-only interruption invented operations: %+v", state.Operations)
				}
				lifecycleModelHistory(t, directory, bootstrap.SessionID, 1)
			}
			t.Logf("%s: frontend joined, worker exited, leases released; shell interruption is not clean shutdown", mode)
		})
	}
}

func lifecycleTerminal(status operation.Status) bool {
	return status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled || status == operation.StatusUnsupported
}

func TestRealWorkerUnexpectedDeathRetainsHostOwnershipAndRecoveryBoundary(t *testing.T) {
	requireLifecyclePidfd(t)
	for _, mode := range []string{"model", "shell"} {
		t.Run(mode, func(t *testing.T) {
			server, calls, started, canceled := lifecycleServer(t, mode == "shell")
			h, records, directory := realHost(t, server, false)
			if err := unix.Mkfifo(filepath.Join(directory, "lifecycle-release"), 0600); err != nil {
				t.Fatal(err)
			}
			send(t, h, lifecycleInputID, "start work")
			before, pid := generation(h)
			worker := lifecycleProcess(t, pid)
			// If shell discovery fails, cooperatively stop its owner before the
			// worker pidfd fallback can turn that failure into an orphaned shell.
			t.Cleanup(func() {
				closeCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
				defer stop()
				if err := h.Close(closeCtx); err != nil {
					t.Errorf("close crash fixture host: %v", err)
				}
			})
			shell := -1
			if mode == "shell" {
				shell = lifecycleShell(t, directory)
			} else {
				awaitSignal(t, started, "model did not start")
			}
			waitInput(t, records, lifecycleInputID)
			if err := unix.PidfdSendSignal(worker, unix.SIGKILL, nil, 0); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-h.Done():
				if err == nil {
					t.Fatal("unexpected worker death reported success")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("worker failure not reported")
			}
			if !lifecycleExited(t, worker, time.Second) {
				t.Fatal("host Done preceded actual worker exit")
			}
			if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrStopped) {
				t.Fatalf("crashed worker reload = %v, want ErrStopped", err)
			}
			h.mu.Lock()
			after := h.generation
			h.mu.Unlock()
			if after != before {
				t.Fatal("unexpected death launched replacement")
			}
			lifecycleLease(t, directory, h.config.Bootstrap.SessionID, false)
			lifecycleLease(t, directory, "host-"+h.config.Bootstrap.SessionID, true)
			if mode == "model" {
				awaitSignal(t, canceled, "dead worker retained provider transport")
			} else if lifecycleExited(t, shell, 0) {
				t.Fatal("fixture expected separately owned shell to outlive killed worker")
			}
			state := lifecycleResume(t, directory, h.config.Bootstrap.SessionID)
			if mode == "model" {
				lifecycleModelHistory(t, directory, h.config.Bootstrap.SessionID, 1)
			}
			if mode == "shell" && len(state.Operations) == 0 {
				t.Fatal("crash lost unresolved shell intent")
			}
			config := h.config
			config.OnItem, config.OnStatus = nil, nil
			if err := h.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
			lifecycleLease(t, directory, "host-"+config.Bootstrap.SessionID, false)
			beforeResume := calls.Load()
			restarted, err := Start(t.Context(), config)
			if mode == "shell" {
				if err == nil {
					_ = restarted.Close(t.Context())
					t.Fatal("unresolved shell was resumed")
				}
				if !strings.Contains(err.Error(), "unresolved status") || calls.Load() != beforeResume {
					t.Fatalf("resume refusal = %v; calls %d -> %d", err, beforeResume, calls.Load())
				}
				effect, readErr := os.ReadFile(filepath.Join(directory, "lifecycle-effect"))
				if readErr != nil || string(effect) != "x" || lifecycleExited(t, shell, 0) {
					t.Fatalf("shell effect/liveness changed across refusal: %q %v", effect, readErr)
				}
				if err := unix.PidfdSendSignal(shell, unix.SIGKILL, nil, 0); err != nil {
					t.Fatal(err)
				}
				if !lifecycleExited(t, shell, 5*time.Second) {
					t.Fatal("fixture-owned surviving shell did not exit")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					closeCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
					defer stop()
					if err := restarted.Close(closeCtx); err != nil {
						t.Errorf("close restarted model worker: %v", err)
					}
				})
				awaitSignal(t, started, "explicit model-only restart did not retry interrupted turn")
				if got := calls.Load(); got != 2 {
					t.Fatalf("model requests = %d, want original and explicit-restart retry", got)
				}
				lifecycleModelHistory(t, directory, config.Bootstrap.SessionID, 2)
				if err := restarted.Close(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("%s: crash reported, no auto replacement, host lease retained until Close; restart policy characterized", mode)
		})
	}
}

func TestRealWorkerCloseDeadlineRetainsHostLeaseAfterWorkerExit(t *testing.T) {
	requireLifecyclePidfd(t)
	server, _, started, _ := lifecycleServer(t, false)
	entered, release := make(chan struct{}), make(chan struct{})
	h, _, directory := realHostWith(t, server, false, callbackGate(lifecycleInputID, entered, release))
	open := releaseGate(t, release)
	send(t, h, lifecycleInputID, "gated callback")
	awaitSignal(t, entered, "input callback did not enter")
	awaitSignal(t, started, "model did not start behind callback")
	_, pid := generation(h)
	worker := lifecycleProcess(t, pid)
	// The callback is causally blocked, so an already-expired caller deadline
	// cannot accidentally pass by winning a race against a short timer.
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if err := h.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close with expired deadline = %v", err)
	}
	if !lifecycleExited(t, worker, 10*time.Second) {
		t.Fatal("canceled worker did not exit while its callback was gated")
	}
	lifecycleLease(t, directory, h.config.Bootstrap.SessionID, false)
	lifecycleLease(t, directory, "host-"+h.config.Bootstrap.SessionID, true)
	journal, err := ingress.Open(filepath.Join(directory, "ingress"), h.config.Bootstrap.SessionID)
	if journal != nil {
		_ = journal.Close()
	}
	if !errors.Is(err, ingress.ErrLocked) {
		t.Fatalf("deadline released ingress ownership: %v", err)
	}
	open()
	if err := h.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	lifecycleLease(t, directory, "host-"+h.config.Bootstrap.SessionID, false)
	journal, err = ingress.Open(filepath.Join(directory, "ingress"), h.config.Bootstrap.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("deadline did not release host/ingress ownership; worker exit alone did not complete Close")
}
