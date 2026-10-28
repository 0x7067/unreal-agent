package workerhost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

// Characterize the existing acknowledgment boundaries, not a future attach or
// crash-recovery protocol. Only the provider is fake: ingress, the worker process,
// coordinator, session files, and history replay use the production paths.
func TestRealWorkerSubmissionAcceptancePersistenceDeliveryAndReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	const text = "unique durable submission trace"
	input := inbox.Input{ID: "submission-lifecycle", Kind: inbox.InputExternal, Payload: []byte(`  "unique durable submission trace"  `)}
	conflict := input
	conflict.Payload = []byte(`"different input"`)

	var calls atomic.Int64
	var releaseOnce sync.Once
	release := make(chan struct{})
	type request struct {
		body []byte
		err  error
	}
	requests := make(chan request, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		body, err := io.ReadAll(r.Body)
		select {
		case requests <- request{body, err}:
		default:
		}
		select {
		case <-release:
			completed(w, id, assistant(id, "submission trace complete"))
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	// Release blocked handlers even if an assertion fails before normal release.
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	h, records, directory := realHost(t, server, false)
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	items := func() []sessionstore.Item {
		t.Helper()
		var result []sessionstore.Item
		after := sessionstore.BeforeFirst
		for {
			page, err := store.Items(ctx, h.config.Bootstrap.SessionID, after, 100)
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, page.Items...)
			if !page.More {
				return result
			}
			after = page.NextAfter
		}
	}
	assertHistory := func(wantInput, wantResponse int) {
		t.Helper()
		inputs, responses, turns := 0, 0, 0
		for _, item := range items() {
			if recorded, ok := item.Data.(inbox.Input); ok && recorded.ID == input.ID {
				inputs++
				if recorded.Kind != input.Kind || string(recorded.Payload) != `"unique durable submission trace"` {
					t.Fatalf("persisted input changed: %+v", recorded)
				}
			}
			if item.Kind == sessionstore.ItemModelResponse {
				responses++
			}
			if item.Kind == sessionstore.ItemTurn {
				turns++
			}
		}
		if inputs != wantInput || responses != wantResponse || turns != wantInput {
			t.Fatalf("history inputs/responses/turns = %d/%d/%d, want %d/%d/%d", inputs, responses, turns, wantInput, wantResponse, wantInput)
		}
	}

	// Hold the existing delivery fence, not a sleep: Submit can fsync ingress,
	// but the dispatcher cannot send this input to the worker yet.
	if err := h.lockDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			h.unlockDelivery()
		}
	}()
	// Host normalization can reuse the supplied raw-JSON buffer; provide a fresh
	// payload, as a wire retry would, rather than reusing a possibly changed slice.
	first := input
	first.Payload = input.Payload.Clone()
	if err := h.Submit(ctx, first); err != nil {
		t.Fatal(err)
	}
	// Host identity is wire-normalized JSON, unlike direct journal byte identity.
	canonical := input
	canonical.Payload = []byte(`"unique durable submission trace"`)
	if err := h.Submit(ctx, canonical); err != nil {
		t.Fatalf("equivalent retry before persistence: %v", err)
	}
	if err := h.Submit(ctx, conflict); !errors.Is(err, ingress.ErrConflict) {
		t.Fatalf("conflicting pending input = %v, want ErrConflict", err)
	}
	pending := h.journal.Pending()
	if len(pending) != 1 || pending[0].ID != input.ID || !bytes.Equal(pending[0].Payload, canonical.Payload) {
		t.Fatalf("accepted pending inputs = %+v, want one normalized input", pending)
	}
	assertHistory(0, 0)
	if got := calls.Load(); got != 0 {
		t.Fatalf("ingress acceptance invoked model %d times before delivery", got)
	}
	t.Log("ingress accepted once; session persistence and model delivery have not happened")
	h.unlockDelivery()
	locked = false

	select {
	case request := <-requests:
		if request.err != nil {
			t.Fatal(request.err)
		}
		if got := bytes.Count(request.body, []byte(`"`+text+`"`)); got != 1 {
			t.Fatalf("provider request contains input %d times, want once: %s", got, request.body)
		}
	case <-ctx.Done():
		t.Fatal("provider did not receive input:", context.Cause(ctx))
	}
	waitInput(t, records, string(input.ID))
	assertHistory(1, 0)
	if pending := h.journal.Pending(); len(pending) != 0 {
		t.Fatalf("persisted input not acknowledged in ingress: %+v", pending)
	}
	if err := h.Submit(ctx, canonical); err != nil {
		t.Fatalf("retry while model is active: %v", err)
	}
	t.Log("one session-persisted input reached the provider; no answer is committed yet")
	releaseOnce.Do(func() { close(release) })
	waitResponses(t, records, 1)
	assertHistory(1, 1)

	// A fresh host reopens ingress (including acknowledged identity tombstones),
	// and a fresh worker replays the same session rather than asking again.
	config := h.config
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	replayed := &recorder{changed: make(chan struct{}, 1)}
	config.OnItem, config.OnStatus = replayed.item, replayed.status
	resumed, err := Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer closeCancel()
		if err := resumed.Close(closeCtx); err != nil {
			t.Errorf("close resumed host: %v", err)
		}
	})
	waitInput(t, replayed, string(input.ID))
	waitResponses(t, replayed, 1)
	replayed.mu.Lock()
	replayItems := append([]sessionstore.Item(nil), replayed.items...)
	replayed.mu.Unlock()
	persisted := items()
	if len(replayItems) != len(persisted) {
		t.Fatalf("replayed %d items, want %d persisted items", len(replayItems), len(persisted))
	}
	for i, item := range replayItems {
		if !reflect.DeepEqual(item, persisted[i]) {
			t.Fatalf("replay item %d changed history content/order: %+v vs %+v", i, item, persisted[i])
		}
	}
	if err := resumed.Submit(ctx, canonical); err != nil {
		t.Fatalf("identical retry after reopening ingress: %v", err)
	}
	if err := resumed.Submit(ctx, conflict); !errors.Is(err, ingress.ErrConflict) {
		t.Fatalf("conflict after reopening ingress = %v, want ErrConflict", err)
	}
	if pending := resumed.journal.Pending(); len(pending) != 0 {
		t.Fatalf("acknowledged identity became pending after retry: %+v", pending)
	}
	// Reload is a causal worker/dispatcher barrier, not an arbitrary quiet sleep.
	// Replay and retry must not have started another model request.
	if err := resumed.Reload(ctx, nil); err != nil {
		t.Fatalf("idle replay/retry barrier: %v", err)
	}
	assertHistory(1, 1)
	if got := calls.Load(); got != 1 {
		t.Fatalf("submission/retry/replay invoked model %d times, want once", got)
	}
	t.Log("fresh-host replay preserved order and acknowledged identity; retries/conflicts made no new turn")
}
