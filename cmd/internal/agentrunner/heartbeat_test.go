package agentrunner

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestRunMainHeartbeatReleasesWaitingBashAndReplays(t *testing.T) {
	workspace, sessions := t.TempDir(), t.TempDir()
	path := filepath.Join(workspace, "release")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	pipe, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pipe.Close(); err != nil {
			t.Error(err)
		}
	})
	var stateMu sync.Mutex
	started, released, finished := false, false, false
	waitingCall := llm.ToolCall{
		CallID: "waiting-call", Name: "Bash", Arguments: `{"command":"read value < release; printf '%s' \"$value\""}`,
	}
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
		stateMu.Lock()
		defer stateMu.Unlock()
		if !started {
			started = true
			return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: waitingCall}}}, nil
		}
		for _, item := range request.Input {
			if item.Type != llm.ItemToolResult {
				continue
			}
			result := item.Data.(llm.ToolResult)
			if result.CallID != "waiting-call" || result.Running {
				continue
			}
			if result.Output[0].Value != "released" {
				return llm.Response{}, fmt.Errorf("unexpected Bash result: %s", result.Output[0].Value)
			}
			finished = true
			return llm.Response{Stop: llm.StopComplete}, nil
		}
		if heartbeatMessageCount(request) == 0 {
			return llm.Response{}, errors.New("waiting turn has no heartbeat")
		}
		if !released {
			if _, err := io.WriteString(pipe, "released\n"); err != nil {
				return llm.Response{}, err
			}
			released = true
		}
		return llm.Response{Stop: llm.StopComplete}, nil
	}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	getenv := func(name string) string {
		switch name {
		case providers.APIKeyEnvironment:
			return "secret"
		case "SHELL":
			return "/bin/sh"
		default:
			return ""
		}
	}
	args := []string{"-workspace", workspace, "-session-directory", sessions, "-tool-heartbeat-interval", "10ms"}
	var stdout, stderr bytes.Buffer
	if code := RunMain(ctx, args, getenv, func() []string { return nil },
		strings.NewReader(`{"prompt":"run it","session_id":"heartbeat-session"}`),
		&stdout, &stderr, testConfig(client)); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	stateMu.Lock()
	if !released || !finished {
		stateMu.Unlock()
		t.Fatal("heartbeat did not release Bash and deliver its result")
	}
	stateMu.Unlock()
	heartbeats := 0
	decoder := jsontext.NewDecoder(&stdout)
	for {
		var item sessionstore.Item
		if err := json.UnmarshalDecode(decoder, &item); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		if item.Kind == sessionstore.ItemInput {
			input := item.Data.(inbox.Input)
			if input.Kind != inbox.InputControl {
				continue
			}
			control, err := input.DecodeControlMessage()
			if err != nil {
				t.Fatal(err)
			}
			if control.Mode == inbox.Heartbeat {
				encoded, ok := strings.CutPrefix(control.Reason, "Heartbeat: waited 0.01 seconds for tool calls.\nPending (execution phase unconfirmed): ")
				if !ok {
					t.Fatalf("heartbeat reason = %q", control.Reason)
				}
				encoded, summary, ok := strings.Cut(encoded, "\nActivity observations: ")
				if !ok || !strings.Contains(summary, "Live supervision snapshot") || !strings.Contains(summary, "in_process_operation_envelopes") {
					t.Fatalf("missing live observations: %q", control.Reason)
				}
				var calls []struct{ CallID, Name string }
				if err := json.Unmarshal([]byte(encoded), &calls); err != nil {
					t.Fatal(err)
				}
				if len(calls) != 1 || calls[0].CallID != waitingCall.CallID || calls[0].Name != waitingCall.Name {
					t.Fatalf("heartbeat pending call identities=%+v", calls)
				}
				if strings.Contains(control.Reason, "Arguments") || strings.Contains(control.Reason, "read value < release") {
					t.Fatalf("heartbeat leaked raw arguments: %q", control.Reason)
				}
				heartbeats++
			}
		}
	}
	if heartbeats == 0 {
		t.Fatal("heartbeat was not recorded in the session log")
	}
	resumed := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
		if got := heartbeatMessageCount(request); got != heartbeats {
			return llm.Response{}, fmt.Errorf("replayed heartbeat messages = %d, want %d", got, heartbeats)
		}
		return llm.Response{Stop: llm.StopComplete}, nil
	}}
	if code := RunMain(ctx, args, getenv, func() []string { return nil },
		strings.NewReader(`{"prompt":"summarize","session_id":"heartbeat-session"}`),
		io.Discard, &stderr, testConfig(resumed)); code != 0 {
		t.Fatalf("resume exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunMainRejectsInvalidHeartbeatInterval(t *testing.T) {
	for _, interval := range []string{"-1s", "invalid", "999999999999999999999s"} {
		t.Run(interval, func(t *testing.T) {
			var stderr bytes.Buffer
			code := RunMain(t.Context(), []string{"-tool-heartbeat-interval", interval},
				func(string) string { return "" }, func() []string { return nil },
				strings.NewReader(`{"prompt":"hello"}`), io.Discard, &stderr, Config{Name: "unreal-agent-runner", ParseRequest: parseTestRequest})
			if code != 1 || !strings.Contains(stderr.String(), "heartbeat") {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
		})
	}
}

func heartbeatMessageCount(request llm.Request) int {
	count := 0
	for _, item := range request.Input {
		if item.Type == llm.ItemMessage {
			message := item.Data.(llm.Message)
			if message.Role == llm.RoleUser && strings.HasPrefix(message.Text, "Heartbeat: waited 0.01 seconds for tool calls.\nPending (execution phase unconfirmed): ") {
				count++
			}
		}
	}
	return count
}
