package workerhost

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
)

func completed(w http.ResponseWriter, id int64, output []map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	body := map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprintf("response-%d", id), "status": "completed", "output": output, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}}}
	data, _ := json.Marshal(body)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}
func assistant(id int64, text string) []map[string]any {
	return []map[string]any{{"id": fmt.Sprintf("message-%d", id), "type": "message", "role": "assistant", "status": "completed", "content": []map[string]any{{"type": "output_text", "text": text}}}}
}
func toolCall(id int64, name string, arguments any) []map[string]any {
	encoded, _ := json.Marshal(arguments)
	return []map[string]any{{"id": fmt.Sprintf("function-%d", id), "type": "function_call", "call_id": fmt.Sprintf("call-%d", id), "name": name, "arguments": string(encoded), "status": "completed"}}
}
func realHost(t *testing.T, server *httptest.Server, subagents bool) (*Host, *recorder, string) {
	t.Helper()
	return realHostWith(t, server, subagents, nil)
}
func realHostWith(t *testing.T, server *httptest.Server, subagents bool, configure func(*Config)) (*Host, *recorder, string) {
	t.Helper()
	t.Setenv("UA_HOST_TEST_REAL", "1")
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(directory, "config"))
	t.Setenv("HOME", directory)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := &recorder{changed: make(chan struct{}, 1)}
	config := Config{Command: []string{executable, "-test.run=^$", "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "real-session", Directory: directory, Workspace: directory, Provider: "openai", BaseURL: server.URL, MaxAttempts: 1, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, Shell: "/bin/sh", Subagents: subagents, SystemPrompt: "Deterministic test"}, OnItem: r.item, OnStatus: r.status, Stderr: os.Stderr}
	if configure != nil {
		configure(&config)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	t.Cleanup(cancel)
	h, err := Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := h.Close(ctx); err != nil {
			t.Errorf("close real host: %v", err)
		}
	})
	return h, r, directory
}
func waitResponses(t *testing.T, r *recorder, count int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	for {
		r.mu.Lock()
		responses := 0
		for _, item := range r.items {
			if item.Kind == sessionstore.ItemModelResponse {
				responses++
			}
		}
		r.mu.Unlock()
		if responses >= count {
			return
		}
		select {
		case <-r.changed:
		case <-ctx.Done():
			t.Fatalf("got fewer than %d model responses", count)
		}
	}
}
func TestRealWorkerIdleReloadAndBusyModel(t *testing.T) {
	var calls atomic.Int64
	started, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		if id == 1 {
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		completed(w, id, assistant(id, "done"))
	}))
	t.Cleanup(server.Close)
	h, records, _ := realHost(t, server, false)
	if calls.Load() != 0 {
		t.Fatal("configuration-only startup called a model")
	}
	send(t, h, "first-real", "first")
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("model not started")
	}
	waitInput(t, records, "first-real")
	before, pid := generation(h)
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy model reload = %v", err)
	}
	if _, after := generation(h); after != pid {
		t.Fatal("busy model killed")
	}
	close(release)
	waitResponses(t, records, 1)
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	after, newPID := generation(h)
	if after != before+1 || newPID == pid {
		t.Fatal("real worker not replaced")
	}
	if calls.Load() != 1 {
		t.Fatal("reload itself called a model")
	}
	send(t, h, "second-real", "second")
	waitResponses(t, records, 2)
	if calls.Load() != 2 {
		t.Fatalf("unexpected model calls: %d", calls.Load())
	}
	records.mu.Lock()
	defer records.mu.Unlock()
	first := 0
	for _, item := range records.items {
		if item.Kind == sessionstore.ItemInput && strings.Contains(fmt.Sprint(item.Data), "first-real") {
			first++
		}
	}
	if first != 1 {
		t.Fatalf("replayed first input %d times", first)
	}
}
func TestRealWorkerBusyShellDoesNotRepeatEffects(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		if id == 1 {
			completed(w, id, toolCall(id, "Bash", map[string]any{"command": "printf x >> effect; touch shell-started; while [ ! -f shell-release ]; do sleep .02; done; printf done"}))
			return
		}
		completed(w, id, assistant(id, "shell finished"))
	}))
	t.Cleanup(server.Close)
	h, records, workspace := realHost(t, server, false)
	send(t, h, "shell-input", "run shell")
	waitFile(t, filepath.Join(workspace, "shell-started"))
	_, pid := generation(h)
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy shell reload = %v", err)
	}
	if _, next := generation(h); next != pid {
		t.Fatal("live shell replaced")
	}
	if err := os.WriteFile(filepath.Join(workspace, "shell-release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitResponses(t, records, 2)
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	effect, err := os.ReadFile(filepath.Join(workspace, "effect"))
	if err != nil || string(effect) != "x" {
		t.Fatalf("side effect repeated: %q %v", effect, err)
	}
}
func TestRealWorkerCompletedUnacceptedGraphBlocksReload(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		if id == 1 {
			arguments := map[string]any{"action": "start", "name": "reload-graph", "tasks": []map[string]any{{"id": "candidate", "command": "printf candidate > candidate.txt", "reads": []string{}, "writes": []string{"candidate.txt"}, "acceptance": "inspect candidate"}}}
			completed(w, id, toolCall(id, "TaskGraph", arguments))
			return
		}
		completed(w, id, assistant(id, "candidate pending acceptance"))
	}))
	t.Cleanup(server.Close)
	h, records, workspace := realHost(t, server, false)
	send(t, h, "graph-input", "run graph")
	waitFile(t, filepath.Join(workspace, "candidate.txt"))
	waitResponses(t, records, 2)

	store, err := localfile.New(h.config.Bootstrap.Directory)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		completedCandidate := false
		state, err := store.Resume(t.Context(), h.config.Bootstrap.SessionID)
		if err == nil {
			for _, op := range state.Operations {
				remote, err := operation.DecodeRemoteJobState(op)
				if err != nil {
					continue
				}
				var checkpoint struct {
					Graph taskgraph.Graph `json:"graph"`
				}
				if json.Unmarshal(remote.Handle, &checkpoint) != nil {
					continue
				}
				if node := checkpoint.Graph.Find("candidate"); node != nil && node.Status == taskgraph.Completed {
					completedCandidate = true
				}
			}
		}
		if completedCandidate {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("graph never persisted completed-unaccepted candidate")
		}
		time.Sleep(time.Millisecond)
	}

	_, pid := generation(h)
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("unaccepted graph reload = %v", err)
	}
	if _, next := generation(h); next != pid {
		t.Fatal("unaccepted graph was replaced")
	}
}
func TestRealWorkerBusySubagentAndIdleJoin(t *testing.T) {
	var calls atomic.Int64
	childStarted, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body any
		_ = json.UnmarshalRead(r.Body, &body)
		data, _ := json.Marshal(body)
		id := calls.Add(1)
		if id == 1 {
			completed(w, id, toolCall(id, "Agent", map[string]any{"name": "helper", "prompt": "unique-child-prompt"}))
			return
		}
		if strings.Contains(string(data), "unique-child-prompt") && !strings.Contains(string(data), "function_call") {
			select {
			case childStarted <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		completed(w, id, assistant(id, "agent done"))
	}))
	t.Cleanup(server.Close)
	h, records, _ := realHost(t, server, true)
	send(t, h, "agent-input", "start child")
	select {
	case <-childStarted:
	case <-time.After(15 * time.Second):
		t.Fatal("child model never started")
	}
	_, pid := generation(h)
	if err := h.Reload(t.Context(), nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy subagent reload = %v", err)
	}
	if _, next := generation(h); next != pid {
		t.Fatal("busy subagent replaced")
	}
	close(release)
	waitResponses(t, records, 2)
	if err := h.Reload(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
}

