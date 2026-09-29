package llm

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"
)

func TestItemJSONRoundTrip(t *testing.T) {
	tests := []Item{
		{ProviderID: "message-1", Type: ItemMessage, Data: Message{
			Role: RoleAssistant, Text: "hello", Phase: "commentary",
		}},
		{ProviderID: "call-1", Type: ItemToolCall, Data: ToolCall{
			CallID: "call-1", Name: "test", Arguments: `{}`,
		}},
		{ProviderID: "result-1", Type: ItemToolResult, Data: ToolResult{
			CallID: "call-1", Output: []ToolResultOutput{{Kind: ToolResultText, Value: "done"}},
		}},
		{ProviderID: "result-running", Type: ItemToolResult, Data: ToolResult{
			CallID: "call-running", Output: []ToolResultOutput{{Kind: ToolResultText, Value: "running"}}, Running: true,
		}},
		{ProviderID: "result-image", Type: ItemToolResult, Data: ToolResult{
			CallID: "call-image", Output: []ToolResultOutput{{Kind: ToolResultImage, Value: "image-data"}},
		}},
		{ProviderID: "result-mixed", Type: ItemToolResult, Data: ToolResult{
			CallID: "call-mixed", Output: []ToolResultOutput{
				{Kind: ToolResultText, Value: "Dimensions: 2000x1500"},
				{Kind: ToolResultImage, Value: "data:image/png;base64,aGVsbG8="},
			},
		}},
		{ProviderID: "reasoning-1", Type: ItemProvider, Data: ProviderItem{Type: "reasoning",
			Display: &ProviderDisplay{Kind: ProviderDisplayReasoning, Text: "inspect"}, Raw: jsontext.Value(`{"encrypted":"opaque"}`),
		}},
	}

	for _, want := range tests {
		t.Run(string(want.Type), func(t *testing.T) {
			encoded, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), `"ProviderID"`) ||
				!strings.Contains(string(encoded), `"Data":{`) {
				t.Fatalf("encoded item = %s", encoded)
			}
			var got Item
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip = %#v, want %#v", got, want)
			}
		})
	}
}

func TestItemJSONRejectsInvalidTagAndData(t *testing.T) {
	invalid := []struct {
		item Item
		want string
	}{
		{item: Item{Type: ItemMessage, Data: ToolCall{}}, want: "llm.Message"},
		{item: Item{Type: ItemToolCall, Data: Message{}}, want: "llm.ToolCall"},
		{item: Item{Type: ItemToolResult, Data: Message{}}, want: "llm.ToolResult"},
		{item: Item{Type: ItemProvider, Data: Message{}}, want: "llm.ProviderItem"},
		{item: Item{Type: ItemProvider, Data: ProviderItem{}}, want: "provider item Raw is required"},
		{item: Item{Type: ItemProvider, Data: ProviderItem{Raw: jsontext.Value{}}}, want: "provider item Raw is required"},
		{item: Item{Type: ItemProvider, Data: ProviderItem{Raw: jsontext.Value(`null`)}}, want: "provider item Raw is required"},
		{item: Item{Type: ItemProvider, Data: ProviderItem{Display: &ProviderDisplay{Kind: ProviderDisplayReasoning, Text: "visible"}}}, want: "provider item Raw is required"},
		{item: Item{Type: "unknown", Data: struct{}{}}, want: `unsupported item type "unknown"`},
	}
	for _, test := range invalid {
		if _, err := json.Marshal(test.item); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("marshal error = %v, want %q", err, test.want)
		}
	}

	for _, encoded := range []string{
		`{`,
		`{"Type":[],"Data":{}}`,
		`{"ProviderID":[],"Type":"message","Data":{"Role":"user"}}`,
		`{"Type":"unknown","Data":{}}`,
		`{"Type":"message","Data":[]}`,
		`{"Type":"tool_call","Data":[]}`,
		`{"Type":"tool_result","Data":[]}`,
		`{"Type":"tool_result","Data":{"Output":"done"}}`,
		`{"Type":"tool_result","Data":{"Output":42}}`,
		`{"Type":"tool_result","Data":{"Output":{}}}`,
		`{"Type":"tool_result","Data":{"Output":["done"]}}`,
		`{"Type":"tool_result","Data":{"Output":[{"Value":42}]}}`,
		`{"Type":"reasoning","Data":[]}`,
		`{"Type":"provider","Data":[]}`,
		`{"Type":"provider","Data":null}`,
		`{"Type":"provider","Data":{}}`,
		`{"Type":"provider","Data":{"Raw":null}}`,
		`{"Type":"provider","Data":{"Display":{"Kind":"reasoning","Text":"visible"}}}`,
		`{"Type":"message","Data":null}`,
		`{"Type":"tool_call","Data":null}`,
		`{"Type":"tool_result","Data":null}`,
		`{"Type":"reasoning","Data":null}`,
	} {
		var item Item
		if err := json.Unmarshal([]byte(encoded), &item); err == nil {
			t.Fatalf("decoded invalid item %s", encoded)
		}
	}
}

func TestItemJSONMigratesLegacyReasoning(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		want ProviderItem
	}{
		{"responses", `{"Summary":["first","second"],"Raw":{"type":"reasoning","encrypted_content":"opaque","extra":9007199254740993}}`, ProviderItem{
			Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning","encrypted_content":"opaque","extra":9007199254740993}`),
			Display: &ProviderDisplay{Kind: ProviderDisplayReasoning, Text: "first\n\nsecond"},
		}},
		{"thinking", `{"Raw":{"type":"thinking","thinking":"original","signature":"signed"}}`, ProviderItem{
			Type: "thinking", Raw: jsontext.Value(`{"type":"thinking","thinking":"original","signature":"signed"}`),
		}},
		{"redacted", `{"Raw":{"type":"redacted_thinking","data":"opaque"}}`, ProviderItem{
			Type: "redacted_thinking", Raw: jsontext.Value(`{"type":"redacted_thinking","data":"opaque"}`),
		}},
		{"summary only", `{"Summary":["visible"]}`, ProviderItem{Display: &ProviderDisplay{Kind: ProviderDisplayReasoning, Text: "visible"}}},
		{"empty", `{}`, ProviderItem{}},
		{"unrecognised payload", `{"Raw":{"type":42,"state":"opaque"}}`, ProviderItem{Raw: jsontext.Value(`{"type":42,"state":"opaque"}`)}},
		{"nonobject payload", `{"Raw":[1,2]}`, ProviderItem{Raw: jsontext.Value(`[1,2]`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var item Item
			if err := json.Unmarshal([]byte(`{"ProviderID":"original-id","Type":"reasoning","Data":`+test.data+`}`), &item); err != nil {
				t.Fatal(err)
			}
			want := Item{ProviderID: "original-id", Type: ItemProvider, Data: test.want}
			if !reflect.DeepEqual(item, want) {
				t.Fatalf("migrated item = %#v, want %#v", item, want)
			}
			encoded, err := json.Marshal(item)
			if len(test.want.Raw) == 0 {
				if err == nil || !strings.Contains(err.Error(), "provider item Raw is required") {
					t.Fatalf("marshal error = %v, want missing Raw error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), `"Type":"reasoning","Data"`) || strings.Contains(string(encoded), `"Summary"`) {
				t.Fatalf("wrote retired representation: %s", encoded)
			}
			var restored Item
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restored, want) {
				t.Fatalf("migration changed on reload: %#v", restored)
			}
		})
	}
}
