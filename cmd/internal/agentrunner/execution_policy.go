package agentrunner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

// Runner defaults match upstream: no wall thresholds or work admission bound.
// Environment or embedding configuration may opt into limits; explicit zero
// disables a threshold. Output limits are not time or resource limits.
func executionPolicy(config Config, getenv func(string) string) (executionbudget.Policy, int, error) {
	policy := config.Budget
	if policy == (executionbudget.Policy{}) {
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
	maxOperations := 0
	if raw := strings.TrimSpace(getenv("UNREAL_HARNESS_MAX_OPERATIONS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return policy, 0, fmt.Errorf("UNREAL_HARNESS_MAX_OPERATIONS must be a nonnegative integer (0 disables the bound)")
		}
		maxOperations = value
	}
	return policy, maxOperations, nil
}
