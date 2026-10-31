package main

import (
	"os"
	"strings"
)

// reviewGetenv reads an isolated secret first, then the process environment.
// isolateSecrets removes provider credentials before reexec, so os.Getenv
// alone cannot see them. The value is what providers.Select and the client
// factory read, including UNREAL_HARNESS_LLM_API_KEY and OPENAI_CODEX_AUTH_FILE.
func reviewGetenv(name string) string {
	if v := strings.TrimSpace(secret(name)); v != "" {
		return v
	}
	return os.Getenv(name)
}
