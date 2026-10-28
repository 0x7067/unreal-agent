package contextbuilder

import (
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// This tests prompt delivery, not model behavior. The registered tool set is the
// only capability input, including when rebuilding a resumed/compacted context.
func TestCoordinationGuidanceFollowsEnabledTools(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		t.Run(strings.Join(scopeToolNames(mask), "+"), func(t *testing.T) {
			current := NewBuilder(tool.Skill{Name: "review", Description: "Review code", Path: "/nonexistent/skills/review"})
			current.SetSystemPrompt("Repository policy: validate changes before release.")
			current.AddTool(llm.Tool{Type: llm.ToolFunction, Name: tool.BashName})
			current.AddTool(llm.Tool{Type: llm.ToolFunction, Name: tool.ViewImageName})
			baseline := buildCompactionContext(t, current)
			for _, name := range scopeToolNames(mask) {
				current.AddTool(llm.Tool{Type: llm.ToolFunction, Name: name})
				// Duplicate registration must not duplicate instructions.
				current.AddTool(llm.Tool{Type: llm.ToolFunction, Name: name})
			}
			registered := scopeSystemText(t, buildCompactionContext(t, current))
			assertScopeGuidance(t, registered, mask)
			if !strings.HasSuffix(registered, "Repository policy: validate changes before release.") {
				t.Fatal("tool registration displaced custom instructions")
			}
			for _, prompt := range []string{"Repository policy: validate changes before release.", "Updated repository instructions.", ""} {
				current.SetSystemPrompt(prompt)
				current.SetModel(llm.Model{ID: "first"})
				current.AddControlMessage(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: inbox.Settings{Model: "second"}})
				result := buildCompactionContext(t, current)
				text := scopeSystemText(t, result)
				assertScopeGuidance(t, text, mask)
				if !strings.Contains(text, skillPreamble) || !strings.Contains(text, "<location>/nonexistent/skills/review</location>") {
					t.Fatal("tool guidance displaced skills instructions")
				}
				if prompt != "" && !strings.HasSuffix(text, prompt) {
					t.Fatal("tool guidance displaced custom system prompt")
				}
				if prompt == "" && strings.Contains(text, "repository instructions.") {
					t.Fatal("clearing custom prompt retained stale instructions")
				}
			}
			assertScopeGuidance(t, scopeSystemText(t, baseline), 0)
		})
	}
}

func TestDisabledCoordinationToolsDoNotAddGuidance(t *testing.T) {
	for _, tools := range [][]llm.Tool{
		nil,
		{{Type: llm.ToolFunction, Name: tool.BashName}, {Type: llm.ToolFunction, Name: tool.ViewImageName}},
		{{Type: llm.ToolFunction, Name: tool.SendMessageName}},
		{{Type: llm.ToolHosted, Name: tool.TaskGraphName}, {Type: llm.ToolHosted, Name: tool.AgentName}, {Type: llm.ToolHosted, Name: tool.AgentControlName}},
		{{Name: tool.TaskGraphName}},
		{{Type: llm.ToolFunction, Name: "taskgraph", Description: "TaskGraph Agent AgentControl"}},
	} {
		current := NewBuilder()
		for _, enabled := range tools {
			current.AddTool(enabled)
		}
		if text := scopeSystemText(t, buildCompactionContext(t, current)); text != preamble {
			t.Fatalf("disabled tools changed baseline prompt: %q", text)
		}
		// Tool mentions in custom instructions or completed history do not
		// activate guidance, nor does a previous builder's enabled tool set.
		current.SetSystemPrompt("TaskGraph Agent AgentControl")
		current.AddModelResponse(usableCompactionResponse("TaskGraph Agent AgentControl"))
		assertScopeGuidance(t, scopeSystemText(t, buildCompactionContext(t, current)), 0)
	}
}

