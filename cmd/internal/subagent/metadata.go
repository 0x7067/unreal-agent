package subagent

import (
	"encoding/json/v2"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

// Decode optional additive metadata without requiring tool translator callers to
// populate it. Empty lifetime retains the historical reusable-agent behavior.
type runMetadata struct {
	Purpose, Lifetime, ParentHandle, ParentTask, GraphID, TaskID string
	OwnedResources                                               []string
	TaskGeneration                                               uint64
}

func metadataFromPlan(encoded []byte) runMetadata {
	var m runMetadata
	_ = json.Unmarshal(encoded, &m)
	m.Purpose = boundedMetadata(m.Purpose)
	m.ParentHandle = boundedMetadata(m.ParentHandle)
	m.GraphID = boundedMetadata(m.GraphID)
	m.TaskID = boundedMetadata(m.TaskID)
	m.ParentTask = boundedMetadata(m.ParentTask)
	m.OwnedResources = m.OwnedResources[:min(len(m.OwnedResources), 16)]
	for i := range m.OwnedResources {
		m.OwnedResources[i] = boundedMetadata(m.OwnedResources[i])
	}
	if m.Lifetime != "task" && m.Lifetime != "background" {
		m.Lifetime = ""
	}
	return m
}
func boundedMetadata(v string) string { truncated := false; return activityIdentity(v, &truncated) }
func applyMetadata(c *ChildActivity, m runMetadata) {
	c.ParentTask = m.ParentTask
	c.OwnedResources = append([]string(nil), m.OwnedResources...)
	c.Purpose = m.Purpose
	c.Lifetime = m.Lifetime
	c.ParentHandle = m.ParentHandle
	c.GraphID = m.GraphID
	c.TaskID = m.TaskID
	c.TaskGeneration = m.TaskGeneration
}
func finalReport(response llm.Response) bool {
	if response.Stop != llm.StopComplete || response.Failure != nil {
		return false
	}
	text := false
	for _, out := range response.Output {
		switch v := out.Data.(type) {
		case llm.ToolCall:
			return false
		case llm.Message:
			if v.Text != "" {
				text = true
			}
		}
	}
	return text
}
