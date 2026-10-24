package main

import (
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
)

func TestShellDetailLiteralWhitespaceAndMarkdown(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	literal := "  indented  columns\tvalue  \n\n# not a heading\n**not bold** _not italic_\n- not a list\n> not a quote\n```go\n[not a link](https://example.test)\n```\n"
	detail := newShellDetail(bash.Result{Output: &operation.ShellResult{Out: literal}})
	if detail.stdout != literal {
		t.Fatalf("literal capture changed: got %q, want %q", detail.stdout, literal)
	}
	if got := ansi.Strip(m.renderShellDetail(*detail)); !strings.Contains(got, "Stdout:\n"+literal) {
		t.Fatalf("shell output was reformatted: %q", got)
	}

	m.tools = []toolCard{{name: "Bash", status: "Completed", shell: detail}}
	m.openTool(0)
	got := ansi.Strip(m.details.GetContent())
	for _, want := range []string{"   indented  columns    value", "# not a heading", "**not bold** _not italic_", "- not a list", "> not a quote", "```go", "[not a link](https://example.test)"} {
		if !strings.Contains(got, want) {
			t.Errorf("details lost literal %q: %q", want, got)
		}
	}
	// The surface adds one left-padding column and right-pads each line. Tabs
	// become four spaces before layout, not terminal-dependent tab stops. Keep
	// the raw assertions above exact; normalize only viewport right padding here.
	lines := strings.Split(got, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	normalized := strings.Join(lines, "\n")
	if !strings.Contains(normalized, " Stdout:\n   indented  columns    value\n\n # not a heading\n **not bold** _not italic_\n - not a list\n > not a quote\n ```go\n [not a link](https://example.test)\n ```\n") {
		t.Fatalf("viewport lost indentation, blank lines, or literal Markdown: %q", normalized)
	}
	if strings.Contains(got, "\t") {
		t.Fatal("viewport contains terminal-dependent tab stops")
	}
}

func TestShellDetailPreservesSafeANSIStyles(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	for _, sgr := range []string{"31", "1;32", "2", "3", "4", "38;5;196", "48;5;17", "38;2;12;34;56", "48;2;65;43;21"} {
		t.Run(sgr, func(t *testing.T) {
			text := "\x1b[" + sgr + "mcolored\x1b[0m"
			detail := newShellDetail(bash.Result{Output: &operation.ShellResult{Out: text}})
			if !strings.Contains(detail.stdout, text) {
				t.Fatalf("safe SGR lost: %q", detail.stdout)
			}
			m.tools = []toolCard{{name: "Bash", status: "Completed", shell: detail}}
			m.openTool(0)
			if !strings.Contains(m.details.GetContent(), "\x1b["+sgr+"mcolored") {
				t.Fatalf("rendered details lost SGR: %q", m.details.GetContent())
			}
		})
	}
}

func TestShellDetailRemovesTerminalControls(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	for name, control := range map[string]string{
		"cursor": "\x1b[2J\x1b[H", "private mode": "\x1b[?1049h",
		"title": "\x1b]0;malicious title\a", "clipboard": "\x1b]52;c;c2VjcmV0\a",
		"hyperlink":    "\x1b]8;;https://evil.test\x1b\\link\x1b]8;;\x1b\\",
		"device query": "\x1b[6n", "DCS": "\x1bPmalicious payload\x1b\\",
		"C0": "\a\b\x00", "C1": "\u009b2J",
	} {
		t.Run(name, func(t *testing.T) {
			payload := "before" + control + "after"
			detail := newShellDetail(bash.Result{Error: payload, Output: &operation.ShellResult{Out: payload, Err: payload}})
			got := m.renderShellDetail(*detail)
			for _, forbidden := range []string{"\x1b[2J", "\x1b[H", "\x1b[?1049h", "\x1b]", "\x1bP", "\x1b[6n", "\u009b", "\a", "\b", "\x00", "malicious title", "c2VjcmV0", "https://evil.test", "malicious payload"} {
				if strings.Contains(got, forbidden) {
					t.Errorf("unsafe sequence %q survived in %q", forbidden, got)
				}
			}
			if strings.Count(ansi.Strip(got), "before") != 3 || strings.Count(ansi.Strip(got), "after") != 3 {
				t.Fatalf("lost surrounding stream/error text: %q", got)
			}
		})
	}
}

func TestShellDetailSectionsStatusAndStyleIsolation(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	for _, code := range []int{0, 7} {
		detail := newShellDetail(bash.Result{Output: &operation.ShellResult{Out: "\x1b[1;31mOUT", Err: "ERR", ExitCode: code}, Error: "failure"})
		got := m.renderShellDetail(*detail)
		color := m.theme.Success
		if code != 0 {
			color = m.theme.Error
		}
		for _, heading := range []string{
			textStyle(color).Bold(true).Render(fmt.Sprintf("Exit code: %d", code)),
			textStyle(m.theme.Accent).Bold(true).Render("Stdout:"),
			textStyle(m.theme.Error).Bold(true).Render("Stderr:"),
		} {
			if !strings.Contains(got, heading) {
				t.Errorf("missing independently styled heading %q in %q", heading, got)
			}
		}
		if !strings.HasPrefix(got, ansi.ResetStyle) || !strings.HasSuffix(got, ansi.ResetStyle) || !strings.Contains(got, "OUT"+ansi.ResetStyle+ansi.ResetStyle+"\n\n") {
			t.Fatalf("unclosed command style not isolated: %q", got)
		}
		if plain := ansi.Strip(got); plain != fmt.Sprintf("Exit code: %d\n\nStdout:\nOUT\n\nStderr:\nERR\n\nfailure", code) {
			t.Fatalf("stream/status boundaries changed: %q", plain)
		}
	}
	if got := ansi.Strip(m.renderShellDetail(*newShellDetail(bash.Result{Running: true}))); got != "Command is still running." {
		t.Fatalf("running status = %q", got)
	}
}

func TestShellDetailBoundsRawCaptureAndKeepsPaths(t *testing.T) {
	result := bash.Result{
		Output: &operation.ShellResult{
			Out: "OUT-HEAD" + strings.Repeat("界\x1b[31m", 20000) + "OUT-TAIL",
			Err: "ERR-HEAD" + strings.Repeat("x", 40000) + "ERR-TAIL",
		},
		Error:   "FAIL-HEAD" + strings.Repeat("y", 40000) + "FAIL-TAIL",
		OutPath: "/run/full-out", ErrPath: "/run/full-err",
	}
	detail := newShellDetail(result)
	for _, tc := range []struct{ text, head, tail string }{
		{detail.stdout, "OUT-HEAD", "OUT-TAIL"}, {detail.stderr, "ERR-HEAD", "ERR-TAIL"}, {detail.failure, "FAIL-HEAD", "FAIL-TAIL"},
	} {
		if !utf8.ValidString(tc.text) || !strings.HasPrefix(tc.text, tc.head) || !strings.Contains(tc.text, tc.tail) || !strings.Contains(tc.text, "display shortened") {
			t.Errorf("invalid bounded head/tail output: %q", tc.text)
		}
	}
	// The budget covers raw runes, with small fixed truncation annotations.
	if count := utf8.RuneCountInString(detail.stdout + detail.stderr + detail.failure); count > 33000 {
		t.Fatalf("display budget exceeded: %d runes", count)
	}
	_, paths := resultText(result)
	if paths != "Full stdout: /run/full-out\nFull stderr: /run/full-err" {
		t.Fatalf("capture paths lost: %q", paths)
	}
	if !strings.HasSuffix(result.Output.Out, "OUT-TAIL") || utf8.RuneCountInString(result.Output.Out) < 100000 {
		t.Fatal("display bounding modified the source capture")
	}
}

func TestShellDetailInstalledFromToolStatus(t *testing.T) {
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	m.registry = newUIRegistry(true, true)
	key := callKey{"turn", "call"}
	m.card(key).name = "Bash"
	state, err := json.Marshal(operation.ShellState{
		Result:  &operation.ShellResult{Out: "\x1b[32mworker colored\x1b[0m", Err: "worker stderr", ExitCode: 7},
		OutPath: "/run/out", ErrPath: "/run/err",
	})
	if err != nil {
		t.Fatal(err)
	}
	m.apply(sessionstore.Item{Kind: sessionstore.ItemToolCallStatus, Data: sessionstore.ToolCallStatus{
		TurnID: key.turn, CallID: key.call,
		Operations: []operation.Operation{{ID: "op", Type: operation.TypeShell, Version: operation.VersionShell, Status: operation.StatusCompleted, State: state, MaxOutputLength: operation.DefaultMaxOutputLength}},
	}})
	card := m.tools[0]
	if card.shell == nil || card.status != "Failed" || !strings.Contains(card.shell.stdout, "\x1b[32m") {
		t.Fatalf("tool status did not install styled shell detail: %+v", card)
	}
	m.openTool(0)
	got := m.details.GetContent()
	for _, want := range []string{"\x1b[32mworker colored", "Stderr:", "Exit code: 7", "Full stdout: /run/out", "Full stderr: /run/err"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in rendered worker result: %q", want, got)
		}
	}
}

func TestNonBashDetailsStillUseMarkdown(t *testing.T) {
	result := tool.SkillUseResult{Content: []byte("**markdown strong**")}
	if newShellDetail(result) != nil {
		t.Fatal("non-Bash result acquired a shell renderer")
	}
	m := toolsModel(t, defaultTheme, 160, 50, 0)
	output, _ := resultText(result)
	m.tools = []toolCard{{name: "SkillUse", status: "Completed", output: output}}
	m.openTool(0)
	got := ansi.Strip(m.details.GetContent())
	if !strings.Contains(got, "markdown strong") || strings.Contains(got, "**markdown strong**") {
		t.Fatalf("non-Bash Markdown behavior changed: %q", got)
	}
}
