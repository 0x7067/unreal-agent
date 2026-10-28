// Package subagent supervises subagent processes. Each subagent is a runner
// process started with -inbox-stdin: the parent writes inbox inputs to its
// stdin and reads its persisted session items from stdout. A subagent's
// SendMessage calls to the parent are delivered to the parent's inbox.
package subagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Flags select subagent mode in a runner process.
const (
	InboxStdinFlag        = "-inbox-stdin"
	SubagentFlag          = "-subagent"
	CleanupProofScopeFlag = "-cleanup-proof-scope"
)

const (
	restartedPrompt = "Your parent agent restarted while you were working. Continue your task."
)

type Config struct {
	// OnWorkChanged is called after authoritative ownership changes. It must
	// not block. Callers must re-read HasPendingWork; this is not a snapshot.
	OnWorkChanged func()
	Hub           *Hub
	// RequireCleanupProof requires a fresh runner work-drain receipt for every launched child.
	RequireCleanupProof bool
	// prepare is a package-private deterministic I/O seam; production uses the
	// existing artifact durability/order contract without a stronger commit ack.
	prepare func(operation.Operation, operation.RemoteJobState, operation.Status) (operation.Step, error)

	// Command is the runner executable followed by any leading arguments.
	Command          []string
	Env              []string // Nil inherits the parent environment.
	SessionDirectory string
	Workspace        string
	Model            string
	ThinkingLevel    string
	DisallowedTools  []string
	// Inbox receives the subagents' messages to the parent.
	Inbox  inbox.Writer
	Stderr io.Writer
}

type Supervisor struct {
	workCtx              context.Context
	stopWorkers          context.CancelFunc
	ctx                  context.Context
	config               Config
	requests             chan func(*state)
	run                  *handler
	message              *handler
	done                 chan struct{}
	cleanupErr           error
	readers              sync.WaitGroup // Adds are owned by loop; joins begin only after it exits.
	activity             *activityProjection
	deliveries           chan deliveryJob
	deliverySlots        chan struct{}
	deliveryMu           sync.Mutex
	deliveryBytes        int
	deliveryChanged      chan struct{}
	runPrep, messagePrep chan prepJob
	prepMu               sync.Mutex
	prepBytes            int
	admissionMu          sync.Mutex
	admissions           map[operation.ID]bool
	admissionBytes       int
	workMu               sync.Mutex
	actorPending         bool
	workRequests         int
	workDeliveries       int
}

// New starts a supervisor. Its child processes stop when ctx is canceled.
func New(ctx context.Context, config Config) (*Supervisor, error) {
	if len(config.Command) == 0 || strings.TrimSpace(config.Command[0]) == "" {
		return nil, errors.New("subagent command must be set")
	}
	if config.Inbox == nil {
		return nil, errors.New("subagent parent inbox must be set")
	}
	current := &Supervisor{ctx: ctx, config: config, requests: make(chan func(*state)), done: make(chan struct{})}
	current.admissions = make(map[operation.ID]bool)
	current.workCtx, current.stopWorkers = context.WithCancel(context.Background())
	current.activity = &activityProjection{snapshot: ActivitySnapshot{ScopeID: uuid.New().String()}}
	current.run = &handler{planType: toolsubagent.RunPlanType, supervisor: current, updates: make(chan operation.Operation)}
	current.message = &handler{planType: toolsubagent.MessagePlanType, supervisor: current, updates: make(chan operation.Operation)}
	current.deliveries = make(chan deliveryJob, 128)
	current.deliverySlots = make(chan struct{}, 128)
	current.deliveryChanged = make(chan struct{})
	current.runPrep = make(chan prepJob, 128)
	current.messagePrep = make(chan prepJob, 128)
	if config.Hub != nil {
		if err := config.Hub.register(current); err != nil {
			return nil, err
		}
	}
	current.readers.Add(5)
	go current.deliveryLoop()
	for range 2 {
		go current.prepareLoop(current.runPrep)
		go current.prepareLoop(current.messagePrep)
	}
	go current.loop()
	return current, nil
}

