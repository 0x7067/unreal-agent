package messagesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestThinkingPersistsAndReplays(t *testing.T) {
	for _, raw := range []string{
		`{"type":"thinking","thinking":"Consider tools","signature":"signed","extra":9007199254740993}`,
		`{"type":"redacted_thinking","data":"opaque","extra":1e1000}`,
	} {
		response, err := decodeResponse(responseBody("end_turn", `[`+raw+`]`))
		if err != nil {
			t.Fatal(err)
		}
		provider := response.Output[0].Data.(llm.ProviderItem)
		if response.Output[0].Type != llm.ItemProvider || string(provider.Raw) != raw {
			t.Fatalf("provider output = %#v", response.Output[0])
		}
		persisted, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		var restored llm.Response
		if err := json.Unmarshal(persisted, &restored); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(restored, response) {
			t.Fatal("persistence changed provider output")
		}
		request := validRequest()
		request.Input = append(request.Input, restored.Output...)
		encoded, err := requestBody(request)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(encoded), raw) {
			t.Fatalf("replay changed provider state: %s", encoded)
		}
	}
}

func TestLegacyReasoningWithoutRawCannotReplay(t *testing.T) {
	for _, data := range []string{`{}`, `{"Summary":["visible"]}`, `{"Raw":null}`} {
		var item llm.Item
		if err := json.Unmarshal([]byte(`{"Type":"reasoning","Data":`+data+`}`), &item); err != nil {
			t.Fatal(err)
		}
		request := validRequest()
		request.Input = append(request.Input, item)
		body, err := requestBody(request)
		if err == nil || !strings.Contains(err.Error(), "provider item Raw is required") || body != nil {
			t.Fatalf("body = %s, error = %v, want missing Raw error", body, err)
		}
	}
}

func TestLegacyReasoningReplay(t *testing.T) {
	for _, raw := range []string{
		`{"type":"reasoning","encrypted_content":"opaque"}`,
		`{"type":"thinking","thinking":"original","signature":"signed"}`,
		`{"type":"redacted_thinking","data":"opaque"}`,
		`{"vendor_payload":{"n":9007199254740993}}`,
	} {
		var item llm.Item
		if err := json.Unmarshal([]byte(`{"Type":"reasoning","Data":{"Summary":["display only"],"Raw":`+raw+`}}`), &item); err != nil {
			t.Fatal(err)
		}
		converted, err := requestInputBlock(item)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := converted.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != raw {
			t.Fatalf("legacy payload changed: %s", encoded)
		}
	}
}

func TestProviderPayloadPassesThrough(t *testing.T) {
	for _, raw := range []string{
		`{"type":"future_block","data":1e1000}`,
		`{"type":42,"data":9007199254740993}`,
		`{"vendor_payload":{"data":"opaque"}}`,
	} {
		item := llm.Item{Type: llm.ItemProvider, Data: llm.ProviderItem{
			Type: "display_metadata", Raw: jsontext.Value(raw),
		}}
		converted, err := requestInputBlock(item)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := converted.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != raw {
			t.Fatalf("provider payload changed: %s", encoded)
		}
	}
}
