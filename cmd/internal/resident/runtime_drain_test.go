package resident

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

// Both branches use real resident+worker and a fake provider only at HTTP.
func TestRuntimeNaturalDrainCompletesOrCancelsOnlyUpgrade(t *testing.T) {
	for _, cancelUpgrade := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelUpgrade), func(t *testing.T) {
			entered, release, canceled := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var calls atomic.Int64
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				calls.Add(1)
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					close(canceled)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"drain-private\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"m\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"finished naturally\"}]}]}}\n\n")
			}))
			t.Cleanup(provider.Close)
			_, backend, state, _, _ := residentLifecycleFixture(t, provider, true)
			ctx, stop := context.WithTimeout(t.Context(), 20*time.Second)
			defer stop()
			c := lifecycleDial(t, ctx, backend, state)
			queued := make(chan attachproto.Event, 8)
			reader := make(chan struct{})
			go func() {
				defer close(reader)
				for {
					e, err := c.Next()
					if err != nil {
						return
					}
					if e.Runtime != nil && e.Runtime.State == "queued" {
						select {
						case queued <- e:
						default:
						}
					}
				}
			}()
			t.Cleanup(func() {
				_ = c.Close()
				select {
				case <-reader:
				case <-time.After(5 * time.Second):
					t.Error("reader did not join")
				}
			})
			gen := c.Snapshot().Header.Status.Generation
			if err := c.Submit(ctx, inbox.Input{ID: "private-natural-drain", Kind: inbox.InputExternal, Payload: []byte(`"finish naturally"`)}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("provider not started")
			}
			type result struct {
				s   attachproto.RuntimeStatus
				err error
			}
			done := make(chan result, 1)
			go func() {
				s, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "drain", ExpectedGeneration: gen})
				done <- result{s, err}
			}()
			var event attachproto.Event
			select {
			case event = <-queued:
			case <-ctx.Done():
				t.Fatal("upgrade never queued")
			}
			if cancelUpgrade {
				if err := c.CancelRuntime(ctx, event.ID); err != nil {
					t.Fatal(err)
				}
				var r result
				select {
				case r = <-done:
				case <-ctx.Done():
					t.Fatal("canceled request never settled")
				}
				if r.err == nil || r.s.State != "canceled" || r.s.Generation != gen {
					t.Fatalf("cancel falsely completed: %+v", r)
				}
				select {
				case <-canceled:
					t.Fatal("upgrade cancellation canceled model")
				default:
				}
				if calls.Load() != 1 {
					t.Fatal("model replayed")
				}
				close(release)
			} else {
				select {
				case r := <-done:
					t.Fatalf("replacement completed while provider held: %+v", r)
				default:
				}
				close(release)
				var r result
				select {
				case r = <-done:
				case <-ctx.Done():
					t.Fatal("natural drain never completed")
				}
				if r.err != nil || r.s.State != "completed" || r.s.Generation != gen+1 {
					t.Fatalf("natural drain: %+v", r)
				}
				if calls.Load() != 1 {
					t.Fatal("model replayed to upgrade")
				}
			}
		})
	}
}

// Compile-time API fence: transport still uses the concrete optional client,
// not a new mandatory shared operation/store interface.
var _ interface {
	CancelRuntime(context.Context, string) error
} = (*attachclient.Client)(nil)
