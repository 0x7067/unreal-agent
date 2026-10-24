// Package retryhint parses server-provided retry floors without imposing a local
// backoff cap. Callers must keep waits cancelable via their request context.
package retryhint

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxDelay = time.Duration(1<<63 - 1)

// Number parses positive numeric seconds or milliseconds, saturating durations
// that cannot be represented. Literal NaN and infinities are not valid hints.
func Number(value string, unit time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	// Preserve exact whole units, including near the duration limit.
	if n, err := strconv.ParseUint(value, 10, 64); err == nil {
		if n > uint64(maxDelay/unit) {
			return maxDelay
		}
		return time.Duration(n) * unit
	}
	n, err := strconv.ParseFloat(value, 64)
	if errors.Is(err, strconv.ErrRange) && math.IsInf(n, 1) {
		// A syntactically valid finite decimal can exceed even float64's range.
		return maxDelay
	}
	if err != nil || n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	nanos := n * float64(unit)
	// float64(maxDelay) rounds up to 1<<63. Check before converting to avoid
	// wrapping a positive hint into a negative duration.
	if nanos >= float64(maxDelay) {
		return maxDelay
	}
	return time.Duration(nanos)
}

// Header accepts numeric seconds or an HTTP date. Time.Sub saturates far-future
// dates at maxDelay, and past dates do not request a wait.
func Header(value string, now time.Time) time.Duration {
	if delay := Number(value, time.Second); delay > 0 {
		return delay
	}
	if deadline, err := http.ParseTime(strings.TrimSpace(value)); err == nil {
		return max(0, deadline.Sub(now))
	}
	return 0
}

// Headers preserves provider precedence: a valid Retry-After-Ms overrides
// Retry-After; an invalid or zero milliseconds value falls back to Retry-After.
func Headers(headers http.Header, now time.Time) time.Duration {
	if delay := Number(headers.Get("Retry-After-Ms"), time.Millisecond); delay > 0 {
		return delay
	}
	return Header(headers.Get("Retry-After"), now)
}
