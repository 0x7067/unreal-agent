//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package agentrunner

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestRemoteArtifactChildProcess(t *testing.T) {
	if os.Getenv("HARNESS_REMOTE_ARTIFACT_CHILD") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("missing arguments")
	}
	os.Args = append(os.Args[:1], os.Args[separator+1:]...)
	Main(testConfig(&fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		return messageResponse(strings.Repeat("remote child full reply\n", 3000)), nil
	}}))
}

func TestRunnerRemoteArtifactDefaults(t *testing.T) {
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("HARNESS_REMOTE_ARTIFACT_CHILD", "1")
	t.Setenv(providers.APIKeyEnvironment, "secret")
	t.Setenv(llmProviderEnvironment, "")
	t.Setenv(llmBaseURLEnvironment, "")
	preview := ""
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
		if text := toolResults(request.Input)["artifact-agent"]; text != "" {
			preview = text
			return messageResponse("done"), nil
		}
		return toolCallResponse("artifact-agent", "Agent", `{"name":"artifact-worker","prompt":"return a large candidate"}`), nil
	}}
	config := testConfig(client)
	config.SubagentCommand = []string{executable, "-test.run=^TestRemoteArtifactChildProcess$", "--"}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	directory := t.TempDir()
	var stdout, stderr strings.Builder
	code := RunMain(ctx, []string{"-workspace", t.TempDir(), "-session-directory", directory}, os.Getenv, os.Environ, strings.NewReader(`{"prompt":"delegate","model":"gpt-test","extra_allowed_tools":["Agent"]}`), &stdout, &stderr, config)
	if code != 0 || !strings.Contains(preview, "complete output in ") {
		t.Fatalf("runner did not advertise artifact: code=%d preview=%s stderr=%s", code, preview, stderr.String())
	}
	paths, e := filepath.Glob(filepath.Join(directory, "operations", "*", "remote-outputs", "*.txt"))
	if e != nil || len(paths) != 1 {
		t.Fatalf("artifact paths: %v %v", paths, e)
	}
	b, e := os.ReadFile(paths[0])
	if e != nil || string(b) != strings.Repeat("remote child full reply\n", 3000) {
		t.Fatalf("runner recovery: %v (%d bytes)", e, len(b))
	}
}
