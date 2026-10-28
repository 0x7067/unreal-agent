// Package residentcli exposes explicit headless-host and attach entry points.
// Attach only connects; it never starts a host or loads provider configuration.
package residentcli

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/resident"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime"
	"github.com/unreallabsai/unreal-agent/cmd/internal/xdgpath"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

func Handles(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "job-executor", "host", "host-retire", "attach", "worker", "worker-probe":
		return true
	default:
		return false
	}
}

func Main(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := Run(ctx, args, os.Getenv, os.Stdin, os.Stdout, os.Stderr)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	_, _ = fmt.Fprintln(os.Stderr, "unreal-agent-runner:", err)
	if ctx.Err() != nil {
		return 130
	}
	return 1
}

func Run(ctx context.Context, args []string, getenv func(string) string, input io.Reader, output, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("expected host, attach, worker or worker-probe")
	}
	if args[0] == "job-executor" {
		return runExecutor(ctx, args[1:], getenv, output, stderr)
	}
	if args[0] == "host-retire" {
		return retireHost(ctx, args[1:], output, stderr)
	}
	identity := workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: "unreal-agent-runner-resident-v1"}
	if args[0] == "worker-probe" {
		if len(args) != 1 {
			return errors.New("worker-probe takes no arguments")
		}
		return workerruntime.Probe(output, identity)
	}
	if args[0] == "worker" {
		if len(args) != 1 {
			return errors.New("worker takes no arguments")
		}
		return workerruntime.Run(ctx, input, output, stderr, getenv, identity)
	}
	if args[0] != "host" && args[0] != "attach" {
		return errors.New("unknown resident command")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeDirectory := flags.String("runtime-directory", "", "existing private 0700 runtime directory; defaults to XDG_RUNTIME_DIR")
	stateDirectory := flags.String("session-directory", "", "session store directory; defaults to the XDG state directory")
	id := flags.String("session", "", "required stable session ID")
	var workspace, executable, provider, baseURL, model, inputID, message string
	var submit bool
	var durableSubagents bool
	var durableGraphs bool
	var runtimeReplacement bool
	if args[0] == "host" {
		flags.BoolVar(&runtimeReplacement, "runtime-replacement", false, "opt-in current-frontend trusted runtime check/replacement controls")
		flags.BoolVar(&durableGraphs, "durable-graphs", false, "opt-in command-only durable graph checkpoints with acknowledged dispatch fences")
		flags.BoolVar(&durableSubagents, "durable-subagents", false, "opt-in independently owned subagent.run/v2 child hosts; legacy child replay remains refused")
		flags.StringVar(&workspace, "workspace", "", "required absolute workspace directory")
		flags.StringVar(&executable, "worker-executable", "", "absolute trusted runner executable; defaults to this executable")
		flags.StringVar(&provider, "provider", "openai", "provider selected only at trusted host startup")
		flags.StringVar(&baseURL, "base-url", "", "optional trusted provider endpoint")
		flags.StringVar(&model, "model", "", "required model ID")
	} else {
		flags.BoolVar(&submit, "submit", false, "submit once after snapshot, then watch; requires input-id")
		flags.StringVar(&inputID, "input-id", "", "stable identity for optional submission; reuse it on uncertain delivery")
		flags.StringVar(&message, "message", "", "message for optional submission")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *id == "" {
		return errors.New("a session ID is required; unexpected positional arguments are not accepted")
	}
	if *runtimeDirectory == "" {
		*runtimeDirectory = getenv("XDG_RUNTIME_DIR")
	}
	if !filepath.IsAbs(*runtimeDirectory) {
		return errors.New("set an absolute private runtime-directory or XDG_RUNTIME_DIR")
	}
	if *stateDirectory == "" {
		var err error
		*stateDirectory, err = xdgpath.SessionDirectory(getenv)
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(*stateDirectory) {
		return errors.New("session-directory must be absolute")
	}
	if args[0] == "attach" {
		payload, err := json.Marshal(message)
		if err != nil {
			return err
		}
		if len(inputID) > 128 || len(payload) > attachproto.MaxPayload {
			return errors.New("submission exceeds identity or payload limit")
		}
		if submit != (inputID != "") || !submit && message != "" {
			return errors.New("use submit with a stable input-id and optional message")
		}
		return attach(ctx, *runtimeDirectory, *stateDirectory, session.ID(*id), submit, inputID, message, output)
	}
	if !filepath.IsAbs(workspace) || model == "" {
		return errors.New("host requires an absolute workspace and a model")
	}
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(executable) {
		return errors.New("worker-executable must be an absolute trusted path")
	}
	config := resident.Config{RuntimeReplacement: runtimeReplacement, RuntimeDirectory: *runtimeDirectory, Worker: workerhost.Config{
		Command: []string{executable, "worker"}, Stderr: stderr,
		Bootstrap: workerproto.Bootstrap{SessionID: session.ID(*id), Directory: *stateDirectory, Workspace: workspace, Provider: provider, BaseURL: baseURL, Model: llm.Model{ID: model, CompactionThreshold: 64000}, MaxAttempts: 5, Shell: "/bin/sh", DurableSubagents: durableSubagents, DurableGraphs: durableGraphs, ChildRuntimeDirectory: *runtimeDirectory, DurableChildCommand: []string{executable}},
	}}
	// Readiness is a startup announcement, not a discovery file or an admission
	// acknowledgment. It must be consumed by the explicitly launching supervisor.
	var readyErr error
	serveCtx, stopServe := context.WithCancel(ctx)
	defer stopServe()
	err := resident.Serve(serveCtx, config, func(info resident.Info) {
		data, err := json.Marshal(struct {
			Type string
			resident.Info
		}{"host_ready", info}, json.Deterministic(true))
		if err == nil {
			_, err = fmt.Fprintf(output, "%s\n", data)
		}
		readyErr = err
		if err != nil {
			stopServe()
		}
	})
	return errors.Join(err, readyErr)
}

func attach(ctx context.Context, runtimeDirectory, stateDirectory string, id session.ID, submit bool, inputID, message string, output io.Writer) (result error) {
	conn, err := attachproto.Dial(ctx, runtimeDirectory, stateDirectory, id)
	if err != nil {
		return err // No automatic process launch, config load or downgrade.
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	write := func(request attachproto.Request) error {
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		return attachproto.Write(conn, request)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if err := write(attachproto.Request{Type: "attach", ID: "attach", Protocol: attachproto.Version, SessionID: id}); err != nil {
		return err
	}
	scanner := attachproto.Scanner(conn)
	hello, err := attachproto.ReadEvent(scanner)
	if err != nil {
		return err
	}
	if hello.Type != "hello" || hello.Protocol != attachproto.Version || hello.SessionID != id || hello.Connection == "" || hello.Incarnation == "" {
		return fmt.Errorf("attach rejected or incompatible: %s %s", hello.Code, hello.Error)
	}
	if err := attachproto.Write(output, hello); err != nil {
		return err
	}
	// The server's snapshot lifetime is bounded; the client also bounds reads
	// until snapshot_end, rather than accepting an endless partial snapshot.
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	state := newView(hello)
	var inFlight bool
	var requestID string
	defer func() {
		if inFlight && result != nil {
			result = fmt.Errorf("submission reply uncertain; retry stable input-id %q: %w", inputID, result)
		}
	}()
	for {
		event, err := attachproto.ReadEvent(scanner)
		if err != nil {
			return err
		}
		if err := state.accept(event); err != nil {
			return err
		}
		if event.Type == "accepted" || event.Type == "error" {
			if !inFlight || event.ID != requestID || event.InputID != inbox.ID(inputID) {
				return errors.New("uncorrelated submission reply")
			}
			inFlight = false
			if event.Type == "error" {
				return fmt.Errorf("host %s for input-id %q: %s", event.Code, inputID, event.Error)
			}
		}
		if err := attachproto.Write(output, event); err != nil {
			return err
		}
		if event.Type == "snapshot_end" {
			if err := conn.SetReadDeadline(time.Time{}); err != nil {
				return err
			}
			if submit {
				payload, err := json.Marshal(message)
				if err != nil {
					return err
				}
				input := inbox.Input{ID: inbox.ID(inputID), Kind: inbox.InputExternal, Payload: payload}
				requestID, inFlight = uuid.New().String(), true
				if err := write(attachproto.Request{Type: "submit", ID: requestID, Protocol: attachproto.Version, SessionID: id, Connection: hello.Connection, Input: &input}); err != nil {
					return err
				}
			}
		}
	}
}
