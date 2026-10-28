package workerruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablechild"
	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	toolgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
	"time"
)

// Graphs can request fresh supervisors concurrently. Join runtimes first, so no
// factory can still append a supervisor when the final supervisor snapshot is made.
type ownedExecution struct {
	extensions      *configuredExtensions
	echoes          []*extension.Handler
	timersEnabled   bool
	timers          *reminder.Handler
	timerDone       chan struct{}
	stderr          io.Writer
	trustedJobs     *trustedJobs
	jobs            []*durablejob.Handler
	hub             *subagent.Hub
	account         *executionbudget.Account
	mu              sync.Mutex
	workChanged     chan struct{}
	runtimes        []*taskgraph.Runtime
	supervisors     []*subagent.Supervisor
	logs            []*os.File
	durableChildren []*durablechild.Handler
	durableRetained []operation.Operation
}

// pendingWork is authoritative for native supervisors, including supervisors
// allocated lazily by graph factories. Durable independent children retain their
// existing ownership policy and are deliberately not folded into this signal.
func (o *ownedExecution) pendingWork() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, supervisor := range o.supervisors {
		if supervisor.HasPendingWork() {
			return true
		}
	}
	return false
}

func (o *ownedExecution) signalWorkChanged() {
	select {
	case o.workChanged <- struct{}{}:
	default:
	}
}

func (o *ownedExecution) wait() error {
	var result error
	for _, h := range o.echoes {
		result = errors.Join(result, h.Close())
	}
	if o.timers != nil {
		result = errors.Join(result, o.timers.Close())
	}
	if o.extensions != nil && o.extensions.done != nil {
		<-o.extensions.done
	}
	if o.timerDone != nil {
		<-o.timerDone
	}

	for _, h := range o.jobs {
		result = errors.Join(result, h.Close())
	}
	for _, runtime := range o.runtimes {
		result = errors.Join(result, runtime.Wait(context.Background()))
	}
	o.mu.Lock()
	supervisors := append([]*subagent.Supervisor(nil), o.supervisors...)
	o.mu.Unlock()
	for _, supervisor := range supervisors {
		result = errors.Join(result, supervisor.Wait(context.Background()))
	}
	for _, child := range o.durableChildren {
		result = errors.Join(result, child.Wait(context.Background()), child.OwnershipProof())
	}
	if o.hub != nil {
		result = errors.Join(result, o.hub.Wait(context.Background()))
	}
	for _, log := range o.logs {
		result = errors.Join(result, log.Close())
	}
	return result
}

