// Package subagent translates the Agent and SendMessage tools. A subagent is a
// separate agent process; agents exchange messages as inbox inputs.
package subagent

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const (
	// RunPlanType starts a named subagent and completes with its final message.
	RunPlanType operation.RemoteJobPlanType = "subagent.run"
	// MessagePlanType delivers a message to a named subagent. It completes on
	// delivery to a running subagent, or with the reply of a resumed one.
	MessagePlanType operation.RemoteJobPlanType    = "subagent.message"
	PlanVersion     operation.RemoteJobPlanVersion = 1

	// ParentName addresses the parent agent from a subagent.
	ParentName = "parent"
)

type RunPlan struct {
	Name   string
	Prompt string
}

type MessagePlan struct {
	To      string
	Message string
}

// RunHandle is stored in a run operation's remote job handle.
type RunHandle struct {
	SessionID string
}

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

func ValidateName(name string) error {
	if name == ParentName {
		return fmt.Errorf("subagent name %q is reserved", ParentName)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("subagent name %q must match %s", name, namePattern)
	}
	return nil
}

// NewAgent returns the parent's Agent translator.
func NewAgent() tool.Translator {
	return agentTranslator{}
}

// NewSendMessage returns the parent's SendMessage translator, which addresses
// subagents by name.
func NewSendMessage() tool.Translator {
	return sendMessageTranslator{}
}

// NewSendToParent returns a subagent's SendMessage translator. The call
// completes immediately; the parent's supervisor reads the call from the
// subagent's session stream and delivers it to the parent's inbox.
func NewSendToParent() tool.Translator {
	return sendToParentTranslator{}
}

type agentTranslator struct{}

func (agentTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	var arguments struct {
		Name   string `json:"name"`
		Prompt string `json:"prompt"`
	}
	if err := decodeArguments(call.Arguments, &arguments); err != nil {
		return tool.ErrorStatus(fmt.Sprintf("decode Agent arguments: %v", err), 0)
	}
	if err := ValidateName(arguments.Name); err != nil {
		return tool.ErrorStatus(err.Error(), 0)
	}
	if strings.TrimSpace(arguments.Prompt) == "" {
		return tool.ErrorStatus(`agent argument "prompt" must be set`, 0)
	}
	return submitRemoteJob(ctx, RunPlanType, RunPlan{Name: arguments.Name, Prompt: arguments.Prompt})
}

func (agentTranslator) TranslateResult(callID string, status tool.CallStatus, operations []operation.Operation) (tool.Result, error) {
	return remoteJobResult(callID, status, operations, "Subagent is running. Messages it sends arrive as user messages; its final message arrives as this tool's result.")
}

type sendMessageTranslator struct{}

func (sendMessageTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	arguments, err := decodeMessageArguments(call.Arguments)
	if err != nil {
		return tool.ErrorStatus(err.Error(), 0)
	}
	if err := ValidateName(arguments.To); err != nil {
		return tool.ErrorStatus(err.Error(), 0)
	}
	return submitRemoteJob(ctx, MessagePlanType, MessagePlan(arguments))
}

func (sendMessageTranslator) TranslateResult(callID string, status tool.CallStatus, operations []operation.Operation) (tool.Result, error) {
	return remoteJobResult(callID, status, operations, "Subagent resumed to handle the message. Its final message arrives as this tool's result.")
}

type sendToParentTranslator struct{}

func (sendToParentTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	arguments, err := decodeMessageArguments(call.Arguments)
	if err != nil {
		return tool.ErrorStatus(err.Error(), 0)
	}
	if arguments.To != ParentName {
		return tool.ErrorStatus(fmt.Sprintf("subagents can only message %q", ParentName), 0)
	}
	value, err := json.Marshal("Message sent to parent.")
	if err != nil {
		return tool.ErrorStatus(fmt.Sprintf("encode SendMessage result: %v", err), 0)
	}
	spec, err := operation.NewValueSpec(value)
	if err != nil {
		return tool.ErrorStatus(fmt.Sprintf("build SendMessage operation: %v", err), 0)
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(spec)}}
}

