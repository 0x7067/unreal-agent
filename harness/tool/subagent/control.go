package subagent

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// ControlPlanType uses version 1 of an additive remote plan. Translation only
// records inert data; a session-owned supervisor dispatches after persistence.
const ControlPlanType operation.RemoteJobPlanType = "subagent.control"

// ControlPlan addresses an exact live attempt. Handles are opaque capabilities
// scoped to one root session; display names must never be used as authority.
type ControlPlan struct {
	Action     string
	Handle     string `json:",omitzero"`
	Generation uint64 `json:",omitzero"`
	Message    string `json:",omitzero"`
}

// ControlOutcome is the version-1 terminal payload emitted by a control handler.
// Delivery identifies the proven barrier (for example "persisted"), not model
// consumption. A completed stop request is not proof of drain.
type ControlOutcome struct {
	Action         string
	Handle         string `json:",omitzero"`
	Generation     uint64 `json:",omitzero"`
	State          string
	Delivery       string `json:",omitzero"`
	DrainConfirmed bool
	Snapshot       jsontext.Value `json:",omitzero"`
}

// ControlResult keeps operation lifecycle separate from the control outcome.
// In particular StatusCompleted may mean cancellation_requested with no drain.
type ControlResult struct {
	CallID  string
	Status  operation.Status
	Running bool
	Outcome *ControlOutcome
	Text    string
	Error   string
}

func (r ControlResult) ToLLMResult() llm.ToolResult {
	return (Result{CallID: r.CallID, Running: r.Running, Text: r.Text, Error: r.Error}).ToLLMResult()
}

func NewAgentControl() tool.Translator { return controlTranslator{} }

type controlTranslator struct{}

func (controlTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	var args struct {
		Action     string `json:"action"`
		Handle     string `json:"handle"`
		Generation uint64 `json:"generation"`
		Message    string `json:"message"`
	}
	if err := decodeArguments(call.Arguments, &args); err != nil {
		return tool.ErrorStatus(fmt.Sprintf("decode AgentControl arguments: %v", err), 0)
	}
	plan := ControlPlan{Action: args.Action, Handle: args.Handle, Generation: args.Generation, Message: args.Message}
	if err := plan.Validate(); err != nil {
		return tool.ErrorStatus(err.Error(), 0)
	}
	return submitRemoteJob(ctx, ControlPlanType, plan)
}

// Validate is shared with dispatchers; persisted/untrusted plans must receive
// the same validation as model arguments, without touching the target.
func (p ControlPlan) Validate() error {
	switch p.Action {
	case "list":
		if p.Handle != "" || p.Generation != 0 || p.Message != "" {
			return fmt.Errorf("list does not take handle, generation or message")
		}
	case "status", "message", "stop":
		if strings.TrimSpace(p.Handle) == "" || p.Generation == 0 {
			return fmt.Errorf("%s requires an opaque handle and positive generation", p.Action)
		}
		if p.Action == "message" && strings.TrimSpace(p.Message) == "" {
			return fmt.Errorf("message requires nonempty message text")
		}
		if p.Action != "message" && p.Message != "" {
			return fmt.Errorf("%s does not take message text", p.Action)
		}
	default:
		return fmt.Errorf("unknown AgentControl action %q", p.Action)
	}
	return nil
}

func (controlTranslator) TranslateResult(callID string, status tool.CallStatus, operations []operation.Operation) (tool.Result, error) {
	generic, err := remoteJobResult(callID, status, operations, "Agent control pending. Admission does not prove delivery, model consumption, or completed stop/drain.")
	if err != nil {
		return nil, err
	}
	result := generic.(Result)
	typed := ControlResult{CallID: result.CallID, Running: result.Running, Text: result.Text, Error: result.Error}
	if len(operations) == 1 {
		typed.Status = operations[0].Status
	}
	if !result.Running && result.Text != "" {
		var outcome ControlOutcome
		// Keep reading historical/raw diagnostic payloads. Only label an outcome
		// typed when it actually declares its barrier; arbitrary JSON is not proof.
		if json.Unmarshal([]byte(result.Text), &outcome) == nil && outcome.Action != "" && outcome.State != "" {
			typed.Outcome = &outcome
		}
	}
	return typed, nil
}
