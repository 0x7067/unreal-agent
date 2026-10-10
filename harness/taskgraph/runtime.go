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
	"sort"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Config supplies execution capabilities. Agent handlers must be fresh for each
// graph because every local manager owns its handlers' update channels.
type Config struct {
	Workspace        string
	Shell            string
	BaseDirectory    string
	Inbox            inbox.Writer
	AllowBash        bool
	NewAgentHandlers func(context.Context) ([]operation.RemoteJobHandler, error)
}
type Runtime struct {
	ctx          context.Context
	config       Config
	requests     chan func(*runtimeState)
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
	select {
	case r.requests <- fn:
		return nil
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}
func (r *Runtime) Handlers() []operation.RemoteJobHandler {
	return []operation.RemoteJobHandler{r.run, r.control}
}
func New(ctx context.Context, c Config) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("runtime context required")
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
	r := &Runtime{ctx: ctx, config: c, requests: make(chan func(*runtimeState))}
	r.run = &runtimeHandler{runtime: r, typ: RunPlanType, updates: make(chan operation.Operation)}
	r.control = &runtimeHandler{runtime: r, typ: ControlPlanType, updates: make(chan operation.Operation)}
	go r.loop()
	return r, nil
}

type attempt struct {
	Generation        uint64              `json:"generation"`
	ChildOperation    operation.Operation `json:"child_operation"`
	BeforeReads       string              `json:"before_reads"`
	OutputFingerprint string              `json:"output_fingerprint,omitempty"`
	Blocker           string              `json:"blocker,omitempty"`
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
}
type runtimeState struct {
	runtime *Runtime
	graphs  map[string]*graphExecution
	active  *graphExecution
	seen    map[operation.ID]bool
}

