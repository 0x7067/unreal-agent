package workerruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

// Graphs can request fresh supervisors concurrently. Join runtimes first, so no
// factory can still append a supervisor when the final supervisor snapshot is made.
type ownedExecution struct {
	mu          sync.Mutex
	runtimes    []*taskgraph.Runtime
	supervisors []*subagent.Supervisor
	logs        []*os.File
}

func (o *ownedExecution) wait() error {
	var result error
	for _, runtime := range o.runtimes {
		result = errors.Join(result, runtime.Wait(context.Background()))
	}
	o.mu.Lock()
	supervisors := append([]*subagent.Supervisor(nil), o.supervisors...)
	o.mu.Unlock()
	for _, supervisor := range supervisors {
		result = errors.Join(result, supervisor.Wait(context.Background()))
	}
	for _, log := range o.logs {
		result = errors.Join(result, log.Close())
	}
	return result
}

func setupOperations(ctx context.Context, b workerproto.Bootstrap, model llm.Model, directory string, inputs inbox.Writer, hasSkills bool, owners *ownedExecution) (tool.Registry, operation.Manager, error) {
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
		config := subagent.Config{Command: command, Env: append(os.Environ(), "UNREAL_HARNESS_LLM_PROVIDER="+b.Provider, "UNREAL_HARNESS_LLM_BASE_URL="+b.BaseURL, "UNREAL_HARNESS_LLM_MAX_ATTEMPTS="+strconv.Itoa(b.MaxAttempts)), SessionDirectory: b.Directory, Workspace: b.Workspace, Model: model.ID, ThinkingLevel: string(model.ReasoningEffort), Inbox: inputs, Stderr: log, DisallowedTools: []string{tool.AgentName, tool.TaskGraphName}}
		factory = func(ctx context.Context) ([]operation.RemoteJobHandler, error) {
			supervisor, err := subagent.New(ctx, config)
			if err != nil {
				return nil, err
			}
			owners.mu.Lock()
			owners.supervisors = append(owners.supervisors, supervisor)
			owners.mu.Unlock()
			return supervisor.Handlers(), nil
		}
		enabled = append(enabled, tool.AgentName, tool.SendMessageName)
		translators.Agent, translators.SendMessage = toolsubagent.NewAgent(), toolsubagent.NewSendMessage()
		handlers, err = factory(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
	registry := tool.NewRegistry(translators, enabled...)
	_, allowBash := registry.Resolve(tool.BashName)
	runtime, err := taskgraph.New(ctx, taskgraph.Config{Workspace: b.Workspace, Shell: b.Shell, BaseDirectory: directory, Inbox: inputs, AllowBash: allowBash, NewAgentHandlers: factory})
	if err != nil {
		return nil, nil, err
	}
	owners.runtimes = append(owners.runtimes, runtime)
	handlers = append(handlers, runtime.Handlers()...)
	return registry, operation.NewLocalOperationManagerWithOptions(ctx, operation.LocalOperationManagerOptions{OutputDirectory: directory}, handlers...), nil
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
