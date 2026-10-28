package subagent

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"sync"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

const ControlPlanType = toolsubagent.ControlPlanType
const MaxFleetAgents = 128
const MaxFleetSupervisors = 64

// ControlPlan is the additive, serializable subagent.control/v1 protocol. Stop
// acknowledges a cancellation REQUEST, never process drain or durable persistence.
type ControlPlan = toolsubagent.ControlPlan

type AgentActivity struct {
	PhaseStartedAt time.Time
	ChildActivity
	Handle, ScopeID, ParentHandle, Purpose, Lifetime, GraphID, TaskID, Phase string
	TaskGeneration                                                           uint64
	BlockerPath                                                              []OperationActivity
}
type FleetSnapshot struct {
	Version           int
	CapturedAt        time.Time
	Agents            []AgentActivity
	OmittedAgents     uint64
	ShutdownRequested bool
}
type hubTarget struct {
	supervisor *Supervisor
	session    string
	generation int
	handle     string
	external   func(context.Context, ControlPlan) (any, error)
	activity   AgentActivity
}

// Hub is a session-local capability registry. Handles are random, never names or
// PIDs. Tombstones and generations prevent delayed controls reaching a new run.
// Snapshots contain only bounded redacted observations; they perform no I/O.
type Hub struct {
	ctx         context.Context
	scopeID     string
	mu          sync.Mutex
	supervisors []*Supervisor
	targets     map[string]hubTarget
	omitted     uint64
	control     *hubHandler
	workers     sync.WaitGroup
	done        chan struct{}
}

func NewHub(ctx context.Context) *Hub {
	h := &Hub{ctx: ctx, scopeID: uuid.New().String(), targets: make(map[string]hubTarget), done: make(chan struct{})}
	h.control = &hubHandler{hub: h, requests: make(chan operation.Operation, 128), updates: make(chan operation.Operation), out: make(chan operation.Operation, 1024), seen: make(map[operation.ID]bool), results: make(chan hubResult, 256), cancelled: make(map[operation.ID]bool), dispatched: make(map[operation.ID]*Supervisor)}
	h.workers.Add(3)
	go func() { defer h.workers.Done(); h.control.loop() }()
	go func() { defer h.workers.Done(); h.control.forward() }()
	go func() { defer h.workers.Done(); h.control.prepareLoop() }()
	go func() { h.workers.Wait(); close(h.done) }()
	return h
}
func (h *Hub) register(s *Supervisor) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ctx.Err() != nil {
		return h.ctx.Err()
	}
	if len(h.supervisors) >= MaxFleetSupervisors {
		return errors.New("subagent hub supervisor capacity exhausted")
	}
	h.supervisors = append(h.supervisors, s)
	return nil
}
func (h *Hub) bind(s *Supervisor, session string, generation int) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.targets) >= MaxFleetAgents {
		h.omitted++
		return "", errors.New("subagent hub handle capacity exhausted")
	}
	handle := uuid.New().String()
	h.targets[handle] = hubTarget{supervisor: s, session: session, generation: generation, handle: handle}
	return handle, nil
}

// BindExternal issues a fresh Hub-local capability for an independently owned
// child. Binding does not start a child or transfer its lifetime to this Hub.
// The callback implements status/message/stop and runs on the control worker;
// it must honor ctx. Wait joins callbacks, not the independent child's drain.
func (h *Hub) BindExternal(activity AgentActivity, control func(context.Context, ControlPlan) (any, error)) (string, uint64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ctx.Err(); err != nil {
		return "", 0, err
	}
	if control == nil {
		return "", 0, errors.New("external control callback required")
	}
	if len(h.targets) >= MaxFleetAgents {
		h.omitted++
		return "", 0, errors.New("subagent hub handle capacity exhausted")
	}
	handle := uuid.New().String()
	activity = h.externalActivity(handle, activity)
	h.targets[handle] = hubTarget{handle: handle, generation: 1, external: control, activity: activity}
	return handle, 1, nil
}

