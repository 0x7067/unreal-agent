// Command unreal-agent-runner executes one JSON request and writes persisted session items as JSONL.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/harness/llm/providers"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(agentrunner.RunMain(
		ctx, os.Args[1:], os.Getenv, os.Environ,
		os.Stdin, os.Stdout, os.Stderr,
		agentrunner.Config{
			Name:         "unreal-agent-runner",
			ParseRequest: parseRequest,
			Providers:    providers.Default(),
		},
	))
}
