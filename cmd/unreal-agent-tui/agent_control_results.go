package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func (m *model) applyAgentControlInput(input inbox.Input) {
	if input.Kind != inbox.InputControl {
		return
	}
	var envelope struct {
		Mode       string
		Parameters jsontext.Value
	}
	if json.Unmarshal(input.Payload, &envelope) != nil {
		return
	}
	switch envelope.Mode {
	case "invoke_tool":
		var invocation struct{ Call llm.ToolCall }
		if json.Unmarshal(envelope.Parameters, &invocation) != nil {
			return
		}
		if m.structuralCalls == nil {
			m.structuralCalls = make(map[string]llm.ToolCall)
		}
		m.structuralCalls[invocation.Call.CallID] = invocation.Call
	case "tool_invocation_finished":
		var result struct {
			InputID                     inbox.ID
			CallID, Status, Text, Error string
		}
		if json.Unmarshal(envelope.Parameters, &result) != nil {
			return
		}
		text := controlOutcomeText(result.Text)
		if result.Error != "" {
			text = "Control " + singleLine(result.Status) + ": " + boundedDetail(result.Error, 4000)
		}
		if text == "" {
			text = "Control " + singleLine(result.Status) + "; outcome details unavailable"
		}
		m.append("Agent control result", text)
	}

}

func controlOutcomeText(text string) string {
	var outcome toolsubagent.ControlOutcome
	if json.Unmarshal([]byte(text), &outcome) != nil || outcome.Action == "" || outcome.State == "" {
		return boundedDetail(text, 8000)
	}
	detail := fmt.Sprintf("%s · %s@%d · %s", outcome.Action, singleLine(outcome.Handle), outcome.Generation, strings.ReplaceAll(outcome.State, "_", " "))
	if outcome.Delivery != "" {
		detail += " · delivery barrier: " + singleLine(outcome.Delivery)
	}
	if outcome.DrainConfirmed {
		detail += " · execution drain confirmed"
	} else if outcome.Action == "stop" {
		detail += " · drain NOT confirmed; inspect /agents"
	}
	return detail
}
