package agentrunner

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm/providers"
)

func TestRunnerSignalProcess(t *testing.T) {
	if os.Getenv("HARNESS_RUNNER_SIGNAL_TEST") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator == -1 {
		t.Fatal("runner arguments are missing")
	}
	os.Args = append(os.Args[:1], os.Args[separator+1:]...)
	Main(Config{Name: "test-runner", ParseRequest: parseTestRequest, Providers: providers.Default()})
}

func TestRunnerSignals(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		signal os.Signal
		bash   bool
	}{
		{name: "model/SIGINT", signal: os.Interrupt},
		{name: "model/SIGTERM", signal: syscall.SIGTERM},
		{name: "bash/SIGINT", signal: os.Interrupt, bash: true},
		{name: "bash/SIGTERM", signal: syscall.SIGTERM, bash: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			workspace := t.TempDir()
			type readiness struct {
				pid int
				err error
			}
			started := make(chan readiness, 1)
			var command string
			if test.bash {
				readyPath := filepath.Join(workspace, "ready")
				holdPath := filepath.Join(workspace, "hold")
				for _, path := range []string{readyPath, holdPath} {
					if err := syscall.Mkfifo(path, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				ready, err := os.OpenFile(readyPath, os.O_RDWR, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer ready.Close()
				go func() {
					var pid int
					_, err := fmt.Fscanln(ready, &pid)
					started <- readiness{pid: pid, err: err}
				}()
				command = `trap '' TERM; printf '%s\n' "$$" > ready; read value < hold`
			}
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if _, err := io.Copy(io.Discard, request.Body); err != nil {
					t.Error(err)
					return
				}
				if !test.bash {
					started <- readiness{}
					<-request.Context().Done()
					return
				}
				arguments, err := json.Marshal(map[string]string{"command": command})
				if err != nil {
					t.Error(err)
					return
				}
				response, err := json.Marshal(map[string]any{
					"type": "response.completed",
					"response": map[string]any{
						"id": "response-1", "object": "response", "status": "completed",
						"output": []map[string]string{{
							"id": "function-1", "type": "function_call", "call_id": "call-1",
							"name": "Bash", "arguments": string(arguments), "status": "completed",
						}},
					},
				})
				if err != nil {
					t.Error(err)
					return
				}
				writer.Header().Set("Content-Type", "text/event-stream")
				if _, err := fmt.Fprintf(writer, "data: %s\n\n", response); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			runner := exec.CommandContext(ctx, executable, "-test.run=^TestRunnerSignalProcess$", "--",
				"-workspace", workspace, "-session-directory", filepath.Join(workspace, "sessions"), "-p", "hello")
			runner.Env = []string{
				"HARNESS_RUNNER_SIGNAL_TEST=1",
				"HOME=" + workspace,
				"XDG_CONFIG_HOME=" + workspace,
				"SHELL=/bin/sh",
				"UNREAL_HARNESS_LLM_PROVIDER=openai",
				"UNREAL_HARNESS_LLM_API_KEY=test-key",
				"UNREAL_HARNESS_LLM_BASE_URL=" + server.URL,
				"UNREAL_HARNESS_LLM_MAX_ATTEMPTS=1",
			}
			var output bytes.Buffer
			runner.Stdout = &output
			runner.Stderr = &output
			if err := runner.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- runner.Wait() }()
			var waited bool
			defer func() {
				cancel()
				if !waited {
					<-done
				}
			}()
			select {
			case ready := <-started:
				if ready.err != nil {
					t.Fatal(ready.err)
				}
				if test.bash {
					if ready.pid <= 1 {
						t.Fatalf("invalid Bash process group: %d", ready.pid)
					}
					defer func() {
						if err := syscall.Kill(-ready.pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
							t.Errorf("clean up Bash process group: %v", err)
						}
					}()
					if err := syscall.Kill(-ready.pid, 0); err != nil {
						t.Fatalf("Bash process group %d does not exist before cancellation: %v", ready.pid, err)
					}
				}
				if err := runner.Process.Signal(test.signal); err != nil {
					t.Fatal(err)
				}
				waitErr := <-done
				waited = true
				var exit *exec.ExitError
				if !errors.As(waitErr, &exit) || exit.ExitCode() != 130 {
					t.Fatalf("exit = %v, want status 130; output:\n%s", waitErr, &output)
				}
				if strings.Contains(output.String(), `"type":"error"`) {
					t.Fatalf("cancellation emitted an error: %s", &output)
				}
				if test.bash {
					if err := syscall.Kill(-ready.pid, 0); !errors.Is(err, syscall.ESRCH) {
						t.Fatalf("Bash process group %d still exists after runner exit: %v", ready.pid, err)
					}
				}
			case err := <-done:
				waited = true
				t.Fatalf("runner exited before readiness: %v; output:\n%s", err, &output)
			case <-ctx.Done():
				t.Fatalf("runner did not become ready: %v", ctx.Err())
			}
		})
	}
}
