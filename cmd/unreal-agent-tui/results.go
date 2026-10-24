package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

func resultState(result tool.Result) (running bool, failure string) {
	switch result := result.(type) {
	case tool.UnavailableResult:
		return false, result.Error
	case bash.Result:
		failure = result.Error
		if failure == "" && result.Output != nil && result.Output.ExitCode != 0 {
			failure = fmt.Sprintf("Bash exited with code %d", result.Output.ExitCode)
		}
		return result.Running, failure
	case viewimage.Result:
		return result.Running, result.Error
	case tool.SkillUseResult:
		return result.Running, result.Error
	case toolsubagent.Result:
		return result.Running, result.Error
	case tooltaskgraph.Result:
		return result.Running, result.Error
	default:
		return false, fmt.Sprintf("Unsupported tool result %T", result)
	}
}

func (m model) readToolResult(card *toolCard, data sessionstore.ToolCallStatus) tool.Result {
	card.status, card.failure = "Failed", ""
	translator, ok := m.registry.Resolve(card.name)
	if !ok {
		card.failure = "Unknown tool: " + card.name
		return nil
	}
	result, err := translator.TranslateResult(data.CallID, data.Status, data.Operations)
	if err != nil {
		card.failure = err.Error()
		return nil
	}
	running, failure := resultState(result)
	card.failure = failure
	switch {
	case running:
		card.status = awaiting
	case slices.ContainsFunc(data.Operations, func(op operation.Operation) bool { return op.Status == operation.StatusCanceled }):
		card.status, card.failure = "Canceled", ""
	case failure == "":
		card.status = "Completed"
	}
	return result
}

func resultText(result tool.Result) (string, string) {
	var parts, paths []string
	switch result := result.(type) {
	case tool.UnavailableResult:
		parts = append(parts, result.Error)
	case bash.Result:
		if result.Running {
			parts = append(parts, "Command is still running.")
		}
		if output := result.Output; output != nil {
			parts = append(parts, fmt.Sprintf("Exit code: %d", output.ExitCode))
			if output.Out != "" {
				parts = append(parts, "Stdout:\n"+output.Out)
			}
			if output.Err != "" {
				parts = append(parts, "Stderr:\n"+output.Err)
			}
		}
		if result.Error != "" {
			parts = append(parts, result.Error)
		}
		if result.OutPath != "" {
			paths = append(paths, "Full stdout: "+singleLine(result.OutPath))
		}
		if result.ErrPath != "" {
			paths = append(paths, "Full stderr: "+singleLine(result.ErrPath))
		}
	case viewimage.Result:
		if result.Running {
			parts = append(parts, "Image is still loading.")
		}
		if result.Error != "" {
			parts = append(parts, result.Error)
		}
		if image := result.Image; image != nil && image.OriginalMIMEType != "" {
			parts = append(parts, fmt.Sprintf("Image: %s · %d×%d", image.OriginalMIMEType, image.OriginalWidth, image.OriginalHeight))
		}
	case tool.SkillUseResult:
		parts = append(parts, string(result.Content))
		if result.Error != "" {
			parts = append(parts, result.Error)
		}
	case tooltaskgraph.Result:
		parts = append(parts, result.Text)
		if result.Error != "" {
			parts = append(parts, result.Error)
		}
	case toolsubagent.Result:
		if result.Text != "" {
			parts = append(parts, result.Text)
		}
		if result.Error != "" {
			parts = append(parts, result.Error)
		}
	}
	return boundedDetail(strings.Join(parts, "\n\n"), 32000), strings.Join(paths, "\n")
}

// shellDetail keeps stream boundaries separate from text emitted by the command.
// Its strings are safe, bounded terminal text, never Markdown.
type shellDetail struct {
	running                 bool
	exitCode                *int
	stdout, stderr, failure string
}

func newShellDetail(result tool.Result) *shellDetail {
	shell, ok := result.(bash.Result)
	if !ok {
		return nil
	}
	detail := &shellDetail{running: shell.Running, failure: shell.Error}
	if shell.Output != nil {
		code := shell.Output.ExitCode
		detail.exitCode = &code
		detail.stdout = shell.Output.Out
		detail.stderr = shell.Output.Err
	}
	// Share the existing display budget across streams, retaining head and tail
	// for each. Bound the raw capture before sanitizing so even control-heavy
	// output stays within the display budget. BoundOutput can split an escape
	// sequence: sanitize after bounding, before adding trusted headings or resets.
	total := utf8.RuneCountInString(detail.stdout) + utf8.RuneCountInString(detail.stderr) + utf8.RuneCountInString(detail.failure)
	for _, text := range []*string{&detail.stdout, &detail.stderr, &detail.failure} {
		limit := 32000
		if total > limit {
			limit = limit * utf8.RuneCountInString(*text) / total
		}
		bounded, shortened := operation.BoundOutput(*text, limit)
		*text = terminaltext.CleanOutput(bounded)
		if shortened {
			*text += ansi.ResetStyle + "\n[display shortened; inspect run files for full text]"
		}
	}
	return detail
}

func (m model) renderShellDetail(detail shellDetail) string {
	var sections []string
	if detail.running {
		sections = append(sections, textStyle(m.theme.Muted).Render("Command is still running."))
	}
	if detail.exitCode != nil {
		color := m.theme.Success
		if *detail.exitCode != 0 {
			color = m.theme.Error
		}
		sections = append(sections, textStyle(color).Bold(true).Render(fmt.Sprintf("Exit code: %d", *detail.exitCode)))
	}
	for _, stream := range []struct{ heading, color, text string }{
		{"Stdout", m.theme.Accent, detail.stdout},
		{"Stderr", m.theme.Error, detail.stderr},
	} {
		if stream.text != "" {
			sections = append(sections, textStyle(stream.color).Bold(true).Render(stream.heading+":")+"\n"+stream.text+ansi.ResetStyle)
		}
	}
	if detail.failure != "" {
		sections = append(sections, textStyle(m.theme.Error).Render(detail.failure+ansi.ResetStyle))
	}
	// Explicit resets isolate unclosed command styles from stderr, paths, and UI
	// chrome, including when a command ends in a non-reset SGR such as bold.
	return ansi.ResetStyle + strings.Join(sections, ansi.ResetStyle+"\n\n") + ansi.ResetStyle
}
