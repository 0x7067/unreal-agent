package messagesapi

import (
	"net/http"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm/internal/retryhint"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func retryableResponseError(err *APIError, headers http.Header) bool {
	switch headers.Get("X-Should-Retry") {
	case "true":
		return true
	case "false":
		return false
	}
	if err.StatusCode >= 200 && err.StatusCode < 300 {
		switch err.Type {
		case "rate_limit_error", "overloaded_error", "api_error", "timeout_error":
			return true
		default:
			return false
		}
	}
	return err.StatusCode == 408 || err.StatusCode == 409 || err.StatusCode == 429 || err.StatusCode >= 500
}

func responseRetryDelay(policy primitives.RemoteRetryPolicy, attempt int, headers http.Header, now time.Time, jitter float64) time.Duration {
	delay := policy.Backoff(attempt)
	local := delay - time.Duration(float64(delay/5)*jitter)
	// Server hints are floors, not subject to local MaxBackoff or jitter.
	return max(local, retryhint.Headers(headers, now))
}
