package main

import (
	"context"
	"encoding/json/v2"
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
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/sessionpath"
	"github.com/unreallabsai/unreal-agent/cmd/xdgpath"
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
	"github.com/unreallabsai/unreal-agent/harness/settings"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

type options struct {
	provider, model, effort, baseURL string
	version, setup                   bool
	modelExplicit, effortExplicit    bool
	maxAttempts                      int
	theme                            theme
}

const defaultTheme = "turbo-vision"
const defaultCodexModel = "gpt-6.1-sol"

func parseOptions(args []string, getenv func(string) string, output io.Writer) (options, error) {
	var opts options
	var themeName string
	flags := flag.NewFlagSet("unreal-agent", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&opts.version, "version", false, "print version")
	flags.BoolVar(&opts.setup, "setup", false, "choose and remember Codex model and reasoning settings")
	flags.StringVar(&opts.provider, "provider", getenv("UNREAL_HARNESS_LLM_PROVIDER"), "provider name (default use saved choice or discover Codex subscription)")
	flags.StringVar(&opts.model, "model", getenv("UNREAL_HARNESS_LLM_MODEL"), "model ID (Codex default "+defaultCodexModel+")")
	flags.StringVar(&opts.effort, "effort", getenv("UNREAL_HARNESS_LLM_REASONING_EFFORT"), strings.Join(reasoning.Choices(), ", ")+" (default high for Codex, medium otherwise)")
	flags.StringVar(&opts.baseURL, "base-url", getenv("UNREAL_HARNESS_LLM_BASE_URL"), "API endpoint override")
	flags.IntVar(&opts.maxAttempts, "max-attempts", responsesapi.DefaultMaxAttempts, fmt.Sprintf("request attempts (default %d)", responsesapi.DefaultMaxAttempts))
	flags.StringVar(&themeName, "theme", defaultTheme, "palette name or JSON path (default "+defaultTheme+")")
	flags.Usage = func() { printHelp(flags, output) }
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if opts.version {
		return opts, nil
	}
	if flags.NArg() != 0 {
		return opts, errors.New("unexpected positional arguments; send prompts inside the TUI")
	}
	opts.modelExplicit = strings.TrimSpace(opts.model) != ""
	opts.effortExplicit = strings.TrimSpace(opts.effort) != ""
	flags.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "model":
			opts.modelExplicit = true
		case "effort":
			opts.effortExplicit = true
		}
	})
	opts.provider = strings.ToLower(strings.TrimSpace(opts.provider))
	if opts.setup && opts.provider != "" && opts.provider != "openai-codex" {
		return opts, errors.New("-setup requires the openai-codex provider; omit -provider or use -provider openai-codex")
	}
	providerName := opts.provider
	if providerName == "" {
		providerName = "openai-codex"
	}
	provider, err := providers.Find(providers.Default(), providerName)
	if err != nil {
		return opts, err
	}
	opts.model, opts.baseURL = strings.TrimSpace(opts.model), strings.TrimSpace(opts.baseURL)
	if opts.model == "" {
		opts.model = provider.DefaultModel
		if providerName == "openai-codex" {
			opts.model = defaultCodexModel
		}
		if opts.model == "" {
			return opts, fmt.Errorf("provider %q requires -model or UNREAL_HARNESS_LLM_MODEL\n\n%s", providerName, launchExamples)
		}
	}
	if providerName == "openai-codex" && (strings.TrimSpace(opts.effort) == "" || strings.EqualFold(strings.TrimSpace(opts.effort), "default")) {
		opts.effort = "high"
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
	if opts.version {
		_, err := fmt.Fprintf(output, "unreal-agent %s (commit %s, built %s)\n", version, commit, date)
		return err
	}
	configDirectory, err := xdgpath.Directory(getenv)
	if err != nil {
		return fmt.Errorf("resolve settings directory: %w", err)
	}
	var preferences startupPreferences
	if opts.provider == "" || opts.provider == "openai-codex" {
		preferences, err = loadStartupPreferences(filepath.Join(configDirectory, "preferences.json"))
		if err != nil && !opts.setup {
			return err
		}
	}
	discoverSubscription := opts.provider == "" || opts.setup
	opts, showSubscriptionDialog := startupSelection(opts, preferences)
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
		if discoverSubscription {
			return fmt.Errorf("no usable Codex subscription login found: %w\n\n%s", err, launchExamples)
		}
		return err
	}
	defer func() { _ = client.Close() }()
	modelSettings, err := settings.Load(filepath.Join(configDirectory, "settings.json"))
	if err != nil {
		return fmt.Errorf("load model settings: %w", err)
	}
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
	if showSubscriptionDialog {
		chosen, accepted, err := confirmSubscription(ctx, opts, modelSettings, input, outputTTY)
		if err != nil {
			return err
		}
		if !accepted {
			_, err := fmt.Fprintln(output, launchExamples)
			return err
		}
		opts = chosen
		if err := saveStartupPreferences(filepath.Join(configDirectory, "preferences.json"), startupPreferences{
			Provider: opts.provider, Model: opts.model, ReasoningEffort: llm.ReasoningEffort(opts.effort),
		}); err != nil {
			return err
		}
	}
	selectedModel := llm.Model{ID: opts.model, ReasoningEffort: llm.ReasoningEffort(opts.effort),
		CompactionThreshold: modelSettings.Model(provider.Name, opts.model).CompactionThreshold}
	workspace, err := os.Getwd()
	if err != nil {
		return err
	}
	directory, err := sessionpath.Resolve(getenv)
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
	settingsPayload, err := json.Marshal(inbox.ControlMessage{
		Mode:       inbox.UpdateSettings,
		Parameters: inbox.Settings{Model: opts.model, CompactionThreshold: new(selectedModel.CompactionThreshold), ReasoningEffort: llm.ReasoningEffort(opts.effort)},
	})
	if err != nil {
		return err
	}
	if err := inputs.Submit(runCtx, inbox.Input{
		ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: settingsPayload,
	}); err != nil {
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
	operationDirectory := filepath.Join(directory, "operations", string(id))
	if err := os.MkdirAll(operationDirectory, 0o700); err != nil {
		return err
	}
	skills, skillErrors := tool.DiscoverSkills(filepath.Join(workspace, ".harness", "skills"))
	enabled := []string{tool.BashName, tool.ViewImageName}
	if len(skills) > 0 {
		enabled = append(enabled, tool.SkillUseName)
	}
	registry := tool.NewRegistry(tool.StaticTranslators{
		Bash:      bash.New(bash.Config{Shell: shell, Directory: workspace, BaseDirectory: operationDirectory}),
		ViewImage: viewimage.New(viewimage.Config{Directory: workspace}),
	}, enabled...)
	for _, skill := range skills {
		if _, err := registry.RegisterSkill(skill); err != nil {
			return fmt.Errorf("register skill %q: %w", skill.Name, err)
		}
	}
	builder := contextbuilder.NewBuilder(skills...)
	builder.SetSystemPrompt("Before starting long-running tools, give the user one brief progress update in the same response as the tool calls. Do not repeat waiting messages.")
	builder.SetModel(selectedModel)
	for _, definition := range registry.StaticDefinitions() {
		builder.AddTool(definition.Tool)
	}
	current := coordinator.New(coordinator.Dependencies{
		SessionID: id, Restored: sessionstore.ResumeState{Snapshot: snapshot}, Sessions: store,
		Inbox: inputs, Operations: operations, Tools: registry, ContextBuilder: builder, LLM: client,
		ToolHeartbeatInterval: 10 * time.Minute,
	})
	ui := newModel(runCtx, inputs, registry, workspace, directory, opts)
	for _, err := range skillErrors {
		ui.append("Skill error", err.Error())
	}
	screen := &mouseScreen{model: ui}
	program := tea.NewProgram(screen,
		tea.WithContext(ctx), tea.WithoutSignalHandler(), tea.WithInput(input), tea.WithOutput(outputTTY), tea.WithFilter(screen.filter))
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
	runErr := runcontrol.Stop(stopCtx, inputs, done, "Unreal Agent exited")
	cancel()
	for range done {
	}
	if ctx.Err() != nil || errors.Is(uiErr, tea.ErrProgramKilled) {
		uiErr = nil
	}
	return errors.Join(runErr, uiErr)
}
