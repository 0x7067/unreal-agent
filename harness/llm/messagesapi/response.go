package messagesapi

import (
	"encoding/json/jsontext"
	"fmt"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/internal/anthropicapi"
	"github.com/unreallabsai/unreal-agent/internal/apijson"
)

func decodeResponse(body []byte) (llm.Response, error) {
	var envelope struct {
		anthropicapi.Message
		Usage jsontext.Value `json:"usage"`
	}
	if err := apijson.Unmarshal(body, &envelope); err != nil {
		return llm.Response{}, fmt.Errorf("decode messages response: %w", err)
	}
	var usage struct {
		anthropicapi.Usage
		Iterations []struct {
			anthropicapi.Usage
			Model string `json:"model"`
		} `json:"iterations"`
	}
	if len(envelope.Usage) != 0 {
		if err := apijson.Unmarshal(envelope.Usage, &usage); err != nil {
			return llm.Response{}, fmt.Errorf("decode messages response usage: %w", err)
		}
	}
	envelope.Message.Usage = usage.Usage
	converted, err := response(envelope.Message)
	if err != nil {
		return llm.Response{}, err
	}
	converted.Usage.Raw = envelope.Usage
	if len(usage.Iterations) != 0 {
		converted.Usage.ByModel = make(map[string]llm.TokenUsage)
		for _, iteration := range usage.Iterations {
			converted.Usage.ByModel[iteration.Model] = converted.Usage.ByModel[iteration.Model].Add(responseUsage(iteration.Usage))
		}
	} else if len(envelope.Usage) != 0 && envelope.Usage.Kind() != jsontext.KindNull {
		converted.Usage.ByModel = map[string]llm.TokenUsage{envelope.Model: converted.Usage.TokenUsage}
	}
	return converted, nil
}

func response(source anthropicapi.Message) (llm.Response, error) {
	if source.Type != "message" || source.Role != "assistant" {
		return llm.Response{}, fmt.Errorf("expected an assistant message, got type %q and role %q", source.Type, source.Role)
	}
	if source.StopReason == nil {
		return llm.Response{}, fmt.Errorf("response must have a stop reason")
	}
	converted := llm.Response{
		ID:    source.Id,
		Usage: llm.Usage{TokenUsage: responseUsage(source.Usage)},
	}
	switch *source.StopReason {
	case "end_turn", "tool_use", "stop_sequence":
		converted.Stop = llm.StopComplete
	case "max_tokens", "model_context_window_exceeded":
		converted.Stop = llm.StopMaxOutputTokens
	case "refusal":
		converted.Stop = llm.StopRefused
		failure := responseRefusal(source.StopDetails)
		converted.Failure = &failure
	default:
		return llm.Response{}, fmt.Errorf("unsupported stop reason %q", *source.StopReason)
	}
	var err error
	converted.Output, err = responseOutput(source.Content)
	return converted, err
}

func responseOutput(content []anthropicapi.ContentBlock) ([]llm.Item, error) {
	output := make([]llm.Item, 0, len(content))
	for index, block := range content {
		kind, err := block.Discriminator()
		if err != nil {
			return nil, fmt.Errorf("output block %d: decode output block type: %w", index, err)
		}
		item, err := responseOutputItem(block, kind)
		if err != nil {
			return nil, fmt.Errorf("output block %d: %w", index, err)
		}
		output = append(output, item)
	}
	return output, nil
}

func responseRefusal(details *anthropicapi.RefusalStopDetails) llm.Failure {
	failure := llm.Failure{Code: "refusal", Message: "response refused"}
	if details != nil {
		if details.Category != nil && *details.Category != "" {
			failure.Code = string(*details.Category)
		}
		if details.Explanation != nil && *details.Explanation != "" {
			failure.Message = *details.Explanation
		}
	}
	return failure
}

func responseOutputItem(source anthropicapi.ContentBlock, kind string) (llm.Item, error) {
	raw, err := source.MarshalJSON()
	if err != nil {
		return llm.Item{}, err
	}
	provider := llm.ProviderItem{Type: kind, Raw: raw}
	switch kind {
	case "text":
		block, err := source.AsResponseTextBlock()
		if err != nil {
			return llm.Item{}, err
		}
		if block.Citations != nil && len(*block.Citations) != 0 {
			return llm.Item{}, fmt.Errorf("citations are not supported")
		}
		return llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: block.Text}}, nil
	case "tool_use":
		var block struct {
			anthropicapi.ResponseToolUseBlock
			Input jsontext.Value `json:"input"`
		}
		if err := apijson.Unmarshal(raw, &block); err != nil {
			return llm.Item{}, err
		}
		if block.Id == "" || block.Name == "" || block.Input.Kind() != jsontext.KindBeginObject {
			return llm.Item{}, fmt.Errorf("tool call must have an ID, name, and input object")
		}
		if block.ToolsetName != nil {
			return llm.Item{}, fmt.Errorf("unsupported toolset %q", *block.ToolsetName)
		}
		caller, err := block.Caller.MarshalJSON()
		if err != nil {
			return llm.Item{}, err
		}
		if string(caller) != "null" {
			kind, err := block.Caller.Discriminator()
			if err != nil {
				return llm.Item{}, err
			}
			if kind != "direct" {
				return llm.Item{}, fmt.Errorf("unsupported tool caller %q", kind)
			}
		}
		return llm.Item{
			ProviderID: block.Id,
			Type:       llm.ItemToolCall,
			Data:       llm.ToolCall{CallID: block.Id, Name: block.Name, Arguments: string(block.Input)},
		}, nil
	case "thinking":
		block, err := source.AsResponseThinkingBlock()
		if err != nil {
			return llm.Item{}, err
		}
		if block.Signature == "" {
			return llm.Item{}, fmt.Errorf("thinking block must have a signature")
		}
		if block.Thinking != "" {
			provider.Display = &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: block.Thinking}
		}
	case "redacted_thinking":
		block, err := source.AsResponseRedactedThinkingBlock()
		if err != nil {
			return llm.Item{}, err
		}
		if block.Data == "" {
			return llm.Item{}, fmt.Errorf("redacted thinking block must have data")
		}
	case "fallback":
		var boundary struct {
			From struct {
				Model string `json:"model"`
			} `json:"from"`
			To struct {
				Model string `json:"model"`
			} `json:"to"`
		}
		if err := apijson.Unmarshal(raw, &boundary); err != nil {
			return llm.Item{}, err
		}
		if boundary.From.Model == "" || boundary.To.Model == "" {
			return llm.Item{}, fmt.Errorf("fallback block must have from and to models")
		}
	default:
		return llm.Item{}, fmt.Errorf("unsupported output block type %q", kind)
	}
	return llm.Item{Type: llm.ItemProvider, Data: provider}, nil
}

func responseUsage(source anthropicapi.Usage) llm.TokenUsage {
	usage := llm.TokenUsage{InputTokens: int64(source.InputTokens), OutputTokens: int64(source.OutputTokens)}
	if source.CacheReadInputTokens != nil {
		usage.CachedInputTokens = int64(*source.CacheReadInputTokens)
	}
	if source.CacheCreationInputTokens != nil {
		usage.CacheWriteInputTokens = int64(*source.CacheCreationInputTokens)
	}
	usage.InputTokens += usage.CachedInputTokens + usage.CacheWriteInputTokens
	if source.OutputTokensDetails != nil {
		usage.ReasoningTokens = int64(source.OutputTokensDetails.ThinkingTokens)
	}
	return usage
}
