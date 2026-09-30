package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/runcontrol"
	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/coordinator"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/llm/providers"
	"github.com/unreallabsai/unreal-agent/harness/llm/responsesapi"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

type options struct {
	provider, model, effort, baseURL string
	maxAttempts                      int
	theme                            theme
	animations                       bool
}

func parseOptions(args []string, getenv func(string) string, output io.Writer) (options, error) {
	var opts options
	var themeName string
	var noAnimations bool
	flags := flag.NewFlagSet("unreal-agent-lite", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&opts.provider, "provider", getenv("UNREAL_HARNESS_LLM_PROVIDER"), "provider name (default openai)")
	flags.StringVar(&opts.model, "model", getenv("UNREAL_HARNESS_LLM_MODEL"), "model ID (required)")
	flags.StringVar(&opts.effort, "effort", getenv("UNREAL_HARNESS_LLM_REASONING_EFFORT"), strings.Join(reasoning.Choices(), ", ")+" (default medium)")
	flags.StringVar(&opts.baseURL, "base-url", getenv("UNREAL_HARNESS_LLM_BASE_URL"), "API endpoint override")
	flags.IntVar(&opts.maxAttempts, "max-attempts", responsesapi.DefaultMaxAttempts, fmt.Sprintf("request attempts (default %d)", responsesapi.DefaultMaxAttempts))
	flags.StringVar(&themeName, "theme", "default", "palette name or JSON path")
	flags.BoolVar(&noAnimations, "no-animations", false, "disable reveal and background animations")
	flags.Usage = func() { printHelp(flags, output) }
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	opts.animations = !noAnimations
	if flags.NArg() != 0 {
		return opts, errors.New("unexpected positional arguments; send prompts inside the TUI")
	}
	opts.provider = strings.ToLower(strings.TrimSpace(opts.provider))
	if opts.provider == "" {
		opts.provider = "openai"
	}
	opts.model, opts.baseURL = strings.TrimSpace(opts.model), strings.TrimSpace(opts.baseURL)
	if opts.model == "" {
		return opts, errors.New("set -model or UNREAL_HARNESS_LLM_MODEL")
	}
	effort, err := reasoning.Parse(opts.effort)
	if err != nil {
		return opts, err
	}
	opts.effort = string(effort)
	if opts.maxAttempts < 1 {
		return opts, errors.New("max-attempts must be positive")
	}
	opts.theme, err = loadTheme(themeName)
	return opts, err
}

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) error {
	opts, err := parseOptions(args, getenv, output)
	if err != nil {
		return err
	}
	provider, err := providers.Find(providers.Default(), opts.provider)
	if err != nil {
		return err
	}
	key := ""
	if provider.APIKeyEnvironment != "" {
		key = strings.TrimSpace(getenv("UNREAL_HARNESS_LLM_API_KEY"))
		if key == "" {
			key = strings.TrimSpace(getenv(provider.APIKeyEnvironment))
		}
		if key == "" {
			return fmt.Errorf("set %s or UNREAL_HARNESS_LLM_API_KEY", provider.APIKeyEnvironment)
		}
	}
	if opts.baseURL == "" {
		opts.baseURL = provider.BaseURL
	}
	client, err := provider.NewClient(key, opts.baseURL, opts.maxAttempts, getenv)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	input, outputTTY, err := tea.OpenTTY()
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	defer func() {
		_ = input.Close()
		if input != outputTTY {
			_ = outputTTY.Close()
		}
	}()
	workspace, err := os.Getwd()
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "unreal-agent-lite-")
	if err != nil {
		return err
	}
	store, err := localfile.New(directory)
	if err != nil {
		return err
	}
	id := session.ID(uuid.New().String())
	snapshot, err := store.Create(ctx, id)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	inputs, err := inbox.New(runCtx, nil)
	if err != nil {
		return err
	}
	operations := operation.NewLocalOperationManager(runCtx)
	defer func() {
		cancel()
		for range operations.Updates() {
		}
	}()
	shell := strings.TrimSpace(getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	operationDirectory := filepath.Join(directory, "operations")
	if err := os.MkdirAll(operationDirectory, 0o700); err != nil {
		return err
	}
	registry := tool.NewRegistry(tool.StaticTranslators{
		Bash:      bash.New(bash.Config{Shell: shell, Directory: workspace, BaseDirectory: operationDirectory}),
		ViewImage: viewimage.New(viewimage.Config{Directory: workspace}),
	}, tool.BashName, tool.ViewImageName)
	builder := contextbuilder.NewBuilder()
	builder.SetModel(llm.Model{ID: opts.model, ReasoningEffort: llm.ReasoningEffort(opts.effort)})
	for _, definition := range registry.StaticDefinitions() {
		builder.AddTool(definition.Tool)
	}
	current := coordinator.New(coordinator.Dependencies{
		SessionID: id, Restored: sessionstore.ResumeState{Snapshot: snapshot}, Sessions: store,
		Inbox: inputs, Operations: operations, Tools: registry, ContextBuilder: builder, LLM: client,
		ToolHeartbeatInterval: 10 * time.Minute,
	})
	ui := newModel(runCtx, inputs, registry, workspace, directory, opts)
	ui.entranceStarted = time.Now()
	program := tea.NewProgram(ui,
		tea.WithContext(ctx), tea.WithoutSignalHandler(), tea.WithInput(input), tea.WithOutput(outputTTY), tea.WithFilter(filterMouseWheel))
	observer := store.AddObserver(func(_ session.ID, item sessionstore.Item) { program.Send(item) })
	defer store.RemoveObserver(observer)
	done := make(chan error, 1)
	go func() {
		err := current.Run(runCtx)
		cancel()
		done <- err
		close(done)
		program.Send(runEnded{err: err})
	}()
	_, uiErr := program.Run()
	stopCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	runErr := runcontrol.Stop(stopCtx, inputs, done, "Unreal Agent Lite exited")
	cancel()
	for range done {
	}
	if ctx.Err() != nil || errors.Is(uiErr, tea.ErrProgramKilled) {
		uiErr = nil
	}
	return errors.Join(runErr, uiErr)
}