func setupOperations(ctx context.Context, b workerproto.Bootstrap, model llm.Model, directory string, inputs inbox.Writer, hasSkills bool, owners *ownedExecution) (tool.Registry, operation.Manager, error) {
	if owners.workChanged == nil {
		owners.workChanged = make(chan struct{}, 1)
	}
	if owners.account == nil {
		owners.account = executionbudget.NewAccount(executionbudget.Limits{MaxOperations: b.MaxOperations})
	}
	ctx = executionbudget.ContextWithAccount(ctx, owners.account)
	if owners.hub == nil {
		owners.hub = subagent.NewHub(ctx)
	}
	enabled := []string{tool.BashName, tool.ViewImageName, tool.TaskGraphName}
	if hasSkills {
		enabled = append(enabled, tool.SkillUseName)
	}
	translators := tool.StaticTranslators{Bash: bash.New(bash.Config{Shell: b.Shell, Directory: b.Workspace, BaseDirectory: directory}), ViewImage: viewimage.New(viewimage.Config{Directory: b.Workspace}), TaskGraph: tooltaskgraph.New()}
	var factory func(context.Context) ([]operation.RemoteJobHandler, error)
	var handlers []operation.RemoteJobHandler
	if b.Subagents {
		command := append([]string(nil), b.SubagentCommand...)
		if len(command) == 0 {
			executable, err := os.Executable()
			if err != nil {
				return nil, nil, fmt.Errorf("resolve worker executable: %w", err)
			}
			// Never use the frontend's historical pathname after binary replacement.
			command = []string{executable, "agent-runner"}
		}
		log, err := os.OpenFile(filepath.Join(directory, "subagents.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open subagent log: %w", err)
		}
		owners.logs = append(owners.logs, log)
		config := subagent.Config{OnWorkChanged: owners.signalWorkChanged, RequireCleanupProof: b.ControlScope != "", Hub: owners.hub, Command: command, Env: selectedEnvironment(os.Environ(), durablejob.ConfigEnvironment+"=", durablejob.TokenEnvironment+"=", "UNREAL_HARNESS_LLM_PROVIDER="+b.Provider, "UNREAL_HARNESS_LLM_BASE_URL="+b.BaseURL, "UNREAL_HARNESS_LLM_MAX_ATTEMPTS="+strconv.Itoa(b.MaxAttempts), "UNREAL_HARNESS_OPERATION_WARN_AFTER="+b.Budget.WarnAfter.String(), "UNREAL_HARNESS_OPERATION_TIMEOUT="+b.Budget.Timeout.String(), "UNREAL_HARNESS_MAX_OPERATIONS="+strconv.Itoa(b.MaxOperations)), SessionDirectory: b.Directory, Workspace: b.Workspace, Model: model.ID, ThinkingLevel: string(model.ReasoningEffort), Inbox: inputs, Stderr: log, DisallowedTools: []string{tool.AgentName, tool.TaskGraphName}}
		factory = func(ctx context.Context) ([]operation.RemoteJobHandler, error) {
			supervisor, err := subagent.New(ctx, config)
			if err != nil {
				return nil, err
			}
			owners.mu.Lock()
			owners.supervisors = append(owners.supervisors, supervisor)
			owners.mu.Unlock()
			owners.signalWorkChanged()
			return supervisor.Handlers(), nil
		}
		enabled = append(enabled, tool.AgentName, tool.SendMessageName, tool.AgentControlName)
		translators.Agent, translators.SendMessage = toolsubagent.NewAgent(), toolsubagent.NewSendMessage()
		translators.AgentControl = toolsubagent.NewAgentControl()
		handlers, err = factory(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
	if b.DurableSubagents {
		cfg, err := durableOwnerConfig(b)
		if err != nil {
			return nil, nil, err
		}
		handler, err := durablechild.NewHandler(ctx, durablechild.HandlerConfig{Owner: cfg, ParentID: b.SessionID, Hub: owners.hub})
		if err != nil {
			return nil, nil, err
		}
		owners.durableChildren = append(owners.durableChildren, handler)
		for _, op := range owners.durableRetained {
			if durablechild.InertControl(op) {
				continue
			}
			if op.Status == operation.StatusCompleted || op.Status == operation.StatusFailed || op.Status == operation.StatusCanceled {
				if err := handler.AdoptRetained(op); err != nil {
					return nil, nil, err
				}
			}
		}
		handlers = append(handlers, handler)
		if !b.Subagents {
			enabled = append(enabled, tool.AgentName, tool.AgentControlName)
		}
		translators.Agent = toolsubagent.NewDurableAgent(b.SessionID)
		translators.AgentControl = toolsubagent.NewAgentControl()
	}
	if b.DurableGraphs {
		translators.TaskGraph = toolgraph.NewDurable(b.SessionID, graphOwnerScope(b))
	}
	var registry tool.Registry = tool.NewRegistry(translators, enabled...)
	if owners.trustedJobs != nil {
		cfg := owners.trustedJobs.config.Authority(b.SessionID)
		client, err := owners.trustedJobs.config.Client(owners.trustedJobs.token)
		if err != nil {
			return nil, nil, err
		}
		cfg.Client = client
		handler, err := durablejob.New(ctx, cfg)
		if err != nil {
			client.Close()
			return nil, nil, err
		}
		owners.jobs = append(owners.jobs, handler)
		handlers = append(handlers, handler)
		registry = &trustedJobRegistry{Registry: registry, translator: &trustedJobTranslator{config: owners.trustedJobs.config, controller: durablejob.Controller(b.SessionID)}}
	}
	if owners.extensions != nil || owners.timersEnabled {
		composed := &workloadRegistry{Registry: registry, echo: owners.extensions}
		if owners.extensions != nil {
			handler, err := extension.NewHandler(extension.Config{Registry: owners.extensions.registry})
			if err != nil {
				return nil, nil, err
			}
			owners.echoes = append(owners.echoes, handler)
			handlers = append(handlers, handler)
			owners.extensions.watch(ctx, owners.stderr)
		}
		if owners.timersEnabled {
			owners.timers = reminder.NewHandler(reminder.Config{Now: func() time.Time { return time.Now().UTC() }})
			handlers = append(handlers, owners.timers)
			composed.timer = &timerTranslator{parent: string(b.SessionID)}
			owners.timerDone = make(chan struct{})
			go func() {
				defer close(owners.timerDone)
				tick := time.NewTicker(100 * time.Millisecond)
				defer tick.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-tick.C:
						if err := owners.timers.Poll(); err != nil {
							_, _ = fmt.Fprintln(owners.stderr, "timer reconciliation:", err)
						}
					}
				}
			}()
		}
		registry = composed
	}
	_, allowBash := registry.Resolve(tool.BashName)
	graphConfig := taskgraph.Config{Workspace: b.Workspace, Shell: b.Shell, BaseDirectory: directory, Inbox: inputs, AllowBash: allowBash, NewAgentHandlers: factory}
	if b.DurableGraphs {
		graphConfig.DurableParentID = b.SessionID
		graphConfig.DurableScope = graphOwnerScope(b)
		graphConfig.NewAgentHandlers = nil
	}
	runtime, err := taskgraph.New(ctx, graphConfig)
	if err != nil {
		return nil, nil, err
	}
	owners.runtimes = append(owners.runtimes, runtime)
	handlers = append(handlers, runtime.Handlers()...)
	handlers = append(handlers, owners.hub.Handlers()...)
	return registry, operation.NewLocalOperationManagerWithOptions(ctx, operation.LocalOperationManagerOptions{OutputDirectory: directory, Budget: b.Budget, ResourceAccount: owners.account}, handlers...), nil
}

