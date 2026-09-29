package responsesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestReasoningPersistsAndReplays(t *testing.T) {
	for _, raw := range []string{
		`{"id":"r","type":"reasoning","summary":[],"encrypted_content":"opaque","extra":1e1000}`,
		`{"id":"r","type":"reasoning","summary":[{"type":"summary_text","text":"Consider tools"}],"encrypted_content":"opaque","extra":9007199254740993}`,
	} {
		response, err := decodeResponse([]byte(`{"id":"response","status":"completed","output":[` + raw + `]}`))
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
			t.Fatal("persistence changed the response")
		}
		builder := contextbuilder.NewBuilder()
		builder.SetModel(llm.Model{ID: "test"})
		builder.AddModelResponse(restored)
		built, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		body, err := requestBody(built.Request, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), raw) {
			t.Fatalf("replay changed provider state: %s", body)
		}
	}
}

func TestResponseRejectsUnsupportedProviderOutput(t *testing.T) {
	for _, kind := range []string{"future_block", "local_shell_call", "shell_call", "computer_call", "custom_tool_call", "mcp_approval_request", "apply_patch_call", "tool_search_call"} {
		t.Run(kind, func(t *testing.T) {
			_, err := decodeResponse([]byte(`{"id":"response","status":"completed","output":[{"type":"` + kind + `"}]}`))
			if err == nil || !strings.Contains(err.Error(), `unsupported output item type "`+kind+`"`) {
				t.Fatalf("error = %v, want unsupported output item type", err)
			}
		})
	}
}

func TestReasoningSummaryPreservesParagraphs(t *testing.T) {
	response, err := decodeResponse([]byte(`{"id":"response","status":"completed","output":[{"id":"r","type":"reasoning","summary":[{"type":"summary_text","text":"first"},{"type":"summary_text","text":"second"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	display := response.Output[0].Data.(llm.ProviderItem).Display
	if display == nil || display.Kind != llm.ProviderDisplayReasoning || display.Text != "first\n\nsecond" {
		t.Fatalf("display = %#v", display)
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
		body, err := requestBody(request, "", nil)
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
		converted, err := requestInputItem(item)
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
		converted, err := requestInputItem(item)
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
