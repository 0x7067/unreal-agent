package contextbuilder

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestCompactionSelectsWholeTurnsFromUsage(t *testing.T) {
	for _, test := range []struct {
		name   string
		totals []int64
		turn   int
	}{
		{name: "no responses", turn: -1},
		{name: "whole history fits", totals: []int64{1_000, 10_000, 20_000}, turn: -1},
		{name: "one oversized response", totals: []int64{55_000}, turn: 1},
		{name: "largest fitting suffix", totals: []int64{10_000, 18_000, 25_000}, turn: 1},
		{name: "C D and E fit at 18k", totals: []int64{1_000, 101_000, 104_000, 109_000, 119_000}, turn: 2},
		{name: "D and E exceed budget", totals: []int64{10_000, 18_000, 31_000, 43_000, 55_000}, turn: 4},
		{name: "D and E exactly fit", totals: []int64{10_000, 18_000, 35_000, 43_000, 55_000}, turn: 3},
		{name: "exact final turn", totals: []int64{10_000, 18_000, 31_000, 43_000, 63_000}, turn: 4},
		{name: "oversized final turn", totals: []int64{10_000, 18_000, 31_000, 43_000, 100_000}, turn: 5},
		{name: "zero intermediate usage", totals: []int64{10_000, 0, 31_000, 0, 55_000}, turn: 5},
		{name: "zero latest usage", totals: []int64{10_000, 18_000, 31_000, 43_000, 0}, turn: -1},
		{name: "all usage missing", totals: []int64{0, 0, 0}, turn: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := NewBuilder().(*builder)
			for index, total := range test.totals {
				addCompactionTestTurn(t, current, index, total)
			}
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "staged heartbeat"})
			before, err := current.Build()
			if err != nil {
				t.Fatal(err)
			}
			assertCompactionTurn(t, current, test.turn)
			after, err := current.Build()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) || len(current.stagedSuffix) != 1 {
				t.Fatal("selection changed the request or committed staged arrivals")
			}
		})
	}
}

func TestCompactionCarriesPendingToolCalls(t *testing.T) {
	for _, test := range []struct {
		name       string
		callTurn   int
		resultTurn int
		running    bool
		wantTurn   int
	}{
		{name: "pending B", callTurn: 1, resultTurn: -1, wantTurn: 3},
		{name: "pending A", callTurn: 0, resultTurn: -1, wantTurn: 3},
		{name: "completed before suffix", callTurn: 1, resultTurn: 2, wantTurn: 3},
		{name: "completed in D", callTurn: 1, resultTurn: 3, wantTurn: 3},
		{name: "staged completion", callTurn: 1, resultTurn: 5, wantTurn: 3},
		{name: "committed running result", callTurn: 1, resultTurn: 2, running: true, wantTurn: 3},
		{name: "staged running result", callTurn: 1, resultTurn: 5, running: true, wantTurn: 3},
		{name: "pending call already in E", callTurn: 4, resultTurn: -1, wantTurn: 3},
		{name: "orphan result in D", callTurn: -1, resultTurn: 3, wantTurn: 3},
		{name: "orphan staged result", callTurn: -1, resultTurn: 5, wantTurn: 3},
		{name: "orphan only in discarded prefix", callTurn: -1, resultTurn: 1, wantTurn: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := NewBuilder().(*builder)
			for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
				if turn == test.resultTurn {
					current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, test.running)
				}
				var output []llm.Item
				if turn == test.callTurn {
					output = []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "test"}}}
				}
				addCompactionTestTurn(t, current, turn, total, output...)
			}
			if test.resultTurn == 5 {
				current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, test.running)
			}
			assertCompactionTurn(t, current, test.wantTurn)
			rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
			if !ok {
				t.Fatal("compaction rejected")
			}
			built, err := rebuilt.Build()
			if err != nil {
				t.Fatal(err)
			}
			var calls int
			for _, item := range built.Request.Input {
				if item.Type == llm.ItemToolCall {
					calls++
				}
				if item.Type == llm.ItemMessage {
					text := item.Data.(llm.Message).Text
					if text == "A input" || text == "B input" || text == "C input" {
						t.Fatalf("compaction retained old message %q", text)
					}
				}
			}
			wantCalls := 0
			if test.callTurn >= 0 && (test.callTurn >= 3 || test.resultTurn < 0 || test.resultTurn >= 3 || test.running) {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("retained calls = %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestCompactionCarriesOnlyUnfinishedChainedCalls(t *testing.T) {
	current := NewBuilder().(*builder)
	for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
		var output []llm.Item
		switch turn {
		case 1:
			output = []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "B", Name: "test"}}}
		case 2:
			current.AddToolResult("B", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done B"}}, false)
			output = []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "C", Name: "test"}}}
		case 3:
			current.AddToolResult("C", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done C"}}, false)
		}
		addCompactionTestTurn(t, current, turn, total, output...)
	}
	assertCompactionTurn(t, current, 3)
	rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
	if !ok {
		t.Fatal("compaction rejected")
	}
	built, _ := rebuilt.Build()
	var calls []string
	for _, item := range built.Request.Input {
		if item.Type == llm.ItemToolCall {
			calls = append(calls, item.Data.(llm.ToolCall).CallID)
		}
	}
	if !reflect.DeepEqual(calls, []string{"C"}) {
		t.Fatalf("carried calls = %v, want C", calls)
	}
}