// Wait waits for the supervisor actor and all started child reader/command.Wait
// goroutines to finish. It does not cancel the supervisor; cancel the context
// supplied to New to request shutdown. A wait timeout does not imply completion.
func (current *Supervisor) Wait(ctx context.Context) error {
	select {
	case <-current.done:
		return current.cleanupErr
	default:
	}
	select {
	case <-current.done:
		return current.cleanupErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Handlers returns the remote job handlers for the operation manager.
func (current *Supervisor) Handlers() []operation.RemoteJobHandler {
	return []operation.RemoteJobHandler{current.run, current.message}
}

// HasPendingWork reports native child ownership, not operation ticket completion.
// It is a pure local read: the actor and admission/delivery goroutines publish
// under workMu, independently of the bounded diagnostic activity projection.
func (current *Supervisor) HasPendingWork() bool {
	current.workMu.Lock()
	defer current.workMu.Unlock()
	return current.pendingWorkLocked()
}

func (current *Supervisor) pendingWorkLocked() bool {
	return current.actorPending || current.workRequests != 0 || current.workDeliveries != 0
}

func (current *Supervisor) changeWork(change func()) {
	current.workMu.Lock()
	before := current.pendingWorkLocked()
	change()
	changed := before != current.pendingWorkLocked()
	current.workMu.Unlock()
	if changed && current.config.OnWorkChanged != nil {
		current.config.OnWorkChanged()
	}
}

func (s *state) publishWork(completedRequests int) {
	current := s.supervisor
	current.changeWork(func() {
		current.actorPending = s.activeProcesses != 0 || s.pendingPreps != 0 || len(s.pendingFinals) != 0 ||
			len(s.owners) != 0 || len(s.messages) != 0 || len(s.runQueue) != 0 || len(s.messageQueue) != 0 ||
			len(s.deliveryInputs) != 0 || current.cleanupErr != nil
		current.workRequests -= completedRequests
	})
}

func (current *Supervisor) do(request func(*state)) error {
	if err := current.ctx.Err(); err != nil {
		return err
	}
	current.changeWork(func() { current.workRequests++ })
	select {
	case current.requests <- func(s *state) {
		request(s)
		s.publishWork(1)
	}:
		return nil
	case <-current.ctx.Done():
		current.changeWork(func() { current.workRequests-- })
		return current.ctx.Err()
	}
}

func (current *Supervisor) loop() {
	defer close(current.done)
	defer close(current.run.updates)
	defer close(current.message.updates)
	// Only this actor starts readers. Once it returns no Add can race Wait,
	// even when callers are concurrently waiting on supervisor termination.
	defer func() {
		current.stopWorkers()
		current.readers.Wait()
		current.activity.joined()
		// All owned workers are joined. Shutdown may discard undelivered
		// operation updates; a missing cleanup proof still retains ownership.
		current.changeWork(func() { current.actorPending = current.cleanupErr != nil })
	}()
	s := &state{
		supervisor: current,
		children:   make(map[string]*child),
		owners:     make(map[operation.ID]*child),
		messages:   make(map[operation.ID]*pendingMessage),
	}
	shutdown := current.ctx.Done()
	for {
		s.publishWork(0)
		if shutdown == nil && s.activeProcesses == 0 && s.pendingPreps == 0 && len(s.pendingFinals) == 0 {
			return
		}
		var runUpdates, messageUpdates chan operation.Operation
		var runNext, messageNext operation.Operation
		if len(s.runQueue) != 0 {
			runUpdates, runNext = current.run.updates, s.runQueue[0]
		}
		if len(s.messageQueue) != 0 {
			messageUpdates, messageNext = current.message.updates, s.messageQueue[0]
		}
		select {
		case request := <-current.requests:
			request(s)
		case runUpdates <- runNext:
			s.runQueue = s.runQueue[1:]
		case messageUpdates <- messageNext:
			s.messageQueue = s.messageQueue[1:]
		case <-shutdown:
			shutdown = nil
		}
	}
}

type handler struct {
	planType   operation.RemoteJobPlanType
	supervisor *Supervisor
	updates    chan operation.Operation
}

var _ operation.RemoteJobHandler = (*handler)(nil)

func (current *handler) RemoteJobPlanType() operation.RemoteJobPlanType { return current.planType }

func (current *handler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion {
	return toolsubagent.PlanVersion
}

func (current *handler) AddRemoteJob(value operation.Operation) error {
	s := current.supervisor
	// Count admission before any actor handoff (including admissionMu waits).
	s.changeWork(func() { s.workRequests++ })
	defer s.changeWork(func() { s.workRequests-- })
	s.admissionMu.Lock()
	if s.admissions[value.ID] {
		s.admissionMu.Unlock()
		return nil
	}
	if len(value.ID) > activityIdentityBytes || len(s.admissions) >= 256 || len(value.State) > MaxSubagentFrameBytes || s.admissionBytes+len(value.State) > 16<<20 {
		s.admissionMu.Unlock()
		return errors.New("subagent operation admission capacity exhausted")
	}
	s.admissions[value.ID] = true
	s.admissionBytes += len(value.State)
	s.admissionMu.Unlock()
	err := s.do(func(state *state) { state.add(current.planType, value) })
	if err != nil {
		s.admissionMu.Lock()
		delete(s.admissions, value.ID)
		s.admissionBytes -= len(value.State)
		s.admissionMu.Unlock()
	}
	return err
}

func (current *handler) CancelRemoteJob(id operation.ID, reason string) error {
	return current.supervisor.do(func(s *state) { s.cancel(id, reason) })
}

func (current *handler) RemoteJobUpdates() <-chan operation.Operation { return current.updates }

// Wait lets owners of handler sets join the supervisor itself, not merely an
// operation manager's update forwarders, which exit as soon as ctx is canceled.
func (current *handler) Wait(ctx context.Context) error { return current.supervisor.Wait(ctx) }

// state is owned by the supervisor loop.
type state struct {
	runIssued, messageIssued, runPublished, messagePublished uint64
	runPrepared, messagePrepared                             map[uint64]prepResult
	activeProcesses, pendingPreps                            int
	pendingFinals                                            []terminalRetry
	retryingFinal                                            bool
	supervisor                                               *Supervisor
	children                                                 map[string]*child
	owners                                                   map[operation.ID]*child // Operations that finish when the process exits.
	messages                                                 map[operation.ID]*pendingMessage
	runQueue                                                 []operation.Operation
	messageQueue                                             []operation.Operation
	deliveryInputs                                           []inbox.Input
}

type child struct {
	lastSequence            sessionstore.Sequence
	reserved                bool
	live                    bool
	name                    string
	sessionID               string
	generation              int
	stdin                   io.WriteCloser // Set while the process runs.
	stop                    context.CancelFunc
	owner                   *operation.Operation
	stopping                bool
	unacked                 []*pendingMessage
	reply                   string
	failure                 string
	handle                  string
	metadata                runMetadata
	writer                  *inputWriter
	processDone, outputDone bool
	exitErr                 error
	parentPending           int
	cleanup                 bool
	launched                bool
	cleanupScope            string
	cleanupProven           bool
	// A strict explicit-background Run ticket can acknowledge a committed response
	// while this child still owns work. Keep owner/owners for structural stop;
	// neither the ticket nor its acknowledgment is a cleanup receipt.
	backgroundAcknowledged bool
}

type pendingMessage struct {
	operation operation.Operation
	child     *child
	input     inbox.Input
	frame     *inputFrame
}

func (s *state) add(planType operation.RemoteJobPlanType, value operation.Operation) {
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		s.fail(value, err.Error())
		return
	}
	switch planType {
	case toolsubagent.RunPlanType:
		var plan toolsubagent.RunPlan
		if err := json.Unmarshal(remote.Plan.Data, &plan); err != nil {
			s.fail(value, fmt.Sprintf("decode subagent plan: %v", err))
			return
		}
		s.addRun(value, remote, plan)
	case toolsubagent.MessagePlanType:
		var plan toolsubagent.MessagePlan
		if err := json.Unmarshal(remote.Plan.Data, &plan); err != nil {
			s.fail(value, fmt.Sprintf("decode subagent message plan: %v", err))
			return
		}
		s.addMessage(value, plan)
	}
}

func (s *state) addRun(value operation.Operation, remote operation.RemoteJobState, plan toolsubagent.RunPlan) {
	if err := toolsubagent.ValidateName(plan.Name); err != nil {
		s.fail(value, err.Error())
		return
	}
	if plan.Lifetime != "" && plan.Lifetime != "task" && plan.Lifetime != "background" {
		s.fail(value, "unsupported subagent lifetime policy")
		return
	}
	prompt := plan.Prompt
	if len(s.children) >= MaxActivityChildren && s.children[plan.Name] == nil {
		s.fail(value, "subagent child capacity exhausted")
		return
	}
	var handle toolsubagent.RunHandle
	if len(remote.Handle) != 0 {
		// The parent restarted; resume the recorded subagent session.
		if err := json.Unmarshal(remote.Handle, &handle); err != nil || handle.SessionID == "" || len(handle.SessionID) > activityIdentityBytes {
			s.fail(value, "subagent handle has no session ID")
			return
		}
		prompt = restartedPrompt
	}
	if existing, exists := s.children[plan.Name]; exists && existing.sessionID != handle.SessionID {
		s.fail(value, fmt.Sprintf("subagent %q already exists; message it with SendMessage or choose another name", plan.Name))
		return
	} else if exists && (existing.stdin != nil || existing.owner != nil) {
		s.fail(value, fmt.Sprintf("subagent %q is already running", plan.Name))
		return
	}
	if handle.SessionID == "" {
		handle.SessionID = uuid.New().String()
		encoded, err := json.Marshal(handle)
		if err != nil {
			s.fail(value, fmt.Sprintf("encode subagent handle: %v", err))
			return
		}
		remote.Handle = encoded
	}
	current := s.children[plan.Name]
	if current == nil {
		if handle.Generation > 1<<31 {
			s.fail(value, "unsupported subagent generation")
			return
		}
		current = &child{name: plan.Name, sessionID: handle.SessionID, generation: int(handle.Generation)}
		s.children[plan.Name] = current
	}
	current.metadata = metadataFromPlan(remote.Plan.Data)
	if !s.reserveAttempt(current, &value) {
		return
	}
	handle.Handle = current.handle
	handle.Generation = uint64(current.generation)
	remote.Handle, _ = json.Marshal(handle)
	value.State, _ = json.Marshal(remote) // The prelaunch owner retains the same fenced capability.
	_, ok := s.updateThen(value, remote, operation.StatusAwaiting, func(state *state, prepared operation.Operation) {
		if state.supervisor.ctx.Err() != nil || current.stopping {
			return
		}
		state.start(current, &prepared, []inbox.Input{newMessageInput(prompt)})
	})
	if !ok {
		s.fail(value, "subagent preparation capacity exhausted")
	}
}

func (s *state) addMessage(value operation.Operation, plan toolsubagent.MessagePlan) {
	current, exists := s.children[plan.To]
	if !exists {
		s.fail(value, fmt.Sprintf("unknown subagent %q; start it with Agent first", plan.To))
		return
	}
	if len(s.messages) >= 128 {
		s.fail(value, "subagent pending-message capacity exhausted")
		return
	}
	input := newMessageInput("Message from your parent agent:\n" + plan.Message)
	if current.stdin == nil {
		if current.owner != nil {
			s.fail(value, "subagent launch already pending")
			return
		}
		remote, err := operation.DecodeRemoteJobState(value)
		if err != nil {
			s.fail(value, err.Error())
			return
		}
		if !s.reserveAttempt(current, &value) {
			return
		}
		remote.Handle, _ = json.Marshal(toolsubagent.RunHandle{SessionID: current.sessionID, Handle: current.handle, Generation: uint64(current.generation)})
		value.State, _ = json.Marshal(remote)
		_, ok := s.updateThen(value, remote, operation.StatusAwaiting, func(state *state, prepared operation.Operation) {
			if current.stopping || state.supervisor.ctx.Err() != nil {
				return
			}
			state.start(current, &prepared, []inbox.Input{input})
		})
		if !ok {
			s.fail(value, "subagent preparation capacity exhausted")
		}
		return
	}
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		s.fail(value, err.Error())
		return
	}
	generation := current.generation
	s.supervisor.activity.input(current.sessionID, generation, string(input.ID), "admitted")
	message := &pendingMessage{operation: value, child: current, input: input}
	s.messages[value.ID] = message
	current.unacked = append(current.unacked, message)
	_, ok := s.updateThen(value, remote, operation.StatusAwaiting, func(state *state, prepared operation.Operation) {
		if state.messages[value.ID] != message || current.generation != generation || current.stopping || current.stdin == nil {
			return
		}
		message.operation = prepared
		if current.writer != nil {
			encoded, _ := json.Marshal(input)
			frame, err := current.writer.enqueue(encoded, input.ID)
			if err != nil {
				if current.processDone || current.writer.ctx.Err() != nil {
					return
				} // Exiting generation resolves replay/failure.
				state.forgetMessage(message)
				state.fail(prepared, err.Error())
				return
			}
			message.frame = frame
			state.supervisor.activity.input(current.sessionID, generation, string(input.ID), "queued")
		} else {
			_ = writeInput(current.stdin, input)
		} // Private fake compatibility; real children always have an owned writer.
	})
	if !ok {
		s.forgetMessage(message)
		s.fail(value, "subagent preparation capacity exhausted")
	}

}

func (s *state) cancel(id operation.ID, reason string) { s.requestStop(id, reason, false) }
func (s *state) requestStop(id operation.ID, reason string, cleanup bool) {
	if current, exists := s.owners[id]; exists && current.owner != nil && current.owner.ID == id {
		if current.stopping {
			if !cleanup && current.cleanup {
				current.cleanup = false
				if value, ok := s.updateStatusUnlessAcknowledged(current, operation.StatusCanceling); ok {
					current.owner = &value
				}
			}
			return
		}
		current.cleanup = cleanup
		current.stopping = true
		s.supervisor.activity.change(current.sessionID, current.generation, cancellationTime)
		if !cleanup {
			if value, ok := s.updateStatusUnlessAcknowledged(current, operation.StatusCanceling); ok {
				current.owner = &value
			}
		}
		if current.stdin == nil {
			s.supervisor.activity.change(current.sessionID, current.generation, func(c *ChildActivity) { c.OwnerExitProcessed = true; c.OwnerExitProcessedAt = time.Now() })
			s.finish(*current.owner, operation.StatusCanceled, "", "subagent canceled before launch")
			delete(s.owners, id)
			current.owner = nil
			return
		}
		if current.writer != nil {
			current.writer.stop()
			if current.stop != nil {
				current.stop()
			}
			return
		}
		payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.StopHard, Reason: reason})
		if err != nil || writeInput(current.stdin, inbox.Input{
			ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload,
		}) != nil {
			current.stop()
		}
		return
	}
	if message, exists := s.messages[id]; exists {
		barrier := "queued_frame_retracted"
		if message.frame != nil {
			barrier = "persistence_unconfirmed"
			message.frame.mu.Lock()
			if message.frame.barrier == "queued" {
				message.frame.canceled = true
				barrier = "queued_frame_retracted"
			}
			message.frame.mu.Unlock()
		}
		s.supervisor.activity.input(message.child.sessionID, message.child.generation, string(message.input.ID), "cancel_requested")
		if barrier == "queued_frame_retracted" {
			s.supervisor.activity.input(message.child.sessionID, message.child.generation, string(message.input.ID), barrier)
		}
		s.forgetMessage(message)
		s.finish(message.operation, operation.StatusCanceled, "", "message canceled; "+barrier+"; bytes already written may still be persisted")
	}
}

