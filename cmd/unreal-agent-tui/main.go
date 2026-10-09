package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if len(os.Args) > 1 && os.Args[1] == "agent-runner" {
		// Subagents are child runner processes started by this binary; they never open the TUI.
		code := agentrunner.RunMain(ctx, os.Args[2:], os.Getenv, os.Environ, os.Stdin, os.Stdout, os.Stderr, agentrunner.Config{
			Name:         "unreal-agent-tui agent-runner",
			ParseRequest: agentrunner.ParseRequest,
			Providers:    providers.Default(),
		})
		stop()
		os.Exit(code)
	}
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil && !errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprintln(os.Stderr, "unreal-agent-tui:", terminaltext.Clean(err.Error()))
		os.Exit(1)
	}
}
