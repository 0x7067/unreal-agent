package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/review/findings"
)

func TestReviewAPIKeyUsesOnlyTheVendorCredential(t *testing.T) {
	secrets["UNREAL_HARNESS_LLM_API_KEY"] = "generic-key"
	secrets["OPENAI_API_KEY"] = "openai-key"
	secrets["OPENROUTER_API_KEY"] = "openrouter-key"
	secrets["FIREWORKS_API_KEY"] = "fireworks-key"
	secrets["ANTHROPIC_API_KEY"] = "anthropic-key"
	t.Cleanup(func() {
		for _, name := range []string{
			"UNREAL_HARNESS_LLM_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY",
			"FIREWORKS_API_KEY", "ANTHROPIC_API_KEY",
		} {
			delete(secrets, name)
		}
	})

	want := map[string]string{
		"openai":       "openai-key",
		"openrouter":   "openrouter-key",
		"fireworks":    "fireworks-key",
		"anthropic":    "anthropic-key",
		"ollama":       "",
		"openai-codex": "",
	}
	for _, provider := range providers.Default() {
		key, err := reviewAPIKey(provider)
		if err != nil {
			t.Fatalf("%s: %v", provider.Name, err)
		}
		if key != want[provider.Name] {
			t.Fatalf("%s key = %q, want %q", provider.Name, key, want[provider.Name])
		}
		if key == "generic-key" {
			t.Fatalf("%s sent the generic harness key", provider.Name)
		}
	}
	if got := reviewClientGetenv(providers.APIKeyEnvironment); got != "" {
		t.Fatalf("client getenv returned the generic key %q", got)
	}
	secrets["OPENAI_CODEX_AUTH_FILE"] = "/tmp/codex-auth.json"
	secrets["CODEX_HOME"] = "/tmp/codex-home"
	t.Cleanup(func() {
		delete(secrets, "OPENAI_CODEX_AUTH_FILE")
		delete(secrets, "CODEX_HOME")
	})
	if got := reviewClientGetenv("OPENAI_CODEX_AUTH_FILE"); got != "" {
		t.Fatalf("client getenv returned the auth file %q", got)
	}
	if got := reviewClientGetenv("CODEX_HOME"); got != "" {
		t.Fatalf("client getenv returned CODEX_HOME %q", got)
	}
}

func TestReviewAPIKeyNamesTheMissingVariable(t *testing.T) {
	delete(secrets, "OPENAI_API_KEY")
	delete(secrets, "OPENROUTER_API_KEY")
	secrets["UNREAL_HARNESS_LLM_API_KEY"] = "generic-key"
	t.Cleanup(func() { delete(secrets, "UNREAL_HARNESS_LLM_API_KEY") })
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	openai, err := providers.Find(providers.Default(), "openai")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reviewAPIKey(openai); err == nil || err.Error() != "set OPENAI_API_KEY" {
		t.Fatalf("openai: %v", err)
	}
	openrouter, err := providers.Find(providers.Default(), "openrouter")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reviewAPIKey(openrouter); err == nil || err.Error() != "set OPENROUTER_API_KEY" {
		t.Fatalf("openrouter: %v", err)
	}
}

func TestUnknownReviewProviderListsNames(t *testing.T) {
	t.Setenv("UNREAL_REVIEW_PROVIDER", "nope")
	t.Setenv("UNREAL_HARNESS_LLM_PROVIDER", "openai")
	_, err := reviewProvider()
	if err == nil {
		t.Fatal("unknown provider succeeded")
	}
	for _, provider := range providers.Default() {
		if !strings.Contains(err.Error(), provider.Name) {
			t.Fatalf("error %q does not list %s", err, provider.Name)
		}
	}
	if !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("error %q does not name the bad value", err)
	}
}

func TestEvalIsUnknownCommand(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	cmdErr := run([]string{"eval"})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = old
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	if cmdErr == nil || cmdErr.Error() != `unknown command "eval"` {
		t.Fatalf("eval = %v", cmdErr)
	}
}

func TestNonOpenRouterRunPrintsUnknownCost(t *testing.T) {
	t.Setenv("UNREAL_REVIEW_PROVIDER", "ollama")
	t.Setenv("UNREAL_HARNESS_LLM_PROVIDER", "openai")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "generic-key")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_OPTIONAL_LOCKS", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	secrets["UNREAL_HARNESS_LLM_API_KEY"] = "generic-key"
	t.Cleanup(func() { delete(secrets, "UNREAL_HARNESS_LLM_API_KEY") })

	dir := gitRepo(t)
	writeRepoFile(t, dir, "note.txt", "same\n")
	gitRun(t, dir, "add", "note.txt")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	out := filepath.Join(t.TempDir(), "findings.jsonl")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	cmdErr := cmdRun([]string{"--model", "test-model", "--workspace", dir, "--from", "HEAD", "--to", "HEAD", "--out", out})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = old
	stderr, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if cmdErr != nil {
		t.Fatal(cmdErr)
	}
	if !strings.Contains(string(stderr), "cost: unknown (provider ollama does not report cost)") {
		t.Fatalf("stderr:\n%s", stderr)
	}
	if strings.Contains(string(stderr), "USD 0.000000") {
		t.Fatalf("ollama printed a dollar cost:\n%s", stderr)
	}
	report, err := findings.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if report.Run == nil || report.Run.Cost.AmountUSD != 0 {
		t.Fatalf("cost = %+v", report.Run)
	}
}

func TestOpenRouterCostLineStaysADollarAmount(t *testing.T) {
	got := formatRunCost(defaultReviewProvider, findings.Cost{Currency: "USD"})
	if got != "cost: USD 0.000000 (0 input, 0 output, 0 requests)" {
		t.Fatalf("openrouter cost = %q", got)
	}
}
