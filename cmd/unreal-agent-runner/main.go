// Command unreal-agent-runner executes one JSON request and writes persisted session items as JSONL.
package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/residentcli"
)

func main() {
	// Dispatch before resident commands and agentrunner so review does not
	// load workspace .env or parse a JSON request.
	if len(os.Args) > 1 && os.Args[1] == "review" {
		// Ignore signals here so this process can wait and return the child's
		// status. The child installs its own handler; ignored signals do not
		// stay ignored once it calls signal.Notify.
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		os.Exit(reviewCommand(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, os.Environ(), os.Executable, exec.LookPath))
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
	})
}
