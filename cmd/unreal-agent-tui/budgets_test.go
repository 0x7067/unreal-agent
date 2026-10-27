package main

import (
	"testing"
	"time"
)

func TestConfiguredBudgetDefaultsExplicitZerosAndInvalidPolicy(t *testing.T) {
	p, n, err := configuredBudgets(func(string) string { return "" })
	if err != nil || p.WarnAfter != 30*time.Second || p.Timeout != 120*time.Second || n != 64 {
		t.Fatalf("defaults %+v %d %v", p, n, err)
	}
	p, n, err = configuredBudgets(func(key string) string {
		if key == "UNREAL_HARNESS_MAX_OPERATIONS" {
			return "0"
		}
		return "0s"
	})
	if err != nil || p.WarnAfter != 0 || p.Timeout != 0 || n != 0 {
		t.Fatalf("explicit disable %+v %d %v", p, n, err)
	}
	if _, _, err = configuredBudgets(func(key string) string {
		if key == "UNREAL_HARNESS_OPERATION_TIMEOUT" {
			return "10s"
		}
		return ""
	}); err == nil {
		t.Fatal("warning after timeout accepted")
	}
}
