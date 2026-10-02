package contextbuilder

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

// This fixed budget is part of the replay contract.
const compactionRetainedTokens = 20_000

//go:embed prompts/compaction.md
var compactionPrompt string

func (current *builder) BuildCompaction() (Result, error) {
	cutoff, ok := current.compactionCutoff()
	if !ok {
		return Result{}, fmt.Errorf("no history eligible for compaction")
	}
	end := current.prefixTokens[cutoff].committedPrefixIndex
	history := current.committedPrefix[1:end]
	summarizer := newBuilder(compactionPrompt)
	summarizer.SetModel(current.request.Model)
	summarizer.stageItems(history...)
	pending := collectPendingToolCalls(history)
	for _, callID := range pending.order {
		if call, exists := pending.calls[callID]; exists && call.result == nil {
			// Native history needs a result even when the call is pending at the cutoff.
			summarizer.AddToolResult(callID, nil, true)
		}
	}
	// Some providers require the last message to be a user message if there are no running tool calls.
	// Add it without checking tool calls for consistency.
	summarizer.stageItems(llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Summarise"}})
	return summarizer.Build()
}

func (current *builder) NeedsCompaction() bool {
	if len(current.prefixTokens) == 0 {
		return false
	}
	threshold := current.request.Model.CompactionThreshold
	tokens := current.prefixTokens[len(current.prefixTokens)-1].Tokens
	return tokens >= threshold
}

// Compact rebuilds from a compaction response without changing the original builder.
func (current *builder) Compact(response llm.Response) (Builder, bool) {
	cutoff, ok := current.compactionCutoff()
	if !ok {
		return current, false
	}
	start := current.prefixTokens[cutoff].committedPrefixIndex
	rebuilt := newBuilder(current.preamble)
	rebuilt.SetModel(current.request.Model)
	rebuilt.SetSystemPrompt(current.systemPrompt)
	for _, tool := range current.request.Tools {
		rebuilt.AddTool(tool)
	}
	rebuilt.addCompactionSummary(response)
	rebuilt.carryForwardContext(current, start)
	return rebuilt, true
}

func (current *builder) addCompactionSummary(response llm.Response) {
	var summary string
	for _, item := range slices.Backward(response.Output) {
		if item.Type == llm.ItemMessage {
			message := item.Data.(llm.Message)
			if message.Role == llm.RoleAssistant && strings.TrimSpace(message.Text) != "" {
				summary = message.Text
				break
			}
		}
	}
	current.stageItems(llm.Item{Type: llm.ItemMessage, Data: llm.Message{
		Role: llm.RoleUser,
		Text: summary,
	}})
}

// compactionCutoff returns the last checkpoint to summarize, retaining whole turns within the budget.
func (current *builder) compactionCutoff() (int, bool) {
	if len(current.prefixTokens) == 0 {
		return 0, false
	}
	latest := current.prefixTokens[len(current.prefixTokens)-1].Tokens
	if latest <= compactionRetainedTokens {
		return 0, false
	}
	start := 0
	for index, checkpoint := range slices.Backward(current.prefixTokens) {
		if latest-checkpoint.Tokens > compactionRetainedTokens {
			break
		}
		start = index
	}
	return start, true
}

type compactionToolCalls struct {
	calls map[string]compactionToolCall
	order []string
}

type compactionToolCall struct {
	item   llm.Item
	result *llm.ToolResult
}

func (current *compactionToolCalls) addToolCall(item llm.Item) {
	call := item.Data.(llm.ToolCall)
	current.calls[call.CallID] = compactionToolCall{item: item}
	current.order = append(current.order, call.CallID)
}

func (current *compactionToolCalls) addToolResult(result llm.ToolResult) {
	call, exists := current.calls[result.CallID]
	if !exists {
		return
	}
	if !result.Running {
		delete(current.calls, result.CallID)
		return
	}
	call.result = &result
	current.calls[result.CallID] = call
}

func collectPendingToolCalls(prefix []llm.Item) compactionToolCalls {
	pending := compactionToolCalls{calls: make(map[string]compactionToolCall)}
	for _, item := range prefix {
		switch item.Type {
		case llm.ItemToolCall:
			pending.addToolCall(item)
		case llm.ItemToolResult:
			pending.addToolResult(item.Data.(llm.ToolResult))
		}
	}
	return pending
}

// preserves tool calls that were pending at the compaction
// cutoff, including their latest running results.
func (current *builder) carryForwardContext(source *builder, start int) {
	pending := collectPendingToolCalls(source.committedPrefix[1:start])
	for _, callID := range pending.order {
		call, exists := pending.calls[callID]
		if !exists {
			continue
		}
		current.stageItems(call.item)
		if call.result != nil {
			current.AddToolResult(callID, call.result.Output, call.result.Running)
		}
	}
	current.Commit()
	current.committedPrefix = append(current.committedPrefix, source.committedPrefix[start:]...)
	current.stagedSuffix = append(current.stagedSuffix, source.stagedSuffix...)
}