func (s *state) reserveAttempt(current *child, owner *operation.Operation) bool {
	if s.supervisor.config.RequireCleanupProof && current.launched && !current.cleanupProven {
		s.fail(*owner, "subagent cleanup proof missing; replacement refused")
		return false
	}
	current.cleanupScope = uuid.New().String()
	current.cleanupProven = false
	current.backgroundAcknowledged = false
	current.launched = false
	current.generation++
	generation := current.generation
	current.processDone, current.outputDone = false, false
	current.exitErr = nil
	current.parentPending = 0
	if hub := s.supervisor.config.Hub; hub != nil {
		handle, err := hub.bind(s.supervisor, current.sessionID, generation)
		if err != nil {
			s.fail(*owner, err.Error())
			return false
		}
		current.handle = handle
	}
	s.supervisor.activity.begin(current.name, current.sessionID, generation, owner.ID, time.Now())
	s.supervisor.activity.change(current.sessionID, generation, func(c *ChildActivity) { c.Handle = current.handle; applyMetadata(c, current.metadata) })
	current.owner = owner
	current.stopping = false
	current.cleanup = false
	current.reply, current.failure = "", ""
	s.owners[owner.ID] = current

	current.reserved = true
	return true
}
func (s *state) start(current *child, owner *operation.Operation, messages []inbox.Input) {
	if !current.reserved || current.owner == nil || current.owner.ID != owner.ID {
		if !s.reserveAttempt(current, owner) {
			return
		}
		remote, err := operation.DecodeRemoteJobState(*owner)
		if err != nil {
			s.fail(*owner, err.Error())
			return
		}
		remote.Handle, _ = json.Marshal(toolsubagent.RunHandle{SessionID: current.sessionID, Handle: current.handle, Generation: uint64(current.generation)})
		owner.State, _ = json.Marshal(remote)
		if _, ok := s.updateThen(*owner, remote, operation.StatusAwaiting, func(state *state, prepared operation.Operation) {
			if !current.stopping && state.supervisor.ctx.Err() == nil {
				state.start(current, &prepared, messages)
			}
		}); !ok {
			s.fail(*owner, "subagent replay preparation capacity exhausted")
		}
		return
	}
	current.reserved = false
	current.owner = owner
	generation := current.generation

	type requestMessage struct {
		Content   string `json:"content"`
		MessageID string `json:"message_id"`
	}
	request := struct {
		SessionID       string           `json:"session_id"`
		Messages        []requestMessage `json:"messages"`
		Model           string           `json:"model,omitempty"`
		ThinkingLevel   string           `json:"thinking_level,omitempty"`
		DisallowedTools []string         `json:"disallowed_tools,omitempty"`
	}{SessionID: current.sessionID, Model: s.supervisor.config.Model, ThinkingLevel: s.supervisor.config.ThinkingLevel, DisallowedTools: s.supervisor.config.DisallowedTools}
	for _, message := range messages {
		var text string
		if err := json.Unmarshal(message.Payload, &text); err != nil {
			s.exited(current, generation, fmt.Errorf("decode subagent message: %w", err))
			return
		}
		request.Messages = append(request.Messages, requestMessage{Content: text, MessageID: string(message.ID)})
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		s.exited(current, generation, fmt.Errorf("encode subagent request: %w", err))
		return
	}

	if len(encoded)+1 > MaxSubagentFrameBytes {
		s.exited(current, generation, errors.New("initial subagent request exceeds 8 MiB frame limit"))
		return
	}
	config := s.supervisor.config
	ctx, stop := context.WithCancel(s.supervisor.ctx)
	args := append(append([]string(nil), config.Command[1:]...), InboxStdinFlag, SubagentFlag)
	if config.RequireCleanupProof {
		args = append(args, CleanupProofScopeFlag, current.cleanupScope)
	}
	if config.SessionDirectory != "" {
		args = append(args, "-session-directory", config.SessionDirectory)
	}
	if config.Workspace != "" {
		args = append(args, "-workspace", config.Workspace)
	}
	// The request goes to stdin, not argv, so other local users cannot read it.
	command := exec.CommandContext(ctx, config.Command[0], args...)
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	// Interrupt is an out-of-band structural request, not proof of descendant
	// drain. Never silently kill just the runner after a timer: that could orphan
	// operations it still owns. Wait(ctx) may time out retaining truthful ownership.
	command.WaitDelay = 0
	command.Dir = config.Workspace
	command.Env = config.Env
	command.Stderr = config.Stderr
	stdin, err := command.StdinPipe()
	if err == nil {
		var stdout, writer *os.File
		stdout, writer, err = os.Pipe()
		if err == nil {
			command.Stdout = writer
			err = command.Start()
			_ = writer.Close()
		}
		if err == nil {
			current.stdin, current.stop = stdin, stop
			current.live = true
			current.launched = true
			s.activeProcesses++
			current.writer = s.supervisor.newInputWriter(ctx, stdin, current.sessionID, generation)
			inputOwner := current.writer
			s.supervisor.activity.change(current.sessionID, generation, func(c *ChildActivity) { c.ProcessStartedAt = time.Now() })
			processDone := make(chan struct{})
			outputDone := make(chan struct{})
			s.supervisor.readers.Add(3)
			go func() {
				defer s.supervisor.readers.Done()
				err := command.Wait()
				inputOwner.stop()
				s.supervisor.activity.change(current.sessionID, generation, func(c *ChildActivity) { c.ProcessWaitReturned = true; c.ProcessWaitReturnedAt = time.Now() })
				_ = s.supervisor.post(func(state *state) {
					if current.generation == generation {
						current.processDone = true
						current.exitErr = err
						state.maybeExit(current)
					}
				})
				close(processDone)
			}()
			go s.supervisor.read(current, generation, stdout, processDone, inputOwner, outputDone)
			go func() {
				defer s.supervisor.readers.Done()
				<-outputDone // reader closed its pipe and released its reader registration
				_ = s.supervisor.post(func(state *state) {
					if current.generation == generation {
						current.outputDone = true
						state.maybeExit(current)
					}
				})
			}()
			if _, err := current.writer.enqueue(encoded, ""); err != nil {
				current.failure = err.Error()
				current.writer.stop()
				stop()
			}
			return
		}
		if stdout != nil {
			_ = stdout.Close()
		}
		_ = stdin.Close()
	}
	stop()
	s.exited(current, generation, fmt.Errorf("start subagent %q: %w", current.name, err))
}

