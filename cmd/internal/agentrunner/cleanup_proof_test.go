package agentrunner

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCleanupProofModelCloseJoins(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c := &joinedModelClient{Client: &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		close(started)
		<-release
		return llm.Response{}, nil
	}}}
	done := make(chan struct{})
	go func() { _, _ = c.Respond(t.Context(), llm.Request{}, llm.RequestOptions{}) }()
	<-started
	go func() { _ = c.Close(); close(done) }()
	select {
	case <-done:
		t.Fatal("Close did not join Respond")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	<-done
	if _, err := c.Respond(t.Context(), llm.Request{}, llm.RequestOptions{}); err == nil {
		t.Fatal("admission after seal")
	}
}

func TestCleanupProofRunnerCanonicalReceipt(t *testing.T) {
	client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		return llm.Response{Stop: llm.StopComplete}, nil
	}}
	var out bytes.Buffer
	err := Run(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", t.TempDir(), "-p", "hello", "-cleanup-proof-scope", "fresh"}, func(key string) string {
		if key == providers.APIKeyEnvironment {
			return "secret"
		}
		return ""
	}, func() []string { return nil }, strings.NewReader(""), &out, io.Discard, testConfig(client))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var item sessionstore.Item
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatal(err)
		}
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
			control, err := input.DecodeControlMessage()
			if err == nil && control.Mode == inbox.ExecutionDrained {
				receipt := control.Parameters.(inbox.ExecutionDrainReceipt)
				found = receipt.Scope == "fresh" && receipt.Version == 1
			}
		}
	}
	if !found {
		t.Fatal("missing canonical receipt")
	}
}

func TestCleanupProofFlagValidation(t *testing.T) {
	for _, scope := range []string{"", " ", strings.Repeat("x", 257)} {
		err := Run(t.Context(), []string{"-cleanup-proof-scope", scope}, func(string) string { return "" }, nil, strings.NewReader(""), io.Discard, io.Discard, testConfig(nil))
		if err == nil || !strings.Contains(err.Error(), "cleanup proof scope") {
			t.Fatalf("scope %q: %v", scope, err)
		}
	}
}

func TestCleanupProofLifecycleFenceBeforeProvider(t *testing.T) {
	for _, phase := range []string{"", "work_drained"} {
		t.Run(phase, func(t *testing.T) {
			directory := t.TempDir()
			store, err := localfile.New(directory)
			if err != nil {
				t.Fatal(err)
			}
			id := session.ID("pending-lifecycle")
			if _, err := store.Create(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ShutdownSession, Parameters: inbox.ShutdownSessionRequest{Version: 1, Scope: "old"}})
			if err := store.AppendInput(t.Context(), id, inbox.Input{ID: "shutdown", Kind: inbox.InputControl, Payload: payload}); err != nil {
				t.Fatal(err)
			}
			if phase != "" {
				payload, _ = json.Marshal(inbox.ControlMessage{Mode: inbox.SessionShutdownFinished, Parameters: inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "old", Phase: phase}})
				if err := store.AppendInput(t.Context(), id, inbox.Input{ID: "outcome", Kind: inbox.InputControl, Payload: payload}); err != nil {
					t.Fatal(err)
				}
			}
			config := testConfig(nil)
			config.Providers[0].NewClient = func(string, string, int, func(string) string) (providers.Client, error) {
				t.Error("provider initialized before lifecycle fence")
				return nil, errors.New("unexpected provider")
			}
			err = Run(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", directory}, func(string) string { return "" }, nil, strings.NewReader(`{"session_id":"pending-lifecycle","prompt":"hello"}`), io.Discard, io.Discard, config)
			if !errors.Is(err, sessionstore.ErrLifecycleRecoveryRequired) {
				t.Fatalf("Run: %v", err)
			}
		})
	}
}

