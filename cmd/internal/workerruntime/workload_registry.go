package workerruntime

import (
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/trustedextension"
)

const timerTool = "DurableReminder"

type remoteWorkloadPolicy interface {
	ClassifyRemoteForeground(operation.Operation, string) (string, bool, error)
	RemoteCleanupProof(operation.Operation) error
}
type workloadRegistry struct {
	tool.Registry
	echo  *configuredExtensions
	timer *timerTranslator
}

func (r *workloadRegistry) Resolve(name string) (tool.Translator, bool) {
	if r.echo != nil && name == "TrustedEcho" {
		return trustedextension.New(r.echo.registry), true
	}
	if r.timer != nil && name == timerTool {
		return r.timer, true
	}
	return r.Registry.Resolve(name)
}
func (r *workloadRegistry) StaticDefinitions() []tool.Definition {
	result := r.Registry.StaticDefinitions()
	if r.echo != nil {
		result = append(result, trustedextension.Definition())
	}
	if r.timer != nil {
		result = append(result, tool.Definition{Tool: llm.Tool{Type: llm.ToolFunction, Name: timerTool, Description: "Persist one absolute-UTC reminder; background survives foreground abort. Completion is one canonical logical notification, not an exactly-once external send.", Parameters: map[string]any{"type": "object", "additionalProperties": false, "required": []any{"due_at", "message", "background"}, "properties": map[string]any{"due_at": map[string]any{"type": "string"}, "message": map[string]any{"type": "string", "maxLength": 4096}, "background": map[string]any{"type": "boolean"}}}}})
	}
	return result
}
func (r *workloadRegistry) ClassifyRemoteForeground(op operation.Operation, target string) (string, bool, error) {
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return "", true, err
	}
	if state.Plan.Type == extension.PlanType && r.echo != nil {
		_, plan, decodeErr := extension.Decode(op)
		if decodeErr != nil {
			return "", true, decodeErr
		}
		if err := extension.ValidateRecovery(op, r.echo.registry); err != nil {
			return "", true, err
		}
		if terminalOperation(op) {
			return "terminal", true, nil
		}
		if plan.OwnerTurnID == "" {
			return "", true, errors.New("exact Echo foreground owner unavailable")
		}
		if plan.OwnerTurnID != target {
			return "background", true, nil
		}
		return "foreground", true, nil
	}
	if state.Plan.Type == reminder.TaskType && r.timer != nil {
		p, _, err := reminder.DecodeTask(op)
		if err != nil {
			return "", true, err
		}
		if p.ParentID != r.timer.parent {
			return "", true, errors.New("foreign timer parent")
		}
		if terminalOperation(op) {
			if !reminder.TaskTerminalProof(op) {
				return "", true, errors.New("timer terminal proof unavailable")
			}
			return "terminal", true, nil
		}
		if err := reminder.ValidateTaskRecovery(op); err != nil {
			return "", true, err
		}
		if p.Ownership.Mode == "foreground" && p.Ownership.OwnerTurnID != target {
			return "background", true, nil
		}
		if p.Ownership.Mode == "background" {
			return "background", true, nil
		}
		return "foreground", true, nil
	}
	if policy, ok := r.Registry.(remoteWorkloadPolicy); ok {
		return policy.ClassifyRemoteForeground(op, target)
	}
	return "", false, nil
}
func (r *workloadRegistry) RemoteCleanupProof(op operation.Operation) error {
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return err
	}
	if state.Plan.Type == extension.PlanType && r.echo != nil {
		if !terminalOperation(op) {
			return errors.New("Echo not terminal")
		}
		return extension.ValidateRecovery(op, r.echo.registry)
	}
	if state.Plan.Type == reminder.TaskType && r.timer != nil {
		if !reminder.TaskTerminalProof(op) {
			return errors.New("timer terminal proof unavailable")
		}
		return nil
	}
	if policy, ok := r.Registry.(remoteWorkloadPolicy); ok {
		return policy.RemoteCleanupProof(op)
	}
	return errors.New("unclassified remote cleanup")
}
func terminalOperation(op operation.Operation) bool {
	return op.Status == operation.StatusCompleted || op.Status == operation.StatusFailed || op.Status == operation.StatusCanceled
}

type timerTranslator struct{ parent string }

func (t *timerTranslator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	var args struct {
		DueAt      string  `json:"due_at"`
		Message    *string `json:"message"`
		Background *bool   `json:"background"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args, json.RejectUnknownMembers(true)); err != nil {
		return tool.ErrorStatus(err.Error(), 4096)
	}
	if args.Message == nil || args.Background == nil {
		return tool.ErrorStatus("message and explicit background ownership required", 4096)
	}
	due, err := time.Parse(time.RFC3339Nano, args.DueAt)
	if err != nil {
		return tool.ErrorStatus(err.Error(), 4096)
	}
	if clock, ok := ctx.(interface{ CurrentTimeUTC() time.Time }); ok {
		if due.Sub(clock.CurrentTimeUTC()) > reminder.MaxDelay {
			return tool.ErrorStatus("reminder exceeds30day policy", 4096)
		}
	} else {
		return tool.ErrorStatus("coordinator time snapshot unavailable", 4096)
	}
	ownership := reminder.Ownership{Mode: "background"}
	if !*args.Background {
		owner, ok := ctx.(interface{ CurrentTurnID() string })
		if !ok {
			return tool.ErrorStatus("foreground owner unavailable", 4096)
		}
		ownership = reminder.Ownership{Mode: "foreground", OwnerTurnID: owner.CurrentTurnID()}
	}
	spec, err := reminder.NewTaskSpec(reminder.TaskPlan{Version: 1, ParentID: t.parent, DueTime: due, Ownership: ownership, Message: *args.Message})
	if err != nil {
		return tool.ErrorStatus(err.Error(), 4096)
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(spec)}}
}
func (t *timerTranslator) TranslateResult(id string, status tool.CallStatus, ops []operation.Operation) (tool.Result, error) {
	if status.Error != "" {
		if len(ops) != 0 {
			return nil, errors.New("timer validation error with operation")
		}
		return trustedJobResult{callID: id, output: status.Error}, nil
	}
	if len(ops) != 1 {
		return nil, errors.New("timer result requires one operation")
	}
	_, _, err := reminder.DecodeTask(ops[0])
	if err != nil {
		return nil, err
	}
	state, err := operation.DecodeRemoteJobState(ops[0])
	if err != nil {
		return nil, err
	}
	return trustedJobResult{callID: id, output: state.TerminalResult + state.TerminalError, running: !terminalOperation(ops[0])}, nil
}
