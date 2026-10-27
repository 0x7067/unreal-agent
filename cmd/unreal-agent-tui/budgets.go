package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

// Frontend defaults are explicit policy, not library defaults. Each wall budget
// warns/requests cancellation; it never claims an uncooperative tool drained.
func configuredBudgets(getenv func(string) string) (executionbudget.Policy, int, error) {
	policy := executionbudget.Policy{WarnAfter: 30 * time.Second, Timeout: 120 * time.Second}
	for _, entry := range []struct {
		name  string
		value *time.Duration
	}{{"UNREAL_HARNESS_OPERATION_WARN_AFTER", &policy.WarnAfter}, {"UNREAL_HARNESS_OPERATION_TIMEOUT", &policy.Timeout}} {
		if text := strings.TrimSpace(getenv(entry.name)); text != "" {
			duration, err := time.ParseDuration(text)
			if err != nil {
				return policy, 0, fmt.Errorf("%s: %w", entry.name, err)
			}
			*entry.value = duration
		}
	}
	limit := 64
	if text := strings.TrimSpace(getenv("UNREAL_HARNESS_MAX_OPERATIONS")); text != "" {
		parsed, err := strconv.Atoi(text)
		if err != nil || parsed < 0 {
			return policy, 0, fmt.Errorf("UNREAL_HARNESS_MAX_OPERATIONS must be a nonnegative integer")
		}
		limit = parsed
	}
	if err := policy.Validate(); err != nil {
		return policy, limit, err
	}
	return policy, limit, nil
}
