package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/review/findings"
)

func TestMain(m *testing.M) {
	// Loopback stubs assume OpenRouter. A developer shell that exported the
	// runner selector, generic key, or base URL must not redirect them.
	for _, name := range []string{
		providers.ProviderEnvironment,
		providers.APIKeyEnvironment,
		providers.BaseURLEnvironment,
		"UNREAL_REVIEW_OPENROUTER_API",
	} {
		if err := os.Unsetenv(name); err != nil {
			panic(err)
		}
	}
	if err := os.Setenv(providers.ProviderEnvironment, "openrouter"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestRunBlankOpenRouterKeyIsRejected(t *testing.T) {
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

	t.Setenv(providers.APIKeyEnvironment, "")
	t.Setenv("OPENROUTER_API_KEY", "")
	secrets["OPENROUTER_API_KEY"] = "   "
	t.Cleanup(func() { delete(secrets, "OPENROUTER_API_KEY") })
	out := filepath.Join(t.TempDir(), "findings.jsonl")
	err := cmdRun([]string{"--model", "test-model", "--workspace", dir, "--out", out})
	if err == nil || err.Error() != "UNREAL_HARNESS_LLM_API_KEY or OPENROUTER_API_KEY must be set" {
		t.Fatalf("blank key: %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("blank key wrote %s: %v", out, statErr)
	}

	secrets["OPENROUTER_API_KEY"] = "test-key"
	if err := cmdRun([]string{"--model", "test-model", "--workspace", dir, "--out", out}); err != nil {
		t.Fatal(err)
	}
	report, err := findings.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary != "No material issues: the selected range has no changes." {
		t.Fatalf("summary=%q", report.Summary)
	}
}

func TestRunOpenRouterPrefersGenericHarnessKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_OPTIONAL_LOCKS", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(name, "")
	}
	secrets[providers.APIKeyEnvironment] = "generic-key"
	secrets["OPENROUTER_API_KEY"] = "openrouter-key"
	t.Cleanup(func() {
		delete(secrets, providers.APIKeyEnvironment)
		delete(secrets, "OPENROUTER_API_KEY")
	})

	dir := gitRepo(t)
	writeRepoFile(t, dir, "note.txt", "before\n")
	gitRun(t, dir, "add", "note.txt")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	base := gitOutput(t, dir, "rev-parse", "HEAD")
	writeRepoFile(t, dir, "note.txt", "after\n")
	gitRun(t, dir, "add", "note.txt")
	gitRun(t, dir, "commit", "-q", "-m", "change")
	head := gitOutput(t, dir, "rev-parse", "HEAD")

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		cleanReviewHandler("No material issues: the assigned changes are safe.").ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv("UNREAL_REVIEW_OPENROUTER_API", server.URL)

	out := filepath.Join(t.TempDir(), "findings.jsonl")
	if err := cmdRun([]string{"--model", "test-model", "--workspace", dir, "--from", base, "--to", head, "--out", out}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer generic-key" {
		t.Fatalf("authorization = %q", gotAuth)
	}

	delete(secrets, providers.APIKeyEnvironment)
	t.Setenv(providers.APIKeyEnvironment, "")
	gotAuth = ""
	out = filepath.Join(t.TempDir(), "findings.jsonl")
	if err := cmdRun([]string{"--model", "test-model", "--workspace", dir, "--from", base, "--to", head, "--out", out}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer openrouter-key" {
		t.Fatalf("vendor authorization = %q", gotAuth)
	}
}

func TestRunDefaultProviderIsOpenAI(t *testing.T) {
	t.Setenv(providers.ProviderEnvironment, "")
	t.Setenv(providers.APIKeyEnvironment, "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	err := cmdRun([]string{"--model", "test-model", "--workspace", t.TempDir(), "--out", filepath.Join(t.TempDir(), "findings.jsonl")})
	if err == nil || err.Error() != "UNREAL_HARNESS_LLM_API_KEY or OPENAI_API_KEY must be set" {
		t.Fatalf("default provider: %v", err)
	}
}

func TestRunOpenRouterOverrideRequiresOpenRouter(t *testing.T) {
	t.Setenv(providers.ProviderEnvironment, "ollama")
	t.Setenv("UNREAL_REVIEW_OPENROUTER_API", "http://127.0.0.1:9/api/v1")
	out := filepath.Join(t.TempDir(), "findings.jsonl")
	err := cmdRun([]string{"--model", "test-model", "--workspace", t.TempDir(), "--out", out})
	if err == nil || err.Error() != "UNREAL_REVIEW_OPENROUTER_API is set but the selected provider is ollama, not openrouter" {
		t.Fatalf("override: %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("override wrote %s: %v", out, statErr)
	}
}

func TestRunAgentLog(t *testing.T) {
	secrets["OPENROUTER_API_KEY"] = "test-key"
	t.Cleanup(func() { delete(secrets, "OPENROUTER_API_KEY") })
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_OPTIONAL_LOCKS", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")

	t.Run("bogus strategy", func(t *testing.T) {
		logPath, original := agentLogFixture(t)
		err := runReview(t, logPath, "--strategy", "bogons", "--workspace", t.TempDir())
		if err == nil {
			t.Fatal("bogus strategy exited 0")
		}
		assertAgentLog(t, logPath, original)
	})

	t.Run("bad range", func(t *testing.T) {
		logPath, original := agentLogFixture(t)
		err := runReview(t, logPath, "--strategy", "single", "--workspace", gitRepo(t), "--from", "not-a-rev")
		if err == nil {
			t.Fatal("bad range exited 0")
		}
		assertAgentLog(t, logPath, original)
	})

	t.Run("empty selection", func(t *testing.T) {
		logPath, original := agentLogFixture(t)
		dir := gitRepo(t)
		writeRepoFile(t, dir, "note.txt", "same\n")
		gitRun(t, dir, "add", "note.txt")
		gitRun(t, dir, "commit", "-q", "-m", "base")
		out, err := runReviewOut(t, logPath, "--strategy", "single", "--workspace", dir)
		if err != nil {
			t.Fatal(err)
		}
		report, err := findings.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if report.Summary != "No material issues: the selected range has no changes." {
			t.Fatalf("summary=%q", report.Summary)
		}
		assertAgentLog(t, logPath, original)
	})

	t.Run("writes session", func(t *testing.T) {
		const summary = "No material issues: the assigned changes are safe."
		logPath, original := agentLogFixture(t)
		dir := gitRepo(t)
		writeRepoFile(t, dir, "note.txt", "before\n")
		gitRun(t, dir, "add", "note.txt")
		gitRun(t, dir, "commit", "-q", "-m", "base")
		base := gitOutput(t, dir, "rev-parse", "HEAD")
		writeRepoFile(t, dir, "note.txt", "after\n")
		gitRun(t, dir, "add", "note.txt")
		gitRun(t, dir, "commit", "-q", "-m", "change")
		head := gitOutput(t, dir, "rev-parse", "HEAD")

		server := httptest.NewServer(cleanReviewHandler(summary))
		t.Cleanup(server.Close)
		t.Setenv("UNREAL_REVIEW_OPENROUTER_API", server.URL)
		for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "ALL_PROXY", "all_proxy"} {
			t.Setenv(name, "")
		}

		out, err := runReviewOut(t, logPath, "--strategy", "single", "--workspace", dir, "--from", base, "--to", head)
		if err != nil {
			t.Fatal(err)
		}
		report, err := findings.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if report.Summary != summary {
			t.Fatalf("summary=%q", report.Summary)
		}
		got, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) == original || !strings.Contains(string(got), summary) {
			t.Fatalf("agent log = %q", got)
		}
	})
}

func runReview(t *testing.T, logPath string, args ...string) error {
	t.Helper()
	_, err := runReviewOut(t, logPath, args...)
	return err
}

func runReviewOut(t *testing.T, logPath string, args ...string) (string, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "findings.jsonl")
	err := run(append([]string{
		"run",
		"--model", "test-model",
		"--thinking-level", "low",
		"--agent-log", logPath,
		"--timeout", "45s",
		"--out", out,
	}, args...))
	return out, err
}

func agentLogFixture(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.jsonl")
	const original = "previous session log\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, original
}

func assertAgentLog(t *testing.T, path, original string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("agent log = %q", got)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "t@t")
	gitRun(t, dir, "config", "user.name", "t")
	return dir
}

func writeRepoFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitOutput(t, dir, args...)
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func cleanReviewHandler(summary string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := map[string]any{
			"id":     "clean-response",
			"object": "response",
			"status": "completed",
			"output": []map[string]any{{
				"id":     "clean-message",
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"phase":  "final_answer",
				"content": []map[string]any{{
					"type": "output_text",
					"text": summary,
				}},
			}},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "cost": 0.001},
		}
		encoded, err := json.Marshal(map[string]any{"type": "response.completed", "response": response})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
			return
		}
	})
}
