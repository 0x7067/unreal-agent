package taskgraph

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// NewDurable selects command-only graph plans under this fresh trusted owner.
// Translation remains pure; the runtime must separately enforce checkpoint ack.
func NewDurable(parent session.ID, scope string) tool.Translator {
	return durableTranslator{parent, scope}
}

type durableTranslator struct {
	parent session.ID
	scope  string
}

func (t durableTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	c := &durableContext{Context: ctx, parent: t.parent, scope: t.scope}
	status := (translator{}).Translate(c, call)
	if c.err != nil {
		return tool.ErrorStatus(c.err.Error(), 0)
	}
	return status
}

type durableContext struct {
	tool.Context
	parent session.ID
	scope  string
	err    error
}

func (c *durableContext) Submit(spec operation.Spec) operation.ID {
	op := operation.Operation{ID: "translation", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusReady}
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		c.err = err
		return ""
	}
	if state.Plan.Version != graph.PlanVersion {
		c.err = errors.New("durable graph requires a fresh v1 specification")
		return ""
	}
	var data []byte
	switch state.Plan.Type {
	case graph.RunPlanType:
		var p graph.RunPlan
		err = json.Unmarshal(state.Plan.Data, &p, json.RejectUnknownMembers(true))
		if err == nil {
			data, err = json.Marshal(graph.DurableRunPlan{Version: 1, ParentID: c.parent, Run: p}, json.FormatNilSliceAsNull(true))
		}
	case graph.ControlPlanType:
		var p graph.ControlPlan
		err = json.Unmarshal(state.Plan.Data, &p, json.RejectUnknownMembers(true))
		if err == nil {
			data, err = json.Marshal(graph.DurableControlPlan{Version: 1, ParentID: c.parent, Scope: c.scope, Control: p}, json.FormatNilSliceAsNull(true))
		}
	default:
		err = errors.New("unsupported durable graph specification")
	}
	if err == nil {
		spec, err = operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: state.Plan.Type, Version: graph.DurablePlanVersion, Data: data})
	}
	if err == nil {
		op.State = spec.State
		if state.Plan.Type == graph.RunPlanType {
			_, _, err = graph.DecodeDurableRun(op)
		} else {
			_, err = graph.DecodeDurableControl(op)
		}
	}
	if err != nil {
		c.err = err
		return ""
	}
	return c.Context.Submit(spec)
}
func (t durableTranslator) TranslateResult(id string, status tool.CallStatus, ops []operation.Operation) (tool.Result, error) {
	projected := append([]operation.Operation(nil), ops...)
	for i, op := range projected {
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			return nil, err
		}
		switch state.Plan.Type {
		case graph.RunPlanType:
			p, _, err := graph.DecodeDurableRun(op)
			if err != nil {
				return nil, err
			}
			state.Plan.Data, _ = json.Marshal(p.Run)
		case graph.ControlPlanType:
			p, err := graph.DecodeDurableControl(op)
			if err != nil {
				return nil, err
			}
			state.Plan.Data, _ = json.Marshal(p.Control)
		default:
			return nil, fmt.Errorf("unsupported durable graph result %s", state.Plan.Type)
		}
		state.Plan.Version = graph.PlanVersion
		projected[i].State, _ = json.Marshal(state)
	}
	return (translator{}).TranslateResult(id, status, projected)
}
