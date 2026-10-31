package providers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/llm/clients/anthropic"
	"github.com/unreallabsai/unreal-agent/harness/llm/clients/fireworks"
	"github.com/unreallabsai/unreal-agent/harness/llm/clients/ollama"
	"github.com/unreallabsai/unreal-agent/harness/llm/clients/openai"
	"github.com/unreallabsai/unreal-agent/harness/llm/clients/openaicodex"
	"github.com/unreallabsai/unreal-agent/harness/llm/clients/openrouter"
	"github.com/unreallabsai/unreal-agent/harness/llm/messagesapi"
)

type Client interface {
	llm.Adapter
	Close() error
}

type Provider struct {
	Name              string
	BaseURL           string
	DefaultModel      string
	APIKeyEnvironment string // Empty delegates authentication to NewClient.
	// The caller must close the returned client.
	NewClient func(apiKey, baseURL string, maxAttempts int, getenv func(string) string) (Client, error)
}

const (
	APIKeyEnvironment   = "UNREAL_HARNESS_LLM_API_KEY"
	ProviderEnvironment = "UNREAL_HARNESS_LLM_PROVIDER"
	BaseURLEnvironment  = "UNREAL_HARNESS_LLM_BASE_URL"
	DefaultProviderName = "openai"
)

// Selection is the provider, credential, and base URL resolved from the
// process environment. agentrunner and unreal-review both call Select.
type Selection struct {
	Provider Provider
	APIKey   string
	BaseURL  string
}

// Select resolves the provider the runner uses. An empty
// UNREAL_HARNESS_LLM_PROVIDER is openai. The API key is Provider.APIKey
// (UNREAL_HARNESS_LLM_API_KEY, then the provider variable). An empty
// UNREAL_HARNESS_LLM_BASE_URL is the registry base URL. openai-codex has no
// API key here; NewClient reads the Codex auth file and token pair.
func Select(available []Provider, getenv func(string) string) (Selection, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	name := strings.TrimSpace(getenv(ProviderEnvironment))
	if name == "" {
		name = DefaultProviderName
	}
	selected, err := Find(available, name)
	if err != nil {
		return Selection{}, err
	}
	base := strings.TrimSpace(getenv(BaseURLEnvironment))
	if base == "" {
		base = selected.BaseURL
	}
	key, err := selected.APIKey(getenv)
	if err != nil {
		return Selection{}, err
	}
	return Selection{Provider: selected, APIKey: key, BaseURL: base}, nil
}

func (provider Provider) APIKey(getenv func(string) string) (string, error) {
	if provider.APIKeyEnvironment == "" {
		return "", nil
	}
	key := getenv(APIKeyEnvironment)
	if key == "" {
		key = getenv(provider.APIKeyEnvironment)
	}
	if key == "" {
		return "", fmt.Errorf("%s or %s must be set", APIKeyEnvironment, provider.APIKeyEnvironment)
	}
	return key, nil
}

func Default() []Provider {
	return []Provider{
		{
			Name:    "ollama",
			BaseURL: ollama.BaseURL,
			NewClient: func(_, baseURL string, maxAttempts int, _ func(string) string) (Client, error) {
				return ollama.NewClient(ollama.Config{BaseURL: baseURL, MaxAttempts: &maxAttempts})
			},
		},
		{
			Name:              "openai",
			BaseURL:           "https://api.openai.com/v1",
			DefaultModel:      "gpt-6-astra",
			APIKeyEnvironment: "OPENAI_API_KEY",
			NewClient: func(apiKey, baseURL string, maxAttempts int, _ func(string) string) (Client, error) {
				return openai.NewClient(openai.Config{APIKey: apiKey, BaseURL: baseURL, MaxAttempts: &maxAttempts})
			},
		},
		{
			Name:         "openai-codex",
			BaseURL:      openaicodex.BaseURL,
			DefaultModel: "gpt-6-astra",
			NewClient: func(_, baseURL string, maxAttempts int, getenv func(string) string) (Client, error) {
				config, err := openaicodex.EnvironmentConfig(getenv)
				if err != nil {
					return nil, err
				}
				config.BaseURL, config.MaxAttempts = baseURL, &maxAttempts
				return openaicodex.NewClient(config)
			},
		},
		{
			Name:              "openrouter",
			BaseURL:           "https://openrouter.ai/api/v1",
			APIKeyEnvironment: "OPENROUTER_API_KEY",
			NewClient: func(apiKey, baseURL string, maxAttempts int, _ func(string) string) (Client, error) {
				return openrouter.NewClient(openrouter.Config{APIKey: apiKey, BaseURL: baseURL, MaxAttempts: &maxAttempts})
			},
		},
		{
			Name:              "fireworks",
			BaseURL:           "https://api.fireworks.ai/inference/v1",
			APIKeyEnvironment: "FIREWORKS_API_KEY",
			NewClient: func(apiKey, baseURL string, maxAttempts int, _ func(string) string) (Client, error) {
				return fireworks.NewClient(fireworks.Config{APIKey: apiKey, BaseURL: baseURL, MaxAttempts: &maxAttempts})
			},
		},
		{
			Name:              "anthropic",
			BaseURL:           anthropic.DefaultBaseURL,
			DefaultModel:      "claude-opus-5-5",
			APIKeyEnvironment: "ANTHROPIC_API_KEY",
			NewClient: func(apiKey, baseURL string, maxAttempts int, getenv func(string) string) (Client, error) {
				cacheTTL := messagesapi.CacheTTL(getenv("ANTHROPIC_CACHE_TTL"))
				if !cacheTTL.Valid() {
					return nil, fmt.Errorf("ANTHROPIC_CACHE_TTL must be 5m or 1h, got %q", cacheTTL)
				}
				var fallback *bool
				if value := getenv("ANTHROPIC_FALLBACK"); value != "" {
					enabled, err := strconv.ParseBool(value)
					if err != nil {
						return nil, fmt.Errorf("parse ANTHROPIC_FALLBACK: %w", err)
					}
					fallback = &enabled
				}
				return anthropic.NewClient(anthropic.Config{
					APIKey: apiKey, BaseURL: baseURL, MaxAttempts: &maxAttempts,
					Fallback: fallback, CacheTTL: cacheTTL,
				})
			},
		},
	}
}

func Find(available []Provider, name string) (Provider, error) {
	for _, provider := range available {
		if provider.Name == name {
			if provider.NewClient == nil {
				return Provider{}, fmt.Errorf("provider %q has no client factory", name)
			}
			return provider, nil
		}
	}
	names := make([]string, 0, len(available))
	for _, provider := range available {
		names = append(names, provider.Name)
	}
	return Provider{}, fmt.Errorf("unsupported provider %q; available providers: %s", name, strings.Join(names, ", "))
}
