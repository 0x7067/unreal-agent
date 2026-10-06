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
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestRunMainExecutesBatchedMessages(t *testing.T) {
	var requestsMu sync.Mutex
	var requests []llm.Request
	client := &fakeClient{
		respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
			requestsMu.Lock()
			requests = append(requests, request)
			requestsMu.Unlock()
			return llm.Response{
				ID: "response-1", Stop: llm.StopComplete,
				Output: []llm.Item{{
					Type: llm.ItemMessage,
					Data: llm.Message{Role: llm.RoleAssistant, Text: "done"},
				}},
				Usage: llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 4, OutputTokens: 2}},
			}, nil
		},
	}
	workspace := t.TempDir()
	configHome := t.TempDir()
	writeUserSettings(t, configHome, `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"gpt-test","name":"GPT Test","context_window":128000,"compaction_threshold":64000}]}}}`)
	skillPath := filepath.Join(workspace, ".harness", "skills", "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".harness", "settings.json"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte(`---
name: review
description: Review code.
---
`), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := t.TempDir()
	logDirectory := filepath.Join(t.TempDir(), "logs")
	var stdout, stderr bytes.Buffer
	code := RunMain(
		t.Context(),
		[]string{"-workspace", workspace, "-session-directory", sessions, "-log-directory", logDirectory},
		func(name string) string {
			switch name {
			case "XDG_CONFIG_HOME":
				return configHome
			case "OPENAI_API_KEY":
				return "secret"
			case "SHELL":
				return "/bin/sh"
			default:
				return ""
			}
		},
		func() []string { return []string{"PATH=/usr/bin:/bin"} },
		strings.NewReader(`{
			"messages":[
				{
					"role":"user",
					"content":"first",
					"message_id":"69621f8d-4f4d-49a5-8f7d-3b24fd855c01"
				},
				{"role":"user","content":"second"}
			],
			"system_prompt":"be concise",
			"model":"gpt-test",
			"max_output_tokens":2048,
			"thinking_level":"medium"
		}`),
		&stdout,
		&stderr,
		testConfig(client),
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	requestsMu.Lock()
	recorded := slices.Clone(requests)
	requestsMu.Unlock()
	wantMessages := []llm.Message{
		{Role: llm.RoleUser, Text: "first"},
		{Role: llm.RoleUser, Text: "second"},
	}
	allMessagesDelivered := false
	for _, request := range recorded {
		if request.Model.CompactionThreshold != 64_000 || request.Model.ID != "gpt-test" || request.Model.ReasoningEffort != llm.ReasoningEffortMedium {
			t.Fatalf("model = %#v", request.Model)
		}
		if request.Model.MaxOutputTokens == nil || *request.Model.MaxOutputTokens != 2048 {
			t.Fatalf("max output tokens = %v, want 2048", request.Model.MaxOutputTokens)
		}
		var messages []llm.Message
		for _, item := range request.Input {
			if item.Type == llm.ItemMessage {
				message := item.Data.(llm.Message)
				if message.Role != llm.RoleAssistant {
					messages = append(messages, message)
				}
			}
		}
		if len(messages) < 2 || len(messages) > 3 || messages[0].Role != llm.RoleSystem ||
			!strings.Contains(messages[0].Text, "<name>review</name>") ||
			!strings.Contains(messages[0].Text, "<location>"+skillPath+"</location>") ||
			!strings.HasSuffix(messages[0].Text, "\n\nbe concise") ||
			!slices.Equal(messages[1:], wantMessages[:len(messages)-1]) {
			t.Fatalf("messages = %#v, want system preamble plus ordered messages from %#v", messages, wantMessages)
		}
		allMessagesDelivered = allMessagesDelivered || len(messages) == 3
		if len(request.Tools) != 3 || !containsTool(request.Tools, "Bash") || !containsTool(request.Tools, "ViewImage") || !containsTool(request.Tools, "SkillUse") {
			t.Fatalf("tools = %#v, want Bash, ViewImage, and SkillUse", request.Tools)
		}
	}
	if !allMessagesDelivered {
		t.Fatal("no model request included both messages")
	}
	if !slices.Contains(itemKinds(t, stdout.String()), sessionstore.ItemModelResponse) {
		t.Fatal("model response was not recorded")
	}
	items := decodeLogItems(t, stdout.Bytes())
	control, err := items[0].Data.(inbox.Input).DecodeControlMessage()
	if err != nil || control.Mode != inbox.UpdateSettings || !reflect.DeepEqual(control.Parameters, inbox.Settings{Model: "gpt-test", CompactionThreshold: new(int64(64_000)), MaxOutputTokens: new(int64(2048)), ReasoningEffort: llm.ReasoningEffortMedium}) {
		t.Fatalf("initial settings = %#v, error = %v", control, err)
	}
	ids := inputIDs(t, stdout.String())
	if len(ids) != 2 || ids[0] != "69621f8d-4f4d-49a5-8f7d-3b24fd855c01" {
		t.Fatalf("input IDs = %#v", ids)
	}
	for _, id := range ids {
		if _, err := uuid.Parse(string(id)); err != nil {
			t.Fatalf("input ID %q is not a UUID: %v", id, err)
		}
	}
	logs, err := filepath.Glob(filepath.Join(logDirectory, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("log files = %#v, want one", logs)
	}
	if _, err := time.Parse("20060102-150405.jsonl", filepath.Base(logs[0])); err != nil {
		t.Fatalf("log filename %q is not a UTC datetime: %v", logs[0], err)
	}
	logged, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(logged) != stdout.String() {
		t.Fatalf("log = %q, stdout = %q", logged, stdout.String())
	}
	if !client.closed {
		t.Fatal("client was not closed")
	}
}

func TestRunMainUsesProviderAuthenticationConfiguration(t *testing.T) {
	const suppliedKey = " \t secret \t "
	client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		return llm.Response{ID: "response-1", Stop: llm.StopComplete}, nil
	}}
	created := false
	available := []providers.Provider{{
		Name: "custom", DefaultModel: "test-model", APIKeyEnvironment: "CUSTOM_CREDENTIAL",
		NewClient: func(apiKey, _ string, _ int, _ func(string) string) (providers.Client, error) {
			created = true
			if apiKey != suppliedKey {
				return nil, errors.New("unexpected API key")
			}
			return client, nil
		},
	}}
	var stderr strings.Builder
	code := RunMain(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()}, func(name string) string {
		return map[string]string{
			llmProviderEnvironment: "custom",
			"CUSTOM_CREDENTIAL":    suppliedKey,
			"CUSTOM_API_KEY":       "must-not-use",
		}[name]
	}, func() []string { return nil }, strings.NewReader(`{"prompt":"hello"}`), io.Discard, &stderr, Config{Name: "unreal-agent-runner", ParseRequest: parseTestRequest, Providers: available})
	if code != 0 || !created || !client.closed {
		t.Fatalf("exit = %d, client created = %v, client closed = %v, stderr = %s", code, created, client.closed, stderr.String())
	}
}

func TestRunMainUsesLLMConfigurationFromEnvironment(t *testing.T) {
	client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
		if request.Model.ID != "environment-model" {
			return llm.Response{}, fmt.Errorf("model = %q", request.Model.ID)
		}
		if request.Model.MaxOutputTokens != nil {
			return llm.Response{}, fmt.Errorf("max output tokens = %d, want unset", *request.Model.MaxOutputTokens)
		}
		return llm.Response{ID: "response-1", Stop: llm.StopComplete}, nil
	}}
	selected := false
	available := []providers.Provider{
		{
			Name:              "openai",
			APIKeyEnvironment: "OPENAI_API_KEY",
			NewClient: func(_ string, _ string, _ int, _ func(string) string) (providers.Client, error) {
				return nil, errors.New("default provider selected")
			},
		},
		{
			Name: "openrouter", BaseURL: "https://default.example/v1", DefaultModel: "router-model",
			APIKeyEnvironment: "OPENROUTER_API_KEY",
			NewClient: func(apiKey, baseURL string, maxAttempts int, _ func(string) string) (providers.Client, error) {
				if apiKey != "custom-secret" || baseURL != "https://custom.example/v1" || maxAttempts != 2 {
					return nil, errors.New("unexpected OpenRouter configuration")
				}
				selected = true
				return client, nil
			},
		},
	}
	var stdout, stderr bytes.Buffer
	code := RunMain(
		t.Context(),
		[]string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()},
		func(name string) string {
			switch name {
			case llmProviderEnvironment:
				return "openrouter"
			case providers.APIKeyEnvironment:
				return "custom-secret"
			case "OPENROUTER_API_KEY":
				return "provider-secret"
			case llmBaseURLEnvironment:
				return "https://custom.example/v1"
			case llmModelEnvironment:
				return "environment-model"
			case llmMaxAttemptsEnvironment:
				return "2"
			default:
				return ""
			}
		},
		func() []string { return []string{"PATH=/usr/bin:/bin"} },
		strings.NewReader(`{
			"messages":[{"role":"user","content":"hello"}]
		}`),
		&stdout,
		&stderr,
		Config{Name: "unreal-agent-runner", ParseRequest: parseTestRequest, Providers: available},
	)
	if code != 0 || !selected {
		t.Fatalf("exit = %d, selected = %t, stderr = %q", code, selected, stderr.String())
	}
}

func TestRunMainExecutesBashToolToCompletion(t *testing.T) {
	t.Setenv(providers.APIKeyEnvironment, "secret")
	for _, test := range []struct {
		name, arguments, want string
		truncated             bool
	}{
		{"default limit", `{"command":"printf hello"}`, "hello", false},
		{"requested limit", `{"command":"printf hello","max_output_length":3}`, "h...2 bytes truncated; complete output in {path}...lo", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeClient{}
			client.respond = func(_ context.Context, request llm.Request) (llm.Response, error) {
				client.mu.Lock()
				defer client.mu.Unlock()
				client.calls++
				if client.calls == 1 {
					return llm.Response{
						ID: "response-1", Stop: llm.StopComplete,
						Output: []llm.Item{{
							Type: llm.ItemToolCall,
							Data: llm.ToolCall{
								CallID: "call-1", Name: "Bash",
								Arguments: test.arguments,
							},
						}},
					}, nil
				}
				foundResult := false
				for _, item := range request.Input {
					if item.Type != llm.ItemToolResult {
						continue
					}
					result := item.Data.(llm.ToolResult)
					if result.CallID != "call-1" || result.Running {
						continue
					}
					if test.truncated {
						prefix, suffix, _ := strings.Cut(test.want, "{path}")
						if !strings.HasPrefix(result.Output[0].Value, prefix) || !strings.HasSuffix(result.Output[0].Value, suffix) {
							return llm.Response{}, fmt.Errorf("unexpected truncated Bash output: %s", result.Output[0].Value)
						}
						path := strings.TrimSuffix(strings.TrimPrefix(result.Output[0].Value, prefix), suffix)
						if !filepath.IsAbs(path) {
							return llm.Response{}, fmt.Errorf("Bash capture path is not absolute: %q", path)
						}
						full, err := os.ReadFile(path)
						if err != nil {
							return llm.Response{}, err
						}
						if string(full) != "hello" {
							return llm.Response{}, fmt.Errorf("unexpected Bash capture: %q", full)
						}
					} else if result.Output[0].Value != test.want {
						return llm.Response{}, fmt.Errorf("unexpected Bash output: %s", result.Output[0].Value)
					}
					foundResult = true
				}
				if !foundResult {
					return llm.Response{}, errors.New("completed Bash result is missing")
				}
				return llm.Response{
					ID: "response-2", Stop: llm.StopComplete,
					Output: []llm.Item{{
						Type: llm.ItemMessage,
						Data: llm.Message{Role: llm.RoleAssistant, Text: "finished"},
					}},
				}, nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			workspace := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := RunMain(
				ctx,
				[]string{"-workspace", workspace, "-session-directory", t.TempDir()},
				func(name string) string {
					if name == providers.APIKeyEnvironment {
						return "secret"
					}
					if name == "SHELL" {
						return "/bin/sh"
					}
					return ""
				},
				func() []string { return []string{"PATH=/usr/bin:/bin", "UNREAL_HARNESS_LLM_API_KEY=secret"} },
				strings.NewReader(`{"messages":[{"role":"user","content":"run it"}],"model":"gpt-test"}`),
				&stdout,
				&stderr,
				testConfig(client),
			)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
			}
			if strings.Contains(stdout.String(), "PATH=") || strings.Contains(stdout.String(), "secret") {
				t.Fatal("persisted session items contain the process environment")
			}
			assertItemSequence(t, stdout.String(),
				"input.control input.external input.control turn model_response tool_call_status tool_call_status turn model_response",
				"input.control input.external turn input.control model_response tool_call_status tool_call_status turn model_response",
				"input.control input.external turn model_response tool_call_status input.control tool_call_status turn model_response",
				"input.control input.external turn model_response tool_call_status tool_call_status turn input.control model_response",
				"input.control input.external turn model_response tool_call_status tool_call_status turn model_response input.control",
			)
		})
	}
}

func TestRunMainEmitsValidationError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunMain(
		t.Context(), nil,
		func(string) string { return "secret" },
		func() []string { return nil },
		strings.NewReader(`{"messages":[],"thinking_level":"maximum"}`),
		&stdout,
		&stderr,
		testConfig(&fakeClient{}),
	)
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if got := eventTypes(t, stdout.String()); !slices.Equal(got, []string{"error"}) {
		t.Fatalf("event types = %#v", got)
	}
	if !strings.Contains(stderr.String(), "thinking_level must be one of") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestValidateRequestRejectsNonUUIDMessageID(t *testing.T) {
	messageID := "message-1"
	_, err := validateRequest(Request{
		Messages: []RequestMessage{{Content: "hello", MessageID: &messageID}},
	})
	if err == nil || err.Error() != "messages[0].message_id must be a UUID" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadDotEnvUsesScopedOverrides(t *testing.T) {
	t.Setenv("HARNESS_RUNNER_EXISTING", "outer")
	t.Setenv("SANDBOX_EGRESS_PROXY", "outer-proxy")
	t.Setenv("HTTPS_PROXY", "outer-https")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(
		"HARNESS_RUNNER_EXISTING=inner\nSANDBOX_EGRESS_PROXY=https://proxy.example\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	scope, err := loadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("HARNESS_RUNNER_EXISTING"); got != "outer" {
		t.Fatalf("existing value = %q", got)
	}
	if got := os.Getenv("SANDBOX_EGRESS_PROXY"); got != "https://proxy.example" {
		t.Fatalf("proxy value = %q", got)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "https://proxy.example" {
		t.Fatalf("HTTPS proxy value = %q", got)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("SANDBOX_EGRESS_PROXY"); got != "outer-proxy" {
		t.Fatalf("restored proxy value = %q", got)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "outer-https" {
		t.Fatalf("restored HTTPS proxy value = %q", got)
	}
}

type fakeClient struct {
	mu      sync.Mutex
	respond func(context.Context, llm.Request) (llm.Response, error)
	calls   int
	closed  bool
}

func (client *fakeClient) Respond(ctx context.Context, request llm.Request, _ llm.RequestOptions) (llm.Response, error) {
	return client.respond(ctx, request)
}

func (client *fakeClient) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.closed = true
	return nil
}

func testConfig(client providers.Client) Config {
	return Config{Name: "unreal-agent-runner", ParseRequest: parseTestRequest, Providers: []providers.Provider{{
		Name: "openai", BaseURL: "https://example.com",
		DefaultModel:      "gpt-default",
		APIKeyEnvironment: "OPENAI_API_KEY",
		NewClient: func(apiKey, baseURL string, maxAttempts int, _ func(string) string) (providers.Client, error) {
			if apiKey != "secret" || baseURL != "https://example.com" || maxAttempts != 5 {
				return nil, errors.New("unexpected provider configuration")
			}
			return client, nil
		},
	}}}
}

func eventTypes(t *testing.T, output string) []string {
	t.Helper()
	decoder := jsontext.NewDecoder(strings.NewReader(output))
	var types []string
	for {
		var value struct {
			Type string `json:"type"`
		}
		if err := json.UnmarshalDecode(decoder, &value); err != nil {
			if errors.Is(err, io.EOF) {
				return types
			}
			t.Fatal(err)
		}
		types = append(types, value.Type)
	}
}

func assertItemSequence(t *testing.T, output string, want ...string) {
	t.Helper()
	decoder := jsontext.NewDecoder(strings.NewReader(output))
	var kinds []string
	for {
		var item sessionstore.Item
		if err := json.UnmarshalDecode(decoder, &item); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		kind := string(item.Kind)
		if item.Kind == sessionstore.ItemInput {
			kind += "." + string(item.Data.(inbox.Input).Kind)
		}
		kinds = append(kinds, kind)
	}
	if got := strings.Join(kinds, " "); !slices.Contains(want, got) {
		t.Fatalf("item sequence = %q, want one of %q", got, want)
	}
}

func itemKinds(t *testing.T, output string) []sessionstore.ItemKind {
	t.Helper()
	decoder := jsontext.NewDecoder(strings.NewReader(output))
	var kinds []sessionstore.ItemKind
	for {
		var item sessionstore.Item
		if err := json.UnmarshalDecode(decoder, &item); err != nil {
			if errors.Is(err, io.EOF) {
				return kinds
			}
			t.Fatal(err)
		}
		kinds = append(kinds, item.Kind)
	}
}

func inputIDs(t *testing.T, output string) []inbox.ID {
	t.Helper()
	decoder := jsontext.NewDecoder(strings.NewReader(output))
	var ids []inbox.ID
	for {
		var item sessionstore.Item
		if err := json.UnmarshalDecode(decoder, &item); err != nil {
			if errors.Is(err, io.EOF) {
				return ids
			}
			t.Fatal(err)
		}
		if item.Kind == sessionstore.ItemInput {
			input := item.Data.(inbox.Input)
			if input.Kind == inbox.InputExternal {
				ids = append(ids, input.ID)
			}
		}
	}
}

func containsTool(tools []llm.Tool, name string) bool {
	for _, current := range tools {
		if current.Name == name {
			return true
		}
	}
	return false
}

func TestReasoningEffortMapsEveryThinkingLevel(t *testing.T) {
	cases := map[string]llm.ReasoningEffort{
		"low":    llm.ReasoningEffortLow,
		"medium": llm.ReasoningEffortMedium,
		"high":   llm.ReasoningEffortHigh,
		"xhigh":  llm.ReasoningEffortXHigh,
		"max":    llm.ReasoningEffortMax,
		"":       llm.ReasoningEffortHigh,
	}
	for level, want := range cases {
		if got := reasoningEffort(level); got != want {
			t.Errorf("reasoningEffort(%q) = %q, want %q", level, got, want)
		}
	}
	for _, level := range []string{"xhigh", "max"} {
		if _, err := validateRequest(Request{Prompt: new(string), ThinkingLevel: level}); err != nil {
			t.Errorf("validateRequest(thinking_level=%q) = %v, want nil", level, err)
		}
	}
}

func TestRunMainWithoutLogDirectoryWritesOnlyStdout(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "unset"},
		{name: "empty", args: []string{"-log-directory", ""}},
		{name: "whitespace", args: []string{"-log-directory", "  "}},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			t.Chdir(workspace)
			client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
				return llm.Response{Output: []llm.Item{{
					Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "done"},
				}}}, nil
			}}
			args := append([]string{"-workspace", workspace, "-session-directory", t.TempDir(), "-p", "hello"}, test.args...)
			var stdout, stderr bytes.Buffer
			code := RunMain(t.Context(), args, func(name string) string {
				if name == providers.APIKeyEnvironment {
					return "secret"
				}
				return ""
			}, func() []string { return nil }, strings.NewReader(""), &stdout, &stderr, testConfig(client))
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
			}
			if !slices.Contains(itemKinds(t, stdout.String()), sessionstore.ItemModelResponse) {
				t.Fatal("stdout is missing the model response")
			}
			entries, err := os.ReadDir(workspace)
			if err != nil || len(entries) != 0 {
				t.Fatalf("workspace entries = %v, %v; want no log files or directories", entries, err)
			}
		})
	}
}

func TestRunMainUsesDefaultSessionDirectory(t *testing.T) {
	workspace, stateHome := t.TempDir(), t.TempDir()
	t.Chdir(workspace)
	started := false
	client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		if started {
			return llm.Response{}, nil
		}
		started = true
		return llm.Response{Output: []llm.Item{{
			Type: llm.ItemToolCall,
			Data: llm.ToolCall{CallID: "call-1", Name: "Bash", Arguments: `{"command":"printf hello"}`},
		}}}, nil
	}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := RunMain(ctx, []string{"-workspace", workspace, "-p", "hello"}, func(name string) string {
		return map[string]string{
			providers.APIKeyEnvironment: "secret",
			"XDG_STATE_HOME":            stateHome,
			"SHELL":                     "/bin/sh",
		}[name]
	}, func() []string { return nil }, strings.NewReader(""), &stdout, &stderr, testConfig(client))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	sessions := filepath.Join(stateHome, "unreal-agent", "sessions")
	for pattern, wantContent := range map[string]string{
		"*.session.jsonl": "model_response",
		filepath.Join("operations", "*", "*", "out"): "hello",
	} {
		paths, err := filepath.Glob(filepath.Join(sessions, pattern))
		if err != nil || len(paths) != 1 {
			t.Fatalf("stored files for %q = %v, %v; want one", pattern, paths, err)
		}
		content, err := os.ReadFile(paths[0])
		if err != nil || !strings.Contains(string(content), wantContent) {
			t.Fatalf("stored file %q = %q, %v; want %q", paths[0], content, err, wantContent)
		}
	}
	entries, err := os.ReadDir(workspace)
	if err != nil || len(entries) != 0 {
		t.Fatalf("workspace entries = %v, %v; want no state in workspace", entries, err)
	}
}

func TestResolveSessionDirectory(t *testing.T) {
	t.Setenv("HOME", "/process/home")
	workspace := t.TempDir()
	t.Chdir(workspace)
	for _, test := range []struct {
		name, configured, stateHome, userHome, want string
	}{
		{name: "XDG state home", stateHome: "/state", userHome: "/home/user", want: "/state/unreal-agent/sessions"},
		{name: "XDG without home", stateHome: "/state", want: "/state/unreal-agent/sessions"},
		{name: "home fallback", userHome: "/home/user", want: "/home/user/.local/state/unreal-agent/sessions"},
		{name: "process home fallback", want: "/process/home/.local/state/unreal-agent/sessions"},
		{name: "relative XDG ignored", stateHome: "relative/state", userHome: "/home/user", want: "/home/user/.local/state/unreal-agent/sessions"},
		{name: "absolute override", configured: " /sessions ", stateHome: "/state", userHome: "/home/user", want: "/sessions"},
		{name: "relative override", configured: "sessions", stateHome: "/state", want: filepath.Join(workspace, "sessions")},
		{name: "override without environment", configured: "/sessions", want: "/sessions"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveSessionDirectory(test.configured, func(name string) string {
				return map[string]string{"XDG_STATE_HOME": test.stateHome, "HOME": test.userHome}[name]
			})
			if err != nil || got != test.want {
				t.Fatalf("resolveSessionDirectory = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestResolveSessionDirectoryRejectsMissingStateLocation(t *testing.T) {
	t.Setenv("HOME", "")
	for _, userHome := range []string{"", "relative/home"} {
		t.Run(userHome, func(t *testing.T) {
			_, err := resolveSessionDirectory("", func(name string) string {
				return map[string]string{"XDG_STATE_HOME": "relative/state", "HOME": userHome}[name]
			})
			if err == nil || !strings.Contains(err.Error(), "-session-directory") {
				t.Fatalf("error = %v, want actionable state location error", err)
			}
		})
	}
}

func TestRunLoadsModelSettings(t *testing.T) {
	for _, test := range []struct {
		name, provider, model, settings string
		threshold                       int64
		wantError                       bool
	}{
		{name: "built-in", model: "gpt-6-astra", threshold: 244_800},
		{name: "Codex built-in", provider: "openai-codex", model: "gpt-6-astra", threshold: 244_800},
		{name: "provider-specific model", provider: "other", model: "gpt-6-astra", settings: `{"providers":{"other":{"info":{"id":"other","name":"Other"},"models":[{"id":"gpt-6-astra","name":"Other Astra","context_window":200000,"compaction_threshold":75000}]}}}`, threshold: 75_000},
		{name: "unconfigured provider", provider: "other", model: "gpt-6-astra"},
		{name: "override", model: "gpt-6-astra", settings: `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"gpt-6-astra","name":"GPT-6 Astra","context_window":200000,"compaction_threshold":64000}]}}}`, threshold: 64_000},
		{name: "custom model", model: "custom", settings: `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"custom","name":"Custom","context_window":200000,"compaction_threshold":75000}]}}}`, threshold: 75_000},
		{name: "default threshold", model: "custom", settings: `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"custom","name":"Custom","context_window":200000}]}}}`, threshold: 100_000},
		{name: "zero threshold", model: "custom", settings: `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"custom","name":"Custom","context_window":200000,"compaction_threshold":0}]}}}`, threshold: 100_000},
		{name: "unknown model", model: "unknown"},
		{name: "invalid settings", model: "gpt-6-astra", settings: `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"gpt-6-astra","name":"GPT-6 Astra","context_window":200000,"compaction_threshold":-1}]}}}`, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			configHome := t.TempDir()
			if test.settings != "" {
				writeUserSettings(t, configHome, test.settings)
			}
			requests := make(chan llm.Request, 1)
			client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
				requests <- request
				return llm.Response{Stop: llm.StopComplete}, nil
			}}
			config := testConfig(client)
			if test.provider != "" {
				config.Providers[0].Name = test.provider
			}
			err := Run(t.Context(), []string{"-workspace", workspace, "-session-directory", t.TempDir()}, func(name string) string {
				if name == llmProviderEnvironment {
					return test.provider
				}
				if name == "XDG_CONFIG_HOME" {
					return configHome
				}
				if name == providers.APIKeyEnvironment {
					return "secret"
				}
				return ""
			}, func() []string { return nil }, strings.NewReader(fmt.Sprintf(`{"prompt":"hello","model":%q}`, test.model)), io.Discard, io.Discard, config)
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "load model settings") || len(requests) != 0 {
					t.Fatalf("invalid settings: error = %v, requests = %d", err, len(requests))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case request := <-requests:
				if request.Model.ID != test.model || request.Model.CompactionThreshold != test.threshold {
					t.Fatalf("model = %#v, want %s with threshold %d", request.Model, test.model, test.threshold)
				}
			default:
				t.Fatal("no model request")
			}
		})
	}
}

func writeUserSettings(t *testing.T, configHome, contents string) {
	t.Helper()
	directory := filepath.Join(configHome, "unreal-agent")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
