package providers_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/llm/providers"
)

func TestAnthropicFallbackConfigurationAndContinuation(t *testing.T) {
	for _, test := range []struct {
		name, value string
		enabled     bool
	}{
		{name: "disabled"},
		{name: "disabled_zero", value: "0"},
		{name: "disabled_false", value: "false"},
		{name: "enabled_one", value: "1", enabled: true},
		{name: "enabled_true", value: "true", enabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			enabled := test.enabled
			var count atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				turn := count.Add(1)
				var request struct {
					Model     string         `json:"model"`
					Fallbacks jsontext.Value `json:"fallbacks"`
					Messages  []struct {
						Role    string           `json:"role"`
						Content []jsontext.Value `json:"content"`
					} `json:"messages"`
				}
				if err := json.UnmarshalRead(r.Body, &request); err != nil {
					t.Error(err)
					return
				}
				if request.Model != "primary-model" {
					t.Errorf("primary model changed to %q", request.Model)
				}
				if !enabled {
					if len(request.Fallbacks) != 0 || r.Header.Get("Anthropic-Beta") != "" {
						t.Error("fallback enabled without configuration")
					}
				} else {
					if string(request.Fallbacks) != `"default"` || r.Header.Get("Anthropic-Beta") != "server-side-fallback-2026-07-01" {
						t.Errorf("fallbacks = %s, beta = %q", request.Fallbacks, r.Header.Get("Anthropic-Beta"))
					}
					if turn == 2 && (len(request.Messages) != 3 || request.Messages[1].Role != "assistant" || len(request.Messages[1].Content) != 2 || !strings.Contains(string(request.Messages[1].Content[0]), `"type":"fallback"`)) {
						t.Errorf("continuation lost fallback boundary: %#v", request.Messages)
					}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				model := "primary-model"
				usage := `{"input_tokens":10,"output_tokens":4}`
				if enabled {
					model = "fallback-model"
					iterations := `{"type":"fallback_message","model":"fallback-model","input_tokens":10,"output_tokens":4}`
					if turn == 1 {
						iterations = `{"type":"message","model":"primary-model","input_tokens":8,"output_tokens":2},` + iterations
					}
					usage = `{"input_tokens":10,"output_tokens":4,"iterations":[` + iterations + `]}`
				}
				stream := fmt.Sprintf("data: {\"type\":\"message_start\",\"message\":{\"id\":\"message\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":%q,\"usage\":{}}}\n\n", model)
				if enabled && turn == 1 {
					stream += `data: {"type":"content_block_start","index":0,"content_block":{"type":"fallback","from":{"model":"primary-model"},"to":{"model":"fallback-model"}}}` + "\n\n" +
						`data: {"type":"content_block_stop","index":0}` + "\n\n"
				}
				stream += `data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":"Answer"}}` + "\n\n" +
					`data: {"type":"content_block_stop","index":1}` + "\n\n"
				stream += `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":` + usage + `}` + "\n\n" +
					`data: {"type":"message_stop"}` + "\n\n"
				if _, err := fmt.Fprint(w, stream); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			provider, err := providers.Find(providers.Default(), "anthropic")
			if err != nil {
				t.Fatal(err)
			}
			client, err := provider.NewClient("test-key", server.URL, 1, func(name string) string {
				if name == "ANTHROPIC_FALLBACK" {
					return test.value
				}
				return ""
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})
			request := llm.Request{Model: llm.Model{ID: "primary-model"}, Input: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Hello"}}}}
			for turn := range 2 {
				response, err := client.Respond(t.Context(), request, llm.RequestOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if enabled && !strings.Contains(string(response.Usage.Raw), `"fallback_message"`) {
					t.Fatal("lost serving-model usage metadata")
				}
				want := map[string]llm.TokenUsage{"primary-model": {InputTokens: 10, OutputTokens: 4}}
				if enabled {
					want = map[string]llm.TokenUsage{"fallback-model": {InputTokens: 10, OutputTokens: 4}}
					if turn == 0 {
						want["primary-model"] = llm.TokenUsage{InputTokens: 8, OutputTokens: 2}
					}
				}
				if !reflect.DeepEqual(response.Usage.ByModel, want) {
					t.Fatalf("usage by model = %#v, want %#v", response.Usage.ByModel, want)
				}
				request.Input = append(request.Input, response.Output...)
				request.Input = append(request.Input, llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Continue"}})
			}
			if count.Load() != 2 {
				t.Fatalf("requests = %d, want 2", count.Load())
			}
		})
	}
}

func TestAnthropicRejectsInvalidFallbackConfiguration(t *testing.T) {
	provider, err := providers.Find(providers.Default(), "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	client, err := provider.NewClient("test-key", provider.BaseURL, 1, func(name string) string {
		if name == "ANTHROPIC_FALLBACK" {
			return "tru"
		}
		return ""
	})
	if client != nil {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("invalid fallback configuration created a client")
	}
	if err == nil || !strings.Contains(err.Error(), "parse ANTHROPIC_FALLBACK:") {
		t.Fatalf("error = %v", err)
	}
}

func TestFindUsesSuppliedProviders(t *testing.T) {
	want := providers.Default()[0]
	want.Name = "custom"
	want.BaseURL = "https://custom.example/v1"
	want.DefaultModel = "custom-model"
	want.APIKeyEnvironment = "CUSTOM_API_KEY"
	available := []providers.Provider{{Name: "unconfigured"}, want}
	got, err := providers.Find(available, "custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.BaseURL != want.BaseURL || got.DefaultModel != want.DefaultModel || got.APIKeyEnvironment != want.APIKeyEnvironment || got.NewClient == nil {
		t.Fatalf("Find returned incorrect provider: %+v", got)
	}
}

func TestFindErrors(t *testing.T) {
	for _, test := range []struct {
		name      string
		available []providers.Provider
		requested string
		want      string
	}{
		{
			name: "unknown provider", available: []providers.Provider{{Name: "second"}, {Name: "first"}}, requested: "unknown",
			want: `unsupported provider "unknown"; available providers: second, first`,
		},
		{
			name: "no providers", requested: "unknown",
			want: `unsupported provider "unknown"; available providers: `,
		},
		{
			name: "missing factory", available: []providers.Provider{{Name: "unconfigured"}}, requested: "unconfigured",
			want: `provider "unconfigured" has no client factory`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := providers.Find(test.available, test.requested)
			if err == nil || err.Error() != test.want {
				t.Fatalf("Find error = %v, want %q", err, test.want)
			}
		})
	}
}
