package providers_test

import (
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
)

func TestAPIKey(t *testing.T) {
	for _, test := range []struct {
		name, variable, generic, specific, want string
		wantError                               bool
	}{
		{name: "provider key", variable: "CUSTOM_CREDENTIAL", specific: "provider-secret", want: "provider-secret"},
		{name: "generic key takes precedence", variable: "CUSTOM_CREDENTIAL", generic: "generic-secret", specific: "provider-secret", want: "generic-secret"},
		{name: "whitespace generic key takes precedence", variable: "CUSTOM_CREDENTIAL", generic: " \t ", specific: "provider-secret", want: " \t "},
		{name: "generic key is passed through", variable: "CUSTOM_CREDENTIAL", generic: " secret ", want: " secret "},
		{name: "provider key is passed through", variable: "CUSTOM_CREDENTIAL", specific: " secret ", want: " secret "},
		{name: "delegated authentication ignores API keys", generic: "generic-secret", specific: "provider-secret"},
		{name: "missing key", variable: "CUSTOM_CREDENTIAL", wantError: true},
		{name: "whitespace provider key is passed through", variable: "CUSTOM_CREDENTIAL", specific: " \t ", want: " \t "},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := providers.Provider{APIKeyEnvironment: test.variable}
			env := map[string]string{providers.APIKeyEnvironment: test.generic, "CUSTOM_CREDENTIAL": test.specific}
			key, err := provider.APIKey(func(name string) string { return env[name] })
			if test.wantError {
				if err == nil || key != "" || !strings.Contains(err.Error(), providers.APIKeyEnvironment) || !strings.Contains(err.Error(), test.variable) {
					t.Fatalf("APIKey = %q, %v; want an error naming both credential variables", key, err)
				}
			} else if err != nil || key != test.want {
				t.Fatalf("APIKey = %q, %v; want %q", key, err, test.want)
			}
		})
	}
}

func TestDefaultProvidersAcceptWhitespaceAPIKeys(t *testing.T) {
	for _, provider := range providers.Default() {
		if provider.APIKeyEnvironment == "" {
			continue
		}
		t.Run(provider.Name, func(t *testing.T) {
			client, err := provider.NewClient(" \t ", provider.BaseURL, 1, func(string) string { return "" })
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
