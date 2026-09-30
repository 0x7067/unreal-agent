package reasoning

import (
	"errors"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

const Default = llm.ReasoningEffortMedium

func Choices() []string { return []string{"low", "medium", "high", "xhigh", "max"} }

func Parse(value string) (llm.ReasoningEffort, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "default" {
		return Default, nil
	}
	effort := llm.ReasoningEffort(value)
	if !effort.Valid() {
		return "", errors.New("reasoning effort must be default, low, medium, high, xhigh, or max")
	}
	return effort, nil
}

func Label(effort llm.ReasoningEffort) string {
	if effort == "" {
		return string(Default)
	}
	return string(effort)
}
