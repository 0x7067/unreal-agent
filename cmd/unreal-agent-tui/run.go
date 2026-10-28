package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"uuid"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/xdgpath"
	"github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/llm/responsesapi"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/settings"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

type options struct {
	provider, model, effort, baseURL string
	version, setup, loadDotEnv       bool
	modelExplicit, effortExplicit    bool
	maxAttempts                      int
	subagents                        bool
	theme                            theme
	workerExecutable, sessionID      string
}

const defaultTheme = "turbo-vision"
const defaultCodexModel = "gpt-6.1-sol"

func parseOptions(args []string, getenv func(string) string, output io.Writer) (options, error) {
	return parseOptionsWithLoader(args, getenv, output, nil)
}

// The loader runs only after flags have been parsed, never for help/version.
func parseOptionsWithLoader(args []string, getenv func(string) string, output io.Writer, loadEnvironment func() error) (options, error) {
	var opts options
	var themeName string
	flags := flag.NewFlagSet("unreal-agent-tui", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&opts.loadDotEnv, "load-dotenv", false, "trust and load workspace .env (may change credential destinations)")
	flags.StringVar(&opts.workerExecutable, "worker-executable", "", "trusted executable path to use for explicit /reload (never selected from workspace config)")
	flags.StringVar(&opts.sessionID, "session", "", "resume this session ID after restarting the frontend")
	flags.BoolVar(&opts.version, "version", false, "print version")
	flags.BoolVar(&opts.setup, "setup", false, "choose and remember Codex model and reasoning settings")
	flags.StringVar(&opts.provider, "provider", getenv("UNREAL_HARNESS_LLM_PROVIDER"), "provider name (default use saved choice or discover Codex subscription)")
	flags.StringVar(&opts.model, "model", getenv("UNREAL_HARNESS_LLM_MODEL"), "model ID (Codex default "+defaultCodexModel+")")
	flags.StringVar(&opts.effort, "effort", getenv("UNREAL_HARNESS_LLM_REASONING_EFFORT"), strings.Join(reasoning.Choices(), ", ")+" (default high for Codex, medium otherwise)")
	flags.StringVar(&opts.baseURL, "base-url", getenv("UNREAL_HARNESS_LLM_BASE_URL"), "API endpoint override")
	flags.IntVar(&opts.maxAttempts, "max-attempts", responsesapi.DefaultMaxAttempts, fmt.Sprintf("request attempts (default %d)", responsesapi.DefaultMaxAttempts))
	flags.BoolVar(&opts.subagents, "subagents", true, "let the agent start subagents with the Agent tool")
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
	if opts.loadDotEnv && loadEnvironment != nil {
		if err := loadEnvironment(); err != nil {
			return opts, err
		}
		// Refresh environment defaults after explicit opt-in, preserving CLI flags.
		explicit := make(map[string]bool)
		flags.Visit(func(option *flag.Flag) { explicit[option.Name] = true })
		for _, item := range []struct {
			flag, env string
			value     *string
		}{
			{"provider", "UNREAL_HARNESS_LLM_PROVIDER", &opts.provider},
			{"model", "UNREAL_HARNESS_LLM_MODEL", &opts.model},
			{"effort", "UNREAL_HARNESS_LLM_REASONING_EFFORT", &opts.effort},
			{"base-url", "UNREAL_HARNESS_LLM_BASE_URL", &opts.baseURL},
		} {
			if !explicit[item.flag] {
				*item.value = getenv(item.env)
			}
		}
	}
	if opts.sessionID != "" {
		for _, c := range opts.sessionID {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return opts, errors.New("session ID must contain only ASCII letters, digits, or hyphens")
			}
		}
	}
	if opts.workerExecutable != "" {
		var err error
		opts.workerExecutable, err = filepath.Abs(opts.workerExecutable)
		if err != nil {
			return opts, fmt.Errorf("resolve trusted worker executable: %w", err)
		}
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

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) (err error) {
	// Attach is dispatched before provider defaults, dotenv, preferences, login,
	// skills or worker startup. It may only connect to an explicitly owned host.
	if len(args) > 0 && args[0] == "attach" {
		return runAttached(ctx, args[1:], getenv, output)
	}

	var environment io.Closer
	defer func() {
		if environment != nil {
			err = errors.Join(err, environment.Close())
		}
	}()
	opts, err := parseOptionsWithLoader(args, getenv, output, func() error {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return err
		}
		environment, err = agentrunner.LoadDotEnv(filepath.Join(workingDirectory, ".env"))
		return err
	})
	if err != nil {
		return err
	}
	if opts.version {
		_, err := fmt.Fprintf(output, "unreal-agent-tui %s (commit %s, built %s)\n", version, commit, date)
		return err
	}
	configDirectory, err := xdgpath.ConfigDirectory(getenv)
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
	key, err := provider.APIKey(getenv)
	if err != nil {
		return err
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
	// Preserve provider/login validation before the startup dialog, but execution
	// belongs exclusively to the resident worker, not this retained frontend.
	if err := client.Close(); err != nil {
		return err
	}
	settingsPath := filepath.Join(configDirectory, "settings.json")
	modelSettings, err := settings.Load(settingsPath)
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
	directory, err := xdgpath.SessionDirectory(getenv)
	if err != nil {
		return err
	}
	if opts.sessionID == "" {
		opts.sessionID = uuid.New().String()
	}
	id := session.ID(opts.sessionID)
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	shell := strings.TrimSpace(getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	skills, skillErrors := tool.DiscoverSkills(filepath.Join(workspace, ".harness", "skills"))
	// Definitions and skill metadata are enough for rendering. Never launch
	// supervisors, operation managers, shell tools, or graph runtimes in the UI.
	registry := newUIRegistry(opts.subagents, len(skills) > 0)
	for _, skill := range skills {
		if _, err := registry.RegisterSkill(skill); err != nil {
			return fmt.Errorf("register skill %q: %w", skill.Name, err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve worker executable: %w", err)
	}
	var candidate []string
	if opts.workerExecutable != "" {
		candidate = []string{opts.workerExecutable, "worker"}
	}
	budget, operationLimit, err := configuredBudgets(getenv)
	if err != nil {
		return fmt.Errorf("operation budgets: %w", err)
	}
	events := newWorkerEvents(runCtx)
	host, err := workerhost.Start(runCtx, workerhost.Config{
		Command: []string{executable, "worker"}, Candidate: candidate,
		Bootstrap: workerproto.Bootstrap{
			SessionID: id, Directory: directory, Workspace: workspace,
			Budget: budget, MaxOperations: operationLimit,
			Provider: opts.provider, BaseURL: opts.baseURL, Model: selectedModel,
			MaxAttempts: opts.maxAttempts, Shell: shell, Subagents: opts.subagents,
			SystemPrompt: "Before starting long-running tools, give the user one brief progress update in the same response as the tool calls. Do not repeat waiting messages.",
		},
		OnItem: events.item, OnStatus: events.status, Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	submissions := newSubmissionQueue(runCtx)
	defer func() {
		submissions.Cancel()
		// The terminal is cooked again once the program has exited, so a second
		// Ctrl+C arrives as SIGINT here. Register only now: a signal that already
		// ended the program must not also skip the graceful worker stop.
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		err = errors.Join(err, shutdown(host, submissions.Wait, signals, os.Stderr, defaultShutdownTiming))
	}()
	ui := newModel(runCtx, host, registry, workspace, directory, opts)
	ui.submissionQueue = submissions
	ui.workerEvents = events
	for _, err := range skillErrors {
		ui.append("Skill error", err.Error())
	}
	screen := &mouseScreen{model: ui}
	program := tea.NewProgram(screen,
		tea.WithContext(ctx), tea.WithoutSignalHandler(), tea.WithInput(input), tea.WithOutput(outputTTY), tea.WithFilter(screen.filter))
	_, uiErr := program.Run()
	if ctx.Err() != nil || errors.Is(uiErr, tea.ErrProgramKilled) {
		uiErr = nil
	}
	return uiErr
}

// newUIRegistry describes capabilities without constructing executable runtimes.
func newUIRegistry(subagents, hasSkills bool) tool.Registry {
	enabled := []string{tool.BashName, tool.ViewImageName, tool.TaskGraphName}
	if hasSkills {
		enabled = append(enabled, tool.SkillUseName)
	}
	if subagents {
		enabled = append(enabled, tool.AgentName, tool.SendMessageName, tool.AgentControlName)
	}
	// These inert translators only decode worker-owned operation state here.
	// Do not construct execution runtimes or submit work in the frontend.
	return tool.NewRegistry(tool.StaticTranslators{
		Bash:         bash.New(bash.Config{}),
		ViewImage:    viewimage.New(viewimage.Config{}),
		Agent:        toolsubagent.NewAgent(),
		SendMessage:  toolsubagent.NewSendMessage(),
		AgentControl: toolsubagent.NewAgentControl(),
		TaskGraph:    tooltaskgraph.New(),
	}, enabled...)
}

// newTaskOperations connects the same registry capabilities to graph and direct work.
func newTaskOperations(ctx context.Context, workspace, operationDirectory, shell string, inputs inbox.Writer, hasSkills bool, newAgentHandlers func(context.Context) ([]operation.RemoteJobHandler, error)) (tool.Registry, operation.Manager, error) {
	enabled := []string{tool.BashName, tool.ViewImageName, tool.TaskGraphName}
	if hasSkills {
		enabled = append(enabled, tool.SkillUseName)
	}
	translators := tool.StaticTranslators{
		Bash:      bash.New(bash.Config{Shell: shell, Directory: workspace, BaseDirectory: operationDirectory}),
		ViewImage: viewimage.New(viewimage.Config{Directory: workspace}), TaskGraph: tooltaskgraph.New(),
	}
	var remoteJobs []operation.RemoteJobHandler
	if newAgentHandlers != nil {
		enabled = append(enabled, tool.AgentName, tool.SendMessageName, tool.AgentControlName)
		translators.Agent, translators.SendMessage = toolsubagent.NewAgent(), toolsubagent.NewSendMessage()
		handlers, err := newAgentHandlers(ctx)
		if err != nil {
			return nil, nil, err
		}
		remoteJobs = append(remoteJobs, handlers...)
	}
	registry := tool.NewRegistry(translators, enabled...)
	_, allowBash := registry.Resolve(tool.BashName)
	runtime, err := taskgraph.New(ctx, taskgraph.Config{Workspace: workspace, Shell: shell, BaseDirectory: operationDirectory, Inbox: inputs, AllowBash: allowBash, NewAgentHandlers: newAgentHandlers})
	if err != nil {
		return nil, nil, err
	}
	remoteJobs = append(remoteJobs, runtime.Handlers()...)
	return registry, operation.NewLocalOperationManagerWithOptions(ctx, operation.LocalOperationManagerOptions{OutputDirectory: operationDirectory}, remoteJobs...), nil
}

// newSubagentFactory captures child configuration and a single caller-owned log.
// Each call creates a supervisor whose channels belong to its operation manager.
func newSubagentFactory(opts options, directory, workspace, operationDirectory string, inputs inbox.Writer) (func(context.Context) ([]operation.RemoteJobHandler, error), *os.File, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve subagent executable: %w", err)
	}
	stderr, err := os.OpenFile(filepath.Join(operationDirectory, "subagents.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open subagent log: %w", err)
	}
	config := subagent.Config{
		Command:          []string{executable, "agent-runner"},
		Env:              append(os.Environ(), "UNREAL_HARNESS_LLM_PROVIDER="+opts.provider, "UNREAL_HARNESS_LLM_BASE_URL="+opts.baseURL, "UNREAL_HARNESS_LLM_MAX_ATTEMPTS="+strconv.Itoa(opts.maxAttempts)),
		SessionDirectory: directory, Workspace: workspace, Model: opts.model, ThinkingLevel: opts.effort, Inbox: inputs, Stderr: stderr,
		DisallowedTools: []string{tool.AgentName, tool.TaskGraphName},
	}
	factory := func(ctx context.Context) ([]operation.RemoteJobHandler, error) {
		supervisor, err := subagent.New(ctx, config)
		if err != nil {
			return nil, err
		}
		return supervisor.Handlers(), nil
	}
	return factory, stderr, nil
}
