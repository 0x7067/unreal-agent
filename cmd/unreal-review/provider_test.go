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

func TestReviewSelectUsesRunnerCredentials(t *testing.T) {
	secrets[providers.ProviderEnvironment] = "openrouter"
	secrets[providers.APIKeyEnvironment] = "generic-key"
	secrets["OPENROUTER_API_KEY"] = "openrouter-key"
	secrets[providers.BaseURLEnvironment] = "http://127.0.0.1:9/api/v1"
	secrets["OPENAI_CODEX_AUTH_FILE"] = "/tmp/codex-auth.json"
	t.Cleanup(func() {
		for _, name := range []string{
			providers.ProviderEnvironment, providers.APIKeyEnvironment, "OPENROUTER_API_KEY",
			providers.BaseURLEnvironment, "OPENAI_CODEX_AUTH_FILE",
		} {
			delete(secrets, name)
		}
	})

	sel, err := providers.Select(providers.Default(), reviewGetenv)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Provider.Name != "openrouter" || sel.APIKey != "generic-key" || sel.BaseURL != "http://127.0.0.1:9/api/v1" {
		t.Fatalf("selection = %+v", sel)
	}
	if got := reviewGetenv("OPENAI_CODEX_AUTH_FILE"); got != "/tmp/codex-auth.json" {
		t.Fatalf("auth file = %q", got)
	}
	if got := reviewGetenv("CODEX_HOME"); got != os.Getenv("CODEX_HOME") {
		t.Fatalf("CODEX_HOME = %q", got)
	}

	delete(secrets, providers.APIKeyEnvironment)
	sel, err = providers.Select(providers.Default(), reviewGetenv)
	if err != nil {
		t.Fatal(err)
	}
	if sel.APIKey != "openrouter-key" {
		t.Fatalf("vendor key = %q", sel.APIKey)
	}
}

func TestReviewSelectNamesTheMissingVariable(t *testing.T) {
	t.Setenv(providers.ProviderEnvironment, "openai")
	t.Setenv(providers.APIKeyEnvironment, "")
	t.Setenv("OPENAI_API_KEY", "")
	delete(secrets, providers.APIKeyEnvironment)
	delete(secrets, "OPENAI_API_KEY")
	_, err := providers.Select(providers.Default(), reviewGetenv)
	if err == nil || err.Error() != "UNREAL_HARNESS_LLM_API_KEY or OPENAI_API_KEY must be set" {
		t.Fatalf("openai: %v", err)
	}

	t.Setenv(providers.ProviderEnvironment, "openrouter")
	t.Setenv("OPENROUTER_API_KEY", "")
	delete(secrets, "OPENROUTER_API_KEY")
	_, err = providers.Select(providers.Default(), reviewGetenv)
	if err == nil || err.Error() != "UNREAL_HARNESS_LLM_API_KEY or OPENROUTER_API_KEY must be set" {
		t.Fatalf("openrouter: %v", err)
	}
}

func TestUnknownHarnessProviderListsNames(t *testing.T) {
	t.Setenv(providers.ProviderEnvironment, "nope")
	_, err := providers.Select(providers.Default(), reviewGetenv)
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
	t.Setenv(providers.ProviderEnvironment, "ollama")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_OPTIONAL_LOCKS", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

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
	got := formatRunCost("openrouter", findings.Cost{Currency: "USD"})
	if got != "cost: USD 0.000000 (0 input, 0 output, 0 requests)" {
		t.Fatalf("openrouter cost = %q", got)
	}
}
