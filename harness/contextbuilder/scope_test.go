package contextbuilder

import (
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// This is a prompt-delivery regression, not an evaluation of model behavior.
func TestBuiltRequestsContainProportionalExecutionGuidance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prompt string
		skills []tool.Skill
	}{
		{name: "default"},
		{name: "repository instructions", prompt: "Repository policy: validate changes before release."},
		{
			name:   "skills and repository instructions",
			prompt: "Repository policy: validate changes before release.",
			skills: []tool.Skill{{Name: "review", Description: "Review code", Path: "/skills/review"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := NewBuilder(tc.skills...)
			// Updating or clearing project instructions must not remove guidance.
			for _, prompt := range []string{tc.prompt, "Updated repository instructions.", ""} {
				current.SetSystemPrompt(prompt)
				result, err := current.Build()
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Request.Input) == 0 || result.Request.Input[0].Type != llm.ItemMessage {
					t.Fatal("built request has no leading system message")
				}
				message, ok := result.Request.Input[0].Data.(llm.Message)
				if !ok || message.Role != llm.RoleSystem {
					t.Fatal("guidance is not delivered as system instructions")
				}
				// Independent expectations deliberately do not compare against the
				// embedded preamble variable: deleting guidance must fail this test.
				for _, guidance := range []string{
					"Choose the smallest operation that satisfies the user's request.",
					"Before delegating, supply a concise purpose",
					"Resolve repository instructions and registered skill locations directly",
					"An outside-workspace or filesystem-root search needs a concrete task-specific purpose",
					"does not bound traversal, wall time, or child processes",
					"Reuse current accepted validation evidence",
					"tagging a release does not by itself require a full test/check/build pipeline",
					"Optional tests and builds are not prerequisites unless the user asks for them, repository policy requires them, or a concrete safety concern makes them necessary.",
					"Preserve necessary verification for code changes and required repository validation",
					"not permission to skip these checks or bypass architecture boundaries",
					"Act on clear, authorized requests without adding a confirmation round by default.",
					"Report the actual execution phase supported by tool or runtime evidence",
					"A submitted command or task is not necessarily running",
					"Use TaskGraph only when meaningful independent work can overlap and its coordination overhead has a net benefit",
				} {
					if !strings.Contains(message.Text, guidance) {
						t.Errorf("built system message missing guidance %q", guidance)
					}
				}
				if prompt != "" && !strings.Contains(message.Text, prompt) {
					t.Error("scope guidance displaced repository instructions")
				}
			}
		})
	}
}
