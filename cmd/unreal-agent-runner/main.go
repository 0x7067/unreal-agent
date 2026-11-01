// Command unreal-agent-runner executes one JSON request and writes persisted session items as JSONL.
package main

import (
	"os"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/residentcli"
	"github.com/unreallabsai/unreal-agent/cmd/internal/reviewcli"
)

const reviewUsage = `Usage: unreal-agent-runner review run|group|render|help [args...]
Reviews a git diff into findings.jsonl and renders it.
Run "unreal-agent-runner review help" for its usage.
`

func main() {
	// Dispatch first: review does not load workspace .env or parse a JSON
	// request, and it re-execs this binary with the same arguments.
	if len(os.Args) > 1 && os.Args[1] == "review" {
		os.Exit(reviewcli.Main(os.Args[2:]))
	}

	if residentcli.Handles(os.Args[1:]) {
		os.Exit(residentcli.Main(os.Args[1:]))
	}

	// workerhost stages one executable and uses its embedded runner for child
	// agents, just as the TUI worker does. Preserve ordinary one-shot arguments.
	if len(os.Args) > 1 && os.Args[1] == "agent-runner" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	agentrunner.Main(agentrunner.Config{
		Name:         "unreal-agent-runner",
		ParseRequest: parseRequest,
		Providers:    providers.Default(),
		UsageSuffix:  reviewUsage,
	})
}