func TestCompactionPreservesOnlyEnabledCoordinationGuidance(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		t.Run(strings.Join(scopeToolNames(mask), "+"), func(t *testing.T) {
			current := NewBuilder(tool.Skill{Name: "review", Path: "/nonexistent/skills/review"}).(*builder)
			current.SetSystemPrompt("Custom project instructions.")
			for _, name := range scopeToolNames(mask) {
				current.AddTool(llm.Tool{Type: llm.ToolFunction, Name: name})
			}
			for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
				addCompactionTestTurn(t, current, turn, total)
			}
			before := buildCompactionContext(t, current)
			for _, unusable := range []llm.Response{
				{Stop: llm.StopMaxOutputTokens, Output: usableCompactionResponse("Partial summary.").Output},
				{Stop: llm.StopComplete, Failure: &llm.Failure{Code: "failed"}, Output: usableCompactionResponse("Failed summary.").Output},
			} {
				if unchanged, ok := current.Compact(unusable); ok || unchanged != current || !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
					t.Fatal("unusable summary changed context or capability guidance")
				}
			}
			summary, err := current.BuildCompaction()
			if err != nil {
				t.Fatal(err)
			}
			if got := scopeSystemText(t, summary); got != strings.TrimSpace(compactionPrompt) {
				t.Fatalf("summarizer inherited ordinary prompt: %q", got)
			}
			if len(summary.Request.Tools) != 0 {
				t.Fatal("summarizer inherited tools")
			}
			rebuilt, ok := current.Compact(usableCompactionResponse("Handoff summary."))
			if !ok || rebuilt == current {
				t.Fatal("compaction did not clone builder")
			}
			after := buildCompactionContext(t, rebuilt)
			if !reflect.DeepEqual(before.Request.Tools, after.Request.Tools) || scopeSystemText(t, before) != scopeSystemText(t, after) {
				t.Fatal("compaction lost or duplicated tools, guidance, skills or custom instructions")
			}
			assertScopeGuidance(t, scopeSystemText(t, after), mask)
			rebuilt.SetSystemPrompt("")
			reset := scopeSystemText(t, buildCompactionContext(t, rebuilt))
			assertScopeGuidance(t, reset, mask)
			if strings.Contains(reset, "Custom project instructions.") || !strings.Contains(reset, skillPreamble) {
				t.Fatal("prompt reset lost skills or retained custom instructions")
			}
			if !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
				t.Fatal("compaction or clone prompt update mutated source builder")
			}
			// Repeated compaction must not accumulate prompt sections.
			clone := rebuilt.(*builder)
			addCompactionTestTurn(t, clone, 5, 20_000)
			addCompactionTestTurn(t, clone, 6, 35_000)
			again, ok := clone.Compact(usableCompactionResponse("Second handoff."))
			if !ok {
				t.Fatal("second compaction failed")
			}
			if got := scopeSystemText(t, buildCompactionContext(t, again)); got != reset {
				t.Fatal("repeated compaction changed prompt")
			}
		})
	}
}

func scopeToolNames(mask int) []string {
	var names []string
	for bit, name := range []string{tool.TaskGraphName, tool.AgentName, tool.AgentControlName} {
		if mask&(1<<bit) != 0 {
			names = append(names, name)
		}
	}
	return names
}

func scopeSystemText(t *testing.T, result Result) string {
	t.Helper()
	if len(result.Request.Input) == 0 || result.Request.Input[0].Type != llm.ItemMessage {
		t.Fatal("request has no leading system message")
	}
	message, ok := result.Request.Input[0].Data.(llm.Message)
	if !ok || message.Role != llm.RoleSystem {
		t.Fatal("prompt is not a system message")
	}
	return message.Text
}

func assertScopeGuidance(t *testing.T, text string, mask int) {
	t.Helper()
	if !strings.HasPrefix(text, preamble) {
		t.Fatal("base preamble changed")
	}
	for bit, guidance := range [][]string{
		{"When using TaskGraph", "relevant reads and all writes, including generated artifacts", "Omitted/null claims cover the whole workspace; empty arrays claim none", "Avoid edits overlapping active graph ownership", "Completion produces a candidate, not acceptance", "accept the current generation with fresh validation evidence", "Relevant changes invalidate prior evidence", "use status after compaction or uncertainty"},
		{"When starting helpers", "purpose, parent task, lifetime and owned resources", "Use task-scoped helpers for temporary work", "retain background helpers only intentionally"},
		{"Use AgentControl", "exact handles and generations", "use structural cancellation, not messages or guessed process IDs", "stop acknowledgment or timeout is not proof of process/output drain"},
	} {
		want := 0
		if mask&(1<<bit) != 0 {
			want = 1
		}
		for _, phrase := range guidance {
			if got := strings.Count(text, phrase); got != want {
				t.Errorf("guidance %q occurs %d times, want %d", phrase, got, want)
			}
		}
	}
	if strings.Contains(text, "## Coordination tools") != (mask != 0) {
		t.Error("coordination heading does not match enabled capabilities")
	}
	for _, removed := range []string{"Scope and proportional execution", "Choose the smallest operation", "Act on clear, authorized requests", "Optimize time to an accepted goal result", "Resolve repository instructions and registered skill locations", "filesystem-root search"} {
		if strings.Contains(text, removed) {
			t.Errorf("prompt retains general fork policy %q", removed)
		}
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
		t.Fatal("base preamble differs from upstream; keep tool guidance separate")
	}
	if preamble != strings.TrimSpace(upstream) {
		t.Fatal("baseline preamble must not include coordination guidance")
	}
}