func (current *Supervisor) read(target *child, generation int, stdout io.ReadCloser, processDone <-chan struct{}, inputOwner *inputWriter, outputDone chan<- struct{}) {
	defer func() {
		_ = stdout.Close()
		current.activity.change(target.sessionID, generation, func(c *ChildActivity) { c.ReaderReturned = true; c.ReaderReturnedAt = time.Now() })
		current.readers.Done()
		close(outputDone)
	}()
	lines := bufio.NewReader(stdout)
	for {
		line, err := boundedLine(lines)
		if line = bytes.TrimSpace(line); len(line) != 0 {
			current.activity.observeLine(target.sessionID, generation, line, time.Now())
			// Reserve finite bytes and a delivery batch slot before actor admission.
			if e := current.reserveDelivery(current.workCtx, len(line)); e != nil {
				break
			}
			select {
			case current.deliverySlots <- struct{}{}:
			case <-current.workCtx.Done():
				current.releaseDelivery(len(line))
				return
			}
			if current.post(func(state *state) {
				state.line(target, generation, line)
				state.flushDelivery(target, generation, len(line))
			}) != nil {
				current.releaseDelivery(len(line))
				<-current.deliverySlots
				break
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				current.activity.change(target.sessionID, generation, func(c *ChildActivity) { c.OutputEOFObserved = true; c.OutputEOFObservedAt = time.Now() })
			} else {
				_ = current.post(func(state *state) {
					target.failure = err.Error()
					if target.stop != nil {
						target.stop()
					}
				})
			}
			break
		}
	}
	<-processDone
	<-inputOwner.done
}