// UpdateExternal replaces observations only; it cannot change binding authority.
func (h *Hub) UpdateExternal(handle string, activity AgentActivity) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ctx.Err(); err != nil {
		return err
	}
	t, ok := h.targets[handle]
	if !ok || t.external == nil {
		return errors.New("unknown external agent handle")
	}
	t.activity = h.externalActivity(handle, activity)
	h.targets[handle] = t
	return nil
}

// External observations retain bounded identity/phase metadata and lifecycle
// evidence only. Arbitrary operation trees, resource paths and input payloads
// are deliberately not retained. Execution generation is separate from binding.
func (h *Hub) externalActivity(handle string, a AgentActivity) AgentActivity {
	c := a.ChildActivity
	c.Operations, c.Inputs, c.OwnedResources = nil, nil, nil
	c.RelevantOperation, c.OverflowOperation = nil, nil
	c.ObservedNonterminalOperations = 0
	c.Name = activityIdentity(c.Name, &c.IdentityTruncated)
	c.SessionID = activityIdentity(c.SessionID, &c.IdentityTruncated)
	c.OwnerOperationID = activityIdentity(c.OwnerOperationID, &c.IdentityTruncated)
	c.ParentTask = activityIdentity(c.ParentTask, &c.IdentityTruncated)
	c.LastInputID, c.InputBarrier, c.DiagnosticCode, c.ProgressEvidence = "", "", "", ""
	a.ChildActivity = c
	a.Handle, a.ChildActivity.Handle, a.ScopeID = handle, handle, h.scopeID
	a.ParentHandle = activityIdentity(a.ParentHandle, &a.IdentityTruncated)
	a.Purpose = activityIdentity(a.Purpose, &a.IdentityTruncated)
	a.Lifetime = activityIdentity(a.Lifetime, &a.IdentityTruncated)
	a.GraphID = activityIdentity(a.GraphID, &a.IdentityTruncated)
	a.TaskID = activityIdentity(a.TaskID, &a.IdentityTruncated)
	a.Phase = activityIdentity(a.Phase, &a.IdentityTruncated)
	a.ChildActivity.ParentHandle, a.ChildActivity.Purpose, a.ChildActivity.Lifetime = a.ParentHandle, a.Purpose, a.Lifetime
	a.ChildActivity.GraphID, a.ChildActivity.TaskID, a.ChildActivity.TaskGeneration = a.GraphID, a.TaskID, a.TaskGeneration
	a.BlockerPath = nil
	return a
}

func (h *Hub) lookup(handle string) (hubTarget, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.targets[handle]
	return t, ok
}
func (h *Hub) Handlers() []operation.RemoteJobHandler { return []operation.RemoteJobHandler{h.control} }
func (h *Hub) Snapshot() FleetSnapshot {
	h.mu.Lock()
	supervisors := append([]*Supervisor(nil), h.supervisors...)
	omitted := h.omitted
	external := make([]AgentActivity, 0)
	for _, target := range h.targets {
		if target.external != nil {
			external = append(external, target.activity)
		}
	}
	h.mu.Unlock()
	result := FleetSnapshot{Version: 1, CapturedAt: time.Now(), OmittedAgents: omitted, ShutdownRequested: h.ctx.Err() != nil, Agents: external}
	for _, s := range supervisors {
		snap := s.ActivitySnapshot()
		for _, c := range snap.Children {
			if len(result.Agents) >= MaxFleetAgents {
				result.OmittedAgents++
				continue
			}
			a := AgentActivity{ChildActivity: c, Handle: c.Handle, ScopeID: snap.ScopeID, ParentHandle: c.ParentHandle, Purpose: c.Purpose, Lifetime: c.Lifetime, GraphID: c.GraphID, TaskID: c.TaskID, TaskGeneration: c.TaskGeneration, Phase: "awaiting"}
			switch {
			case c.OwnerTerminalEnqueued:
				a.Phase = "drained"
			case c.PreparationFailed:
				a.Phase = "publication_failed"
			case c.CancellationRequested:
				a.Phase = "stopping"
			case c.ProcessWaitReturned:
				a.Phase = "draining"
			case c.ReportAvailable:
				a.Phase = "report_available"
			case c.ProcessStartedAt.IsZero():
				a.Phase = "admitted"
			}
			if c.OwnerTerminalEnqueued {
				a.RelevantOperation = nil
				a.ObservedNonterminalOperations = 0
			}
			if a.RelevantOperation != nil {
				current := a.RelevantOperation
				for depth := 0; current != nil && depth < 5; depth++ {
					op := *current
					op.Descendants = nil
					a.BlockerPath = append(a.BlockerPath, op)
					if len(current.Descendants) == 0 {
						current = nil
					} else {
						current = &current.Descendants[0]
					}
				}
			}
			result.Agents = append(result.Agents, a)
		}
	}
	return result
}

