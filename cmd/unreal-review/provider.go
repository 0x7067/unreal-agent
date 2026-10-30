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

// reviewAPIKey reads the selected registry entry's own credential.
// APIKeyEnvironment is empty for ollama and openai-codex. The generic
// UNREAL_HARNESS_LLM_API_KEY is never read. A missing key is "set <VAR>",
// which is "set OPENROUTER_API_KEY" for the default provider.
func reviewAPIKey(selected providers.Provider) (string, error) {
	env := selected.APIKeyEnvironment
	if env == "" {
		return "", nil
	}
	if key := strings.TrimSpace(reviewGetenv(env)); key != "" {
		return key, nil
	}
	return "", fmt.Errorf("set %s", env)
}