func (s *state) line(current *child, generation int, line []byte) {
	if current.generation != generation {
		return
	}
	var event struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(line, &event) == nil && event.Type == "error" {
		current.failure = event.Message
		return
	}
	var item sessionstore.Item
	if err := json.Unmarshal(line, &item); err != nil {
		return
	}
	if item.Sequence == 0 || item.Sequence <= current.lastSequence {
		return
	}
	current.lastSequence = item.Sequence
	switch data := item.Data.(type) {
	case inbox.Input:
		if s.supervisor.config.RequireCleanupProof && current.launched && data.Kind == inbox.InputControl {
			control, err := data.DecodeControlMessage()
			if err == nil && control.Mode == inbox.ExecutionDrained {
				if receipt, ok := control.Parameters.(inbox.ExecutionDrainReceipt); ok && receipt.Version == 1 && receipt.Scope == current.cleanupScope {
					current.cleanupProven = true
				}
			}
		}
		for _, message := range current.unacked {
			if message.input.ID == data.ID {
				s.forgetMessage(message)
				s.supervisor.activity.input(current.sessionID, generation, string(data.ID), "persisted")
				s.finishBarrier(message.operation, operation.StatusCompleted,
					fmt.Sprintf("Delivered to subagent %q. It replies with SendMessage or in its Agent result.", current.name), "", "persisted")
				break
			}
		}
	case sessionstore.ModelResponse:
		if finalReport(data.Response) {
			s.supervisor.activity.change(current.sessionID, generation, func(c *ChildActivity) {
				if !c.ReportAvailable {
					c.ReportAvailable = true
					c.ReportAvailableAt = time.Now()
				}
			})
		}
		var texts []string
		for _, output := range data.Response.Output {
			switch value := output.Data.(type) {
			case llm.Message:
				if strings.TrimSpace(value.Text) != "" {
					texts = append(texts, value.Text)
				}
			case llm.ToolCall:
				if value.Name != tool.SendMessageName {
					continue
				}
				message, err := toolsubagent.ParseMessage(value.Arguments)
				if err != nil || message.To != toolsubagent.ParentName {
					continue
				}
				s.deliverToParent(current, data, value.CallID, message.Message)
			}
		}
		if len(texts) != 0 {
			current.reply = strings.Join(texts, "\n\n")
			s.acknowledgeBackground(current, data.Response)
			if current.metadata.Lifetime == "task" && current.owner != nil && finalReport(data.Response) {
				s.requestStop(current.owner.ID, "task-scoped final report cleanup", true)
			}
		}
	}
}

