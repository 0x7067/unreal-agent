package agentrunner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestRunMainTaskGraphAcceptance(t *testing.T) {
	workspace := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	started, accepted, finished := false, false, false
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
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
	code := RunMain(ctx, []string{"-workspace", workspace, "-session-directory", t.TempDir()}, func(name string) string {
		if name == "XDG_CONFIG_HOME" {
			return workspace
		}
		if name == "OPENAI_API_KEY" {
			return "secret"
		}
		return ""
	}, os.Environ, strings.NewReader(`{"prompt":"execute graph","model":"gpt-test"}`), &out, &stderr, testConfig(client))
	if code != 0 || !accepted || !finished {
		t.Fatalf("exit=%d accepted=%v finished=%v stderr=%s output=%s", code, accepted, finished, stderr.String(), out.String())
	}
}

func TestRunMainTaskGraphPermissions(t *testing.T) {
	for _, test := range []struct{ name, task, denied, want string }{
		{"shell denied", `"command":"touch forbidden"`, `"Bash"`, "shell tasks require Bash capability"},
		{"agent denied", `"prompt":"touch forbidden"`, `"Agent"`, "agent tasks require agent capability"},
		{"graph denied", `"command":"touch forbidden"`, `"TaskGraph"`, "TaskGraph"},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			started, failed := false, false
			client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
				if text := toolResults(request.Input)["graph"]; text != "" {
					failed = strings.Contains(text, test.want)
					return messageResponse("done"), nil
				}
				if !started {
					started = true
					return toolCallResponse("graph", "TaskGraph", `{"action":"start","name":"denied","tasks":[{"id":"bad",`+test.task+`,"reads":[],"writes":["forbidden"],"acceptance":"file exists"}]}`), nil
				}
				return messageResponse("waiting"), nil
			}}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var out, stderr strings.Builder
			code := RunMain(ctx, []string{"-workspace", workspace, "-session-directory", t.TempDir()}, func(name string) string {
				if name == "OPENAI_API_KEY" {
					return "secret"
				}
				if name == "XDG_CONFIG_HOME" {
					return workspace
				}
				return ""
			}, os.Environ, strings.NewReader(`{"prompt":"try denied work","model":"gpt-test","disallowed_tools":[`+test.denied+`]}`), &out, &stderr, testConfig(client))
			if code != 0 || !failed {
				t.Fatalf("exit=%d denied=%v stderr=%s output=%s", code, failed, stderr.String(), out.String())
			}
			if _, err := os.Stat(filepath.Join(workspace, "forbidden")); !os.IsNotExist(err) {
				t.Fatalf("forbidden side effect: %v", err)
			}
		})
	}
}

func TestRunMainTinyTaskNeedsNoGraphOrChild(t *testing.T) {
	workspace := t.TempDir()
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) { return messageResponse("two"), nil }}
	config := testConfig(client)
	config.SubagentCommand = []string{filepath.Join(workspace, "must-not-start")}
	var out, stderr strings.Builder
	code := RunMain(t.Context(), []string{"-workspace", workspace, "-session-directory", t.TempDir()}, func(name string) string {
		if name == "OPENAI_API_KEY" {
			return "secret"
		}
		if name == "XDG_CONFIG_HOME" {
			return workspace
		}
		return ""
	}, os.Environ, strings.NewReader(`{"prompt":"one plus one","model":"gpt-test"}`), &out, &stderr, config)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	for _, item := range decodeLogItems(t, []byte(out.String())) {
		if item.Kind == sessionstore.ItemToolCallStatus {
			t.Fatalf("tiny task started work: %#v", item)
		}
	}
}
