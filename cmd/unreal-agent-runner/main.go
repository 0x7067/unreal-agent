// Command unreal-agent-runner executes one JSON request and writes persisted session items as JSONL.
package main

import (
	"os"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/reviewcli"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "review" {
		os.Exit(reviewcli.Main(os.Args[2:]))
	}

	agentrunner.Main(agentrunner.Config{
		Name:         "unreal-agent-runner",
		ParseRequest: parseRequest,
		Providers:    providers.Default(),
	})
}