// acknowledgeBackground releases only the strict explicit-background Run ticket.
// It accepts full committed assistant responses, including responses dispatching
// more tools, but makes no final-report or effect/descendant-cleanup assertion.
func (s *state) acknowledgeBackground(current *child, response llm.Response) {
	if !s.supervisor.config.RequireCleanupProof || !current.launched || current.cleanupScope == "" ||
		current.metadata.Lifetime != "background" || current.backgroundAcknowledged || current.stopping ||
		current.owner == nil || current.failure != "" || response.Stop != llm.StopComplete || response.Failure != nil {
		return
	}
	owner := *current.owner
	if owner.Status != operation.StatusReady && owner.Status != operation.StatusAwaiting {
		return
	}
	remote, err := operation.DecodeRemoteJobState(owner)
	if err != nil || remote.Plan.Type != toolsubagent.RunPlanType || remote.Plan.Version != toolsubagent.PlanVersion {
		return
	}
	var plan toolsubagent.RunPlan
	if json.Unmarshal(remote.Plan.Data, &plan) != nil || plan.Lifetime != "background" {
		return
	}
	var texts []string
	for _, output := range response.Output {
		if message, ok := output.Data.(llm.Message); output.Type == llm.ItemMessage && ok &&
			message.Role == llm.RoleAssistant && strings.TrimSpace(message.Text) != "" {
			texts = append(texts, message.Text)
		}
	}
	if len(texts) == 0 {
		return
	}
	remote.TerminalResult = "Background branch acknowledged; child remains owned/running (not cleanup or task completion).\n\n" + strings.Join(texts, "\n\n")
	remote.TerminalError = ""
	remote.ResultTruncated, remote.ErrorTruncated = false, false
	// Do not use finish: its terminalEnqueued diagnostic implies owner drain.
	// As with finish, preparation backpressure retains a bounded terminal retry.
	if _, ok := s.updateThen(owner, remote, operation.StatusCompleted, nil); !ok {
		s.pendingFinals = append(s.pendingFinals, terminalRetry{value: owner, remote: remote, status: operation.StatusCompleted})
	}
	current.backgroundAcknowledged = true
}

