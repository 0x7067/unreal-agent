package workerhost

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func TestEventMailboxDrainsBeforeTerminalError(t *testing.T) {
	q := newEventMailbox()
	terminal := errors.New("injected terminal error")
	// Exceed ordinary bounded-channel capacities without any consumer running.
	for i := range 10000 {
		q.push(workerproto.Event{Type: "item", Item: &sessionstore.Item{Sequence: sessionstore.Sequence(i + 1)}})
	}
	q.finish(terminal)
	for i := range 10000 {
		event, err, ok := q.next()
		if !ok || err != nil || event.Item.Sequence != sessionstore.Sequence(i+1) {
			t.Fatalf("event %d: %#v %v %v", i, event, err, ok)
		}
	}
	if _, err, ok := q.next(); ok || !errors.Is(err, terminal) {
		t.Fatalf("terminal result: %v %v", err, ok)
	}
	if q.queue != nil {
		t.Fatal("drained queue retained frames")
	}
}

func callbackGate(id inbox.ID, entered, release chan struct{}) func(*Config) {
	return func(config *Config) {
		original := config.OnItem
		config.OnItem = func(item sessionstore.Item) {
			if input, ok := item.Data.(inbox.Input); ok && input.ID == id {
				close(entered)
				<-release
			}
			original(item)
		}
	}
}
func awaitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal(what)
	}
}
func releaseGate(t *testing.T, release chan struct{}) func() {
	t.Helper()
	var once sync.Once
	open := func() { once.Do(func() { close(release) }) }
	t.Cleanup(open)
	return open
}

func TestSlowCallbackDoesNotBlockPipeOrLaterModel(t *testing.T) {
	var calls atomic.Int64
	firstRequest, second := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		text := "second response"
		if id == 1 {
			close(firstRequest)
			// Larger than any ordinary pipe buffer, but below MaxFrame. The
			// synchronous worker store observer must finish writing this item
			// before the coordinator can process the next user input.
			text = strings.Repeat("large assistant output ", 100000)
		} else if id == 2 {
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "second input") {
				t.Error("second model request omitted the later input")
			}
			close(second)
		}
		completed(w, id, assistant(id, text))
	}))
	t.Cleanup(server.Close)
	entered, release := make(chan struct{}), make(chan struct{})
	h, records, directory := realHostWith(t, server, false, func(config *Config) {
		callbackGate("slow-first", entered, release)(config)
		config.Bootstrap.Model.CompactionThreshold = 10000000
	})
	open := releaseGate(t, release)
	send(t, h, "slow-first", "first input")
	awaitSignal(t, entered, "first callback not entered")
	awaitSignal(t, firstRequest, "first model request not started")
	// A new external input can interrupt a live model turn. Wait for durable
	// response persistence (not its gated callback) so this test really forces
	// the huge stdout frame through the synchronous observer before later work.
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		page, err := store.Items(t.Context(), h.config.Bootstrap.SessionID, sessionstore.BeforeFirst, 100)
		persisted := false
		if err == nil {
			for _, item := range page.Items {
				persisted = persisted || item.Kind == sessionstore.ItemModelResponse
			}
		}
		if persisted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("large model response not persisted")
		case <-time.After(time.Millisecond):
		}
	}
	send(t, h, "slow-second", "second input")
	awaitSignal(t, second, "slow callback backpressured worker/model progress")
	open()
	waitInput(t, records, "slow-second")
	waitResponses(t, records, 2)
	records.mu.Lock()
	for i, item := range records.items {
		if item.Sequence != sessionstore.Sequence(i+1) {
			t.Errorf("callback sequence %d at index %d", item.Sequence, i)
		}
	}
	records.mu.Unlock()
}

