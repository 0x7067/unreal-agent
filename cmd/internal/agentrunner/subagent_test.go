package agentrunner

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

const subagentChildEnvironment = "HARNESS_SUBAGENT_CHILD_TEST"

// TestSubagentChildProcess is the subagent process started by
// TestRunMainSubagentsExchangeInboxMessages. It messages its parent once, then
// answers each message with the text it last received.
func TestSubagentChildProcess(t *testing.T) {
	if os.Getenv(subagentChildEnvironment) != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator == -1 {
		t.Fatal("subagent arguments are missing")
	}
	os.Args = append(os.Args[:1], os.Args[separator+1:]...)
	client := &fakeClient{}
	client.respond = func(_ context.Context, request llm.Request) (llm.Response, error) {
		if os.Getenv("HARNESS_SUBAGENT_PERMISSION_TEST") == "1" {
			if containsTool(request.Tools, "Bash") {
				return messageResponse("Bash available"), nil
			}
			return messageResponse("Bash denied"), nil
		}
		last := request.Input[len(request.Input)-1]
		text := lastUserText(request.Input)
		if message, ok := last.Data.(llm.Message); ok && message.Role == llm.RoleUser &&
			!strings.HasPrefix(text, "Message from your parent agent") {
			return toolCallResponse("child-send", "SendMessage", `{"to":"parent","message":"progress from child"}`), nil
		}
		return messageResponse("child saw: " + text), nil
	}
	Main(testConfig(client))
}

func TestSubagentsInheritDisallowedTools(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(subagentChildEnvironment, "1")
	t.Setenv("HARNESS_SUBAGENT_PERMISSION_TEST", "1")
	t.Setenv(providers.APIKeyEnvironment, "secret")
	t.Setenv(llmProviderEnvironment, "")
	t.Setenv(llmBaseURLEnvironment, "")
	finished := false
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
		if text := toolResults(request.Input)["agent"]; text != "" {
			finished = text == "Bash denied"
			return messageResponse("done"), nil
		}
		return toolCallResponse("agent", "Agent", `{"name":"worker","prompt":"check permissions"}`), nil
	}}
	config := testConfig(client)
	config.SubagentCommand = []string{executable, "-test.run=^TestSubagentChildProcess$", "--"}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var stdout, stderr strings.Builder
	code := RunMain(ctx, []string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()}, os.Getenv, os.Environ,
		strings.NewReader(`{"prompt":"delegate","model":"gpt-test","extra_allowed_tools":["Agent"],"disallowed_tools":["Bash"]}`), &stdout, &stderr, config)
	if code != 0 || !finished {
		t.Fatalf("exit=%d inherited denial=%v stderr=%s", code, finished, stderr.String())
	}
}

func TestRunMainSubagentsExchangeInboxMessages(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(subagentChildEnvironment, "1")
	t.Setenv(providers.APIKeyEnvironment, "secret")
	t.Setenv(llmProviderEnvironment, "")
	t.Setenv(llmBaseURLEnvironment, "")

	var mu sync.Mutex
	finished := false
	client := &fakeClient{}
	client.respond = func(_ context.Context, request llm.Request) (llm.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		results := toolResults(request.Input)
		heardFromChild := slices.ContainsFunc(request.Input, func(item llm.Item) bool {
			message, ok := item.Data.(llm.Message)
			return ok && message.Role == llm.RoleUser &&
				message.Text == "Message from subagent \"worker\":\nprogress from child"
		})
		switch {
		case !slices.ContainsFunc(request.Input, func(item llm.Item) bool { return item.Type == llm.ItemToolCall }):
			return toolCallResponse("agent", "Agent", `{"name":"worker","prompt":"count files"}`), nil
		case results["send"] == "child saw: Message from your parent agent:\nfollow-up":
			finished = true
			return messageResponse("all done"), nil
		case results["agent"] == "child saw: count files" && heardFromChild && results["send"] == "":
			return toolCallResponse("send", "SendMessage", `{"to":"worker","message":"follow-up"}`), nil
		default:
			return messageResponse("waiting"), nil
		}
	}
	config := testConfig(client)
	config.SubagentCommand = []string{executable, "-test.run=^TestSubagentChildProcess$", "--"}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var stdout, stderr strings.Builder
	code := RunMain(ctx,
		[]string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()},
		os.Getenv, os.Environ,
		strings.NewReader(`{"prompt":"delegate","model":"gpt-test","extra_allowed_tools":["Agent"]}`),
		&stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if !finished {
		t.Fatalf("parent never received the resumed subagent's reply; stderr = %q, stdout = %q", stderr.String(), stdout.String())
	}
}

func toolResults(input []llm.Item) map[string]string {
	results := make(map[string]string)
	for _, item := range input {
		result, ok := item.Data.(llm.ToolResult)
		if ok && !result.Running && len(result.Output) != 0 {
			results[result.CallID] = result.Output[0].Value
		}
	}
	return results
}

func lastUserText(input []llm.Item) string {
	for _, item := range slices.Backward(input) {
		if message, ok := item.Data.(llm.Message); ok && message.Role == llm.RoleUser {
			return message.Text
		}
	}
	return ""
}

func toolCallResponse(callID, name, arguments string) llm.Response {
	return llm.Response{ID: "response-" + callID, Stop: llm.StopComplete, Output: []llm.Item{{
		Type: llm.ItemToolCall,
		Data: llm.ToolCall{CallID: callID, Name: name, Arguments: arguments},
	}}}
}

func messageResponse(text string) llm.Response {
	return llm.Response{ID: "response", Stop: llm.StopComplete, Output: []llm.Item{{
		Type: llm.ItemMessage,
		Data: llm.Message{Role: llm.RoleAssistant, Text: text},
	}}}
}
