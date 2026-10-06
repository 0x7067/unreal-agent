// Command unreal-agent-runner executes one JSON request and writes persisted session items as JSONL.
package main

import (
	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
)

func main() {
	agentrunner.Main(agentrunner.Config{
		Name:         "unreal-agent-runner",
		ParseRequest: parseRequest,
		Providers:    providers.Default(),
	})
}
