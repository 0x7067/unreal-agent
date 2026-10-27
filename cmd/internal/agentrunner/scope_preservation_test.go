package agentrunner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

// This proves prompt delivery and a no-delegation execution path, not live-model
// judgment about whether to test or delegate. Those behavioral evaluations stay
// distinct from deterministic harness integration.
func TestSupervisionPreservesScopeAndVerificationPolicy(t *testing.T) {
	for _, scenario := range []struct{ name, request, policy string }{
		{"tag only", "Tag v1.2.4 on HEAD; no push.", "Tagging metadata alone requires no build."},
		{"code change", "Fix the parser and tag v1.2.4.", "After code changes, targeted tests and rebuild are required."},
		{"required release gate", "Tag v1.2.4 on HEAD.", "Release policy requires successful test/check/build before any release tag."},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			workspace := t.TempDir()
			observed := false
			client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
				var system string
				for _, item := range request.Input {
					if message, ok := item.Data.(llm.Message); ok && message.Role == llm.RoleSystem {
						system += message.Text
					}
				}
				for _, guidance := range []string{
					"Choose the smallest operation that satisfies the user's request.",
					"tagging a release does not by itself require a full test/check/build pipeline",
					"Preserve necessary verification for code changes and required repository validation",
					scenario.policy,
				} {
					if !strings.Contains(system, guidance) {
						t.Errorf("missing guidance %q", guidance)
					}
				}
				observed = true
				return messageResponse("scope inspected; fixture performs no work"), nil
			}}
			config := testConfig(client)
			config.SubagentCommand = []string{filepath.Join(workspace, "must-not-start")}
			var out, stderr strings.Builder
			code := RunMain(t.Context(), []string{"-workspace", workspace, "-session-directory", t.TempDir()}, func(name string) string {
				if name == "OPENAI_API_KEY" {
					return "secret"
				}
				if name == "XDG_CONFIG_HOME" {
					return workspace
				}
				return ""
			}, os.Environ, strings.NewReader(`{"prompt":"`+scenario.request+`","model":"gpt-test","system_prompt":"`+scenario.policy+`","extra_allowed_tools":["Agent"]}`), &out, &stderr, config)
			if code != 0 || !observed {
				t.Fatalf("exit=%d observed=%v stderr=%s", code, observed, stderr.String())
			}
		})
	}
}
