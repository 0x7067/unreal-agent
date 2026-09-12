package reasoning

import (
	"errors"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

const Default = llm.ReasoningEffortMedium

func Choices() []string { return []string{"low", "medium", "high", "xhigh", "max"} }

func Parse(value string) (llm.ReasoningEffort, error) {
	switch value = strings.ToLower(strings.TrimSpace(value)); value {
	case "", "default":
		return Default, nil
	case "low", "medium", "high", "xhigh", "max":
		return llm.ReasoningEffort(value), nil
	default:
		return "", errors.New("reasoning effort must be default, low, medium, high, xhigh, or max")
	}
}

func Label(effort llm.ReasoningEffort) string {
	if effort == "" {
		return string(Default)
	}
	return string(effort)
}
