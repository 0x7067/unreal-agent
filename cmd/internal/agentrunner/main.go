package agentrunner

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func Main(config Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := RunMain(
		ctx, os.Args[1:], os.Getenv, os.Environ,
		os.Stdin, os.Stdout, os.Stderr, config,
	)
	stop()
	os.Exit(code)
}
