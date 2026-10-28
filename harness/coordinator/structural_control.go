package coordinator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type structuralControl struct {
	InputID inbox.ID
	Call    llm.ToolCall
	Status  tool.CallStatus
}

// The input ID fixes the operation identity across a crash between input
// persistence, operation persistence, dispatch and the result observation.
func structuralOperationID(id inbox.ID) operation.ID {
	return operation.ID(fmt.Sprintf("control-%x", sha256.Sum256([]byte(id))))
}

func (c *coordinator) admitStructuralControl(ctx context.Context, id inbox.ID, dispatch bool) error {
	if c.state.controlCompleted[id] {
		return nil
	}
	invocation, ok := c.state.controlInputs[id]
	if !ok {
		return nil
	}
	fail := func(message string) error {
		return c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.ToolInvocationFinished, Parameters: inbox.ToolInvocationResult{InputID: id, CallID: invocation.Call.CallID, Status: "failed", Error: boundedControlText(message, 2048)}})
	}
	opID := structuralOperationID(id)
	meta := structuralControl{InputID: id, Call: invocation.Call, Status: tool.CallStatus{WaitingFor: []operation.ID{opID}}}
	if invocation.Scope != "" && invocation.Scope != c.dependencies.ControlScope {
		if value, exists := c.state.operations[opID]; exists && !operationIsTerminal(value.Status) {
			return fmt.Errorf("unfinished structural intervention belongs to an old runtime scope; execution outcome unknown")
		}
		if _, exists := c.state.operations[opID]; !exists {
			return fail("runtime scope changed; intervention not dispatched")
		}
	}
	if value, exists := c.state.operations[opID]; exists {
		c.state.controlOperations[opID] = meta
		return c.finishStructuralControl(ctx, value)
	}
	if c.stop.cancellationRequested {
		return fail("session shutdown already requested; control not dispatched")
	}
	translator, enabled := c.dependencies.Tools.Resolve(invocation.Call.Name)
	if !enabled {
		return fail("structural tool is not enabled for this session")
	}
	collector := &toolCallContext{}
	status := translator.Translate(collector, invocation.Call)
	if status.Error != "" {
		return fail(status.Error)
	}
	if len(collector.operations) != 1 || len(status.WaitingFor) != 1 {
		return fail("structural control must produce exactly one operation")
	}
	value := collector.operations[0]
	value.ID = opID
	// No operation reaches the manager until both its input and complete inert
	// spec are durable. Translators remain pure; no model request is involved.
	initializer, supported := c.dependencies.Sessions.(sessionstore.ControlOperationInitializer)
	if !supported {
		return fail("session store does not support structural control initialization")
	}
	if err := initializer.InitializeControlOperation(ctx, c.dependencies.SessionID, id, value); err != nil {
		return fmt.Errorf("initialize structural control: %w", err)
	}
	c.addOperationToLocalState(value)
	c.state.controlOperations[opID] = meta
	if dispatch {
		return c.dispatchOperationToManager(value)
	}
	return nil
}

func (c *coordinator) finishStructuralControl(ctx context.Context, value operation.Operation) error {
	meta, exists := c.state.controlOperations[value.ID]
	if !exists || !operationIsTerminal(value.Status) || c.state.controlCompleted[meta.InputID] {
		return nil
	}
	outcome := inbox.ToolInvocationResult{InputID: meta.InputID, CallID: meta.Call.CallID, Status: string(value.Status)}
	translator, enabled := c.dependencies.Tools.Resolve(meta.Call.Name)
	if !enabled {
		outcome.Error = "structural result translator unavailable"
	} else {
		result, err := translator.TranslateResult(meta.Call.CallID, meta.Status, []operation.Operation{value})
		if err != nil {
			outcome.Error = err.Error()
		} else {
			var texts []string
			for _, output := range result.ToLLMResult().Output {
				if output.Kind == llm.ToolResultText {
					texts = append(texts, output.Value)
				}
			}
			outcome.Text = strings.Join(texts, "\n")
		}
	}
	outcome.Text = boundedControlText(outcome.Text, 4096)
	outcome.Error = boundedControlText(outcome.Error, 2048)
	if err := c.recordControl(ctx, inbox.ControlMessage{Mode: inbox.ToolInvocationFinished, Parameters: outcome}); err != nil {
		return err
	}
	delete(c.state.controlOperations, value.ID)
	return nil
}

func boundedControlText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	suffix := " [truncated; inspect activity]"
	text = text[:limit-len(suffix)]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + suffix
}