func TestCleanupProofRunRetainedHostAuthorityBeforeProvider(t *testing.T) {
	directory := t.TempDir()
	host, err := localfile.AcquireLease(directory, "host-retained-host")
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	worker, err := localfile.AcquireLease(directory, "retained-host")
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Close(); err != nil {
		t.Fatal(err)
	}
	config := testConfig(nil)
	config.Providers[0].NewClient = func(string, string, int, func(string) string) (providers.Client, error) {
		t.Error("provider initialized despite retained host authority")
		return nil, errors.New("unexpected provider")
	}
	err = Run(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", directory}, func(string) string { return "" }, nil, strings.NewReader(`{"session_id":"retained-host","prompt":"hello"}`), io.Discard, io.Discard, config)
	if !errors.Is(err, localfile.ErrSessionLocked) {
		t.Fatalf("Run with retained host authority: %v", err)
	}
	// A failed authority acquisition must not reacquire the released writer lease.
	lease, err := localfile.AcquireLease(directory, "retained-host")
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupProofRunnerKnownGraph(t *testing.T) {
	workspace := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var responseMu sync.Mutex
	started, accepted, finished := false, false, false
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
		// An interrupted request can overlap its replacement; serialize the
		// fixture's state transitions without changing production concurrency.
		responseMu.Lock()
		defer responseMu.Unlock()
		if !containsTool(request.Tools, "TaskGraph") {
			return llm.Response{}, context.Canceled
		}
		results := toolResults(request.Input)
		if strings.Contains(results["graph"], "all current task generations accepted") {
			finished = true
			return messageResponse("done"), nil
		}
		if !started {
			started = true
			return toolCallResponse("graph", "TaskGraph", `{"action":"start","name":"proof","tasks":[{"id":"write","command":"printf proof > result.txt; printf candidate","reads":[],"writes":["result.txt"],"acceptance":"result.txt contains proof"}]}`), nil
		}
		if !accepted && strings.Contains(lastUserText(request.Input), "task write generation 1: completed") {
			contents, err := os.ReadFile(filepath.Join(workspace, "result.txt"))
			if err != nil || string(contents) != "proof" {
				t.Errorf("candidate file=%q error=%v", contents, err)
			}
			accepted = true
			return toolCallResponse("accept", "TaskGraph", `{"action":"accept","name":"proof","task_id":"write","generation":1,"evidence":"read result.txt: proof"}`), nil
		}
		return messageResponse("waiting for candidate or acceptance"), nil
	}}
	var out, stderr strings.Builder
	code := RunMain(ctx, []string{"-workspace", workspace, "-session-directory", t.TempDir(), "-cleanup-proof-scope", "known-graph"}, func(name string) string {
		if name == "XDG_CONFIG_HOME" {
			return workspace
		}
		if name == "OPENAI_API_KEY" {
			return "secret"
		}
		return ""
	}, os.Environ, strings.NewReader(`{"prompt":"execute graph","model":"gpt-test"}`), &out, &stderr, testConfig(client))
	responseMu.Lock()
	defer responseMu.Unlock()
	if code != 0 || !accepted || !finished {
		t.Fatalf("exit=%d accepted=%v finished=%v stderr=%s output=%s", code, accepted, finished, stderr.String(), out.String())
	}
	found := false
	for _, item := range decodeLogItems(t, []byte(out.String())) {
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
			control, err := input.DecodeControlMessage()
			if err == nil && control.Mode == inbox.ExecutionDrained {
				receipt := control.Parameters.(inbox.ExecutionDrainReceipt)
				found = receipt.Scope == "known-graph" && receipt.Version == 1
			}
		}
	}
	if !found {
		t.Fatal("known graph owner joins did not produce canonical receipt")
	}

}

func TestCleanupProofRunnerToolCloseFailureSuppressesReceipt(t *testing.T) {
	client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		return llm.Response{Stop: llm.StopComplete}, nil
	}}
	failure := errors.New("actual tool cleanup failed")
	config := testConfig(client)
	config.ParseRequest = func(input io.Reader) (Request, ToolFactory, error) {
		request, factory, err := ParseRequest(input)
		return request, func(ctx context.Context, cfg ToolConfig) (Tools, error) {
			tools, err := factory(ctx, cfg)
			tools.Close = func() error { return failure }
			return tools, err
		}, err
	}
	var out bytes.Buffer
	err := Run(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", t.TempDir(), "-p", "hello", "-cleanup-proof-scope", "failed-tools"}, func(key string) string {
		if key == providers.APIKeyEnvironment {
			return "secret"
		}
		return ""
	}, os.Environ, strings.NewReader(""), &out, io.Discard, config)
	if !errors.Is(err, failure) {
		t.Fatalf("lost cleanup failure: %v", err)
	}
	for _, item := range decodeLogItems(t, out.Bytes()) {
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
			control, err := input.DecodeControlMessage()
			if err == nil && control.Mode == inbox.ExecutionDrained {
				t.Fatal("receipt despite actual tool close failure")
			}
		}
	}
}
