package agentrunner

import (
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

func TestRunnerExecutionPolicyDefaultsOverridesAndDisable(t *testing.T) {
	for _, test := range []struct {
		name   string
		env    map[string]string
		config Config
		want   executionbudget.Policy
		max    int
		bad    bool
	}{
		{name: "executable defaults", want: executionbudget.Policy{WarnAfter: 30 * time.Second, Timeout: 120 * time.Second}, max: 64},
		{name: "disabled", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "0", "UNREAL_HARNESS_OPERATION_TIMEOUT": "0", "UNREAL_HARNESS_MAX_OPERATIONS": "0"}, max: 0},
		{name: "custom", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "2s", "UNREAL_HARNESS_OPERATION_TIMEOUT": "5s", "UNREAL_HARNESS_MAX_OPERATIONS": "7"}, want: executionbudget.Policy{WarnAfter: 2 * time.Second, Timeout: 5 * time.Second}, max: 7},
		{name: "embedding policy", env: map[string]string{"UNREAL_HARNESS_OPERATION_TIMEOUT": "wrong"}, config: Config{Budget: executionbudget.Policy{Timeout: time.Second}}, want: executionbudget.Policy{Timeout: time.Second}, max: 64},
		{name: "invalid duration", env: map[string]string{"UNREAL_HARNESS_OPERATION_TIMEOUT": "wrong"}, bad: true},
		{name: "negative threshold", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "-1s"}, bad: true},
		{name: "invalid order", env: map[string]string{"UNREAL_HARNESS_OPERATION_TIMEOUT": "1s"}, bad: true},
		{name: "negative resource bound", env: map[string]string{"UNREAL_HARNESS_MAX_OPERATIONS": "-1"}, bad: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, maxOperations, err := executionPolicy(test.config, func(name string) string { return test.env[name] })
			if (err != nil) != test.bad {
				t.Fatalf("error=%v bad=%v", err, test.bad)
			}
			if !test.bad && (got != test.want || maxOperations != test.max) {
				t.Fatalf("policy=%+v max=%d want=%+v/%d", got, maxOperations, test.want, test.max)
			}
		})
	}
}