func TestCompactionKeepsInterruptedTurnsAfterTheCut(t *testing.T) {
	current := NewBuilder().(*builder)
	addCompactionTestTurn(t, current, 0, 10_000)
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "interrupted turn"})
	current.Commit()
	current.Commit()
	addCompactionTestTurn(t, current, 1, 25_000)
	start, ok := current.compactionCutoff()
	if !ok || start != 0 {
		t.Fatalf("selection = %d, %t, want checkpoint 0 before the interrupted turn", start, ok)
	}
	if got := current.committedPrefix[current.prefixTokens[start].committedPrefixIndex].Data.(llm.Message).Text; got != "interrupted turn" {
		t.Fatalf("first retained input = %q", got)
	}
}

func addCompactionTestTurn(t *testing.T, current *builder, turn int, total int64, output ...llm.Item) {
	t.Helper()
	label := string(rune('A' + turn))
	payload, err := json.Marshal(label + " input")
	if err != nil {
		t.Fatal(err)
	}
	if err := current.AddExternalInput(inbox.Input{ID: inbox.ID(label), Kind: inbox.InputExternal, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	current.Commit()
	response := llm.Response{Output: []llm.Item{
		{Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning"}`), Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: label + " reasoning"}}},
		{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: label + " output"}},
	}}
	response.Output = append(response.Output, output...)
	if total != 0 {
		response.Usage = llm.Usage{
			InputTokens: total - 1_000, OutputTokens: 1_000,
			CachedInputTokens: 2_000, CacheWriteInputTokens: 500, ReasoningTokens: 400,
		}
	}
	current.AddModelResponse(response)
}

func assertCompactionTurn(t *testing.T, current *builder, turn int) {
	t.Helper()
	start, ok := current.compactionCutoff()
	if turn < 0 {
		if ok || start != 0 {
			t.Fatalf("selection = %d, %t, want no compaction", start, ok)
		}
		return
	}
	want := turn - 1
	if !ok || start != want {
		t.Fatalf("selection = %d, %t, want turn %d at %d", start, ok, turn, want)
	}
}

func TestCompactionUsesExplicitResultTerminality(t *testing.T) {
	current := NewBuilder().(*builder)
	for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
		var output []llm.Item
		if turn == 0 {
			for _, id := range []string{"finished", "pending"} {
				output = append(output, llm.Item{ProviderID: id, Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: id, Name: "test"}})
			}
		}
		if turn == 1 {
			current.AddToolResult("finished", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
			current.AddToolResult("finished", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ToolCallRunningPayload}}, false)
			current.AddToolResult("pending", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
		}
		addCompactionTestTurn(t, current, turn, total, output...)
	}
	rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
	if !ok {
		t.Fatal("compaction rejected")
	}
	request := buildCompactionContext(t, rebuilt).Request
	if request.Input[2].ProviderID != "pending" {
		t.Fatalf("carried call = %#v", request.Input[2])
	}
	for _, item := range request.Input {
		if item.Type == llm.ItemToolCall && item.Data.(llm.ToolCall).CallID == "finished" {
			t.Fatal("terminal result was treated as running because of its text")
		}
	}
	carried := rebuilt.(*builder)
	if !carried.committedPrefix[3].Data.(llm.ToolResult).Running {
		t.Fatal("carried running result lost its lifecycle flag")
	}
	// The carried call must survive a second reconstruction.
	addCompactionTestTurn(t, carried, 5, 20_000)
	addCompactionTestTurn(t, carried, 6, 35_000)
	again, ok := carried.Compact(usableCompactionResponse("second summary"))
	if !ok || buildCompactionContext(t, again).Request.Input[2].ProviderID != "pending" {
		t.Fatal("second compaction lost the carried call")
	}
}

func TestCompactionSummarizesOversizedNewestTurn(t *testing.T) {
	current := NewBuilder().(*builder)
	call := llm.Item{ProviderID: "pending", Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "test"}}
	addCompactionTestTurn(t, current, 0, 10_000, call)
	addCompactionTestTurn(t, current, 1, 40_000)
	current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, false)
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "arrived during compaction"})

	rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
	if !ok {
		t.Fatal("compaction rejected")
	}
	want := withPreamble(
		llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "summary"}},
		call,
		llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "call", Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}}},
		llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "arrived during compaction"}},
	)
	if got := buildCompactionContext(t, rebuilt).Request.Input; !reflect.DeepEqual(got, want) {
		t.Fatalf("rebuilt context = %#v, want %#v", got, want)
	}
}

func TestCompactionPreservesOrderedPendingCallsAndResults(t *testing.T) {
	current := NewBuilder().(*builder)
	first := llm.Item{ProviderID: "provider-z", Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "z", Name: "first", Arguments: `{"path":"first"}`}}
	second := llm.Item{ProviderID: "provider-a", Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "a", Name: "second", Arguments: `{"path":"second"}`}}
	addCompactionTestTurn(t, current, 0, 10_000, first)
	current.AddToolResult("z", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
	addCompactionTestTurn(t, current, 1, 40_000, second)
	current.AddToolResult("a", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
	addCompactionTestTurn(t, current, 2, 70_000)

	rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
	if !ok {
		t.Fatal("compaction rejected")
	}
	want := withPreamble(
		llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "summary"}},
		first,
		llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "z", Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ToolCallRunningPayload}}, Running: true}},
		second,
		llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "a", Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ToolCallRunningPayload}}, Running: true}},
	)
	if got := buildCompactionContext(t, rebuilt).Request.Input; !reflect.DeepEqual(got, want) {
		t.Fatalf("carried prefix = %#v, want %#v", got, want)
	}
}
