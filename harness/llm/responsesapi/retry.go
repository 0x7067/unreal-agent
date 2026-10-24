package responsesapi

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm/internal/retryhint"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

var retryAfterMessagePattern = regexp.MustCompile(`(?i)\btry again in\s*(\d+(?:\.\d+)?)\s*(ms|milliseconds?|s|seconds?)\b`)

// Providers report failures inconsistently, so this classifier intentionally
// fails open, favoring retries.
func retryableResponseError(err *APIError, retryableStatuses []int) bool {
	if err == nil {
		return false
	}
	switch err.Code {
	case "context_length_exceeded", "insufficient_quota", "usage_not_included", "usage_limit_reached",
		"credit_balance_exhausted", "billing_hard_limit_reached",
		"cyber_policy", "misalignment_policy_violation", "invalid_prompt", "bio_policy",
		"invalid_api_key", "invalid_token":
		return false
	}
	switch err.Type {
	case "authentication_error", "permission_error", "insufficient_quota":
		return false
	}
	if err.StatusCode < http.StatusOK || err.StatusCode >= http.StatusMultipleChoices {
		return slices.Contains(retryableStatuses, err.StatusCode)
	}
	return true
}

func responseRetryDelay(policy primitives.RemoteRetryPolicy, attempt int, err *APIError, headers http.Header, now time.Time, jitter float64) time.Duration {
	hint := retryhint.Headers(headers, now)
	if err != nil && err.Code == "rate_limit_exceeded" {
		hint = max(hint, retryAfterMessage(err.Message))
	}
	if err != nil && (err.Code == "server_is_overloaded" || err.Code == "slow_down") {
		// Unlike the Codex CLI, unattended runs retry overloads with a longer backoff.
		policy.InitialBackoff = 10 * time.Second
		policy.MaxBackoff = time.Minute
	}
	delay := policy.Backoff(attempt)
	// Apply jitter only to local backoff; even long server floors remain cancelable.
	return max(hint, delay-time.Duration(float64(delay/5)*jitter))
}

func retryAfterHeader(value string, now time.Time) time.Duration {
	return retryhint.Header(value, now)
}

func retryAfterMessage(message string) time.Duration {
	match := retryAfterMessagePattern.FindStringSubmatch(message)
	if match == nil {
		return 0
	}
	unit := time.Second
	if strings.HasPrefix(strings.ToLower(match[2]), "m") {
		unit = time.Millisecond
	}
	return retryhint.Number(match[1], unit)
}
