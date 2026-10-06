package providers

import "strings"

const (
	ProviderEnvironment = "UNREAL_HARNESS_LLM_PROVIDER"
	BaseURLEnvironment  = "UNREAL_HARNESS_LLM_BASE_URL"
	DefaultProviderName = "openai"
)

// Selection is the provider, credential, and base URL resolved from the
// process environment for reviewcli.
type Selection struct {
	Provider Provider
	APIKey   string
	BaseURL  string
}

// Select resolves a provider using the runner-compatible environment. An empty
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
