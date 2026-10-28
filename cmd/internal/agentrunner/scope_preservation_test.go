package agentrunner

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// This proves prompt delivery and a no-delegation execution path, not live-model
// judgment about whether to test or delegate. Those behavioral evaluations stay
// distinct from deterministic harness integration.
func TestSupervisionPreservesRepositoryPolicyWithConditionalCoordination(t *testing.T) {
	for _, scenario := range []struct{ name, request, policy string }{
		{"tag only", "Tag v1.2.4 on HEAD; no push.", "Tagging metadata alone requires no build."},
		{"code change", "Fix the parser and tag v1.2.4.", "After code changes, targeted tests and rebuild are required."},
		{"required release gate", "Tag v1.2.4 on HEAD.", "Release policy requires successful test/check/build before any release tag."},
	} {
		for _, capabilities := range []struct {
			name          string
			extra, denied []string
			enabled       []string
		}{
			{name: "graph only", enabled: []string{tool.TaskGraphName}},
			{name: "all coordination", extra: []string{tool.AgentName}, enabled: []string{tool.TaskGraphName, tool.AgentName, tool.AgentControlName}},
			{name: "baseline", denied: []string{tool.TaskGraphName}},
			{name: "helpers only", extra: []string{tool.AgentName}, denied: []string{tool.TaskGraphName}, enabled: []string{tool.AgentName, tool.AgentControlName}},
			{name: "helper control disabled", extra: []string{tool.AgentName}, denied: []string{tool.AgentControlName}, enabled: []string{tool.TaskGraphName, tool.AgentName}},
		} {
			t.Run(scenario.name+"/"+capabilities.name, func(t *testing.T) {
				workspace := t.TempDir()
				observed := false
				client := &fakeClient{respond: func(_ context.Context, request llm.Request) (llm.Response, error) {
					var system string
					for _, item := range request.Input {
						if message, ok := item.Data.(llm.Message); ok && message.Role == llm.RoleSystem {
							system += message.Text
						}
					}
					if !strings.Contains(system, scenario.policy) {
						t.Errorf("missing repository policy %q", scenario.policy)
					}
					for _, capability := range []struct {
						name     string
						guidance []string
					}{
						{tool.TaskGraphName, []string{"When using TaskGraph", "Completion produces a candidate, not acceptance", "accept the current generation with fresh validation evidence"}},
						{tool.AgentName, []string{"When starting helpers", "purpose, parent task, lifetime and owned resources"}},
						{tool.AgentControlName, []string{"Use AgentControl", "exact handles and generations", "use structural cancellation", "not proof of process/output drain or goal completion"}},
					} {
						want := slices.Contains(capabilities.enabled, capability.name)
						if got := containsTool(request.Tools, capability.name); got != want {
							t.Errorf("tool %s enabled=%v, want %v", capability.name, got, want)
						}
						for _, guidance := range capability.guidance {
							if got := strings.Contains(system, guidance); got != want {
								t.Errorf("guidance %q present=%v, want %v", guidance, got, want)
							}
						}
					}
					if strings.Contains(system, "Scope and proportional execution") {
						t.Error("general fork policy remains in the core instructions")
					}
					observed = true
					return messageResponse("scope inspected; fixture performs no work"), nil
				}}
				config := testConfig(client)
				config.SubagentCommand = []string{filepath.Join(workspace, "must-not-start")}
				input, err := json.Marshal(map[string]any{
					"prompt": scenario.request, "model": "gpt-test", "system_prompt": scenario.policy,
					"extra_allowed_tools": capabilities.extra, "disallowed_tools": capabilities.denied,
				})
				if err != nil {
					t.Fatal(err)
				}
				var out, stderr strings.Builder
				code := RunMain(t.Context(), []string{"-workspace", workspace, "-session-directory", t.TempDir()}, func(name string) string {
					if name == "OPENAI_API_KEY" {
						return "secret"
					}
					if name == "XDG_CONFIG_HOME" {
						return workspace
					}
					return ""
				}, os.Environ, strings.NewReader(string(input)), &out, &stderr, config)
				if code != 0 || !observed {
					t.Fatalf("exit=%d observed=%v stderr=%s", code, observed, stderr.String())
				}
			})
		}
	}
}
