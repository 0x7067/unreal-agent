package contextbuilder

import (
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// This is a prompt-delivery regression, not an evaluation of model behavior.
func TestBuiltRequestsContainConditionalCoordinationGuidance(t *testing.T) {
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
			// Updating or clearing project instructions must preserve tool guidance.
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
					"When using TaskGraph",
					"relevant reads and all writes, including generated artifacts",
					"Omitted/null claims cover the whole workspace; empty arrays claim none",
					"Avoid edits overlapping active graph ownership",
					"Completion produces a candidate, not acceptance",
					"accept the current generation with fresh validation evidence",
					"Relevant changes invalidate prior evidence",
					"use status after compaction or uncertainty",
					"When starting helpers",
					"purpose, parent task, lifetime and owned resources",
					"Use task-scoped helpers for temporary work",
					"When AgentControl is available",
					"exact handles and generations",
					"use structural cancellation, not SendMessage or guessed process IDs",
					"stop acknowledgment or timeout is not proof of process/output drain",
				} {
					if !strings.Contains(message.Text, guidance) {
						t.Errorf("built system message missing guidance %q", guidance)
					}
				}
				for _, removed := range []string{
					"Scope and proportional execution",
					"Choose the smallest operation",
					"Act on clear, authorized requests",
					"Optimize time to an accepted goal result",
					"Resolve repository instructions and registered skill locations",
					"filesystem-root search",
				} {
					if strings.Contains(message.Text, removed) {
						t.Errorf("built system message retains general fork policy %q", removed)
					}
				}
				if prompt != "" && !strings.Contains(message.Text, prompt) {
					t.Error("coordination guidance displaced repository instructions")
				}
			}
		})
	}
}

// Snapshot of upstream/main a5f3fd1: broad fork policies must not enter this file.
func TestPreamblePreservesUpstreamBaseline(t *testing.T) {
	const upstream = `You run on Unreal Agent Harness built by Unreal Labs.

You work in turns. A turn is one reading of the conversation and one reply: text, tool calls, or both. Each turn re-sends the whole conversation, so prefer to go wider with tool calls — they are cheap — rather than chaining them across a longer sequence of turns. When the next commands do not depend on each other's output (inspecting several files, running the build and the tests, probing two hypotheses), issue them as separate tool calls in the same turn instead of one at a time.

Tool calls are asynchronous: each starts the moment you issue it and runs in the background, so issuing one never blocks you and many run at once. As each finishes, its result is appended and wakes a new turn; results that land together arrive in the same turn, and a call still running shows a placeholder until its own result comes.

You never have to babysit a running call: harness does it for you. As a backup, if calls are active and nothing has happened for ten minutes, a heartbeat wakes you, and this is an opportunity to check that all is well.

Ending a turn with no tool calls while calls are running means you sleep until one finishes; ending a turn with nothing running ends the session, so do that only when the task is complete.

Treat the prompt as a goal and keep working until it is met. I believe in you!
`
	if preambleFile != upstream {
		t.Fatal("base preamble differs from upstream; keep fork guidance in coordination.md")
	}
	if !strings.HasPrefix(preamble, strings.TrimSpace(upstream)+"\n\n## Coordination tools (when available)\n") {
		t.Fatal("conditional tool guidance must follow the intact upstream preamble")
	}
}
