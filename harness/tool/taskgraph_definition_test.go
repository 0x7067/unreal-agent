package tool

import (
	"strings"
	"testing"
)

func TestTaskGraphDefinitionExplainsScopeStatusAndInputPolicy(t *testing.T) {
	definition := taskGraphDefinition().Tool
	for _, want := range []string{"meaningful independent work", "tiny or serial tasks", "pure snapshot", "scan phases", "does not revalidate freshness"} {
		if !strings.Contains(definition.Description, want) {
			t.Errorf("TaskGraph description omits %q", want)
		}
	}
	properties := definition.Parameters["properties"].(map[string]any)
	tasks := properties["tasks"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	reads := tasks["reads"].(map[string]any)["description"].(string)
	for _, want := range []string{"Prefer explicit paths", "git-ignored files", "Symlinks are rejected", "serialize tasks", "Explicit [] declares no reads"} {
		if !strings.Contains(reads, want) {
			t.Errorf("TaskGraph reads description omits %q", want)
		}
	}
}
