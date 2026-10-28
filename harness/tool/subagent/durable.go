package subagent

import (
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// DurablePlanVersion is the opt-in subagent.run remote plan version. It does
// not change NewAgent's version-1 wire contract.
const DurablePlanVersion operation.RemoteJobPlanVersion = 2

// DurableRunPlan is inert, parent-bound intent, not execution authority. A
// dispatcher must compare ParentID with its explicitly configured stable parent;
// fresh context/hub capabilities and graph ownership are never inferred here.
// Version versions this payload independently of the remote plan envelope.
type DurableRunPlan struct {
	Version  int
	ParentID session.ID
	Run      RunPlan
}

// Validate checks canonical intent without I/O, environment, or runtime state.
func (p DurableRunPlan) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("unsupported durable run payload version %d", p.Version)
	}
	if err := durableID("parent ID", string(p.ParentID), true); err != nil {
		return err
	}
	if err := ValidateName(p.Run.Name); err != nil {
		return err
	}
	if strings.TrimSpace(p.Run.Prompt) == "" || len(p.Run.Prompt) > 256*1024 {
		return fmt.Errorf("prompt must be nonempty and at most 256 KiB")
	}
	if p.Run.Lifetime != "background" {
		return fmt.Errorf("durable children currently require explicit lifetime=background; task/default lifetime cleanup is unsupported")
	}
	if len(p.Run.Purpose) > 2048 || len(p.Run.ParentTask) > 2048 {
		return fmt.Errorf("purpose and parent task must be at most 2048 bytes")
	}
	if len(p.Run.OwnedResources) > 64 {
		return fmt.Errorf("owned resources must have at most 64 entries")
	}
	for _, resource := range p.Run.OwnedResources {
		if len(resource) > 2048 {
			return fmt.Errorf("owned resource must be at most 2048 bytes")
		}
	}
	if err := durableID("graph ID", p.Run.GraphID, false); err != nil {
		return err
	}
	return durableID("task ID", p.Run.TaskID, false)
}

func durableID(label, value string, required bool) error {
	if !required && value == "" {
		return nil
	}
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || len(value) > 128 {
		return fmt.Errorf("%s must be trimmed, nonempty and at most 128 bytes", label)
	}
	return nil
}

// DecodeDurableRun validates the remote envelope and strictly decodes all
// canonical plan members. Parent authorization remains the dispatcher's job.
func DecodeDurableRun(op operation.Operation) (DurableRunPlan, error) {
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return DurableRunPlan{}, err
	}
	if state.Plan.Type != RunPlanType || state.Plan.Version != DurablePlanVersion {
		return DurableRunPlan{}, fmt.Errorf("unsupported durable run plan %q version %d: %w", state.Plan.Type, state.Plan.Version, operation.ErrUnsupported)
	}
	if err := json.Unmarshal(op.State, &state, json.RejectUnknownMembers(true)); err != nil {
		return DurableRunPlan{}, fmt.Errorf("decode durable run envelope: %w", err)
	}
	var plan DurableRunPlan
	if err := json.Unmarshal(state.Plan.Data, &plan, json.RejectUnknownMembers(true)); err != nil {
		return DurableRunPlan{}, fmt.Errorf("decode durable run: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return DurableRunPlan{}, err
	}
	return plan, nil
}

// NewDurableAgent returns the opt-in Agent translator using the same arguments
// and results as NewAgent, bound to an explicitly configured stable parent ID.
func NewDurableAgent(parent session.ID) tool.Translator {
	return durableAgentTranslator{parent: parent}
}

type durableAgentTranslator struct{ parent session.ID }

func (t durableAgentTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	intercepted := &durableSubmitContext{Context: ctx, parent: t.parent}
	status := (agentTranslator{}).Translate(intercepted, call)
	if intercepted.err != nil {
		return tool.ErrorStatus(intercepted.err.Error(), 0)
	}
	return status
}

func (durableAgentTranslator) TranslateResult(callID string, status tool.CallStatus, ops []operation.Operation) (tool.Result, error) {
	result, err := (agentTranslator{}).TranslateResult(callID, status, ops)
	if r, ok := result.(Result); ok && r.Running {
		r.Text = "Durable child owner pending; launch, prompt admission, or answer may still be uncertain. Status is not proof of execution or drain."
		return r, err
	}
	return result, err
}

type durableSubmitContext struct {
	tool.Context
	parent session.ID
	err    error
}

func (ctx *durableSubmitContext) Submit(spec operation.Spec) operation.ID {
	var state operation.RemoteJobState
	ctx.err = json.Unmarshal(spec.State, &state, json.RejectUnknownMembers(true))
	if ctx.err != nil {
		return ""
	}
	if spec.Type != operation.TypeRemoteJob || spec.Version != operation.VersionRemoteJob || state.Plan.Type != RunPlanType || state.Plan.Version != PlanVersion {
		ctx.err = fmt.Errorf("durable Agent requires a fresh version-1 run spec")
		return ""
	}
	var run RunPlan
	ctx.err = json.Unmarshal(state.Plan.Data, &run, json.RejectUnknownMembers(true))
	if ctx.err != nil {
		return ""
	}
	plan := DurableRunPlan{Version: 1, ParentID: ctx.parent, Run: run}
	ctx.err = plan.Validate()
	if ctx.err != nil {
		return ""
	}
	data, err := json.Marshal(plan)
	if err != nil {
		ctx.err = err
		return ""
	}
	converted, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: RunPlanType, Version: DurablePlanVersion, Data: data})
	if err != nil {
		ctx.err = err
		return ""
	}
	return ctx.Context.Submit(converted)
}
