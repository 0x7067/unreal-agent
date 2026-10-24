package messagesapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func TestRetryDelay(t *testing.T) {
	policy := primitives.DefaultRemoteRequest("test", "test", "http://example.invalid").RetryPolicy
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		headers http.Header
		attempt int
		want    time.Duration
	}{
		{http.Header{"Retry-After": {"2.5"}}, 1, 2500 * time.Millisecond},
		{http.Header{"Retry-After-Ms": {"150"}, "Retry-After": {"5"}}, 1, 2 * time.Second},
		{http.Header{"Retry-After": {"120"}}, 1, 120 * time.Second},
		{http.Header{"Retry-After": {now.Add(10 * time.Second).Format(http.TimeFormat)}}, 1, 10 * time.Second},
		{http.Header{"Retry-After": {"invalid"}}, 1, 2 * time.Second},
		{http.Header{"Retry-After": {"NaN"}}, 1, 2 * time.Second},
		{http.Header{"Retry-After": {"+Inf"}}, 1, 2 * time.Second},
		{nil, 2, 4 * time.Second},
		{nil, 20, policy.MaxBackoff},
	} {
		if got := responseRetryDelay(policy, test.attempt, test.headers, now, 0); got != test.want {
			t.Fatalf("delay=%s want=%s headers=%v", got, test.want, test.headers)
		}
	}
}

func TestRetryHintFloors(t *testing.T) {
	policy := primitives.DefaultRemoteRequest("test", "test", "http://example.invalid").RetryPolicy
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name    string
		headers http.Header
		attempt int
		jitter  float64
		want    time.Duration
	}{
		{"long hint unjittered", http.Header{"Retry-After": {"120"}}, 100, 0.99, 120 * time.Second},
		{"short hint high attempt", http.Header{"Retry-After": {"1"}}, 100, 0, 30 * time.Second},
		{"jittered local wins", http.Header{"Retry-After": {"1"}}, 1, 0.5, 1800 * time.Millisecond},
		{"hint between jittered and base", http.Header{"Retry-After": {"1.9"}}, 1, 0.5, 1900 * time.Millisecond},
		{"ms precedence", http.Header{"Retry-After-Ms": {"3000"}, "Retry-After": {"120"}}, 1, 0.99, 3 * time.Second},
		{"invalid ms falls back", http.Header{"Retry-After-Ms": {"NaN"}, "Retry-After": {"120"}}, 1, 0.5, 120 * time.Second},
		{"long date", http.Header{"Retry-After": {now.Add(time.Hour).Format(http.TimeFormat)}}, 100, 0.99, time.Hour},
		{"huge seconds", http.Header{"Retry-After": {"1e300"}}, 100, 0.99, time.Duration(1<<63 - 1)},
		{"huge ms", http.Header{"Retry-After-Ms": {"1e300"}}, 100, 0.99, time.Duration(1<<63 - 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := responseRetryDelay(policy, test.attempt, test.headers, now, test.jitter); got != test.want {
				t.Fatalf("delay=%s want=%s", got, test.want)
			}
		})
	}
	for _, value := range []string{"NaN", "+Inf", "-Inf", "0", "-1", "-1e999", "garbage", now.Add(-time.Hour).Format(http.TimeFormat)} {
		if got := responseRetryDelay(policy, 1, http.Header{"Retry-After": {value}}, now, 0.5); got != 1800*time.Millisecond {
			t.Errorf("invalid hint %q: %s", value, got)
		}
	}
}