func (s *state) updateStatusUnlessAcknowledged(current *child, status operation.Status) (operation.Operation, bool) {
	if current.backgroundAcknowledged {
		return *current.owner, false
	}
	return s.updateStatus(*current.owner, status)
}

func (s *state) deliverToParent(current *child, response sessionstore.ModelResponse, callID, message string) {
	payload, err := json.Marshal(fmt.Sprintf("Message from subagent %q:\n%s", current.name, message))
	if err != nil {
		return
	}
	// Session, turn and call IDs keep the ID stable across redeliveries.
	id := inbox.ID(fmt.Sprintf("subagent:%s:%s:%s", current.sessionID, response.TurnID, callID))
	s.deliveryInputs = append(s.deliveryInputs, inbox.Input{ID: id, Kind: inbox.InputExternal, Payload: payload})
}

func (s *state) exited(current *child, generation int, err error) {
	if current.generation != generation {
		return
	}
	if s.supervisor.config.RequireCleanupProof && current.launched && !current.cleanupProven {
		proofErr := fmt.Errorf("subagent %q generation %d exited without fresh cleanup proof", current.name, generation)
		s.supervisor.cleanupErr = errors.Join(s.supervisor.cleanupErr, proofErr)
		err = errors.Join(err, proofErr)
		current.failure = proofErr.Error()
		current.cleanup = false
	}
	if current.live {
		current.live = false
		s.activeProcesses--
	}
	defer s.supervisor.activity.change(current.sessionID, generation, func(c *ChildActivity) {
		c.OwnerExitProcessed = true
		c.OwnerExitProcessedAt = time.Now()
		c.StartFailed = c.ProcessStartedAt.IsZero()
	})
	if current.stdin != nil {
		_ = current.stdin.Close()
		current.stdin = nil
	}
	if current.stop != nil {
		current.stop()
		current.stop = nil
	}
	terminalEnqueued := false
	if owner := current.owner; owner != nil {
		delete(s.owners, owner.ID)
		current.owner = nil
		switch {
		case current.backgroundAcknowledged:
			// The Run ticket was already published. Only now, after actual process,
			// reader and fresh receipt checks, may diagnostics call its owner drained.
			if current.cleanupProven {
				s.supervisor.activity.terminalEnqueued(owner.ID)
			}
		case s.supervisor.config.RequireCleanupProof && current.launched && !current.cleanupProven:
			terminalEnqueued = s.finish(*owner, operation.StatusFailed, current.reply, current.failure)
		case current.failure == "parent inbox admission failed":
			terminalEnqueued = s.finish(*owner, operation.StatusFailed, current.reply, current.failure)
		case current.stopping && current.cleanup:
			terminalEnqueued = s.finish(*owner, operation.StatusCompleted, current.reply, "")
		case current.stopping:
			terminalEnqueued = s.finish(*owner, operation.StatusCanceled, current.reply, "subagent stopped")
		case err != nil:
			failure := current.failure
			if failure == "" {
				failure = err.Error()
			}
			terminalEnqueued = s.finish(*owner, operation.StatusFailed, current.reply, fmt.Sprintf("subagent %q failed: %s", current.name, failure))
		default:
			reply := current.reply
			if reply == "" {
				reply = "Subagent finished without a final message."
			}
			terminalEnqueued = s.finish(*owner, operation.StatusCompleted, reply, "")
		}
	}
	_ = terminalEnqueued // Actual publication is recorded by the preparation continuation.
	if err != nil || current.stopping || len(current.unacked) == 0 || s.supervisor.ctx.Err() != nil {
		for _, message := range current.unacked {
			delete(s.messages, message.operation.ID)
			s.finish(message.operation, operation.StatusFailed, "", fmt.Sprintf("subagent %q exited before receiving the message", current.name))
		}
		current.unacked = nil
		return
	}
	// Messages the process never persisted resume its session. Their IDs are
	// unchanged, so a message that was persisted after all is not repeated.
	unacked := current.unacked
	current.unacked = nil
	owner := unacked[0].operation
	inputs := make([]inbox.Input, 0, len(unacked))
	for _, message := range unacked {
		inputs = append(inputs, message.input)
	}
	delete(s.messages, owner.ID)
	s.start(current, &owner, inputs)
	// Only the first replayed message owns the process. Remaining messages still
	// require their matching persisted inbox item; initial-request write is not ack.
	for _, message := range unacked[1:] {
		message.frame = nil
		current.unacked = append(current.unacked, message)
	}

}

