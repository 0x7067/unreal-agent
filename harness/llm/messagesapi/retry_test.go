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
		{http.Header{"Retry-After-Ms": {"150"}, "Retry-After": {"5"}}, 1, 150 * time.Millisecond},
		{http.Header{"Retry-After": {"120"}}, 1, policy.MaxBackoff},
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