func TestExitDrainsFinalFramesAfterSlowCallback(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	h, records, cfg := testHostWith(t, "", callbackGate("slow-first", entered, release), nil)
	open := releaseGate(t, release)
	send(t, h, "slow-first", "first")
	awaitSignal(t, entered, "first callback not entered")
	send(t, h, "slow-final", "final")
	store, err := localfile.New(cfg.Bootstrap.Directory)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		page, err := store.Items(t.Context(), cfg.Bootstrap.SessionID, sessionstore.BeforeFirst, 100)
		if err == nil && len(page.Items) == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("final input did not persist while callback blocked")
		case <-time.After(time.Millisecond):
		}
	}
	h.mu.Lock()
	p := h.current
	h.mu.Unlock()
	p.expectedStop.Store(true)
	_ = p.input.Close()
	// scanDone is decoder-owned: the callback is still blocked, yet all final
	// output must reach EOF independently of ordered delivery.
	awaitSignal(t, p.scanDone, "decoder waited for callback")
	for syscall.Kill(p.command.Process.Pid, 0) == nil {
		select {
		case <-deadline:
			t.Fatal("worker did not exit")
		case <-time.After(time.Millisecond):
		}
	}
	time.Sleep(1500 * time.Millisecond) // Regress the former post-Wait close timer.
	open()
	awaitSignal(t, p.done, "worker delivery did not finish")
	if err := p.result(); err != nil {
		t.Fatal(err)
	}
	waitInput(t, records, "slow-final")
	records.mu.Lock()
	defer records.mu.Unlock()
	if len(records.items) != 2 || records.items[0].Sequence != 1 || records.items[1].Sequence != 2 {
		t.Fatalf("lost/reordered final frames: %#v", records.items)
	}
}

func TestCloseTimeoutThenRetryJoinsCallbacks(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	h, records, cfg := testHostWith(t, "", callbackGate("slow-close", entered, release), nil)
	open := releaseGate(t, release)
	send(t, h, "slow-close", "first")
	awaitSignal(t, entered, "first callback not entered")
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := h.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close while callback blocked: %v", err)
	}
	if lease, err := localfile.AcquireLease(cfg.Bootstrap.Directory, session.ID("host-"+string(cfg.Bootstrap.SessionID))); !errors.Is(err, localfile.ErrSessionLocked) {
		if lease != nil {
			_ = lease.Close()
		}
		t.Fatalf("timed-out Close released frontend ownership: %v", err)
	}
	open()
	if err := h.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	records.mu.Lock()
	items, statuses := len(records.items), len(records.statuses)
	records.mu.Unlock()
	if err := h.Submit(t.Context(), inbox.Input{ID: "after-close", Kind: inbox.InputExternal}); !errors.Is(err, ErrClosed) {
		t.Fatalf("post-close admission: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	records.mu.Lock()
	defer records.mu.Unlock()
	if len(records.items) != items || len(records.statuses) != statuses {
		t.Fatal("callback after successful Close")
	}
}

// gatedJournal models an accepted mutation stalled in the durable-I/O phase.
// Host must not hold its lifecycle mutex or close this owner while it is active.
type gatedJournal struct {
	admissionJournal
	entered chan struct{}
	release chan struct{}
}

func (j *gatedJournal) Submit(ctx context.Context, input inbox.Input) error {
	close(j.entered)
	<-j.release
	return j.admissionJournal.Submit(ctx, input)
}

func TestSlowDurableAdmissionDoesNotBlockShutdownCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	h, _, cfg := testHostWith(t, "", nil, func(j admissionJournal) admissionJournal {
		return &gatedJournal{admissionJournal: j, entered: entered, release: release}
	})
	open := releaseGate(t, release)
	_, pid := generation(h)
	submitted := make(chan error, 1)
	go func() {
		submitted <- h.Submit(context.Background(), inbox.Input{ID: "slow-durable", Kind: inbox.InputExternal, Payload: []byte(`"durable"`)})
	}()
	awaitSignal(t, entered, "admission did not reach durable phase")
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	closed := make(chan error, 1)
	go func() { closed <- h.Close(ctx) }()
	select {
	case err := <-closed:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Close during durable I/O: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("durable Submit held lifecycle mutex past Close deadline")
	}
	deadline := time.After(3 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		select {
		case <-deadline:
			t.Fatal("slow durable Submit prevented worker cancellation/join")
		case <-time.After(time.Millisecond):
		}
	}
	if err := h.Submit(t.Context(), inbox.Input{ID: "rejected", Kind: inbox.InputExternal}); !errors.Is(err, ErrClosed) {
		t.Fatalf("post-closing admission: %v", err)
	}
	if lease, err := localfile.AcquireLease(cfg.Bootstrap.Directory, session.ID("host-"+string(cfg.Bootstrap.SessionID))); !errors.Is(err, localfile.ErrSessionLocked) {
		if lease != nil {
			_ = lease.Close()
		}
		t.Fatalf("slow admission lost ownership: %v", err)
	}
	open()
	if err := <-submitted; err != nil {
		t.Fatalf("accepted admission lost durable outcome: %v", err)
	}
	if pending := h.journal.Pending(); len(pending) != 1 || pending[0].ID != "slow-durable" {
		t.Fatalf("accepted input not retained: %#v", pending)
	}
	if err := h.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}
