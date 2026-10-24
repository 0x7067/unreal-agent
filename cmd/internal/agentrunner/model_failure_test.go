package agentrunner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func TestFailedModelResponseExitsNonzeroWithoutExecutingPartialTools(t *testing.T) {
	for _, output := range []string{
		`[]`,
		`[{"type":"function_call","id":"fc-partial","call_id":"partial","name":"Bash","arguments":"{\"command\":\"printf unsafe > failure-marker\"}"}]`,
	} {
		t.Run(output, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				_, err := fmt.Fprintf(w, "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"failed\",\"status\":\"failed\",\"output\":%s,\"error\":{\"code\":\"insufficient_quota\",\"message\":\"fixture failure\"},\"usage\":{\"input_tokens\":17,\"output_tokens\":5}}}\n\n", output)
				if err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			workspace, sessions := t.TempDir(), t.TempDir()
			env := map[string]string{"UNREAL_HARNESS_LLM_PROVIDER": "openai", "UNREAL_HARNESS_LLM_API_KEY": "fake-key", "UNREAL_HARNESS_LLM_BASE_URL": server.URL, "UNREAL_HARNESS_LLM_MAX_ATTEMPTS": "1"}
			var stdout, stderr bytes.Buffer
			code := RunMain(ctx, []string{"-workspace", workspace, "-session-directory", sessions, "-p", "hello"}, func(key string) string { return env[key] }, func() []string { return nil }, strings.NewReader(""), &stdout, &stderr, Config{Name: "failure-test", ParseRequest: parseTestRequest, Providers: providers.Default()})
			if code != 1 || !strings.Contains(stdout.String(), `"Type":"error"`) && !strings.Contains(stdout.String(), `"type":"error"`) || !strings.Contains(stderr.String(), "insufficient_quota") {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if requests.Load() != 1 {
				t.Fatalf("requests=%d", requests.Load())
			}
			if _, err := os.Stat(filepath.Join(workspace, "failure-marker")); !os.IsNotExist(err) {
				t.Fatalf("failed partial tool ran: %v", err)
			}
			store, err := localfile.New(sessions)
			if err != nil {
				t.Fatal(err)
			}
			infos, err := store.ListSessions(ctx)
			if err != nil || len(infos) != 1 {
				t.Fatalf("sessions=%+v err=%v", infos, err)
			}
			page, err := store.Items(ctx, infos[0].ID, sessionstore.BeforeFirst, 100)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range page.Items {
				if item.Kind == sessionstore.ItemToolCallStatus {
					t.Fatal("failed tool was scheduled")
				}
				if response, ok := item.Data.(sessionstore.ModelResponse); ok {
					found = true
					if response.Response.Failure == nil || response.Response.Failure.Code != "insufficient_quota" || response.Response.Usage.InputTokens != 17 || response.Response.Usage.OutputTokens != 5 {
						t.Fatalf("durable response=%+v", response)
					}
				}
			}
			if !found {
				t.Fatal("failed response was not persisted")
			}
		})
	}
}