// Unlike os.Pipe in in-process tests, exec inherits a blocking stdin fd. A
// worker must still exit on SIGINT with the parent's write end held open.
func TestRealWorkerSignalJoinsInheritedStdin(t *testing.T) {
	t.Setenv("UA_HOST_TEST_REAL", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^$", "worker")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() { _ = command.Process.Kill(); <-done })
	scanner := workerproto.Scanner(output)
	if !scanner.Scan() {
		t.Fatal("missing hello", scanner.Err())
	}
	// Let the child enter Scan with an incomplete frame and an open writer.
	if _, err := input.Write([]byte("{")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		done <- err // leave result for cleanup
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not exit on signal while inherited stdin remained open")
	}
}

func TestRealWorkerCloseJoinsActiveOwners(t *testing.T) {
	for _, mode := range []string{"idle", "model", "shell", "graph", "subagent"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int64
			started := make(chan struct{}, 1)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body any
				_ = json.UnmarshalRead(r.Body, &body)
				id := calls.Add(1)
				if id == 1 && (mode == "shell" || mode == "graph" || mode == "subagent") {
					command := "printf x >> shutdown-effect; echo $$ > shell.pid; touch shell-started; sleep 30"
					switch mode {
					case "shell":
						completed(w, id, toolCall(id, "Bash", map[string]any{"command": command}))
					case "graph":
						completed(w, id, toolCall(id, "TaskGraph", map[string]any{"action": "start", "name": "shutdown", "tasks": []map[string]any{{"id": "shell", "command": command, "reads": []string{}, "writes": []string{"shell.pid", "shell-started", "shutdown-effect"}, "acceptance": "shutdown joins shell"}}}))
					case "subagent":
						completed(w, id, toolCall(id, "Agent", map[string]any{"name": "helper", "prompt": "shutdown-child"}))
					}
					return
				}
				select {
				case started <- struct{}{}:
				default:
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			t.Cleanup(server.Close)
			t.Cleanup(func() { close(release) })
			h, _, directory := realHost(t, server, mode == "subagent")
			_, pid := generation(h)
			shellPID := 0
			if mode != "idle" {
				send(t, h, "shutdown-input", "start work")
				if mode == "shell" || mode == "graph" {
					waitFile(t, filepath.Join(directory, "shell-started"))
					data, err := os.ReadFile(filepath.Join(directory, "shell.pid"))
					if err != nil {
						t.Fatal(err)
					}
					shellPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
					if err != nil {
						t.Fatal(err)
					}
				} else {
					select {
					case <-started:
					case <-time.After(10 * time.Second):
						t.Fatal("model did not start")
					}
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			if err := h.Close(ctx); err != nil {
				t.Fatal(err)
			}
			store, err := localfile.New(directory)
			if err != nil {
				t.Fatal(err)
			}
			resumed, err := store.Resume(t.Context(), "real-session")
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range resumed.Operations {
				if op.Status != operation.StatusCompleted && op.Status != operation.StatusCanceled && op.Status != operation.StatusFailed && op.Status != operation.StatusUnsupported {
					t.Errorf("clean Close left unresolved operation: id=%s type=%s status=%s", op.ID, op.Type, op.Status)
				}
			}
			// Exercise the actual startup refusal fence, not just a store snapshot.
			// Replay must not resubmit canceled work. A canceled tool result may
			// still need a model acknowledgment. The first host's recorder isn't
			// reused for this new replay sequence.
			restartConfig := h.config
			restartConfig.OnItem = nil
			restartConfig.OnStatus = nil
			restarted, err := Start(t.Context(), restartConfig)
			if err != nil {
				t.Fatalf("restart after clean Close: %v", err)
			}
			if err := restarted.Close(ctx); err != nil {
				t.Fatalf("close restarted worker: %v", err)
			}
			if mode == "shell" || mode == "graph" {
				effect, err := os.ReadFile(filepath.Join(directory, "shutdown-effect"))
				if err != nil || string(effect) != "x" {
					t.Fatalf("restart repeated canceled owner effects: %q %v", effect, err)
				}
			}
			for _, child := range []int{pid, shellPID} {
				if child != 0 {
					if err := syscall.Kill(child, 0); !errors.Is(err, syscall.ESRCH) {
						t.Fatalf("process %d remains after Close: %v", child, err)
					}
				}
			}
			// Includes every embedded runner's lease, not just frontend and worker.
			locks, err := filepath.Glob(filepath.Join(directory, "*.session.lock"))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "subagent" && len(locks) < 3 {
				t.Fatalf("no child lease: %v", locks)
			}
			for _, path := range locks {
				id := session.ID(strings.TrimSuffix(filepath.Base(path), ".session.lock"))
				lease, err := localfile.AcquireLease(directory, id)
				if err != nil {
					t.Fatalf("owner %s not joined: %v", id, err)
				}
				_ = lease.Close()
			}
		})
	}
}

func TestRealWorkerForcedCloseKeepsUnresolvedResumeFailClosed(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		completed(w, id, toolCall(id, "Bash", map[string]any{"command": "echo $$ > force-shell.pid; touch force-started; sleep 30"}))
	}))
	t.Cleanup(server.Close)
	h, _, directory := realHostWith(t, server, false, func(config *Config) {
		config.ShutdownGrace = 50 * time.Millisecond
	})
	send(t, h, "force-input", "start shell")
	waitFile(t, filepath.Join(directory, "force-started"))
	data, err := os.ReadFile(filepath.Join(directory, "force-shell.pid"))
	if err != nil {
		t.Fatal(err)
	}
	shellPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	// Bash owns a separate process group: force-killing a stopped worker cannot
	// prove or persist that group's termination. Clean up only this fixture's
	// group, without pretending that the worker itself acknowledged cancellation.
	t.Cleanup(func() { _ = syscall.Kill(-shellPID, syscall.SIGKILL) })
	_, pid := generation(h)
	// A stopped worker cannot consume StopHard or persist its owner's terminal
	// updates. Only the host's force boundary can finish Close in this test.
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := store.Resume(t.Context(), "real-session")
	if err != nil {
		t.Fatal(err)
	}
	unresolved := false
	for _, op := range resumed.Operations {
		if op.Status != operation.StatusCompleted && op.Status != operation.StatusCanceled && op.Status != operation.StatusFailed && op.Status != operation.StatusUnsupported {
			unresolved = true
		}
	}
	if !unresolved {
		t.Fatal("force boundary fabricated resolved operation states")
	}
	restartConfig := h.config
	restartConfig.OnItem, restartConfig.OnStatus = nil, nil
	restartCtx, restartCancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer restartCancel()
	restarted, err := Start(restartCtx, restartConfig)
	if err == nil {
		_ = restarted.Close(restartCtx)
		t.Fatal("force-killed unresolved session resumed")
	}
	if !strings.Contains(err.Error(), "unresolved status") {
		t.Fatalf("restart failed for wrong reason: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("unsafe resume replayed model/tool work: %d calls", calls.Load())
	}
}
