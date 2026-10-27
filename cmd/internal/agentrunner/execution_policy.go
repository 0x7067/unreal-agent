package agentrunner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

// Runner defaults are an executable policy, not library defaults. The underlying
// manager/runtime retain zero-value compatibility. Explicit environment zero
// disables a threshold; output limits are not time or resource limits.
func executionPolicy(config Config, getenv func(string) string) (executionbudget.Policy, int, error) {
	policy := config.Budget
	if policy == (executionbudget.Policy{}) {
		policy = executionbudget.Policy{WarnAfter: 30 * time.Second, Timeout: 120 * time.Second}
		for _, field := range []struct {
			name   string
			target *time.Duration
		}{
			{"UNREAL_HARNESS_OPERATION_WARN_AFTER", &policy.WarnAfter},
			{"UNREAL_HARNESS_OPERATION_TIMEOUT", &policy.Timeout},
		} {
			raw := strings.TrimSpace(getenv(field.name))
			if raw == "" {
				continue
			}
			value, err := time.ParseDuration(raw)
			if err != nil || value < 0 {
				return policy, 0, fmt.Errorf("%s must be a nonnegative duration (0 disables it)", field.name)
			}
			*field.target = value
		}
	}
	if err := policy.Validate(); err != nil {
		return policy, 0, fmt.Errorf("operation budget: %w", err)
	}
	maxOperations := 64
	if raw := strings.TrimSpace(getenv("UNREAL_HARNESS_MAX_OPERATIONS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return policy, 0, fmt.Errorf("UNREAL_HARNESS_MAX_OPERATIONS must be a nonnegative integer (0 disables the bound)")
		}
		maxOperations = value
	}
	return policy, maxOperations, nil
}
