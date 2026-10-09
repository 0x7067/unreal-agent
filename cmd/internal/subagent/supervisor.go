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
	InboxStdinFlag = "-inbox-stdin"
	SubagentFlag   = "-subagent"
)

const (
	restartedPrompt = "Your parent agent restarted while you were working. Continue your task."
	stopWaitDelay   = 10 * time.Second
)

type Config struct {
	// Command is the runner executable followed by any leading arguments.
	Command          []string
	Env              []string // Nil inherits the parent environment.
	SessionDirectory string
	Workspace        string
	Model            string
	ThinkingLevel    string
	// Inbox receives the subagents' messages to the parent.
	Inbox  inbox.Writer
	Stderr io.Writer
}

type Supervisor struct {
	ctx      context.Context
	config   Config
	requests chan func(*state)
	run      *handler
	message  *handler
}

// New starts a supervisor. Its child processes stop when ctx is canceled.
func New(ctx context.Context, config Config) (*Supervisor, error) {
	if len(config.Command) == 0 || strings.TrimSpace(config.Command[0]) == "" {
		return nil, errors.New("subagent command must be set")
	}
	if config.Inbox == nil {
		return nil, errors.New("subagent parent inbox must be set")
	}
	current := &Supervisor{ctx: ctx, config: config, requests: make(chan func(*state))}
	current.run = &handler{planType: toolsubagent.RunPlanType, supervisor: current, updates: make(chan operation.Operation)}
	current.message = &handler{planType: toolsubagent.MessagePlanType, supervisor: current, updates: make(chan operation.Operation)}
	go current.loop()
	return current, nil
}

// Handlers returns the remote job handlers for the operation manager.
func (current *Supervisor) Handlers() []operation.RemoteJobHandler {
	return []operation.RemoteJobHandler{current.run, current.message}
}

func (current *Supervisor) do(request func(*state)) error {
	select {
	case current.requests <- request:
		return nil
	case <-current.ctx.Done():
		return current.ctx.Err()
	}
}

