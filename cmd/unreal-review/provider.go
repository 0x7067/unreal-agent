package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
)

const (
	defaultReviewProvider = "openrouter"
	reviewProviderEnv     = "UNREAL_REVIEW_PROVIDER"
)

// reviewCredentialEnv is the only credential each provider may send.
// An empty value means the provider takes no API key (ollama, and Codex,
// whose auth is the token or auth file read by its client). The generic
// UNREAL_HARNESS_LLM_API_KEY is intentionally absent.
var reviewCredentialEnv = map[string]string{
	"ollama":       "",
	"openai":       "OPENAI_API_KEY",
	"openai-codex": "",
	"openrouter":   "OPENROUTER_API_KEY",
	"fireworks":    "FIREWORKS_API_KEY",
	"anthropic":    "ANTHROPIC_API_KEY",
}

// reviewGetenv reads an isolated secret first, then the process environment.
// isolateSecrets removes provider credentials before reexec, so os.Getenv
// alone cannot see them.
func reviewGetenv(name string) string {
	if v := strings.TrimSpace(secret(name)); v != "" {
		return v
	}
	return os.Getenv(name)
}

// reviewClientGetenv is the getenv passed to provider clients. The generic
// harness key is blank, so a client cannot attach it to a request.
func reviewClientGetenv(name string) string {
	if name == providers.APIKeyEnvironment {
		return ""
	}
	return reviewGetenv(name)
}

// reviewProvider selects the registry entry from UNREAL_REVIEW_PROVIDER.
// An empty value keeps OpenRouter. UNREAL_HARNESS_LLM_PROVIDER is not read.
func reviewProvider() (providers.Provider, error) {
	name := strings.TrimSpace(reviewGetenv(reviewProviderEnv))
	if name == "" {
		name = defaultReviewProvider
	}
	return providers.Find(providers.Default(), name)
}

// reviewAPIKey resolves the selected provider's own credential. OpenRouter
// keeps the historical "set OPENROUTER_API_KEY" error. Other missing keys
// name their own variable.
func reviewAPIKey(selected providers.Provider) (string, error) {
	env, ok := reviewCredentialEnv[selected.Name]
	if !ok {
		return "", fmt.Errorf("provider %q has no review credential mapping", selected.Name)
	}
	if env == "" {
		return "", nil
	}
	if key := strings.TrimSpace(reviewGetenv(env)); key != "" {
		return key, nil
	}
	if env == "OPENROUTER_API_KEY" {
		return "", ErrAPIKey
	}
	return "", fmt.Errorf("set %s", env)
}
