package taskgraph

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Config supplies execution capabilities. Agent handlers must be fresh for each
// graph because every local manager owns its handlers' update channels.
type Config struct {
	Workspace     string
	Shell         string
	BaseDirectory string
	Inbox         inbox.Writer
	AllowBash     bool
	// Budget and ResourceAccount are live execution policy, not reducer state or
	// checkpoint data. The account remains owned until actual manager drain.
	Budget           executionbudget.Policy
	ResourceAccount  *executionbudget.Account
	NewAgentHandlers func(context.Context) ([]operation.RemoteJobHandler, error)
	// scanHook is a private deterministic test gate, run only on the scan worker.
	scanHook func(context.Context, string) error
}
type Runtime struct {
	ctx          context.Context
	config       Config
	requests     *runtimeRequests
	done         chan struct{}
	scans        sync.WaitGroup
	scanResults  chan scanResult
	run, control *runtimeHandler
}
type runtimeHandler struct {
	runtime *Runtime
	typ     operation.RemoteJobPlanType
	updates chan operation.Operation
	queue   []operation.Operation
}

func (h *runtimeHandler) RemoteJobPlanType() operation.RemoteJobPlanType       { return h.typ }
func (h *runtimeHandler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion { return PlanVersion }
func (h *runtimeHandler) RemoteJobUpdates() <-chan operation.Operation         { return h.updates }
func (h *runtimeHandler) AddRemoteJob(op operation.Operation) error {
	return h.runtime.request(func(s *runtimeState) { s.add(h, op) })
}
func (h *runtimeHandler) CancelRemoteJob(id operation.ID, reason string) error {
	return h.runtime.request(func(s *runtimeState) { s.cancelOperation(h, id, reason) })
}
func (r *Runtime) request(fn func(*runtimeState)) error {
	return r.requests.submit(fn)
}
func (r *Runtime) Handlers() []operation.RemoteJobHandler {
	return []operation.RemoteJobHandler{r.run, r.control}
}
func New(ctx context.Context, c Config) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("runtime context required")
	}
	if err := c.Budget.Validate(); err != nil {
		return nil, err
	}
	if c.ResourceAccount == nil {
		c.ResourceAccount = executionbudget.AccountFromContext(ctx)
	}
	if c.Workspace == "" || c.BaseDirectory == "" {
		return nil, errors.New("workspace and operation base directory required")
	}
	root, err := filepath.Abs(c.Workspace)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("workspace is not a directory")
	}
	c.Workspace = root
	base, err := filepath.Abs(c.BaseDirectory)
	if err != nil {
		return nil, err
	}
	c.BaseDirectory = base
	if c.Shell == "" {
		c.Shell = "/bin/sh"
	}
	r := &Runtime{ctx: ctx, config: c, requests: newRuntimeRequests(ctx), done: make(chan struct{}), scanResults: make(chan scanResult, 1)}
	r.run = &runtimeHandler{runtime: r, typ: RunPlanType, updates: make(chan operation.Operation)}
	r.control = &runtimeHandler{runtime: r, typ: ControlPlanType, updates: make(chan operation.Operation)}
	go r.loop()
	return r, nil
}

