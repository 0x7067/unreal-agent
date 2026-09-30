package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

func resultState(result tool.Result) (running bool, failure string) {
	switch result := result.(type) {
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
	}
	return boundedDetail(strings.Join(parts, "\n\n"), 32000), strings.Join(paths, "\n")
}
