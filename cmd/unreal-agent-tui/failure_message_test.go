package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

func TestResultStatePreservesExistingErrorDiagnostics(t *testing.T) {
	const message = "open /capture/out: permission denied"
	for _, result := range []tool.Result{
		bash.Result{Error: message}, viewimage.Result{Error: message},
		tool.SkillUseResult{Error: message}, toolsubagent.Result{Error: message},
		tooltaskgraph.Result{Error: message}, tool.UnavailableResult{Error: message},
	} {
		running, failure := resultState(result)
		if running || failure != message {
			t.Fatalf("%T: running=%v failure=%q", result, running, failure)
		}
	}
	running, failure := resultState(bash.Result{Output: &operation.ShellResult{ExitCode: 7}})
	if running || failure != "Bash exited with code 7" {
		t.Fatalf("exit state: %v %q", running, failure)
	}
}

func TestToolDetailsFailureSummaryIsFirstBoundedAndLiteral(t *testing.T) {
	for _, name := range []string{"Bash", "TaskGraph"} {
		m := toolsModel(t, defaultTheme, 160, 50, 0)
		m.tools = []toolCard{{name: name, status: "Failed", failure: "**permission denied**\x1b]52;c;c2VjcmV0\a\x1b[2J\n" + strings.Repeat("x", 10000) + " diagnostic-tail", output: "raw-output-marker"}}
		m.openTool(0)
		got := m.details.GetContent()
		plain := ansi.Strip(got)
		summary, output := strings.Index(plain, "Failure:"), strings.Index(plain, "raw-output-marker")
		if summary < 0 || output < summary {
			t.Fatalf("failure not before output: %q", plain)
		}
		for _, want := range []string{"**permission denied**", "diagnostic-tail", "display shortened"} {
			if !strings.Contains(plain, want) {
				t.Errorf("missing literal summary %q", want)
			}
		}
		for _, unsafe := range []string{"\x1b]52", "c2VjcmV0", "\x1b[2J"} {
			if strings.Contains(got, unsafe) {
				t.Errorf("unsafe summary %q", unsafe)
			}
		}
		if strings.Count(plain, "x") > 4100 {
			t.Fatal("failure summary is unbounded")
		}
	}
}

func TestToolDetailsTranslationFailureIsVisibleWithoutResult(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	card := m.card(callKey{"turn", "call"})
	card.name = "UnknownTool"
	if got := m.readToolResult(card, sessionstore.ToolCallStatus{CallID: "call"}); got != nil {
		t.Fatalf("unexpected result: %+v", got)
	}
	m.openTool(0)
	if got := ansi.Strip(m.details.GetContent()); !strings.Contains(got, "Failure:") || !strings.Contains(got, "Unknown tool: UnknownTool") {
		t.Fatalf("missing translation failure: %q", got)
	}
}
