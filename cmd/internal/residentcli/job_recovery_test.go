package residentcli

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/jobexecutor"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

// Real separately supervised production executor CLI + real worker A/B. The
// owned proxy drops acknowledgment only AFTER the executor launched the job.
func TestM43WorkerRestartReattachesIndependentExecutor(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "j43.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	path := func(name string) string { return filepath.Join(root, name) }
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	wait := func(label string, f func() bool) {
		t.Helper()
		for !f() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", label, ctx.Err())
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	for _, dir := range []string{"work", "state", "home"} {
		if err := os.Mkdir(path(dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig := func(name string, value any) {
		t.Helper()
		b, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path(name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	token := "private-m43-scoped-executor-credential"
	t.Setenv("HOME", path("home"))
	t.Setenv("XDG_CONFIG_HOME", path("home"))
	t.Setenv("XDG_STATE_HOME", path("home"))
	t.Setenv("OPENAI_API_KEY", "fake")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake")
	t.Setenv(durablejob.TokenEnvironment, token)
	command := "printf x >> " + path("effect") + "; printf first; while [ ! -e " + path("release") + " ]; do /bin/sleep 0.01; done; printf second"
	writeConfig("executor.json", struct {
		Version                  int
		Directory, EnvironmentID string
		Jobs                     map[string]jobexecutor.TrustedJob
	}{1, path("executor"), "private-tests", map[string]jobexecutor.TrustedJob{"tests": {Path: "/bin/sh", Args: []string{"-c", command}, Dir: path("work"), Env: []string{"PATH=/usr/bin:/bin"}}}})
	executor := startCLI(t, nil, "job-executor", "--config", path("executor.json"))
	var ready struct {
		Version               int
		EnvironmentID, Socket string
		PID                   int
	}
	if err = json.Unmarshal(executor.frame(t, ctx), &ready); err != nil || ready.Version != 1 || ready.EnvironmentID != "private-tests" || ready.PID != executor.cmd.Process.Pid {
		t.Fatalf("executor readiness: %+v %v", ready, err)
	}
	var hold atomic.Bool
	var dropped atomic.Bool
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", ready.Socket)
	}}
	defer transport.CloseIdleConnections()
	listener, err := net.Listen("unix", path("proxy.sock"))
	if err != nil {
		t.Fatal(err)
	}
	proxy := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hold.Load() {
			http.Error(w, "private transport unavailable", 503)
			return
		}
		clone := r.Clone(r.Context())
		clone.RequestURI = ""
		clone.URL.Scheme = "http"
		clone.URL.Host = "executor"
		response, e := transport.RoundTrip(clone)
		if e != nil {
			http.Error(w, e.Error(), 502)
			return
		}
		defer response.Body.Close()
		if r.URL.Path == "/submit" && !dropped.Swap(true) {
			hold.Store(true)
			conn, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				_ = conn.Close()
			}
			return
		}
		for name, values := range response.Header {
			for _, v := range values {
				w.Header().Add(name, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	})}
	proxyDone := make(chan struct{})
	go func() { defer close(proxyDone); _ = proxy.Serve(listener) }()
	t.Cleanup(func() { _ = proxy.Close(); <-proxyDone })
	writeConfig("runtime.json", durablejob.RuntimeConfig{Version: 1, EnvironmentID: "private-tests", Socket: path("proxy.sock"), Jobs: []string{"tests"}})
	t.Setenv(durablejob.ConfigEnvironment, path("runtime.json"))
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		n := calls.Add(1)
		output := []map[string]any{{"type": "message", "id": "answer", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "job completed"}}}}
		if n == 1 {
			output = []map[string]any{{"type": "function_call", "id": "job-call", "call_id": "job-call", "name": "RunTrustedJob", "arguments": `{"job":"tests"}`}}
		}
		body, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprint("response-", n), "status": "completed", "output": output, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}})
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
	}))
	defer provider.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := workerhost.Config{Command: []string{binary, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "private-job", Directory: path("state"), Workspace: path("work"), Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "private-model", CompactionThreshold: 64000}, MaxAttempts: 2, Shell: "/bin/sh"}}
	first, err := workerhost.Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close(context.Background()) })
	input := inbox.Input{ID: "original-job-input", Kind: inbox.InputExternal, Payload: []byte(`"run private tests"`)}
	if err = first.Submit(ctx, input); err != nil {
		t.Fatal(err)
	}
	wait("postlaunch response dropped and effect", func() bool {
		b, e := os.ReadFile(path("effect"))
		return dropped.Load() && e == nil && string(b) == "x"
	})
	store, err := localfile.New(path("state"))
	if err != nil {
		t.Fatal(err)
	}
	find := func() operation.Operation {
		t.Helper()
		r, e := store.Resume(ctx, "private-job")
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range r.Operations {
			if op.Type == operation.TypeRemoteJob {
				return op
			}
		}
		history, e := sessionstore.ReadLifecycle(ctx, store, "private-job")
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range history.RemoteOperations {
			if op.Type == operation.TypeRemoteJob {
				return op
			}
		}
		return operation.Operation{}
	}
	wait("committed job intent", func() bool { return find().ID != "" })
	original := find().ID
	if err = first.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-first.Done():
		if e == nil {
			t.Fatal("crashed worker falsely successful")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(path("release")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("job gate unexpectedly released")
	}
	select {
	case <-executor.done:
		t.Fatal("independent executor exited with worker")
	default:
	}
	hold.Store(false)
	second, err := workerhost.Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close(context.Background()) })
	if err = os.WriteFile(path("release"), []byte("explicit finish"), 0600); err != nil {
		t.Fatal(err)
	}
	wait("same original operation terminal after reattach", func() bool { op := find(); return op.ID == original && op.Status == operation.StatusCompleted })
	op := find()
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	if err = durablejob.TerminalCleanupProof(op, durablejob.RuntimeConfig{EnvironmentID: "private-tests", Jobs: []string{"tests"}}.Authority("private-job")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state.TerminalResult, "firstsecond") {
		t.Fatalf("output lost: %q", state.TerminalResult)
	}
	if err = second.Submit(ctx, input); err != nil {
		t.Fatal(err)
	}
	wait("final model result", func() bool { return calls.Load() == 2 })
	if err = second.Close(ctx); err != nil {
		t.Fatal(err)
	}
	effect, err := os.ReadFile(path("effect"))
	if err != nil || string(effect) != "x" {
		t.Fatalf("duplicate effect %q %v", effect, err)
	}
	canonical, err := os.ReadFile(path("state/private-job.session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(canonical), token) || strings.Contains(string(canonical), path("proxy.sock")) {
		t.Fatal("credential/transport path leaked to canonical history")
	}
	executor.stop(t)
	if executor.result != nil {
		t.Fatalf("executor actual join: %v %s", executor.result, executor.stderr.String())
	}
}
