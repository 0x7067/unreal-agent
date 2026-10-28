package workerruntime

import (
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"reflect"
	"testing"
)

func TestDurableOwnerDefaultEndpointStableBeforeReconciliation(t *testing.T) {
	b := workerproto.Bootstrap{DurableSubagents: true, Provider: "openai", Directory: "/private/parent", Workspace: "/private/work", ChildRuntimeDirectory: "/private/runtime", DurableChildCommand: []string{"/private/stage-a"}, Model: llm.Model{ID: "test"}}
	first, err := normalizeDurableOwner(b, providers.Default())
	if err != nil || first.BaseURL == "" {
		t.Fatalf("default endpoint: %+v %v", first, err)
	}
	a, err := durableOwnerConfig(first)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := normalizeDurableOwner(b, providers.Default())
	if err != nil {
		t.Fatal(err)
	}
	c, err := durableOwnerConfig(restarted)
	if err != nil || !reflect.DeepEqual(a, c) {
		t.Fatalf("launch/resume differ: %+v %+v %v", a, c, err)
	}
	b.BaseURL = "http://127.0.0.1:1234"
	explicit, err := normalizeDurableOwner(b, nil)
	if err != nil || explicit.BaseURL != b.BaseURL {
		t.Fatal("explicit endpoint altered", err)
	}
	b.BaseURL = ""
	if _, err := normalizeDurableOwner(b, nil); err == nil {
		t.Fatal("unknown provider guessed endpoint")
	}
}
