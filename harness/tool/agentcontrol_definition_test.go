package tool

import (
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"testing"
)

func TestAgentControlSelectionAndSchema(t *testing.T) {
	configured := &fixedTranslator{}
	selected := NewRegistry(StaticTranslators{AgentControl: configured}, AgentControlName)
	got, ok := selected.Resolve(AgentControlName)
	if !ok || got != configured {
		t.Fatal("configured control translator not selected")
	}
	defs := selected.StaticDefinitions()
	if len(defs) != 1 || defs[0].Tool.Name != AgentControlName {
		t.Fatalf("definitions=%+v", defs)
	}
	unselected := NewRegistry(StaticTranslators{AgentControl: configured}, AgentName)
	if _, ok := unselected.Resolve(AgentControlName); ok {
		t.Fatal("Agent selection implicitly enabled control")
	}
	unavailable, _ := NewRegistry(StaticTranslators{}, AgentControlName).Resolve(AgentControlName)
	if unavailable.Translate(nil, llm.ToolCall{}).Error == "" {
		t.Fatal("unconfigured control must fail")
	}
}
