package main

import (
	"encoding/json/v2"
	"io"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
)

func TestToolSummarySubagents(t *testing.T) {
	if got := toolSummary("Agent", `{"name":"research","prompt":"find the cause"}`); got != "research: find the cause" {
		t.Fatalf("Agent summary = %q", got)
	}
	if got := toolSummary("SendMessage", `{"to":"parent","message":"need a decision"}`); got != "to parent: need a decision" {
		t.Fatalf("SendMessage summary = %q", got)
	}
}

func TestSubagentResultState(t *testing.T) {
	for _, test := range []struct {
		name    string
		result  toolsubagent.Result
		running bool
		failure string
		text    string
	}{
		{name: "running", result: toolsubagent.Result{Running: true}, running: true},
		{name: "completed", result: toolsubagent.Result{Text: "done"}, text: "done"},
		{name: "failed", result: toolsubagent.Result{Error: "boom"}, failure: "boom"},
	} {
		t.Run(test.name, func(t *testing.T) {
			running, failure := resultState(test.result)
			if running != test.running || failure != test.failure {
				t.Fatalf("resultState = %v, %q; want %v, %q", running, failure, test.running, test.failure)
			}
			text, _ := resultText(test.result)
			if !strings.Contains(text, test.text) || (test.failure != "" && !strings.Contains(text, test.failure)) {
				t.Fatalf("resultText = %q", text)
			}
		})
	}
}

func TestParseOptionsSubagents(t *testing.T) {
	opts, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.subagents {
		t.Fatal("subagents default = false, want true")
	}
	opts, err = parseOptions([]string{"-subagents=false"}, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if opts.subagents {
		t.Fatal("subagents with -subagents=false = true, want false")
	}
}

func TestApplySubagentMessageLabel(t *testing.T) {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	subagentPayload, err := json.Marshal(`Message from subagent "research":` + "\nfound it")
	if err != nil {
		t.Fatal(err)
	}
	m.apply(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{Kind: inbox.InputExternal, Payload: subagentPayload}})
	if len(m.lines) != 1 || m.lines[0].label != "Subagent" {
		t.Fatalf("lines = %+v, want one Subagent line", m.lines)
	}
	userPayload, err := json.Marshal("hello")
	if err != nil {
		t.Fatal(err)
	}
	m.apply(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{Kind: inbox.InputExternal, Payload: userPayload}})
	if len(m.lines) != 2 || m.lines[1].label != "You" {
		t.Fatalf("lines = %+v, want a You line", m.lines)
	}
}

func TestTaskGraphResultState(t *testing.T) {
	for _, result := range []tooltaskgraph.Result{{Running: true, Text: "candidate awaiting acceptance"}, {Text: "accepted"}, {Error: "denied"}} {
		running, failure := resultState(result)
		if running != result.Running || failure != result.Error {
			t.Fatalf("state=%v %q", running, failure)
		}
		text, _ := resultText(result)
		if !strings.Contains(text, result.Text) || !strings.Contains(text, result.Error) {
			t.Fatalf("text=%q", text)
		}
	}
}
