package main

import (
	"os"
	"strings"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
)

const defaultReviewProvider = "openrouter"

// reviewGetenv reads an isolated secret first, then the process environment.
// isolateSecrets removes provider credentials before reexec, so os.Getenv
// alone cannot see them.
func reviewGetenv(name string) string {
	if v := strings.TrimSpace(secret(name)); v != "" {
		return v
	}
	return os.Getenv(name)
}

// reviewProvider selects the registry entry. An empty
// UNREAL_HARNESS_LLM_PROVIDER keeps OpenRouter, which is the review default.
func reviewProvider() (providers.Provider, error) {
	name := strings.TrimSpace(reviewGetenv("UNREAL_HARNESS_LLM_PROVIDER"))
	if name == "" {
		name = defaultReviewProvider
	}
	return providers.Find(providers.Default(), name)
}

// reviewAPIKey resolves the selected provider's credential. The default
// OpenRouter path keeps the historical "set OPENROUTER_API_KEY" error when
// neither that variable nor UNREAL_HARNESS_LLM_API_KEY is set.
func reviewAPIKey(selected providers.Provider) (string, error) {
	key, err := selected.APIKey(func(name string) string {
		return strings.TrimSpace(reviewGetenv(name))
	})
	if err != nil && selected.Name == defaultReviewProvider {
		return "", ErrAPIKey
	}
	return key, err
}
