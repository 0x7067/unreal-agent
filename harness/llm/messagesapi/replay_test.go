package messagesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestReplayPreservesCommittedResultsWhileDeliveringOldCompletions(t *testing.T) {
	var firstEncoding []byte
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		builder := contextbuilder.NewBuilder()
		builder.SetModel(llm.Model{ID: "claude-test"})
		if err := builder.AddExternalInput(inbox.Input{ID: "input-1", Kind: inbox.InputExternal, Payload: jsontext.Value(`"Start"`)}); err != nil {
			t.Fatal(err)
		}
		builder.Commit()
		calls, err := decodeResponse(responseBody("tool_use", `[
			{"type":"thinking","thinking":"Two calls","signature":"signed"},
			{"type":"tool_use","id":"a","name":"Bash","input":{}},
			{"type":"tool_use","id":"b","name":"Bash","input":{}}
		]`))
		if err != nil {
			t.Fatal(err)
		}
		builder.AddModelResponse(calls)
		for _, id := range order {
			builder.AddToolResult(id, nil, true)
		}
		builder.AddToolResult("b", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "B completed"}}, false)
		firstRequest, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		firstBody, first := encodeRequest(t, firstRequest.Request)
		assertPersistedRequestReplay(t, firstRequest.Request, firstBody)
		if firstEncoding == nil {
			firstEncoding = firstBody
		} else if string(firstBody) != string(firstEncoding) {
			t.Fatal("running result arrival order changed the wire request")
		}
		results := first.Messages[2].Content
		if len(results) != 2 || results[0].ToolUseID != "a" || results[0].Content[0].Text != contextbuilder.ToolCallRunningPayload ||
			results[1].ToolUseID != "b" || results[1].Content[0].Text != "B completed" {
			t.Fatalf("staged replacement did not become the primary result: %#v", results)
		}
		builder.Commit()

		// A finishes while a model request is in flight. The response adds C
		// ahead of the staged completion, just as the coordinator does.
		builder.AddToolResult("a", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "A completed"}}, false)
		builder.AddModelResponse(llm.Response{Output: []llm.Item{toolCall("c")}})
		builder.AddToolResult("c", nil, true)
		secondRequest, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		secondBody, second := encodeRequest(t, secondRequest.Request)
		assertPersistedRequestReplay(t, secondRequest.Request, secondBody)
		if !reflect.DeepEqual(first.Messages, second.Messages[:len(first.Messages)]) {
			t.Fatal("a late completion changed the committed prefix")
		}
		results = second.Messages[4].Content
		if len(results) != 3 || results[0].ToolUseID != "c" || results[0].Content[0].Text != contextbuilder.ToolCallRunningPayload ||
			results[1].Type != "text" || !strings.Contains(results[1].Text, `call "a", completed`) || results[2].Text != "A completed" {
			t.Fatalf("new call result must precede the older completion update: %#v", results)
		}
		builder.Commit()
		builder.AddModelResponse(llm.Response{Output: []llm.Item{message(llm.RoleAssistant, "Waiting for C")}})
		builder.AddToolResult("c", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "C completed"}}, false)
		thirdRequest, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		thirdBody, third := encodeRequest(t, thirdRequest.Request)
		assertPersistedRequestReplay(t, thirdRequest.Request, thirdBody)
		if !reflect.DeepEqual(second.Messages, third.Messages[:len(second.Messages)]) {
			t.Fatal("C's completion changed an earlier result")
		}
		results = third.Messages[6].Content
		if len(results) != 2 || results[0].Type != "text" || !strings.Contains(results[0].Text, `call "c", completed`) || results[1].Text != "C completed" {
			t.Fatalf("C's completion = %#v", results)
		}
	}
}

func assertPersistedRequestReplay(t *testing.T, request llm.Request, expected []byte) {
	t.Helper()
	persisted, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var restored llm.Request
	if err := json.Unmarshal(persisted, &restored); err != nil {
		t.Fatal(err)
	}
	body, _ := encodeRequest(t, restored)
	if string(body) != string(expected) {
		t.Fatalf("request changed after persistence:\n got %s\nwant %s", body, expected)
	}
}

func TestRequestUsesRawThinkingInsteadOfSummary(t *testing.T) {
	_, wire := encodeRequest(t, llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{
		message(llm.RoleUser, "Start"),
		{Type: llm.ItemReasoning, Data: llm.Reasoning{
			Summary: []string{"stale display text"},
			Raw:     jsontext.Value(`{"type":"thinking","thinking":"original","signature":"signed"}`),
		}},
		toolCall("a"), toolResult("a", "done", false),
	}})
	if wire.Messages[1].Content[0].Type != "thinking" {
		t.Fatal("thinking block was not preserved")
	}
	block, err := requestInputBlock(llm.Item{Type: llm.ItemReasoning, Data: llm.Reasoning{
		Summary: []string{"stale display text"}, Raw: jsontext.Value(`{"type":"thinking","thinking":"original","signature":"signed"}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	thinking, err := block.AsRequestThinkingBlock()
	if err != nil {
		t.Fatal(err)
	}
	if thinking.Thinking != "original" || thinking.Signature != "signed" {
		t.Fatalf("thinking = %#v", thinking)
	}
}
