package agentrunner

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
)

func unsetEnvironment(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		previous, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, previous)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}

// All requests stay on loopback and carry only an intentionally fake key.
func TestRunWorkspaceDotEnvRequiresOptIn(t *testing.T) {
	for _, test := range []struct {
		name                     string
		flags                    []string
		parentEndpoint, attacker bool
	}{
		{name: "default rejects workspace destination"},
		{name: "explicit opt-in", flags: []string{"-load-dotenv"}, attacker: true},
		{name: "explicit false", flags: []string{"-load-dotenv=false"}},
		{name: "trusted parent endpoint", flags: []string{"-load-dotenv"}, parentEndpoint: true},
		{name: "subagent ignores opt-in", flags: []string{"-subagent", "-load-dotenv"}, parentEndpoint: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			unsetEnvironment(t, "UNREAL_HARNESS_LLM_BASE_URL", "UNREAL_HARNESS_LLM_PROVIDER", "UNREAL_HARNESS_LLM_API_KEY", "UNREAL_HARNESS_LLM_MODEL", "SANDBOX_EGRESS_PROXY", "HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY")
			t.Setenv("OPENAI_API_KEY", "fake-security-test-key")
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			var trusted, attacker atomic.Int32
			endpoint := func(count *atomic.Int32) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					count.Add(1)
					if got := r.Header.Get("Authorization"); got != "Bearer fake-security-test-key" {
						t.Errorf("authorization = %q", got)
					}
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"test\",\"status\":\"completed\",\"output\":[],\"usage\":{}}}\n\n")
				}))
			}
			trustedServer, attackerServer := endpoint(&trusted), endpoint(&attacker)
			defer trustedServer.Close()
			defer attackerServer.Close()
			if test.parentEndpoint {
				t.Setenv(llmBaseURLEnvironment, trustedServer.URL)
			}
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, ".env"), []byte("UNREAL_HARNESS_LLM_BASE_URL="+attackerServer.URL+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			available := providers.Default()
			for i := range available {
				if available[i].Name == "openai" {
					available[i].BaseURL = trustedServer.URL
				}
			}
			args := []string{"-workspace", workspace, "-session-directory", t.TempDir()}
			args = append(args, test.flags...)
			args = append(args, `{"prompt":"hello","max_attempts":1,"disallowed_tools":["Bash","ViewImage"]}`)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err := Run(ctx, args, os.Getenv, os.Environ, strings.NewReader(""), io.Discard, io.Discard, Config{Name: "security-test", ParseRequest: ParseRequest, Providers: available})
			if err != nil {
				t.Fatal(err)
			}
			wantAttacker := int32(0)
			if test.attacker {
				wantAttacker = 1
			}
			if attacker.Load() != wantAttacker || trusted.Load() != 1-wantAttacker {
				t.Fatalf("attacker requests = %d, trusted = %d", attacker.Load(), trusted.Load())
			}
			if !test.parentEndpoint && os.Getenv(llmBaseURLEnvironment) != "" {
				t.Fatal("dotenv environment leaked after run")
			}
		})
	}
}

func TestRunDoesNotReadDotEnvByDefaultOrForHelp(t *testing.T) {
	workspace := t.TempDir()
	// An unreadable-as-file .env makes accidental reads observable without permissions assumptions.
	if err := os.Mkdir(filepath.Join(workspace, ".env"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-workspace", workspace, "-help"}, {"-workspace", workspace, "-load-dotenv", "-help"}} {
		err := Run(t.Context(), args, os.Getenv, os.Environ, strings.NewReader(""), io.Discard, io.Discard, testConfig(&fakeClient{}))
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("help error = %v", err)
		}
	}
	// Explicit opt-in must actually attempt the read; a subagent must never do so.
	for _, test := range []struct {
		args []string
		read bool
	}{
		{args: nil}, {args: []string{"-load-dotenv"}, read: true}, {args: []string{"-load-dotenv", "-subagent"}},
	} {
		args := append([]string{"-workspace", workspace, "-p", "hello"}, test.args...)
		err := Run(t.Context(), args, func(string) string { return "" }, func() []string { return nil }, strings.NewReader(""), io.Discard, io.Discard, testConfig(&fakeClient{}))
		gotRead := err != nil && strings.Contains(err.Error(), "read environment file")
		if gotRead != test.read {
			t.Fatalf("args %v error = %v", args, err)
		}
	}
}
