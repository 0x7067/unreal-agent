// Command unreal-agent-runner executes one JSON request and writes persisted session items as JSONL.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/residentcli"
)

func main() {
	// Dispatch before resident commands and agentrunner so review does not
	// load workspace .env or parse a JSON request.
	if len(os.Args) > 1 && os.Args[1] == "review" {
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
	if isRunnerHelp(os.Args[1:]) {
		os.Exit(runRunnerHelp(os.Args[1:], os.Getenv, os.Environ, os.Stdin, os.Stdout, os.Stderr))
	}
	agentrunner.Main(runnerConfig())
}

func runnerConfig() agentrunner.Config {
	return agentrunner.Config{
		Name:         "unreal-agent-runner",
		ParseRequest: parseRequest,
		Providers:    providers.Default(),
	}
}

// isRunnerHelp reports flags that make this binary print its own usage.
// The review alias is named there, not in agentrunner's shared usage, so
// unreal-agent-tui agent-runner -h does not advertise it.
func isRunnerHelp(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "-help", "--help", "--h":
			return true
		}
	}
	return false
}

func runRunnerHelp(args []string, getenv func(string) string, environ func() []string, input io.Reader, output, stderr io.Writer) int {
	code := agentrunner.RunMain(context.Background(), args, getenv, environ, input, output, stderr, runnerConfig())
	if code == 0 {
		fmt.Fprint(stderr, "\n"+reviewUsage)
	}
	return code
}
