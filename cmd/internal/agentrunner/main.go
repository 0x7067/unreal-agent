package agentrunner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func Main(config Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go forceExitOnSecondSignal(ctx, config.Name, os.Stderr, os.Exit)
	code := RunMain(
		ctx, os.Args[1:], os.Getenv, os.Environ,
		os.Stdin, os.Stdout, os.Stderr, config,
	)
	stop()
	os.Exit(code)
}

// The first signal cancels the run and waits for bounded tool and subagent
// cleanup (shells that ignore SIGTERM take 5s). A second one exits at once.
// Tool processes own separate groups, so a forced exit can leave them behind.
func forceExitOnSecondSignal(ctx context.Context, name string, stderr *os.File, exit func(int)) {
	<-ctx.Done()
	forced := make(chan os.Signal, 1)
	signal.Notify(forced, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(forced)
	<-forced
	prefix := ""
	if name != "" {
		prefix = name + ": "
	}
	_, _ = fmt.Fprintln(stderr, prefix+"forced exit; running tool processes may be left behind")
	exit(130)
}
