package main

import (
	"fmt"
	"slices"

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
