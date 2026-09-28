package agentrunner

import (
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm/clients/anthropic"
	"github.com/unreallabsai/unreal-agent/harness/llm/providers"
)

func TestRunnerAnthropicUsesMessagesAPI(t *testing.T) {
	selected, err := providers.Find(providers.Default(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if selected.BaseURL != anthropic.DefaultBaseURL {
		t.Fatalf("default base URL = %q", selected.BaseURL)
	}
	for _, override := range []bool{false, true} {
		t.Run("key_override="+strconv.FormatBool(override), func(t *testing.T) {
			wantKey := "anthropic-key"
			if override {
				wantKey = "override-key"
			}
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != wantKey || r.Header.Get("Anthropic-Version") != "2023-06-01" {
					t.Error("incorrect Messages API endpoint or authentication")
				}
				var body struct {
					Model        string                        `json:"model"`
					Stream       bool                          `json:"stream"`
					System       []struct{ Text string }       `json:"system"`
					Messages     []struct{ Role string }       `json:"messages"`
					Tools        []struct{ Type, Name string } `json:"tools"`
					Thinking     struct{ Type string }         `json:"thinking"`
					OutputConfig struct{ Effort string }       `json:"output_config"`
				}
				if err := json.UnmarshalRead(r.Body, &body, json.MatchCaseInsensitiveNames(true)); err != nil {
					t.Error(err)
					return
				}
				if body.Model != "claude-opus-5-5" || !body.Stream || len(body.System) != 1 || !strings.HasSuffix(body.System[0].Text, "\n\nmy system prompt") || body.Thinking.Type != "adaptive" || body.OutputConfig.Effort != "high" {
					t.Errorf("request = %#v", body)
				}
				if len(body.Tools) != 2 || body.Tools[0].Name != "Bash" || body.Tools[1].Name != "ViewImage" || body.Tools[0].Type != "custom" || body.Tools[1].Type != "custom" {
					t.Errorf("tools = %#v", body.Tools)
				}
				for _, message := range body.Messages {
					if message.Role != "user" {
						t.Errorf("unexpected message role %q", message.Role)
					}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, `data: {"type":"message_start","message":{"id":"msg-1","type":"message","role":"assistant","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1}}}

data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"anthropic works"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}

data: {"type":"message_stop"}

`)
			}))
			defer server.Close()
			environment := map[string]string{
				"UNREAL_HARNESS_LLM_PROVIDER": "anthropic",
				"UNREAL_HARNESS_LLM_BASE_URL": server.URL + "/v1",
				"ANTHROPIC_API_KEY":           "anthropic-key",
			}
			if override {
				environment["UNREAL_HARNESS_LLM_API_KEY"] = wantKey
			}
			var output, stderr strings.Builder
			code := RunMain(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()},
				func(key string) string { return environment[key] }, func() []string { return nil },
				strings.NewReader(`{"prompt":"hello","system_prompt":"my system prompt"}`), &output, &stderr,
				Config{Name: "agent-runner", ParseRequest: parseTestRequest, Providers: providers.Default()})
			if code != 0 || requests.Load() != 1 || !strings.Contains(output.String(), "anthropic works") {
				t.Fatalf("exit = %d, requests = %d, stderr = %s", code, requests.Load(), stderr.String())
			}
			if strings.Contains(output.String(), wantKey) {
				t.Fatal("credential leaked into session output")
			}
		})
	}
}

func TestRunnerProviderRetries(t *testing.T) {
	for _, provider := range providers.Default() {
		for _, maxAttempts := range []int{1, 2} {
			t.Run(provider.Name+"/"+strconv.Itoa(maxAttempts), func(t *testing.T) {
				t.Parallel()
				var attempts atomic.Int64
				server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					var body struct {
						MaxAttempts *int `json:"max_attempts"`
					}
					if err := json.UnmarshalRead(request.Body, &body); err != nil {
						t.Error(err)
					}
					if body.MaxAttempts != nil {
						t.Error("harness retry configuration leaked into provider request")
					}
					attempts.Add(1)
					writer.WriteHeader(http.StatusServiceUnavailable)
				}))
				defer server.Close()
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				code := RunMain(ctx,
					[]string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()},
					func(name string) string {
						switch name {
						case "UNREAL_HARNESS_LLM_PROVIDER":
							return provider.Name
						case "UNREAL_HARNESS_LLM_BASE_URL":
							return server.URL
						case "OPENAI_CODEX_ACCESS_TOKEN":
							return "subscription-token"
						case "OPENAI_CODEX_ACCOUNT_ID":
							return "account-1"
						case "UNREAL_HARNESS_LLM_API_KEY":
							return "test-key"
						default:
							return ""
						}
					}, func() []string { return nil },
					strings.NewReader(`{"prompt":"hello","model":"test","max_attempts":`+strconv.Itoa(maxAttempts)+`}`),
					io.Discard, io.Discard, Config{Name: "unreal-agent-runner", ParseRequest: parseTestRequest, Providers: providers.Default()})
				if code != 1 || attempts.Load() != int64(maxAttempts) {
					t.Fatalf("exit = %d, attempts = %d, want %d", code, attempts.Load(), maxAttempts)
				}
			})
		}
	}
}

func TestRunnerCodexUsesSubscriptionWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer subscription-token" || r.Header.Get("ChatGPT-Account-ID") != "account" {
			t.Error("wrong authentication")
		}
		var body struct {
			Stream bool   `json:"stream"`
			Model  string `json:"model"`
		}
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Error(err)
		}
		if !body.Stream || body.Model != "gpt-6-astra" {
			t.Errorf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"status\":\"completed\",\"output\":[{\"id\":\"msg-1\",\"type\":\"message\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"subscription works\"}]}]}}\n\n")
	}))
	defer server.Close()
	var output, stderr strings.Builder
	code := RunMain(t.Context(), []string{"-workspace", t.TempDir(), "-session-directory", t.TempDir()}, func(key string) string {
		return map[string]string{
			"UNREAL_HARNESS_LLM_PROVIDER": "openai-codex",
			"UNREAL_HARNESS_LLM_BASE_URL": server.URL,
			"OPENAI_CODEX_ACCESS_TOKEN":   "subscription-token",
			"OPENAI_CODEX_ACCOUNT_ID":     "account",
		}[key]
	}, func() []string { return nil }, strings.NewReader(`{"prompt":"hello","system_prompt":"my system prompt"}`), &output, &stderr, Config{Name: "unreal-agent-runner", ParseRequest: parseTestRequest, Providers: providers.Default()})
	if code != 0 || !strings.Contains(output.String(), "subscription works") {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(output.String(), "subscription-token") {
		t.Fatal("credential leaked into session output")
	}
}
