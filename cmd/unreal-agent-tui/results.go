package main

import (
	"fmt"

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
