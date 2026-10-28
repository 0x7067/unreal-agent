package workerruntime

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const trustedJobTool = "RunTrustedJob"

// The registry interface and its fixed native selection remain unchanged. This
// explicit opt-in composition adds one fixed-schema trusted translator only.
type trustedJobRegistry struct {
	tool.Registry
	translator *trustedJobTranslator
}

func (r *trustedJobRegistry) Resolve(name string) (tool.Translator, bool) {
	if name == trustedJobTool {
		return r.translator, true
	}
	return r.Registry.Resolve(name)
}
func (r *trustedJobRegistry) StaticDefinitions() []tool.Definition {
	base := r.Registry.StaticDefinitions()
	jobs := make([]any, len(r.translator.config.Jobs))
	for i, j := range r.translator.config.Jobs {
		jobs[i] = j
	}
	return append(base, tool.Definition{Tool: llm.Tool{Type: llm.ToolFunction, Name: trustedJobTool, Description: "Run one explicitly trusted independently owned job; stable identity, bounded output, no arbitrary commands. Unavailable/unknown is not completion.", Parameters: map[string]any{"type": "object", "additionalProperties": false, "required": []any{"job"}, "properties": map[string]any{"job": map[string]any{"type": "string", "enum": jobs}}}}})
}

type trustedJobTranslator struct {
	config     durablejob.RuntimeConfig
	controller string
}

func (t *trustedJobTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	var args struct {
		Job string `json:"job"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args, json.RejectUnknownMembers(true)); err != nil {
		return tool.ErrorStatus(err.Error(), 4096)
	}
	if !slices.Contains(t.config.Jobs, args.Job) {
		return tool.ErrorStatus("untrusted job selection", 4096)
	}
	data, err := json.Marshal(durablejob.Plan{Version: 1, EnvironmentID: t.config.EnvironmentID, ControllerID: t.controller, JobName: args.Job})
	if err != nil {
		return tool.ErrorStatus(err.Error(), 4096)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: durablejob.PlanType, Version: durablejob.Version, Data: data})
	if err != nil {
		return tool.ErrorStatus(err.Error(), 4096)
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(spec)}}
}

type trustedJobResult struct {
	callID, output string
	running        bool
}

func (r trustedJobResult) ToLLMResult() llm.ToolResult {
	return llm.ToolResult{CallID: r.callID, Running: r.running, Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: r.output}}}
}
func (t *trustedJobTranslator) TranslateResult(id string, status tool.CallStatus, ops []operation.Operation) (tool.Result, error) {
	if status.Error != "" {
		if len(ops) != 0 {
			return nil, errors.New("job validation error with operation")
		}
		return trustedJobResult{callID: id, output: status.Error}, nil
	}
	if len(ops) != 1 {
		return nil, errors.New("job result needs one operation")
	}
	op := ops[0]
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return nil, err
	}
	if state.Plan.Type != durablejob.PlanType || state.Plan.Version != durablejob.Version {
		return nil, errors.New("unsupported job result plan")
	}
	running := op.Status == operation.StatusReady || op.Status == operation.StatusAwaiting || op.Status == operation.StatusCanceling
	if !running && op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed && op.Status != operation.StatusCanceled {
		return nil, errors.New("invalid job result status")
	}
	output := strings.TrimSpace(state.TerminalResult + "\n" + state.TerminalError)
	if output == "" {
		output = fmt.Sprintf("independent job %s; completion not confirmed", op.Status)
	}
	return trustedJobResult{callID: id, output: output, running: running}, nil
}

func (r *trustedJobRegistry) ClassifyRemoteForeground(op operation.Operation, _ string) (string, bool, error) {
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return "", true, err
	}
	if state.Plan.Type != durablejob.PlanType {
		return "", false, nil
	}
	cfg := r.translator.config.Authority("")
	cfg.ControllerID = r.translator.controller
	if op.Status == operation.StatusCompleted || op.Status == operation.StatusFailed || op.Status == operation.StatusCanceled {
		return "terminal", true, durablejob.TerminalCleanupProof(op, cfg)
	}
	if err := durablejob.ValidateRecovery(op, cfg); err != nil {
		return "", true, err
	}
	return "foreground", true, nil
}
func (r *trustedJobRegistry) RemoteCleanupProof(op operation.Operation) error {
	cfg := r.translator.config.Authority("")
	cfg.ControllerID = r.translator.controller
	return durablejob.TerminalCleanupProof(op, cfg)
}
