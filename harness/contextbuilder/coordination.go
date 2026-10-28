package contextbuilder

import (
	_ "embed"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

//go:embed prompts/taskgraph.md
var taskGraphGuidance string

//go:embed prompts/helpers.md
var helperGuidance string

//go:embed prompts/agentcontrol.md
var agentControlGuidance string

// Tool registration is the capability boundary; history and project instructions
// cannot enable guidance. Assemble once per capability, in a stable order.
func coordinationGuidance(tools []llm.Tool) string {
	var graph, helpers, control bool
	for _, enabled := range tools {
		if enabled.Type != llm.ToolFunction {
			continue
		}
		switch enabled.Name {
		case tool.TaskGraphName:
			graph = true
		case tool.AgentName:
			helpers = true
		case tool.AgentControlName:
			control = true
		}
	}
	var sections []string
	if graph {
		sections = append(sections, strings.TrimSpace(taskGraphGuidance))
	}
	if helpers {
		sections = append(sections, strings.TrimSpace(helperGuidance))
	}
	if control {
		sections = append(sections, strings.TrimSpace(agentControlGuidance))
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n## Coordination tools\n\n" + strings.Join(sections, "\n\n")
}
