package tool

import (
	"fmt"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

type unavailableTranslator struct {
	name string
}

func (translator unavailableTranslator) Translate(Context, llm.ToolCall) CallStatus {
	return CallStatus{Error: translator.errorMessage()}
}

func (translator unavailableTranslator) TranslateResult(
	callID string,
	_ CallStatus,
	_ []operation.Operation,
) (Result, error) {
	return UnavailableResult{CallID: callID, Error: translator.errorMessage()}, nil
}

type UnavailableResult struct {
	CallID string
	Error  string
}

func (result UnavailableResult) ToLLMResult() llm.ToolResult {
	return llm.ToolResult{CallID: result.CallID, Output: []llm.ToolResultOutput{{
		Kind: llm.ToolResultText, Value: result.Error,
	}}}
}

func (translator unavailableTranslator) errorMessage() string {
	return fmt.Sprintf("static tool %q is not configured", translator.name)
}

func StaticNames() []string {
	definitions := staticDefinitions()
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Tool.Name)
	}
	return names
}

func staticDefinitions() []Definition {
	return []Definition{
		{Tool: llm.Tool{
			Type:        llm.ToolFunction,
			Name:        BashName,
			Description: "Execute a shell command in background. Independent commands may be issued as parallel tool calls in one turn. Command child processes are killed when the shell exits.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute.",
					},
					"max_output_length": maxOutputLengthSchema(),
				},
				"required": []any{"command"},
			},
		}},
		{Tool: llm.Tool{
			Type:        llm.ToolFunction,
			Name:        ViewImageName,
			Description: "View a local JPEG, PNG, BMP, TIFF, or WebP image.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Image file path, absolute or relative to the workspace.",
					},
				},
				"required": []any{"path"},
			},
		}},
		{Tool: llm.Tool{
			Type:        llm.ToolFunction,
			Name:        SkillUseName,
			Description: "Load the instructions for a registered skill.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The exact name of the skill to load.",
					},
				},
				"required": []any{"name"},
			},
		}},
		{Tool: llm.Tool{
			Type:        llm.ToolFunction,
			Name:        AgentName,
			Description: "Start a subagent: a separate agent with its own context and the same workspace. The call keeps running while the subagent works; its final message becomes the result. Messages it sends you arrive as user messages prefixed with its name. Start independent subagents as parallel tool calls in one turn.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Unique name for the subagent, used to message it: lowercase letters, digits, '-' or '_', at most 40 characters.",
					},
					"prompt": map[string]any{
						"type":        "string",
						"description": "The complete task for the subagent. It cannot see your conversation, so include every detail it needs.",
					},
				},
				"required": []any{"name", "prompt"},
			},
		}},
		{Tool: llm.Tool{
			Type:        llm.ToolFunction,
			Name:        SendMessageName,
			Description: "Send a message to another agent's inbox. A parent addresses subagents by name; messaging a finished subagent resumes it, and its reply becomes the result. A subagent addresses its parent as \"parent\".",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to": map[string]any{
						"type":        "string",
						"description": "Recipient: a subagent name, or \"parent\".",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "The message text.",
					},
				},
				"required": []any{"to", "message"},
			},
		}},
	}
}

func maxOutputLengthSchema() map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": fmt.Sprintf("Maximum characters per output text field. Truncated text keeps its head and tail, around a marker stating how much was omitted, and path to the file with the complete stream. Defaults to %d.", operation.DefaultMaxOutputLength),
		"minimum":     1,
		"maximum":     operation.MaxOutputLength,
		"default":     operation.DefaultMaxOutputLength,
	}
}
