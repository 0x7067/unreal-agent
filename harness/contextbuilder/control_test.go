package contextbuilder

import (
	"encoding/json/jsontext"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestBuilderControlMessages(t *testing.T) {
	for _, test := range []struct {
		mode inbox.ControlMode
		role llm.Role
		text string
	}{
		{mode: inbox.Heartbeat, role: llm.RoleUser, text: "requested"},
		{mode: inbox.StopHard},
		{mode: inbox.StopWhenIdle},
	} {
		t.Run(string(test.mode), func(t *testing.T) {
			builder := NewBuilder()
			builder.AddControlMessage(inbox.ControlMessage{Mode: test.mode, Reason: "requested"})
			result, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			input := result.Request.Input[1:]
			if test.text == "" {
				if len(input) != 0 {
					t.Fatal("control added a model message")
				}
				return
			}
			if len(input) != 1 || input[0].Type != llm.ItemMessage {
				t.Fatalf("input = %#v", input)
			}
			message := input[0].Data.(llm.Message)
			if message.Role != test.role || message.Text != test.text {
				t.Fatalf("message = %#v", message)
			}
		})
	}
}

func TestBuilderSettingsApplySuppliedFields(t *testing.T) {
	initialLimit, nextLimit := int64(123), int64(456)
	initialModel := llm.Model{ID: "initial", MaxOutputTokens: &initialLimit, ReasoningEffort: llm.ReasoningEffortHigh}
	for _, test := range []struct {
		name     string
		settings inbox.Settings
		model    llm.Model
	}{
		{"omitted", inbox.Settings{}, initialModel},
		{"model", inbox.Settings{Model: "next"}, llm.Model{ID: "next", MaxOutputTokens: &initialLimit, ReasoningEffort: llm.ReasoningEffortHigh}},
		{"limit", inbox.Settings{MaxOutputTokens: &nextLimit}, llm.Model{ID: "initial", MaxOutputTokens: &nextLimit, ReasoningEffort: llm.ReasoningEffortHigh}},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := NewBuilder()
			builder.SetModel(initialModel)
			builder.SetSystemPrompt("initial prompt")
			builder.AddTool(llm.Tool{Name: "tool"})
			if err := builder.AddExternalInput(inbox.Input{ID: "prompt", Kind: inbox.InputExternal, Payload: []byte(`"hello"`)}); err != nil {
				t.Fatal(err)
			}
			original, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			builder.AddControlMessage(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: test.settings})
			built, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			want := original
			want.Request.Model = test.model
			if !reflect.DeepEqual(built, want) {
				t.Fatalf("request = %#v, want %#v", built, want)
			}
			if !reflect.DeepEqual(original.Request.Model, initialModel) || original.Request.Input[0].Data.(llm.Message).Text != preamble+"\n\ninitial prompt" {
				t.Fatal("settings mutated an already built request")
			}
		})
	}
}

func TestBuilderSettingsOnlyChangeEffortInSubsequentRequests(t *testing.T) {
	limit := int64(123)
	builder := NewBuilder()
	builder.SetModel(llm.Model{ID: "model", MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortHigh})
	builder.AddTool(llm.Tool{Name: "tool"})
	builder.AddModelResponse(llm.Response{Output: []llm.Item{{Type: llm.ItemProvider, Data: llm.ProviderItem{
		Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning"}`),
		Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: "thought"},
	}}}})
	original, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	builder.AddControlMessage(inbox.ControlMessage{
		Mode: inbox.UpdateSettings, Parameters: inbox.Settings{ReasoningEffort: llm.ReasoningEffortLow},
	})
	built, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	want := original
	want.Request.Model.ReasoningEffort = llm.ReasoningEffortLow
	if !reflect.DeepEqual(built, want) {
		t.Fatalf("settings changed other request content: got %#v, want %#v", built, want)
	}
	if original.Request.Model.ReasoningEffort != llm.ReasoningEffortHigh {
		t.Fatal("settings mutated an already built request")
	}
}
