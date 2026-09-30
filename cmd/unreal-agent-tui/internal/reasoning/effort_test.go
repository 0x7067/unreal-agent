package reasoning

import (
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestEffortChoicesRoundTrip(t *testing.T) {
	for _, value := range Choices() {
		effort, err := Parse(value)
		if err != nil || string(effort) != value {
			t.Fatalf("round trip %q: %q, %v", value, effort, err)
		}
	}
	for _, test := range []struct {
		input string
		want  llm.ReasoningEffort
	}{
		{"", llm.ReasoningEffortMedium}, {" DEFAULT ", llm.ReasoningEffortMedium}, {" High ", llm.ReasoningEffortHigh},
	} {
		if got, err := Parse(test.input); err != nil || got != test.want {
			t.Fatalf("Parse(%q) = %q, %v", test.input, got, err)
		}
	}
	for _, value := range []string{"none", "maximum", "high extra", "high\x1b[2J"} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