type hubHandler struct {
	hub           *Hub
	requests      chan operation.Operation
	updates       chan operation.Operation
	out           chan operation.Operation
	mu            sync.Mutex
	seen          map[operation.ID]bool
	cancelled     map[operation.ID]bool
	dispatched    map[operation.ID]*Supervisor
	results       chan hubResult
	admittedBytes int
}

func (*hubHandler) RemoteJobPlanType() operation.RemoteJobPlanType       { return ControlPlanType }
func (*hubHandler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion { return 1 }
func (c *hubHandler) RemoteJobUpdates() <-chan operation.Operation       { return c.updates }
func (c *hubHandler) AddRemoteJob(op operation.Operation) error {
	remote, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return err
	}
	if remote.Plan.Type != ControlPlanType || remote.Plan.Version != 1 {
		return fmt.Errorf("subagent control plan: %w", operation.ErrUnsupported)
	}
	if err := c.hub.ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen[op.ID] {
		return nil
	}
	if len(op.ID) > activityIdentityBytes || len(c.seen) >= 256 || len(op.State) > MaxSubagentFrameBytes || c.admittedBytes+len(op.State) > 16<<20 {
		return errors.New("subagent control dedup capacity exhausted")
	}
	select {
	case <-c.hub.ctx.Done():
		return c.hub.ctx.Err()
	case c.requests <- op:
		c.seen[op.ID] = true
		c.admittedBytes += len(op.State)
		return nil
	default:
		return errors.New("subagent control queue capacity exhausted")
	}
}
func (c *hubHandler) CancelRemoteJob(id operation.ID, reason string) error {
	c.mu.Lock()
	if c.seen[id] {
		c.cancelled[id] = true
	}
	s := c.dispatched[id]
	c.mu.Unlock()
	if s != nil {
		return s.do(func(state *state) { state.cancel(id, reason) })
	}
	return nil // A requested structural stop cannot be undone by canceling its receipt.
}
func (c *hubHandler) forward() {
	defer close(c.updates)
	for {
		select {
		case op := <-c.out:
			select {
			case c.updates <- op:
			case <-c.hub.ctx.Done():
				return
			}
		case <-c.hub.ctx.Done():
			return
		}
	}
}
func (c *hubHandler) publish(op operation.Operation) {
	select {
	case c.out <- op:
	case <-c.hub.ctx.Done():
	}
}

type hubResult struct {
	op     operation.Operation
	text   string
	err    error
	status operation.Status
}

