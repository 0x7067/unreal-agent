package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/unreallabsai/unreal-agent/cmd/internal/reviewcli"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
)

func main() {
	// Review re-execs this binary; dispatch before credentials or TTY startup.
	if len(os.Args) > 1 && os.Args[1] == "review" {
		os.Exit(reviewcli.Main(os.Args[2:]))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil && !errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprintln(os.Stderr, "unreal-agent-tui:", terminaltext.Clean(err.Error()))
		os.Exit(1)
	}
}