// Wait waits for actual runtime termination. It does not cancel the runtime.
// Completion includes the mailbox, executing callbacks, every nested operation
// manager (including terminal graphs), filesystem scan workers, and
// lifecycle-aware remote handlers.
// A callback or handler that ignores cancellation can keep Wait blocked; ctx
// bounds only this wait, never reports unfinished work as complete.
func (r *Runtime) Wait(ctx context.Context) error {
	select {
	case <-r.done:
		return nil
	default:
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// A manager's forwarding goroutines do not own the underlying remote handler
// actors or their children. Lifecycle-aware handlers must be joined separately.
type runtimeWaiter interface {
	Wait(context.Context) error
}

type attempt struct {
	Generation        uint64              `json:"generation"`
	ChildOperation    operation.Operation `json:"child_operation"`
	BeforeReads       string              `json:"before_reads"`
	OutputFingerprint string              `json:"output_fingerprint,omitempty"`
	Blocker           string              `json:"blocker,omitempty"`
	Phase             string              `json:"phase,omitempty"`
	CommandDispatched bool                `json:"command_dispatched"`
	FailureCategory   string              `json:"failure_category,omitempty"`
	FailureReason     string              `json:"failure_reason,omitempty"`
	ExitCode          *int                `json:"exit_code,omitempty"`
	StartedAt         time.Time           `json:"started_at,omitzero"`
	PhaseStartedAt    time.Time           `json:"phase_started_at,omitzero"`
	FinishedAt        time.Time           `json:"finished_at,omitzero"`
}
type checkpoint struct {
	Version   int                 `json:"version"`
	Graph     Graph               `json:"graph"`
	Attempts  map[string]*attempt `json:"attempts"`
	Canceling bool                `json:"canceling,omitempty"`
}
type graphExecution struct {
	checkpoint
	owner          operation.Operation
	manager        operation.Manager
	stop           context.CancelFunc
	cancelControls []operation.Operation
	terminal       bool
	epoch          uint64
	pendingAccept  *acceptRequest
	acceptQueue    []*acceptRequest
	finalFresh     bool
}
type runtimeState struct {
	runtime  *Runtime
	graphs   map[string]*graphExecution
	active   *graphExecution
	seen     map[operation.ID]bool
	waiters  []runtimeWaiter
	scanning *scanJob
}

func (r *Runtime) loop() {
	s := &runtimeState{runtime: r, graphs: map[string]*graphExecution{}, seen: map[operation.ID]bool{}}
	defer close(r.done)
	defer close(r.run.updates)
	defer close(r.control.updates)
	defer func() {
		if s.scanning != nil {
			s.scanning.cancel()
		}
		// Stop every execution before ANY join, including scans. Terminal graphs
		// leave s.active but can still own shutting-down managers/handlers.
		for _, g := range s.graphs {
			g.stop()
		}
		if s.active != nil {
			// Cancel may block behind a handler; all nested contexts are now stopped.
			s.cancelGraph(s.active)
		}
		r.scans.Wait()
		for _, g := range s.graphs {
			// Manager updates close only after dispatched primitives and its
			// remote forwarding goroutines have finished. Drain, don't wait
			// on a forwarder alone or depend on an external update consumer.
			for range g.manager.Updates() {
			}
		}
		for _, waiter := range s.waiters {
			// The runtime's cancellation must not short-circuit child joins.
			_ = waiter.Wait(context.Background())
		}
		<-r.requests.done
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	done := r.ctx.Done()
	for {
		if r.ctx.Err() != nil {
			return
		}
		var runOut, controlOut chan operation.Operation
		var runOp, controlOp operation.Operation
		if len(r.run.queue) > 0 {
			runOut = r.run.updates
			runOp = r.run.queue[0]
		}
		if len(r.control.queue) > 0 {
			controlOut = r.control.updates
			controlOp = r.control.queue[0]
		}
		var children <-chan operation.Operation
		if s.active != nil {
			children = s.active.manager.Updates()
		}
		select {
		case fn := <-r.requests.output:
			if r.ctx.Err() == nil {
				fn(s)
			}
		case runOut <- runOp:
			r.run.queue = r.run.queue[1:]
		case controlOut <- controlOp:
			r.control.queue = r.control.queue[1:]
		case result := <-r.scanResults:
			s.scanned(result)
		case child, ok := <-children:
			if ok {
				s.child(child)
			}
		case <-tick.C:
			if s.active != nil {
				if s.probe(s.active) {
					s.advance(s.active)
				}
			}
		case <-done:
			// The defer stops ALL nested contexts before cancellation calls or joins.
			return
		}
	}
}
func terminal(status operation.Status) bool {
	return status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled || status == operation.StatusUnsupported
}
func (s *runtimeState) emit(h *runtimeHandler, op operation.Operation, status operation.Status, cp *checkpoint, result string, err error) {
	state, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		return
	}
	if cp != nil {
		state.Handle, e = json.Marshal(cp)
		if e != nil {
			return
		}
	}
	state.ResultTruncated, state.ErrorTruncated = false, false
	state.TerminalResult = result
	if err != nil {
		state.TerminalError = err.Error()
	} else {
		state.TerminalError = ""
	}
	step, e := operation.UpdateRemoteJob(op, state, status)
	if e == nil {
		h.queue = append(h.queue, *step.Operation)
	}
}
func (s *runtimeState) permissions(tasks []Task) error {
	for _, t := range tasks {
		if strings.TrimSpace(t.Command) != "" && !s.runtime.config.AllowBash {
			return errors.New("shell tasks require Bash capability")
		}
		if strings.TrimSpace(t.Prompt) != "" && s.runtime.config.NewAgentHandlers == nil {
			return errors.New("agent tasks require agent capability")
		}
	}
	return nil
}
func (s *runtimeState) add(h *runtimeHandler, op operation.Operation) {
	if s.seen[op.ID] {
		return
	}
	s.seen[op.ID] = true
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return
	}
	if state.Plan.Type != h.typ || state.Plan.Version != PlanVersion {
		s.emit(h, op, operation.StatusFailed, nil, "", errors.New("unsupported graph plan version or type"))
		return
	}
	if h.typ == ControlPlanType {
		var p ControlPlan
		err = json.Unmarshal(state.Plan.Data, &p)
		if err == nil {
			err = s.control(op, p)
		}
		if err != nil {
			s.emit(h, op, operation.StatusFailed, nil, "", err)
		}
		return
	}
	var p RunPlan
	if err = json.Unmarshal(state.Plan.Data, &p); err != nil {
		s.emit(h, op, operation.StatusFailed, nil, "", err)
		return
	}
	if s.active != nil {
		s.emit(h, op, operation.StatusFailed, nil, "", errors.New("another graph is active"))
		return
	}
	if _, exists := s.graphs[p.Name]; exists {
		s.emit(h, op, operation.StatusFailed, nil, "", errors.New("graph name already exists"))
		return
	}
	cp := checkpoint{Version: 1, Attempts: map[string]*attempt{}}
	if len(state.Handle) > 0 {
		err = json.Unmarshal(state.Handle, &cp)
		if err == nil && cp.Version != 1 {
			err = errors.New("unsupported graph checkpoint version")
		}
		if err == nil {
			err = cp.Graph.Validate()
		}
		if err == nil && cp.Graph.Name != p.Name {
			err = errors.New("checkpoint graph name mismatch")
		}
	} else {
		cp.Graph, err = NewGraph(p.Name, p.Tasks, p.Concurrency)
	}
	if err == nil {
		tasks := make([]Task, len(cp.Graph.Nodes))
		for i, n := range cp.Graph.Nodes {
			tasks[i] = n.Task
		}
		err = s.permissions(tasks)
	}
	if err == nil {
		err = validateAttempts(cp)
	}
	if err != nil {
		s.emit(h, op, operation.StatusFailed, nil, "", err)
		return
	}
	nestedCtx, stop := context.WithCancel(executionbudget.ContextWithAccount(context.WithoutCancel(s.runtime.ctx), s.runtime.config.ResourceAccount))
	var handlers []operation.RemoteJobHandler
	if s.runtime.config.NewAgentHandlers != nil {
		handlers, err = s.runtime.config.NewAgentHandlers(nestedCtx)
	}
	// Retain joins even when setup returns an error after creating handlers.
	for _, handler := range handlers {
		if waiter, ok := handler.(runtimeWaiter); ok {
			s.waiters = append(s.waiters, waiter)
		}
	}
	if err != nil {
		stop()
		s.emit(h, op, operation.StatusFailed, nil, "", err)
		return
	}
	g := &graphExecution{checkpoint: cp, owner: op, manager: operation.NewLocalOperationManagerWithOptions(nestedCtx, operation.LocalOperationManagerOptions{OutputDirectory: s.runtime.config.BaseDirectory, Budget: s.runtime.config.Budget, ResourceAccount: s.runtime.config.ResourceAccount}, handlers...), stop: stop}
	s.graphs[p.Name] = g
	s.active = g
	for _, n := range g.Graph.Nodes {
		if a := g.Attempts[n.Task.ID]; a != nil && a.Generation == n.Generation && a.Phase == "" {
			// Older checkpoints cannot prove a prelaunch boundary. Never replay Ready.
			a.CommandDispatched = true
			a.Phase = "execution"
			if n.Status == Completed || n.Status == Accepted {
				a.Phase = "awaiting_acceptance"
			}
		}
		if !active(&n) {
			if n.Status == Completed || n.Status == Failed {
				s.notify(g, g.Graph.Find(n.Task.ID))
			}
			continue
		}
		a := g.Attempts[n.Task.ID]
		child := a.ChildOperation
		if a.Phase == "input_validation" && !a.CommandDispatched {
			// This checkpoint proves no child was dispatched. Revalidate, never
			// reinterpret an old Ready child without this explicit marker as safe.
			if n.Status == Canceling {
				g.Graph.Canceled(n.Task.ID, n.Generation)
			}
		} else if terminal(child.Status) {
			a.CommandDispatched = true
			s.child(child)
		} else if child.Type == operation.TypeShell {
			a.CommandDispatched = true
			state, err := operation.DecodeShellState(child)
			// Only output collection proves the command already exited. A
			// persisted Ready or preparation phase can lag a live launch.
			if err == nil && shellOutputPhase(state.Phase) {
				if n.Status == Canceling {
					child.Status = operation.StatusCanceling
				}
				g.manager.Add(child)
			} else {
				a.Blocker = "resumed shell outcome unknown; reservation retained"
				s.notify(g, g.Graph.Find(n.Task.ID))
			}
		} else {
			a.CommandDispatched = true
			a.Blocker = "resumed child outcome unknown; reservation retained"
			if child.Type == operation.TypeRemoteJob {
				a.Blocker = "resumed agent has no process reattachment proof; reservation retained"
			}
			s.notify(g, g.Graph.Find(n.Task.ID))
		}
	}
	s.probe(g)
	s.advance(g)
}
func shellOutputPhase(phase operation.ShellPhase) bool {
	switch phase {
	case operation.ShellPhaseReadOut, operation.ShellPhaseReadOutTail, operation.ShellPhaseReadErr, operation.ShellPhaseReadErrTail:
		return true
	default:
		return false
	}
}

// JSON round-trips empty byte slices as non-nil; both represent no output.
func initialShellState(shell operation.ShellState) bool {
	if len(shell.InlineOut)+len(shell.InlineErr)+len(shell.InlineOutTail)+len(shell.InlineErrTail) != 0 {
		return false
	}
	shell.InlineOut, shell.InlineErr, shell.InlineOutTail, shell.InlineErrTail = nil, nil, nil, nil
	return reflect.DeepEqual(shell, operation.ShellState{
		Input: shell.Input, BaseDirectory: shell.BaseDirectory,
		OutTruncated: true, ErrTruncated: true,
	})
}

func validateAttempts(cp checkpoint) error {
	if cp.Attempts == nil {
		return errors.New("checkpoint lacks attempt map")
	}
	currentChildren := map[operation.ID]string{}
	for _, n := range cp.Graph.Nodes {
		a := cp.Attempts[n.Task.ID]
		if a != nil && a.Generation == n.Generation {
			switch a.Phase {
			case "": // Version 1 checkpoints before structured diagnostics.
			case "input_validation":
				if a.CommandDispatched || a.ChildOperation.Status != operation.StatusReady || a.BeforeReads != "" || a.OutputFingerprint != "" || n.Status == Completed || n.Status == Accepted {
					return fmt.Errorf("task %s has inconsistent prelaunch checkpoint", n.Task.ID)
				}
				if a.ChildOperation.Type == operation.TypeShell {
					shell, err := operation.DecodeShellState(a.ChildOperation)
					if err != nil || !initialShellState(shell) {
						return fmt.Errorf("task %s prelaunch shell has execution state", n.Task.ID)
					}
				} else if a.ChildOperation.Type == operation.TypeRemoteJob {
					remote, err := operation.DecodeRemoteJobState(a.ChildOperation)
					if err != nil || !reflect.DeepEqual(remote, operation.RemoteJobState{Plan: remote.Plan}) {
						return fmt.Errorf("task %s prelaunch agent has execution state", n.Task.ID)
					}
				}
			case "execution", "output_validation", "awaiting_acceptance":
				if !a.CommandDispatched {
					return fmt.Errorf("task %s execution phase lacks dispatch marker", n.Task.ID)
				}
			default:
				return fmt.Errorf("task %s has unknown attempt phase %q", n.Task.ID, a.Phase)
			}
			// child routes updates by operation ID among current attempts. An
			// obsolete attempt cannot receive them and does not reserve its ID.
			if owner, exists := currentChildren[a.ChildOperation.ID]; exists {
				return fmt.Errorf("tasks %s and %s have duplicate current child operation ID %q", owner, n.Task.ID, a.ChildOperation.ID)
			}
			currentChildren[a.ChildOperation.ID] = n.Task.ID
			var err error
			switch a.ChildOperation.Type {
			case operation.TypeShell:
				_, err = operation.NewShell(a.ChildOperation)
				if strings.TrimSpace(n.Task.Command) == "" {
					err = errors.New("shell child belongs to an agent task")
				}
			case operation.TypeRemoteJob:
				var remote operation.RemoteJobState
				remote, err = operation.DecodeRemoteJobState(a.ChildOperation)
				if err == nil && (remote.Plan.Type != toolsubagent.RunPlanType || remote.Plan.Version != toolsubagent.PlanVersion || strings.TrimSpace(n.Task.Prompt) == "") {
					err = errors.New("invalid agent child plan")
				}
			default:
				err = errors.New("unsupported child operation checkpoint")
			}
			if err != nil {
				return fmt.Errorf("task %s child checkpoint: %w", n.Task.ID, err)
			}
		}
		if active(&n) || n.Status == Completed || n.Status == Accepted {
			if a == nil || a.Generation != n.Generation || a.ChildOperation.ID == "" {
				return fmt.Errorf("task %s lacks current child checkpoint", n.Task.ID)
			}
		}
		if a != nil && a.Generation == n.Generation && (n.Status == Completed || n.Status == Accepted) && a.OutputFingerprint == "" {
			return fmt.Errorf("task %s lacks result fingerprint", n.Task.ID)
		}
	}
	return nil
}
func (s *runtimeState) save(g *graphExecution) {
	s.emit(s.runtime.run, g.owner, operation.StatusAwaiting, &g.checkpoint, strings.Join(planWarnings(g.Graph), "; "), nil)
}
func (s *runtimeState) notify(g *graphExecution, n *Node) {
	if s.runtime.config.Inbox == nil {
		return
	}
	text := fmt.Sprintf("Task graph %s task %s generation %d: %s. Candidate result: %s. Acceptance conditions: %s. Evidence: %s. Blockers: %s", g.Graph.Name, n.Task.ID, n.Generation, n.Status, n.Result, n.Task.Acceptance, n.Evidence, s.blockers(g))
	if a := g.Attempts[n.Task.ID]; a != nil {
		text += fmt.Sprintf(". Phase: %s. Command dispatched: %t. Failure: %s: %s", a.Phase, a.CommandDispatched, a.FailureCategory, a.FailureReason)
	}
	payload, _ := json.Marshal(text)
	id := fmt.Sprintf("taskgraph-%s-%s-%d-%s", g.owner.ID, n.Task.ID, n.Generation, n.Status)
	// Inbox actor owns deduplication; submitting this notification precedes its
	// corresponding durable checkpoint so a resume can redeliver the same ID.
	s.runtime.config.Inbox.Submit(s.runtime.ctx, inbox.Input{ID: inbox.ID(id), Kind: inbox.InputExternal, Payload: payload})
}
func (s *runtimeState) blockers(g *graphExecution) string {
	var out []string
	for _, n := range g.Graph.Nodes {
		if n.Status != Accepted {
			reason := n.Status
			if a := g.Attempts[n.Task.ID]; a != nil && a.Blocker != "" {
				reason = a.Blocker
			} else if a := g.Attempts[n.Task.ID]; a != nil && a.FailureReason != "" {
				reason = a.FailureCategory + ": " + a.FailureReason
			} else if wait := g.Graph.WaitReason(n.Task.ID); wait != "" {
				reason = wait
			}
			out = append(out, n.Task.ID+": "+reason)
		}
	}
	return strings.Join(out, "; ")
}
func unionClaims(a, b []string) []string {
	if a == nil || b == nil {
		return nil
	}
	return append(append([]string{}, a...), b...)
}

// Read-only inputs exclude declared writable subtrees. Expanding directories
// on each check detects added and removed input files as well as changed bytes.
func readOnlyFingerprint(workspace string, t Task) (string, error) {
	return readOnlyFingerprintContext(context.Background(), workspace, t)
}
func readOnlyFingerprintContext(ctx context.Context, workspace string, t Task) (string, error) {
	if len(claims(t.Reads)) == 0 {
		return fingerprintContext(ctx, workspace, []string{})
	}
	paths := []string{}
	directories := map[string]bool{}
	for _, read := range claims(t.Reads) {
		err := filepath.WalkDir(filepath.Join(workspace, filepath.FromSlash(read)), func(full string, d fs.DirEntry, e error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			rel, err := filepath.Rel(workspace, full)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if d != nil && d.Name() == ".git" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			for _, write := range claims(t.Writes) {
				if overlaps(write, rel) && (write == "." || strings.EqualFold(write, rel) || strings.HasPrefix(strings.ToLower(rel), strings.ToLower(write)+"/")) {
					if d != nil && d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if e != nil {
				if os.IsNotExist(e) {
					paths = append(paths, rel)
					return nil
				}
				return e
			}
			if d.IsDir() {
				directories[rel] = true
				return nil
			}
			paths = append(paths, rel)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	fingerprint, err := fingerprintContext(ctx, workspace, paths)
	if err != nil {
		return "", err
	}
	// Retain directory membership even when a directory has no readable files.
	names := make([]string, 0, len(directories))
	for name := range directories {
		names = append(names, name)
	}
	sort.Strings(names)
	encoded, err := json.Marshal(names)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(fingerprint), encoded...))
	return fmt.Sprintf("%x", sum), nil
}
func (s *runtimeState) newChild(g *graphExecution, n *Node) (operation.Operation, error) {
	key := fmt.Sprintf("%s/%s/%d", g.owner.ID, n.Task.ID, n.Generation)
	digest := sha256.Sum256([]byte(key))
	id := fmt.Sprintf("tg-%x", digest[:16])
	var spec operation.Spec
	var err error
	if strings.TrimSpace(n.Task.Command) != "" {
		spec, err = operation.NewShellSpec(operation.ShellInput{Command: n.Task.Command, Shell: s.runtime.config.Shell, Directory: s.runtime.config.Workspace}, s.runtime.config.BaseDirectory, operation.DefaultMaxOutputLength)
	} else {
		declaration, e := json.Marshal(n.Task)
		if e != nil {
			return operation.Operation{}, e
		}
		prompt := "Execute only this graph task's declared work. Respect the declared read and write ownership; do not edit outside its writes. Your final result is a candidate requiring parent acceptance. Task declaration: " + string(declaration)
		for _, dependency := range n.Task.DependsOn {
			parent := g.Graph.Find(dependency)
			prompt += fmt.Sprintf("\nAccepted prerequisite %s generation %d: %s\nAcceptance evidence: %s", dependency, parent.Generation, parent.Result, parent.Evidence)
		}
		prompt += "\nTask instructions: " + n.Task.Prompt
		data, e := json.Marshal(toolsubagent.RunPlan{Name: id, Prompt: prompt,
			Purpose: n.Task.Description, ParentTask: g.Graph.Name + "/" + n.Task.ID,
			Lifetime: "task", OwnedResources: slices.Clone(n.Task.Writes),
			GraphID: g.Graph.Name, TaskID: n.Task.ID, TaskGeneration: n.Generation})
		if e != nil {
			return operation.Operation{}, e
		}
		spec, err = operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: data})
	}
	if err != nil {
		return operation.Operation{}, err
	}
	return operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}, nil
}
func (s *runtimeState) child(child operation.Operation) {
	if s.runtime.ctx.Err() != nil {
		return
	}
	g := s.active
	if g == nil {
		return
	}
	for i := range g.Graph.Nodes {
		n := &g.Graph.Nodes[i]
		a := g.Attempts[n.Task.ID]
		if a == nil || a.ChildOperation.ID != child.ID || a.Generation != n.Generation {
			continue
		}
		a.ChildOperation = child
		if !terminal(child.Status) {
			s.save(g)
			return
		}
		a.Blocker = ""
		if n.Status == Canceling {
			if s.scanning == nil || s.scanning.id != n.Task.ID {
				g.Graph.Canceled(n.Task.ID, n.Generation)
			}
		} else if n.Status == Running {
			a.setPhase("output_validation")
		}

		s.notify(g, n)
		s.advance(g)
		return
	}
}
func childResult(child operation.Operation) (string, error) {
	var result string
	var err error
	switch child.Type {
	case operation.TypeShell:
		var state operation.ShellState
		err = json.Unmarshal(child.State, &state)
		if err == nil {
			if state.Result != nil {
				result = state.Result.Out
				if state.Result.Err != "" {
					result += "\n" + state.Result.Err
				}
				if state.Result.ExitCode != 0 {
					err = fmt.Errorf("shell exited with code %d", state.Result.ExitCode)
				}
			}
			if state.TerminalError != "" {
				err = errors.New(state.TerminalError)
			}
		}
	case operation.TypeRemoteJob:
		state, e := operation.DecodeRemoteJobState(child)
		err = e
		if e == nil {
			result = state.TerminalResult
			if state.TerminalError != "" {
				err = errors.New(state.TerminalError)
			}
		}
	default:
		err = errors.New("unsupported child operation")
	}
	if child.Status != operation.StatusCompleted && err == nil {
		err = fmt.Errorf("child operation %s", child.Status)
	}
	return result, err
}
func (s *runtimeState) control(op operation.Operation, p ControlPlan) error {
	g := s.graphs[p.Name]
	if g == nil {
		return errors.New("unknown graph")
	}
	if p.Action == "status" {
		return s.snapshot(op, g)
	} // Pure snapshot: no advancement or filesystem work.
	if g.terminal {
		return errors.New("graph is terminal; only status is available")
	}
	var err error
	switch p.Action {
	case "revise":
		err = s.permissions(p.Tasks)
		if err == nil {
			err = g.Graph.Revise(p.Tasks)
		}
	case "accept":
		n, e := g.Graph.attempt(p.TaskID, p.Generation)
		if e != nil {
			return e
		}
		if n.Status != Completed {
			return errors.New("task has no current completed result")
		}
		if strings.TrimSpace(p.Evidence) == "" {
			return errors.New("acceptance evidence is required")
		}
		if g.pendingAccept != nil {
			if len(g.acceptQueue) >= 64 {
				return errors.New("acceptance validation queue full; wait for pending results")
			}
			g.acceptQueue = append(g.acceptQueue, &acceptRequest{op: op, plan: p})
		} else {
			g.pendingAccept = &acceptRequest{op: op, plan: p}
		}
		// Acceptance completes after the worker returns; status stays available meanwhile.
		s.advance(g)
		return nil
	case "reject":
		err = g.Graph.Reject(p.TaskID, p.Generation, p.Evidence)
	case "invalidate":
		err = g.Graph.Invalidate(p.TaskID)
	case "retry":
		err = g.Graph.Retry(p.TaskID)
	case "cancel":
		if p.TaskID == "" {
			g.cancelControls = append(g.cancelControls, op)
			s.emit(s.runtime.control, op, operation.StatusAwaiting, &g.checkpoint, "cancellation requested; waiting for scans and children to drain", nil)
			s.cancelGraph(g)
			s.advance(g)
			return nil
		}
		err = g.Graph.Cancel(p.TaskID)
		if err == nil {
			s.cancelTask(g, p.TaskID)
		}
	default:
		return fmt.Errorf("unsupported graph action %q", p.Action)
	}
	if err != nil {
		return err
	}
	g.epoch++
	if s.scanning != nil {
		s.scanning.cancel()
	}
	s.advance(g)
	return s.snapshot(op, g)
}
func (s *runtimeState) snapshot(op operation.Operation, g *graphExecution) error {
	snapshot, err := json.Marshal(struct {
		Graph    Graph               `json:"graph"`
		Blockers string              `json:"blockers"`
		Attempts map[string]*attempt `json:"attempts"`
		Warnings []string            `json:"warnings"`
		Scanning bool                `json:"scanning"`
		Scan     *scanStatus         `json:"scan,omitempty"`
	}{g.Graph, s.blockers(g), g.Attempts, planWarnings(g.Graph), s.scanning != nil, s.scanStatus()})
	if err != nil {
		return err
	}
	s.emit(s.runtime.control, op, operation.StatusCompleted, &g.checkpoint, string(snapshot), nil)
	return nil
}
func (s *runtimeState) cancelGraph(g *graphExecution) {
	g.Canceling = true
	g.epoch++
	if s.scanning != nil {
		s.scanning.cancel()
	}
	for i := range g.Graph.Nodes {
		id := g.Graph.Nodes[i].Task.ID
		g.Graph.Cancel(id)
		s.cancelTask(g, id)
	}
	s.save(g)
}
func (s *runtimeState) cancelTask(g *graphExecution, id string) {
	n := g.Graph.Find(id)
	a := g.Attempts[id]
	if a == nil || !active(n) {
		return
	}
	if s.scanning != nil && s.scanning.id == id {
		s.scanning.cancel()
		return
	}
	if !a.CommandDispatched && a.Phase == "input_validation" || (a.CommandDispatched && terminal(a.ChildOperation.Status)) {
		g.Graph.Canceled(id, n.Generation)
	} else if a.Blocker == "" {
		g.manager.Cancel(a.ChildOperation.ID, "task canceled")
	}
}
func (s *runtimeState) cancelOperation(h *runtimeHandler, id operation.ID, reason string) {
	if h.typ == RunPlanType {
		for _, g := range s.graphs {
			if g.owner.ID == id && !g.terminal {
				s.cancelGraph(g)
				s.advance(g)
				return
			}
		}
	}
	// Acceptance is asynchronous; cancel its validation, not the task candidate.
	for _, g := range s.graphs {
		if g.pendingAccept != nil && g.pendingAccept.op.ID == id {
			g.pendingAccept.canceled = true
			g.epoch++
			if s.scanning != nil && s.scanning.kind == "accept" {
				s.scanning.cancel()
			}
			s.advance(g)
			return
		}
		for i, pending := range g.acceptQueue {
			if pending.op.ID == id {
				g.acceptQueue = append(g.acceptQueue[:i], g.acceptQueue[i+1:]...)
				s.emit(h, pending.op, operation.StatusCanceled, &g.checkpoint, "acceptance canceled", nil)
				return
			}
		}
	}
	// Canceling a pending graph cancellation does not revoke shutdown.
	for _, g := range s.graphs {
		for i, op := range g.cancelControls {
			if op.ID == id {
				s.emit(h, op, operation.StatusCanceled, &g.checkpoint, "", nil)
				g.cancelControls = append(g.cancelControls[:i], g.cancelControls[i+1:]...)
				return
			}
		}
	}
}
func (s *runtimeState) probe(g *graphExecution) bool {
	changed := false
	for i := range g.Graph.Nodes {
		n := &g.Graph.Nodes[i]
		if !active(n) {
			continue
		}
		a := g.Attempts[n.Task.ID]
		if a == nil || a.Blocker == "" || a.ChildOperation.Type != operation.TypeShell {
			continue
		}
		var state operation.ShellState
		if err := json.Unmarshal(a.ChildOperation.State, &state); err != nil {
			continue
		}
		if state.Phase != operation.ShellPhaseProcess || state.ProcessGroupID <= 1 {
			continue
		}
		absent := processGroupAbsent(state.ProcessGroupID)
		if !absent {
			continue
		}
		a.Blocker = ""
		changed = true
		if n.Status == Canceling {
			g.Graph.Canceled(n.Task.ID, n.Generation)
		} else {
			g.Graph.Complete(n.Task.ID, n.Generation, "", errors.New("resumed shell process is absent; execution outcome unknown"))
		}
		s.notify(g, n)
	}
	return changed
}