func (r *Runtime) loop() {
	s := &runtimeState{runtime: r, graphs: map[string]*graphExecution{}, seen: map[operation.ID]bool{}}
	defer close(r.run.updates)
	defer close(r.control.updates)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	done := r.ctx.Done()
	for {
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
		case fn := <-r.requests:
			fn(s)
		case runOut <- runOp:
			r.run.queue = r.run.queue[1:]
		case controlOut <- controlOp:
			r.control.queue = r.control.queue[1:]
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
			done = nil
			if s.active == nil {
				return
			}
			s.cancelGraph(s.active)
			// Nested manager drains all dispatched primitives before closing its updates.
			s.active.stop()
			for range s.active.manager.Updates() {
			}
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
	nestedCtx, stop := context.WithCancel(context.WithoutCancel(s.runtime.ctx))
	var handlers []operation.RemoteJobHandler
	if s.runtime.config.NewAgentHandlers != nil {
		handlers, err = s.runtime.config.NewAgentHandlers(nestedCtx)
	}
	if err != nil {
		stop()
		s.emit(h, op, operation.StatusFailed, nil, "", err)
		return
	}
	g := &graphExecution{checkpoint: cp, owner: op, manager: operation.NewLocalOperationManager(nestedCtx, handlers...), stop: stop}
	s.graphs[p.Name] = g
	s.active = g
	for _, n := range g.Graph.Nodes {
		if !active(&n) {
			if n.Status == Completed || n.Status == Failed {
				s.notify(g, g.Graph.Find(n.Task.ID))
			}
			continue
		}
		a := g.Attempts[n.Task.ID]
		child := a.ChildOperation
		if child.Status == operation.StatusReady {
			if n.Status == Canceling {
				// Cancellation was recorded before dispatch. Do not turn a
				// restored cancellation into a new process or agent run.
				child.Status = operation.StatusCanceled
				a.ChildOperation = child
				g.Graph.Canceled(n.Task.ID, n.Generation)
				s.notify(g, g.Graph.Find(n.Task.ID))
				continue
			}
			g.manager.Add(child)
		} else if terminal(child.Status) {
			s.child(child)
		} else if child.Type == operation.TypeShell {
			state, err := operation.DecodeShellState(child)
			if err == nil && state.Phase != operation.ShellPhaseProcess {
				if n.Status == Canceling {
					// Resume through the shell's cancellation reducer rather
					// than letting preparation proceed to command execution.
					child.Status = operation.StatusCanceling
				}
				g.manager.Add(child)
			} else {
				a.Blocker = "resumed shell outcome unknown; reservation retained"
				s.notify(g, g.Graph.Find(n.Task.ID))
			}
		} else {
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
func validateAttempts(cp checkpoint) error {
	if cp.Attempts == nil {
		return errors.New("checkpoint lacks attempt map")
	}
	for _, n := range cp.Graph.Nodes {
		a := cp.Attempts[n.Task.ID]
		if a != nil && a.Generation == n.Generation {
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
	s.emit(s.runtime.run, g.owner, operation.StatusAwaiting, &g.checkpoint, "", nil)
}
func (s *runtimeState) notify(g *graphExecution, n *Node) {
	if s.runtime.config.Inbox == nil {
		return
	}
	text := fmt.Sprintf("Task graph %s task %s generation %d: %s. Candidate result: %s. Acceptance conditions: %s. Evidence: %s. Blockers: %s", g.Graph.Name, n.Task.ID, n.Generation, n.Status, n.Result, n.Task.Acceptance, n.Evidence, s.blockers(g))
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
	if len(claims(t.Reads)) == 0 {
		return Fingerprint(workspace, []string{})
	}
	paths := []string{}
	directories := map[string]bool{}
	for _, read := range claims(t.Reads) {
		err := filepath.WalkDir(filepath.Join(workspace, filepath.FromSlash(read)), func(full string, d fs.DirEntry, e error) error {
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
	fingerprint, err := Fingerprint(workspace, paths)
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
func (s *runtimeState) fresh(g *graphExecution, n *Node) error {
	a := g.Attempts[n.Task.ID]
	if a == nil || a.Generation != n.Generation {
		return errors.New("missing current result attempt")
	}
	now, err := Fingerprint(s.runtime.config.Workspace, unionClaims(n.Task.Reads, n.Task.Writes))
	if err != nil {
		return err
	}
	if now != a.OutputFingerprint {
		return errors.New("result inputs or outputs changed")
	}
	return nil
}
func (s *runtimeState) recheck(g *graphExecution) bool {
	changed := false
	for i := range g.Graph.Nodes {
		n := &g.Graph.Nodes[i]
		if n.Status == Accepted {
			if err := s.fresh(g, n); err != nil {
				g.Graph.Invalidate(n.Task.ID)
				changed = true
			}
		}
	}
	return changed
}
func (s *runtimeState) advance(g *graphExecution) {
	if g.terminal {
		return
	}
	s.recheck(g)
	if g.Canceling {
		for _, n := range g.Graph.Nodes {
			if active(&n) {
				s.save(g)
				return
			}
		}
		g.terminal = true
		s.emit(s.runtime.run, g.owner, operation.StatusCanceled, &g.checkpoint, "", nil)
		for _, op := range g.cancelControls {
			s.emit(s.runtime.control, op, operation.StatusCompleted, &g.checkpoint, "graph canceled after children terminated", nil)
		}
		g.stop()
		if s.active == g {
			s.active = nil
		}
		return
	}
	if g.Graph.AllAccepted() {
		g.terminal = true
		s.emit(s.runtime.run, g.owner, operation.StatusCompleted, &g.checkpoint, "all current task generations accepted", nil)
		g.stop()
		if s.active == g {
			s.active = nil
		}
		return
	}
	for _, id := range g.Graph.Ready() {
		n := g.Graph.Find(id)
		// Validate the complete ownership claim, including writable aliases,
		// before launching a process that could otherwise follow a symlink.
		_, err := Fingerprint(s.runtime.config.Workspace, unionClaims(n.Task.Reads, n.Task.Writes))
		before := ""
		if err == nil {
			before, err = readOnlyFingerprint(s.runtime.config.Workspace, n.Task)
		}
		if err != nil {
			g.Graph.Start(id)
			g.Graph.Complete(id, n.Generation, "", err)
			s.notify(g, n)
			continue
		}
		child, err := s.newChild(g, n)
		if err != nil {
			g.Graph.Start(id)
			g.Graph.Complete(id, n.Generation, "", err)
			s.notify(g, n)
			continue
		}
		if err = g.Graph.Start(id); err != nil {
			continue
		}
		g.Attempts[id] = &attempt{Generation: n.Generation, ChildOperation: child, BeforeReads: before}
		s.save(g)
		if err = g.manager.Add(child); err != nil {
			g.Graph.Complete(id, n.Generation, "", err)
			s.notify(g, n)
		}
	}
	s.save(g)
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
		data, e := json.Marshal(toolsubagent.RunPlan{Name: id, Prompt: prompt})
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
		result, runErr := childResult(child)
		if n.Status == Canceling {
			g.Graph.Canceled(n.Task.ID, n.Generation)
		} else if n.Status == Running {
			after, err := readOnlyFingerprint(s.runtime.config.Workspace, n.Task)
			if err != nil {
				runErr = err
			} else if after != a.BeforeReads {
				runErr = errors.New("read-only inputs changed during execution; result is stale")
			}
			a.OutputFingerprint, err = Fingerprint(s.runtime.config.Workspace, unionClaims(n.Task.Reads, n.Task.Writes))
			if err != nil {
				runErr = err
			}
			g.Graph.Complete(n.Task.ID, n.Generation, result, runErr)
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
	var err error
	if g.terminal && p.Action != "status" {
		return errors.New("graph is terminal; only status is available")
	}
	invalidated := false
	if p.Action == "accept" && !g.terminal {
		invalidated = s.recheck(g)
	}
	switch p.Action {
	case "status":
	case "revise":
		err = s.permissions(p.Tasks)
		if err == nil {
			err = g.Graph.Revise(p.Tasks)
		}
	case "accept":
		n, e := g.Graph.attempt(p.TaskID, p.Generation)
		err = e
		if err == nil && n.Status != Completed {
			err = errors.New("task has no current completed result")
		}
		if err == nil {
			err = s.fresh(g, n)
			if err != nil {
				g.Graph.Invalidate(p.TaskID)
				invalidated = true
			}
		}
		if err == nil {
			err = g.Graph.Accept(p.TaskID, p.Generation, p.Evidence)
		}
	case "reject":
		err = g.Graph.Reject(p.TaskID, p.Generation, p.Evidence)
	case "invalidate":
		err = g.Graph.Invalidate(p.TaskID)
	case "retry":
		err = g.Graph.Retry(p.TaskID)
	case "cancel":
		if p.TaskID == "" {
			g.cancelControls = append(g.cancelControls, op)
			s.cancelGraph(g)
			s.advance(g)
			return nil
		}
		err = g.Graph.Cancel(p.TaskID)
		if err == nil {
			a := g.Attempts[p.TaskID]
			if a != nil && active(g.Graph.Find(p.TaskID)) {
				err = g.manager.Cancel(a.ChildOperation.ID, "task canceled")
			}
		}
	default:
		err = fmt.Errorf("unsupported graph action %q", p.Action)
	}
	if err != nil {
		if invalidated {
			// The acceptance may be stale because its precheck invalidated
			// prerequisites. Persist and admit reruns even when it fails.
			s.advance(g)
		}
		return err
	}
	if !g.terminal {
		s.advance(g)
	}
	snapshot, err := json.Marshal(struct {
		Graph    Graph  `json:"graph"`
		Blockers string `json:"blockers"`
	}{g.Graph, s.blockers(g)})
	if err != nil {
		return err
	}
	s.emit(s.runtime.control, op, operation.StatusCompleted, &g.checkpoint, string(snapshot), nil)
	return nil
}
func (s *runtimeState) cancelGraph(g *graphExecution) {
	g.Canceling = true
	for i := range g.Graph.Nodes {
		n := &g.Graph.Nodes[i]
		wasActive := active(n)
		g.Graph.Cancel(n.Task.ID)
		if wasActive {
			a := g.Attempts[n.Task.ID]
			if a != nil && a.Blocker == "" {
				g.manager.Cancel(a.ChildOperation.ID, "graph canceled")
			}
		}
	}
	s.save(g)
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
	// Control operations complete atomically in the actor. Canceling a pending
	// graph cancellation does not revoke the already requested graph shutdown.
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