func (current *Supervisor) loop() {
	s := &state{
		supervisor: current,
		children:   make(map[string]*child),
		owners:     make(map[operation.ID]*child),
		messages:   make(map[operation.ID]*pendingMessage),
	}
	for {
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
		case <-current.ctx.Done():
			return
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
	return current.supervisor.do(func(s *state) { s.add(current.planType, value) })
}

func (current *handler) CancelRemoteJob(id operation.ID, reason string) error {
	return current.supervisor.do(func(s *state) { s.cancel(id, reason) })
}

func (current *handler) RemoteJobUpdates() <-chan operation.Operation { return current.updates }

// state is owned by the supervisor loop.
type state struct {
	supervisor   *Supervisor
	children     map[string]*child
	owners       map[operation.ID]*child // Operations that finish when the process exits.
	messages     map[operation.ID]*pendingMessage
	runQueue     []operation.Operation
	messageQueue []operation.Operation
}

type child struct {
	name       string
	sessionID  string
	generation int
	stdin      io.WriteCloser // Set while the process runs.
	stop       context.CancelFunc
	owner      *operation.Operation
	stopping   bool
	unacked    []*pendingMessage
	reply      string
	failure    string
}

type pendingMessage struct {
	operation operation.Operation
	child     *child
	input     inbox.Input
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
	prompt := plan.Prompt
	var handle toolsubagent.RunHandle
	if len(remote.Handle) != 0 {
		// The parent restarted; resume the recorded subagent session.
		if err := json.Unmarshal(remote.Handle, &handle); err != nil || handle.SessionID == "" {
			s.fail(value, "subagent handle has no session ID")
			return
		}
		prompt = restartedPrompt
	}
	if existing, exists := s.children[plan.Name]; exists && existing.sessionID != handle.SessionID {
		s.fail(value, fmt.Sprintf("subagent %q already exists; message it with SendMessage or choose another name", plan.Name))
		return
	} else if exists && existing.stdin != nil {
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
		current = &child{name: plan.Name, sessionID: handle.SessionID}
		s.children[plan.Name] = current
	}
	value, ok := s.update(value, remote, operation.StatusAwaiting)
	if !ok {
		return
	}
	s.start(current, &value, []inbox.Input{newMessageInput(prompt)})
}

func (s *state) addMessage(value operation.Operation, plan toolsubagent.MessagePlan) {
	current, exists := s.children[plan.To]
	if !exists {
		s.fail(value, fmt.Sprintf("unknown subagent %q; start it with Agent first", plan.To))
		return
	}
	value, ok := s.updateStatus(value, operation.StatusAwaiting)
	if !ok {
		return
	}
	input := newMessageInput("Message from your parent agent:\n" + plan.Message)
	if current.stdin == nil {
		s.start(current, &value, []inbox.Input{input})
		return
	}
	message := &pendingMessage{operation: value, child: current, input: input}
	s.messages[value.ID] = message
	current.unacked = append(current.unacked, message)
	if err := writeInput(current.stdin, input); err != nil {
		// The process is exiting; its exit handler resumes it with this message.
		return
	}
}

func (s *state) cancel(id operation.ID, reason string) {
	if current, exists := s.owners[id]; exists && current.owner != nil && current.owner.ID == id {
		if current.stopping {
			return
		}
		current.stopping = true
		if value, ok := s.updateStatus(*current.owner, operation.StatusCanceling); ok {
			current.owner = &value
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
		s.forgetMessage(message)
		s.finish(message.operation, operation.StatusCanceled, "", "message canceled before the subagent received it")
	}
}

func (s *state) start(current *child, owner *operation.Operation, messages []inbox.Input) {
	current.generation++
	generation := current.generation
	current.owner = owner
	current.stopping = false
	current.reply, current.failure = "", ""
	s.owners[owner.ID] = current

	type requestMessage struct {
		Content   string `json:"content"`
		MessageID string `json:"message_id"`
	}
	request := struct {
		SessionID     string           `json:"session_id"`
		Messages      []requestMessage `json:"messages"`
		Model         string           `json:"model,omitempty"`
		ThinkingLevel string           `json:"thinking_level,omitempty"`
	}{SessionID: current.sessionID, Model: s.supervisor.config.Model, ThinkingLevel: s.supervisor.config.ThinkingLevel}
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

	config := s.supervisor.config
	ctx, stop := context.WithCancel(s.supervisor.ctx)
	args := append(append([]string(nil), config.Command[1:]...), InboxStdinFlag, SubagentFlag)
	if config.SessionDirectory != "" {
		args = append(args, "-session-directory", config.SessionDirectory)
	}
	if config.Workspace != "" {
		args = append(args, "-workspace", config.Workspace)
	}
	// The request goes to stdin, not argv, so other local users cannot read it.
	command := exec.CommandContext(ctx, config.Command[0], args...)
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = stopWaitDelay
	command.Dir = config.Workspace
	command.Env = config.Env
	command.Stderr = config.Stderr
	stdin, err := command.StdinPipe()
	if err == nil {
		var stdout io.ReadCloser
		stdout, err = command.StdoutPipe()
		if err == nil {
			err = command.Start()
		}
		if err == nil {
			current.stdin, current.stop = stdin, stop
			go s.supervisor.read(current, generation, command, stdout)
			if _, err := stdin.Write(append(encoded, '\n')); err != nil {
				// The reader reports the exit and fails the owner.
				stop()
			}
			return
		}
	}
	stop()
	s.exited(current, generation, fmt.Errorf("start subagent %q: %w", current.name, err))
}

func (current *Supervisor) read(target *child, generation int, command *exec.Cmd, stdout io.Reader) {
	lines := bufio.NewReader(stdout)
	for {
		line, err := lines.ReadBytes('\n')
		if line = bytes.TrimSpace(line); len(line) != 0 {
			if current.do(func(s *state) { s.line(target, generation, line) }) != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	err := command.Wait()
	_ = current.do(func(s *state) { s.exited(target, generation, err) })
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
	switch data := item.Data.(type) {
	case inbox.Input:
		for _, message := range current.unacked {
			if message.input.ID == data.ID {
				s.forgetMessage(message)
				s.finish(message.operation, operation.StatusCompleted,
					fmt.Sprintf("Delivered to subagent %q. It replies with SendMessage or in its Agent result.", current.name), "")
				break
			}
		}
	case sessionstore.ModelResponse:
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
		}
	}
}

func (s *state) deliverToParent(current *child, response sessionstore.ModelResponse, callID, message string) {
	payload, err := json.Marshal(fmt.Sprintf("Message from subagent %q:\n%s", current.name, message))
	if err != nil {
		return
	}
	// Session, turn and call IDs keep the ID stable across redeliveries.
	id := inbox.ID(fmt.Sprintf("subagent:%s:%s:%s", current.sessionID, response.TurnID, callID))
	_ = s.supervisor.config.Inbox.Submit(s.supervisor.ctx, inbox.Input{ID: id, Kind: inbox.InputExternal, Payload: payload})
}

func (s *state) exited(current *child, generation int, err error) {
	if current.generation != generation {
		return
	}
	if current.stdin != nil {
		_ = current.stdin.Close()
		current.stdin = nil
	}
	if current.stop != nil {
		current.stop()
		current.stop = nil
	}
	if owner := current.owner; owner != nil {
		delete(s.owners, owner.ID)
		current.owner = nil
		switch {
		case current.stopping:
			s.finish(*owner, operation.StatusCanceled, current.reply, "subagent stopped")
		case err != nil:
			failure := current.failure
			if failure == "" {
				failure = err.Error()
			}
			s.finish(*owner, operation.StatusFailed, current.reply, fmt.Sprintf("subagent %q failed: %s", current.name, failure))
		default:
			reply := current.reply
			if reply == "" {
				reply = "Subagent finished without a final message."
			}
			s.finish(*owner, operation.StatusCompleted, reply, "")
		}
	}
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
	for index, message := range unacked {
		delete(s.messages, message.operation.ID)
		inputs = append(inputs, message.input)
		if index != 0 {
			s.finish(message.operation, operation.StatusCompleted,
				fmt.Sprintf("Delivered to subagent %q with an earlier message; its reply arrives in that message's result.", current.name), "")
		}
	}
	s.start(current, &owner, inputs)
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

func (s *state) finish(value operation.Operation, status operation.Status, result, failure string) {
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		remote = operation.RemoteJobState{}
	}
	remote.TerminalResult, remote.TerminalError = result, failure
	remote.ResultTruncated, remote.ErrorTruncated = false, false
	s.update(value, remote, status)
}

func (s *state) updateStatus(value operation.Operation, status operation.Status) (operation.Operation, bool) {
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		return value, false
	}
	return s.update(value, remote, status)
}

func (s *state) update(value operation.Operation, remote operation.RemoteJobState, status operation.Status) (operation.Operation, bool) {
	step, err := operation.UpdateRemoteJob(value, remote, status)
	if err != nil || step.Operation == nil {
		return value, false
	}
	updated := *step.Operation
	if planType(updated) == toolsubagent.MessagePlanType {
		s.messageQueue = append(s.messageQueue, updated)
	} else {
		s.runQueue = append(s.runQueue, updated)
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