// A stopped coordinator may have canceled requests still returning from their
// transport. Gate late starts and join active calls before closing the client.
// The condition, unlike WaitGroup.Add in Respond, is safe against a goroutine
// that has been created by the coordinator but has not entered Respond yet.
type joinedClient struct {
	client  providers.Client
	mu      sync.Mutex
	idle    *sync.Cond
	active  int
	stopped bool
}

func newJoinedClient(client providers.Client) *joinedClient {
	joined := &joinedClient{client: client}
	joined.idle = sync.NewCond(&joined.mu)
	return joined
}
func (c *joinedClient) Respond(ctx context.Context, request llm.Request, options llm.RequestOptions) (llm.Response, error) {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return llm.Response{}, context.Canceled
	}
	c.active++
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.active--; c.idle.Broadcast(); c.mu.Unlock() }()
	return c.client.Respond(ctx, request, options)
}
func (c *joinedClient) shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	for c.active != 0 {
		c.idle.Wait()
	}
}

// Explicit worker settings override inherited environment exactly once. Keeping
// duplicate keys makes effective child policy platform/runtime-dependent.
func selectedEnvironment(base []string, selected ...string) []string {
	keys := make(map[string]bool, len(selected))
	for _, entry := range selected {
		key, _, _ := strings.Cut(entry, "=")
		keys[key] = true
	}
	result := make([]string, 0, len(base)+len(selected))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if !keys[key] {
			result = append(result, entry)
		}
	}
	return append(result, selected...)
}
