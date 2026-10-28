package messagesapi

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	for _, hint := range []struct {
		value string
		unit  time.Duration
	}{{headers.Get("Retry-After-Ms"), time.Millisecond}, {headers.Get("Retry-After"), time.Second}} {
		if number, err := strconv.ParseFloat(strings.TrimSpace(hint.value), 64); err == nil && number > 0 && !math.IsInf(number, 0) {
			nanos := min(number*float64(hint.unit), float64(policy.MaxBackoff))
			return time.Duration(nanos)
		}
	}
	if deadline, err := http.ParseTime(headers.Get("Retry-After")); err == nil && deadline.After(now) {
		return min(deadline.Sub(now), policy.MaxBackoff)
	}
	delay := policy.Backoff(attempt)
	return delay - time.Duration(float64(delay/5)*jitter)
}