func (c *hubHandler) finish(op operation.Operation, text string, err error) {
	status := operation.StatusCompleted
	if err != nil {
		status = operation.StatusFailed
	}
	select {
	case c.results <- hubResult{op: op, text: text, err: err, status: status}:
	case <-c.hub.ctx.Done():
	}
}
func (c *hubHandler) prepareLoop() {
	for {
		select {
		case result := <-c.results:
			remote, e := operation.DecodeRemoteJobState(result.op)
			if e != nil {
				continue
			}
			remote.TerminalResult = result.text
			if result.err != nil {
				remote.TerminalError = result.err.Error()
			}
			step, e := operation.UpdateRemoteJob(result.op, remote, result.status)
			if e == nil && step.Operation != nil {
				c.publish(*step.Operation)
			}
		case <-c.hub.ctx.Done():
			return
		}
	}
}
func (c *hubHandler) loop() {
	for {
		select {
		case op := <-c.requests:
			c.execute(op)
		case <-c.hub.ctx.Done():
			return
		}
	}
}
func (c *hubHandler) execute(op operation.Operation) {
	c.mu.Lock()
	cancelled := c.cancelled[op.ID]
	c.mu.Unlock()
	if cancelled {
		c.finish(op, "", errors.New("control canceled before dispatch"))
		return
	}
	remote, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return
	}
	var p ControlPlan
	if err = json.Unmarshal(remote.Plan.Data, &p); err != nil {
		c.finish(op, "", err)
		return
	}
	if err := p.Validate(); err != nil {
		c.finish(op, "", err)
		return
	}
	if p.Action == "list" {
		b, _ := json.Marshal(c.hub.Snapshot())
		c.finish(op, controlOutcome(p, "snapshot", "", false, b), nil)
		return
	}
	target, ok := c.hub.lookup(p.Handle)
	if !ok || uint64(target.generation) != p.Generation {
		c.finish(op, "", errors.New("unknown, stale-generation or cross-session agent handle"))
		return
	}
	if target.external != nil {
		// Check immediately before effects, not only at admission: the root may
		// have died while the request waited behind another callback.
		if err := c.hub.ctx.Err(); err != nil {
			c.finish(op, "", err)
			return
		}
		value, err := target.external(c.hub.ctx, p)
		text := ""
		if err == nil {
			if raw, ok := value.(string); ok {
				text = raw
			} else {
				var b []byte
				b, err = json.Marshal(value)
				text = string(b)
			}
		}
		c.finish(op, text, err)
		return
	}
	if p.Action == "status" {
		for _, a := range c.hub.Snapshot().Agents {
			if a.Handle == p.Handle {
				b, _ := json.Marshal(a)
				c.finish(op, controlOutcome(p, "snapshot", "", a.OwnerTerminalEnqueued, b), nil)
				return
			}
		}
		c.finish(op, "", errors.New("stale agent handle"))
		return
	}

	c.mu.Lock()
	c.dispatched[op.ID] = target.supervisor
	c.mu.Unlock()
	err = target.supervisor.do(func(s *state) {
		var current *child
		for _, v := range s.children {
			if v.sessionID == target.session && v.generation == target.generation && v.handle == p.Handle {
				current = v
				break
			}
		}
		if current == nil {
			s.finish(op, operation.StatusFailed, "", "stale agent generation")
			return
		}
		switch p.Action {
		case "message":
			if p.Message == "" {
				s.finish(op, operation.StatusFailed, "", "message must be nonempty")
				return
			}
			s.addMessage(op, toolsubagent.MessagePlan{To: current.name, Message: p.Message})
		case "stop":
			if current.owner != nil {
				s.cancel(current.owner.ID, "structural AgentControl stop")
			}
			state := "cancellation_requested"
			drained := false
			snap := target.supervisor.ActivitySnapshot()
			for _, a := range snap.Children {
				if a.Handle == p.Handle && a.OwnerTerminalEnqueued {
					state = "drained"
					drained = true
				}
			}
			s.finish(op, operation.StatusCompleted, controlOutcome(p, state, "", drained, nil), "")
		default:
			s.finish(op, operation.StatusFailed, "", fmt.Sprintf("unknown agent control action %q", p.Action))
		}
	})
	if err != nil {
		c.finish(op, "", err)
	}
}

// Wait joins Hub-owned control/preparation/publication workers after its context
// is canceled. It never substitutes for the supervisors' process/drain joins.
func (h *Hub) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *hubHandler) Wait(ctx context.Context) error { return c.hub.Wait(ctx) }

func controlOutcome(plan toolsubagent.ControlPlan, state, delivery string, drained bool, snapshot []byte) string {
	encoded, _ := json.Marshal(toolsubagent.ControlOutcome{Action: plan.Action, Handle: plan.Handle, Generation: plan.Generation, State: state, Delivery: delivery, DrainConfirmed: drained, Snapshot: snapshot})
	return string(encoded)
}