func (sendToParentTranslator) TranslateResult(callID string, status tool.CallStatus, operations []operation.Operation) (tool.Result, error) {
	if status.Error != "" {
		return Result{CallID: callID, Error: status.Error}, nil
	}
	if len(operations) != 1 {
		return nil, fmt.Errorf("SendMessage call %q has %d operations, want 1", callID, len(operations))
	}
	current := operations[0]
	switch current.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling:
		return Result{CallID: callID, Running: true, Text: "Sending message."}, nil
	case operation.StatusCompleted:
		value, err := operation.DecodeValue(current)
		if err != nil {
			return nil, err
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return nil, fmt.Errorf("decode SendMessage call %q result: %w", callID, err)
		}
		return Result{CallID: callID, Text: text}, nil
	default:
		return Result{CallID: callID, Error: "SendMessage operation " + string(current.Status)}, nil
	}
}

// ParseMessage decodes SendMessage arguments as a subagent's supervisor reads
// them from the subagent's session stream.
func ParseMessage(arguments string) (MessagePlan, error) {
	parsed, err := decodeMessageArguments(arguments)
	return MessagePlan(parsed), err
}

type messageArguments struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

func decodeMessageArguments(encoded string) (messageArguments, error) {
	var arguments messageArguments
	if err := decodeArguments(encoded, &arguments); err != nil {
		return messageArguments{}, fmt.Errorf("decode SendMessage arguments: %w", err)
	}
	if strings.TrimSpace(arguments.To) == "" {
		return messageArguments{}, errors.New(`sendMessage argument "to" must be set`)
	}
	if strings.TrimSpace(arguments.Message) == "" {
		return messageArguments{}, errors.New(`sendMessage argument "message" must be set`)
	}
	return arguments, nil
}

func decodeArguments(encoded string, destination any) error {
	if strings.TrimSpace(encoded) == "" {
		encoded = "{}"
	}
	return json.Unmarshal([]byte(encoded), destination)
}

func submitRemoteJob(ctx tool.Context, planType operation.RemoteJobPlanType, plan any) tool.CallStatus {
	data, err := json.Marshal(plan)
	if err != nil {
		return tool.ErrorStatus(fmt.Sprintf("encode %s plan: %v", planType, err), 0)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: planType, Version: PlanVersion, Data: data})
	if err != nil {
		return tool.ErrorStatus(fmt.Sprintf("build %s operation: %v", planType, err), 0)
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(spec)}}
}

func remoteJobResult(callID string, status tool.CallStatus, operations []operation.Operation, running string) (tool.Result, error) {
	if status.Error != "" {
		if len(operations) != 0 {
			return nil, fmt.Errorf("subagent tool call %q has both a validation error and operations", callID)
		}
		return Result{CallID: callID, Error: status.Error}, nil
	}
	if len(operations) != 1 {
		return nil, fmt.Errorf("subagent tool call %q has %d operations, want 1", callID, len(operations))
	}
	current := operations[0]
	state, err := operation.DecodeRemoteJobState(current)
	if err != nil {
		return nil, fmt.Errorf("decode subagent tool call %q state: %w", callID, err)
	}
	switch current.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling:
		return Result{CallID: callID, Running: true, Text: running}, nil
	case operation.StatusCompleted:
		return Result{CallID: callID, Text: state.TerminalResult}, nil
	case operation.StatusFailed, operation.StatusCanceled:
		if state.TerminalError == "" {
			state.TerminalError = "subagent operation " + string(current.Status)
		}
		return Result{CallID: callID, Text: state.TerminalResult, Error: state.TerminalError}, nil
	default:
		return nil, fmt.Errorf("subagent tool call %q operation %q has invalid status %q", callID, current.ID, current.Status)
	}
}

type Result struct {
	CallID  string
	Running bool
	Text    string
	Error   string
}

func (result Result) ToLLMResult() llm.ToolResult {
	parts := make([]string, 0, 2)
	if result.Text != "" {
		parts = append(parts, result.Text)
	}
	if result.Error != "" {
		parts = append(parts, "Error: "+result.Error)
	}
	if len(parts) == 0 {
		parts = append(parts, "(no output)")
	}
	return llm.ToolResult{
		CallID:  result.CallID,
		Output:  []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: strings.Join(parts, "\n")}},
		Running: result.Running,
	}
}
