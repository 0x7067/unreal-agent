package main

import (
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

func TestConfiguredBudgetDefaultsExplicitLimitsAndInvalidPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want executionbudget.Policy
		max  int
		bad  bool
	}{
		{name: "upstream defaults"},
		{name: "explicit zeros", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "0", "UNREAL_HARNESS_OPERATION_TIMEOUT": "0", "UNREAL_HARNESS_MAX_OPERATIONS": "0"}},
		{name: "explicit limits", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "2s", "UNREAL_HARNESS_OPERATION_TIMEOUT": "5s", "UNREAL_HARNESS_MAX_OPERATIONS": "7"}, want: executionbudget.Policy{WarnAfter: 2 * time.Second, Timeout: 5 * time.Second}, max: 7},
		{name: "timeout only", env: map[string]string{"UNREAL_HARNESS_OPERATION_TIMEOUT": "10s"}, want: executionbudget.Policy{Timeout: 10 * time.Second}},
		{name: "warning only", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "10s"}, want: executionbudget.Policy{WarnAfter: 10 * time.Second}},
		{name: "invalid order", env: map[string]string{"UNREAL_HARNESS_OPERATION_WARN_AFTER": "20s", "UNREAL_HARNESS_OPERATION_TIMEOUT": "10s"}, bad: true},
		{name: "negative duration", env: map[string]string{"UNREAL_HARNESS_OPERATION_TIMEOUT": "-1s"}, bad: true},
		{name: "invalid duration", env: map[string]string{"UNREAL_HARNESS_OPERATION_TIMEOUT": "wrong"}, bad: true},
		{name: "negative capacity", env: map[string]string{"UNREAL_HARNESS_MAX_OPERATIONS": "-1"}, bad: true},
		{name: "invalid capacity", env: map[string]string{"UNREAL_HARNESS_MAX_OPERATIONS": "wrong"}, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, n, err := configuredBudgets(func(key string) string { return tc.env[key] })
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v bad=%v", err, tc.bad)
			}
			if !tc.bad && (p != tc.want || n != tc.max) {
				t.Fatalf("policy=%+v max=%d, want %+v/%d", p, n, tc.want, tc.max)
			}
		})
	}
}
