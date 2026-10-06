package providers_test

import (
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
)

func TestSelectUsesRunnerEnvironment(t *testing.T) {
	env := map[string]string{}
	getenv := func(name string) string { return env[name] }
	_, err := providers.Select(providers.Default(), getenv)
	if err == nil || err.Error() != "UNREAL_HARNESS_LLM_API_KEY or OPENAI_API_KEY must be set" {
		t.Fatalf("default openai: %v", err)
	}

	env[providers.APIKeyEnvironment] = "generic"
	env["OPENAI_API_KEY"] = "vendor"
	sel, err := providers.Select(providers.Default(), getenv)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Provider.Name != "openai" || sel.APIKey != "generic" || sel.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("selection = %+v", sel)
	}

	delete(env, providers.APIKeyEnvironment)
	sel, err = providers.Select(providers.Default(), getenv)
	if err != nil {
		t.Fatal(err)
	}
	if sel.APIKey != "vendor" {
		t.Fatalf("vendor key = %q", sel.APIKey)
	}

	env[providers.BaseURLEnvironment] = " http://127.0.0.1:9/v1 "
	sel, err = providers.Select(providers.Default(), getenv)
	if err != nil {
		t.Fatal(err)
	}
	if sel.BaseURL != "http://127.0.0.1:9/v1" {
		t.Fatalf("base = %q", sel.BaseURL)
	}

	env[providers.ProviderEnvironment] = " openai-codex "
	sel, err = providers.Select(providers.Default(), getenv)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Provider.Name != "openai-codex" || sel.APIKey != "" {
		t.Fatalf("codex = %+v", sel)
	}

	env[providers.ProviderEnvironment] = "nope"
	_, err = providers.Select(providers.Default(), getenv)
	if err == nil || !strings.Contains(err.Error(), `"nope"`) || !strings.Contains(err.Error(), "openrouter") {
		t.Fatalf("unknown: %v", err)
	}
}
func TestSelectUsesSuppliedProviders(t *testing.T) {
	custom, err := providers.Find(providers.Default(), "ollama")
	if err != nil {
		t.Fatal(err)
	}
	custom.Name = "local"
	custom.BaseURL = "http://127.0.0.1:1234/v1"
	selected, err := providers.Select([]providers.Provider{custom}, func(name string) string {
		if name == providers.ProviderEnvironment {
			return " local "
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider.Name != custom.Name || selected.BaseURL != custom.BaseURL || selected.APIKey != "" {
		t.Fatalf("selection = %+v", selected)
	}
	if _, err := providers.Select([]providers.Provider{custom}, nil); err == nil {
		t.Fatal("nil environment must select the default provider, not the first supplied provider")
	}
}

func TestSelectNilEnvironment(t *testing.T) {
	_, err := providers.Select(providers.Default(), nil)
	if err == nil || err.Error() != "UNREAL_HARNESS_LLM_API_KEY or OPENAI_API_KEY must be set" {
		t.Fatalf("nil environment: %v", err)
	}
}
