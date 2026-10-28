// Package trustedextension is the pure model-facing TrustedEcho translator.
package trustedextension

import (
	"encoding/json/v2"
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type translator struct{ registry *extension.Registry }

func New(r *extension.Registry) tool.Translator { return translator{r} }
func Definition() tool.Definition {
	return tool.Definition{Tool: llm.Tool{Type: llm.ToolFunction, Name: "TrustedEcho", Description: "Run the selected trusted compiled Echo implementation.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"input": map[string]any{"type": "string", "maxLength": 4096}}, "required": []string{"input"}, "additionalProperties": false}}}
}
func (t translator) Translate(ctx tool.Context, c llm.ToolCall) tool.CallStatus {
	var a struct {
		Input *string `json:"input"`
	}
	if e := json.Unmarshal([]byte(c.Arguments), &a, json.RejectUnknownMembers(true)); e != nil {
		return tool.ErrorStatus(e.Error(), 0)
	}
	if a.Input == nil {
		return tool.ErrorStatus("input string required", 0)
	}
	if t.registry == nil {
		return tool.ErrorStatus("trusted registry unavailable", 0)
	}
	d, e := t.registry.Selected()
	if e != nil {
		return tool.ErrorStatus(e.Error(), 0)
	}
	owner, ok := ctx.(interface{ CurrentTurnID() string })
	if !ok || owner.CurrentTurnID() == "" {
		return tool.ErrorStatus("exact foreground owner unavailable", 0)
	}
	s, e := extension.NewSpec(d, *a.Input, owner.CurrentTurnID())
	if e != nil {
		return tool.ErrorStatus(e.Error(), 0)
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(s)}}
}

type Result struct {
	CallID, Output string
	Running        bool
}

func (r Result) ToLLMResult() llm.ToolResult {
	return llm.ToolResult{CallID: r.CallID, Running: r.Running, Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: r.Output}}}
}
func (t translator) TranslateResult(id string, status tool.CallStatus, ops []operation.Operation) (tool.Result, error) {
	if status.Error != "" {
		if len(ops) != 0 {
			return nil, errors.New("validation failure with operations")
		}
		return Result{CallID: id, Output: status.Error}, nil
	}
	if len(ops) != 1 {
		return nil, errors.New("TrustedEcho requires exactly one operation")
	}
	s, _, e := extension.Decode(ops[0])
	if e != nil {
		return nil, e
	}
	r := Result{CallID: id}
	switch ops[0].Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling:
		r.Running = true
	case operation.StatusCompleted:
		r.Output = s.TerminalResult
	case operation.StatusFailed, operation.StatusCanceled:
		r.Output = s.TerminalError
	}
	return r, nil
}