func (s *state) forgetMessage(message *pendingMessage) {
	delete(s.messages, message.operation.ID)
	current := message.child
	for index, pending := range current.unacked {
		if pending == message {
			current.unacked = append(current.unacked[:index], current.unacked[index+1:]...)
			return
		}
	}
}

func (s *state) fail(value operation.Operation, message string) {
	s.finish(value, operation.StatusFailed, "", message)
}

func (s *state) finish(value operation.Operation, status operation.Status, result, failure string) bool {
	return s.finishBarrier(value, status, result, failure, "")
}
func (s *state) finishBarrier(value operation.Operation, status operation.Status, result, failure, barrier string) bool {
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		remote = operation.RemoteJobState{}
	}
	if remote.Plan.Type == ControlPlanType && status == operation.StatusCompleted {
		var p toolsubagent.ControlPlan
		if json.Unmarshal(remote.Plan.Data, &p) == nil && p.Action == "message" {
			delivery := "persisted"
			state := "delivered"
			drained := false
			// Run owners reach finish from actual process/output/delivery drain. A
			// matching inbox Item finishes ordinary messages with its distinct barrier.
			if barrier == "persisted" {
				delivery = "persisted"
			} else {
				delivery = "process_drained"
				state = "drained"
				drained = true
			}
			result = controlOutcome(p, state, delivery, drained, nil)
		}
	}
	remote.TerminalResult, remote.TerminalError = result, failure
	remote.ResultTruncated, remote.ErrorTruncated = false, false
	done := func(state *state, prepared operation.Operation) {
		if state.supervisor != nil {
			state.supervisor.activity.terminalEnqueued(prepared.ID)
		}
	}
	_, ok := s.updateThen(value, remote, status, done)
	if !ok && s.supervisor != nil {
		// A full preparation budget is backpressure, not a lost terminal frame. The
		// bounded admitted-operation set bounds this FIFO; strings reference owned
		// immutable replies rather than an unbounded spool or per-event goroutine.
		s.pendingFinals = append(s.pendingFinals, terminalRetry{value: value, remote: remote, status: status, done: done})
		return true
	}
	return ok
}

func (s *state) updateStatus(value operation.Operation, status operation.Status) (operation.Operation, bool) {
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		return value, false
	}
	return s.update(value, remote, status)
}

func (s *state) update(value operation.Operation, remote operation.RemoteJobState, status operation.Status) (operation.Operation, bool) {
	return s.updateThen(value, remote, status, nil)
}
func (s *state) updateThen(value operation.Operation, remote operation.RemoteJobState, status operation.Status, done func(*state, operation.Operation)) (operation.Operation, bool) {
	if s.supervisor != nil {
		return s.prepare(value, remote, status, done)
	}
	step, err := operation.UpdateRemoteJob(value, remote, status)
	if err != nil || step.Operation == nil {
		return value, false
	}
	updated := *step.Operation
	s.enqueue(updated)
	if done != nil {
		done(s, updated)
	}
	return updated, true
}

func planType(value operation.Operation) operation.RemoteJobPlanType {
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		return ""
	}
	return remote.Plan.Type
}

func newMessageInput(text string) inbox.Input {
	payload, _ := json.Marshal(text)
	return inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputExternal, Payload: payload}
}

func writeInput(stdin io.Writer, input inbox.Input) error {
	if stdin == nil {
		return errors.New("subagent is not running")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	_, err = stdin.Write(append(encoded, '\n'))
	return err
}

func (s *state) enqueue(updated operation.Operation) {
	if planType(updated) == ControlPlanType && s.supervisor != nil && s.supervisor.config.Hub != nil {
		s.supervisor.config.Hub.control.out <- updated // <= 3 updates per each of 256 admitted control IDs.
	} else if planType(updated) == toolsubagent.MessagePlanType {
		s.messageQueue = append(s.messageQueue, updated)
	} else {
		s.runQueue = append(s.runQueue, updated)
	}
}
func (s *state) flushDelivery(c *child, generation int, n int) {
	inputs := s.deliveryInputs
	s.deliveryInputs = nil
	if len(inputs) == 0 {
		s.supervisor.releaseDelivery(n)
		<-s.supervisor.deliverySlots
		return
	}
	c.parentPending++
	s.supervisor.deliveries <- deliveryJob{target: c, generation: generation, inputs: inputs, bytes: n}
}

// post is reserved for owned workers. Shutdown stops external admission but must
// still process their finite completion/drain continuations before actor exit.
func (current *Supervisor) post(request func(*state)) error {
	current.changeWork(func() { current.workRequests++ })
	select {
	case current.requests <- func(s *state) {
		request(s)
		s.publishWork(1)
	}:
		return nil
	case <-current.done:
		current.changeWork(func() { current.workRequests-- })
		return errors.New("subagent supervisor joined")
	}
}
