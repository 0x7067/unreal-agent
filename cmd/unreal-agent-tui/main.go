package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if handled, err := runWorkerCommand(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv); handled {
		stop()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "unreal-agent-tui worker:", terminaltext.Clean(err.Error()))
			os.Exit(1)
		}
		return
	}
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

// Dispatch before flags, dotenv, settings, credentials, or terminal startup.
// Probe deliberately does not even call getenv or consume input.
func runWorkerCommand(ctx context.Context, args []string, input io.Reader, output, stderr io.Writer, getenv func(string) string) (bool, error) {
	if len(args) == 0 || args[0] != "worker" && args[0] != "worker-probe" {
		return false, nil
	}
	if len(args) != 1 {
		return true, errors.New("worker subcommands do not accept arguments")
	}
	identity := workerIdentity()
	if args[0] == "worker-probe" {
		return true, workerruntime.Probe(output, identity)
	}
	return true, workerruntime.Run(ctx, input, output, stderr, getenv, identity)
}

func workerIdentity() workerproto.Identity {
	return workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: version + "+" + commit + "+" + date}
}
