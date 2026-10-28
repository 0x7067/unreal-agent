package workerhost

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// These adapters are deliberately test-only. They consume current typed Items;
// they do not implement an attach socket, an atomic full-agent snapshot, or a
// replacement for the production host's currently unbounded event mailbox.
const (
	lateJoinFrameLimit = 1 << 20
	lateJoinViewLimit  = 256 << 10
)

var (
	errLateJoinResync   = errors.New("late-join resync required")
	errLateJoinConflict = errors.New("conflicting duplicate item")
	errLateJoinCursor   = errors.New("invalid late-join cursor")
	errLateJoinFrame    = errors.New("late-join frame too large")
	errLateJoinBusy     = errors.New("late-join client already attached")
)

type lateJoinFrame struct {
	session session.ID
	data    []byte
}

type lateJoinBacklog struct {
	mu                        sync.Mutex
	frames                    []lateJoinFrame
	bytes, peakBytes, peakLen int
	maxFrames, maxBytes       int
	maxFrame                  int
	err                       error
}

func newLateJoinBacklog(frames, bytes int) *lateJoinBacklog {
	return &lateJoinBacklog{maxFrames: frames, maxBytes: bytes, maxFrame: lateJoinFrameLimit}
}

func (q *lateJoinBacklog) offer(frame lateJoinFrame) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return q.err
	}
	if len(frame.data) > q.maxFrame {
		q.err = errLateJoinFrame
	} else if len(q.frames) >= q.maxFrames || len(frame.data) > q.maxBytes-q.bytes {
		q.err = errLateJoinResync
	}
	if q.err != nil {
		q.frames, q.bytes = nil, 0
		return q.err
	}
	frame.data = bytes.Clone(frame.data)
	q.frames = append(q.frames, frame)
	q.bytes += len(frame.data)
	q.peakLen, q.peakBytes = max(q.peakLen, len(q.frames)), max(q.peakBytes, q.bytes)
	return nil
}

func (q *lateJoinBacklog) take() ([]lateJoinFrame, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return nil, q.err
	}
	frames := q.frames
	q.frames, q.bytes = nil, 0
	return frames, nil
}

// The callback never waits for a consumer: overflow invalidates only this test
// subscription. Its mutex serializes subscription changes, not session writes.
// Register BEFORE reading history; callbacks delayed until after that read are
// harmless exact duplicates, while later commits are already subscribed.
type lateJoinFeed struct {
	mu      sync.Mutex
	session session.ID
	active  *lateJoinBacklog
}

func (f *lateJoinFeed) attach(q *lateJoinBacklog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active != nil {
		return errLateJoinBusy
	}
	f.active = q
	return nil
}

func (f *lateJoinFeed) detach(q *lateJoinBacklog) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active == q {
		f.active = nil
	}
}

func (f *lateJoinFeed) item(item sessionstore.Item) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active == nil {
		return
	}
	data, err := json.Marshal(item)
	if err != nil {
		// Surface corruption to the test consumer without panicking on the
		// host callback goroutine and bypassing owned-process cleanup.
		f.active.mu.Lock()
		f.active.err = fmt.Errorf("encode late-join item: %w", err)
		f.active.frames, f.active.bytes = nil, 0
		f.active.mu.Unlock()
		f.active = nil
		return
	}
	if f.active.offer(lateJoinFrame{f.session, data}) != nil {
		f.active = nil
	}
}

type lateJoinView struct {
	session session.ID
	items   []sessionstore.Item
	encoded [][]byte
	bytes   int
}

func (v *lateJoinView) apply(frame lateJoinFrame) (bool, error) {
	if frame.session != v.session {
		return false, errLateJoinCursor
	}
	if len(frame.data) > lateJoinFrameLimit {
		return false, errLateJoinFrame
	}
	var item sessionstore.Item
	if err := json.Unmarshal(frame.data, &item); err != nil {
		return false, fmt.Errorf("decode late-join item: %w", err)
	}
	if item.Sequence == 0 || item.Sequence > sessionstore.Sequence(len(v.items)+1) {
		return false, errLateJoinResync
	}
	if item.Sequence <= sessionstore.Sequence(len(v.items)) {
		if !bytes.Equal(frame.data, v.encoded[item.Sequence-1]) {
			return false, errLateJoinConflict
		}
		return false, nil
	}
	if len(v.items) >= 64 || len(frame.data) > lateJoinViewLimit-v.bytes {
		return false, errLateJoinResync
	}
	v.items = append(v.items, item)
	v.encoded = append(v.encoded, bytes.Clone(frame.data))
	v.bytes += len(frame.data)
	return true, nil
}

func validateLateJoinCursor(sessionID, requested session.ID, after, head sessionstore.Sequence) error {
	if requested != sessionID || after > head {
		return errLateJoinCursor
	}
	return nil
}

func lateJoinEncode(t *testing.T, id session.ID, item sessionstore.Item) lateJoinFrame {
	t.Helper()
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	return lateJoinFrame{id, data}
}

// Small pages exercise pagination, not a bounded-storage implementation: current
// localfile.Items reconstructs the entire log for each call. The fixture's view
// itself rejects >64 Items or >256 KiB instead of retaining unbounded history.
func lateJoinSnapshot(t *testing.T, ctx context.Context, store *localfile.Store, id session.ID) *lateJoinView {
	t.Helper()
	return lateJoinSnapshotAtBoundary(t, ctx, store, id, nil)
}

func lateJoinSnapshotAtBoundary(t *testing.T, ctx context.Context, store *localfile.Store, id session.ID, afterRead func()) *lateJoinView {
	t.Helper()
	view := &lateJoinView{session: id}
	after := sessionstore.BeforeFirst
	for {
		page, err := store.Items(ctx, id, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if added, err := view.apply(lateJoinEncode(t, id, item)); err != nil || !added {
				t.Fatalf("snapshot item %d: added=%t, err=%v", item.Sequence, added, err)
			}
		}
		if !page.More {
			// Deliberately commit before returning control to any caller that
			// might try the unsafe read-then-register ordering.
			if afterRead != nil {
				afterRead()
			}
			return view
		}
		after = page.NextAfter
	}
}

func lateJoinApplyBacklog(t *testing.T, view *lateJoinView, q *lateJoinBacklog) {
	t.Helper()
	frames, err := q.take()
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range frames {
		if _, err := view.apply(frame); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	var calls atomic.Int64
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		id := calls.Add(1)
		if id == 1 {
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		completed(w, id, assistant(id, fmt.Sprintf("late-join answer %d", id)))
	}))
	t.Cleanup(server.Close)
	feed := &lateJoinFeed{}
	h, records, directory := realHostWith(t, server, false, func(config *Config) {
		feed.session = config.Bootstrap.SessionID
		original := config.OnItem
		config.OnItem = func(item sessionstore.Item) { feed.item(item); original(item) }
	})
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	originalWorker := h.current
	h.mu.Unlock()
	submit := func(number int) {
		t.Helper()
		if err := h.Submit(ctx, inbox.Input{ID: inbox.ID(fmt.Sprintf("late-%d", number)), Kind: inbox.InputExternal, Payload: []byte(fmt.Sprintf(`"late-join input %d"`, number))}); err != nil {
			t.Fatal(err)
		}
	}
	submit(1)
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("provider did not start:", ctx.Err())
	}
	waitInput(t, records, "late-1")
	q := newLateJoinBacklog(16, 64<<10)
	if err := feed.attach(q); err != nil {
		t.Fatal(err)
	}
	if err := feed.attach(newLateJoinBacklog(1, 1024)); !errors.Is(err, errLateJoinBusy) {
		t.Fatalf("second client = %v", err)
	}
	view := lateJoinSnapshotAtBoundary(t, ctx, store, feed.session, func() {
		// Commit AFTER reading the prefix but BEFORE the snapshot call
		// returns. Registering only after that return would miss the answer.
		releaseOnce.Do(func() { close(release) })
		waitResponses(t, records, 1)
	})
	head := sessionstore.Sequence(len(view.items))
	if head != 4 || view.items[0].Kind != sessionstore.ItemInput || view.items[1].Kind != sessionstore.ItemInput || view.items[2].Kind != sessionstore.ItemInput || view.items[3].Kind != sessionstore.ItemTurn {
		t.Fatalf("expected H=4 settings/input/request-charge/turn prefix, got %+v", view.items)
	}
	settings, ok := view.items[0].Data.(inbox.Input)
	if !ok || settings.Kind != inbox.InputControl {
		t.Fatalf("first prefix item is not initial settings: %+v", view.items[0])
	}
	control, err := settings.DecodeControlMessage()
	if err != nil || control.Mode != inbox.UpdateSettings {
		t.Fatalf("first prefix item is not settings: %+v %v", control, err)
	}
	if input, ok := view.items[1].Data.(inbox.Input); !ok || input.ID != "late-1" || input.Kind != inbox.InputExternal {
		t.Fatalf("snapshot input identity changed: %+v", view.items[1])
	}
	charge, ok := view.items[2].Data.(inbox.Input)
	if !ok {
		t.Fatal("late join omitted request charge")
	}
	charged, err := charge.DecodeControlMessage()
	if err != nil || charged.Mode != inbox.ModelRequest {
		t.Fatalf("invalid request charge %+v %v", charged, err)
	}
	request := charged.Parameters.(inbox.ModelRequestRecord)
	turn, ok := view.items[3].Data.(session.Turn)
	if !ok || request.Version != 1 || request.Action != "started" || request.Attempt != 1 || request.Limit != 2 || request.Inputs != 1 || request.ExternalInputs != 1 || request.RequestID == "" || request.TurnID != string(turn.ID) || request.TurnType != string(turn.Type) {
		t.Fatalf("invalid charge/turn snapshot: %+v %+v", request, turn)
	}
	canonical := lateJoinSnapshot(t, ctx, store, feed.session)
	if len(canonical.items) != len(view.items)+1 || canonical.items[len(view.items)].Kind != sessionstore.ItemModelResponse {
		t.Fatal("controlled boundary did not contain exactly the committed answer")
	}
	lateJoinApplyBacklog(t, view, q)
	if !reflect.DeepEqual(view.items, canonical.items) {
		t.Fatal("snapshot/watch reconstruction lost or duplicated boundary commit")
	}
	duplicate := lateJoinEncode(t, feed.session, canonical.items[len(canonical.items)-1])
	if err := q.offer(duplicate); err != nil {
		t.Fatal(err)
	}
	lateJoinApplyBacklog(t, view, q)
	if !reflect.DeepEqual(view.items, canonical.items) {
		t.Fatal("duplicate update changed reconstructed history")
	}
	t.Logf("snapshot head=%d; answer committed before watch consumption; exact history and duplicate convergence", head)

	// No client exists while the next answer commits. This only detaches the
	// TEST subscription, not a production socket or frontend process.
	feed.detach(q)
	submit(2)
	waitResponses(t, records, 2)
	q = newLateJoinBacklog(16, 64<<10)
	if err := feed.attach(q); err != nil {
		t.Fatal(err)
	}
	view = lateJoinSnapshot(t, ctx, store, feed.session)
	lateJoinApplyBacklog(t, view, q)
	canonical = lateJoinSnapshot(t, ctx, store, feed.session)
	if !reflect.DeepEqual(view.items, canonical.items) {
		t.Fatal("late reconnect did not recover commits made without a subscriber")
	}
	feed.detach(q)

	// Deliberately never consume this one-frame subscription. More commits must
	// invalidate the client, not block host callbacks, ingress or model progress.
	slow := newLateJoinBacklog(1, 64<<10)
	if err := feed.attach(slow); err != nil {
		t.Fatal(err)
	}
	submit(3)
	waitResponses(t, records, 3)
	if _, err := slow.take(); !errors.Is(err, errLateJoinResync) {
		t.Fatalf("overflow must force resync, not return an incomplete success: %v", err)
	}
	slow.mu.Lock()
	bounded := slow.peakLen == 1 && slow.peakBytes <= slow.maxBytes && len(slow.frames) == 0 && slow.bytes == 0
	slow.mu.Unlock()
	if !bounded {
		t.Fatal("stalled client exceeded bounds or retained invalid backlog")
	}
	q = newLateJoinBacklog(16, 64<<10)
	if err := feed.attach(q); err != nil {
		t.Fatalf("overflow did not release only the client slot: %v", err)
	}
	view = lateJoinSnapshot(t, ctx, store, feed.session)
	lateJoinApplyBacklog(t, view, q)
	canonical = lateJoinSnapshot(t, ctx, store, feed.session)
	if !reflect.DeepEqual(view.items, canonical.items) {
		t.Fatal("fresh snapshot after overflow did not converge")
	}
	inputs, responses := map[inbox.ID]int{}, 0
	for _, item := range view.items {
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputExternal {
			inputs[input.ID]++
		}
		if item.Kind == sessionstore.ItemModelResponse {
			responses++
		}
	}
	if len(inputs) != 3 || inputs["late-1"] != 1 || inputs["late-2"] != 1 || inputs["late-3"] != 1 || responses != 3 {
		t.Fatalf("reconstruction input identities/responses=%v/%d, want each late-1/2/3 once and 3 answers", inputs, responses)
	}
	_, generation, err := h.queryActivity(ctx)
	h.mu.Lock()
	sameWorker := h.current == originalWorker
	h.mu.Unlock()
	if err != nil || !sameWorker || generation != originalWorker.generation {
		t.Fatalf("test subscription lifecycle changed/unresponsive worker: same=%t generation=%d err=%v", sameWorker, generation, err)
	}
	feed.detach(q)
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("attach/replay repeated provider work: %d calls, want 3", calls.Load())
	}
	t.Log("no-subscriber commit and overflow recovered from disk; same worker stayed responsive; three inputs/answers/provider calls")
}

func TestLateJoinDriverRejectsGapsConflictsAndBounds(t *testing.T) {
	item := sessionstore.Item{Sequence: 1, Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "input", Kind: inbox.InputExternal, Payload: []byte(`"original"`)}}
	frame := lateJoinEncode(t, "session", item)
	view := &lateJoinView{session: "session"}
	if added, err := view.apply(frame); err != nil || !added {
		t.Fatal(added, err)
	}
	if added, err := view.apply(frame); err != nil || added {
		t.Fatal("exact duplicate was not a no-op:", added, err)
	}
	item.Data = inbox.Input{ID: "input", Kind: inbox.InputExternal, Payload: []byte(`"conflicting"`)}
	if _, err := view.apply(lateJoinEncode(t, "session", item)); !errors.Is(err, errLateJoinConflict) {
		t.Fatal("conflicting duplicate:", err)
	}
	item.Sequence = 3
	if _, err := view.apply(lateJoinEncode(t, "session", item)); !errors.Is(err, errLateJoinResync) {
		t.Fatal("forward gap:", err)
	}
	if _, err := view.apply(lateJoinFrame{"wrong-session", frame.data}); !errors.Is(err, errLateJoinCursor) {
		t.Fatal("wrong session:", err)
	}
	if len(view.items) != 1 || !bytes.Equal(view.encoded[0], frame.data) {
		t.Fatal("rejected event mutated accepted prefix")
	}
	for _, tc := range []struct {
		name string
		q    *lateJoinBacklog
		want error
	}{
		{"frame-count", newLateJoinBacklog(2, 3*len(frame.data)), errLateJoinResync},
		{"byte-count", newLateJoinBacklog(3, 2*len(frame.data)), errLateJoinResync},
		{"oversize-frame", &lateJoinBacklog{maxFrames: 3, maxBytes: 3 * len(frame.data), maxFrame: len(frame.data) - 1}, errLateJoinFrame},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 3 {
				_ = tc.q.offer(frame)
			}
			if _, err := tc.q.take(); !errors.Is(err, tc.want) || tc.q.peakLen > tc.q.maxFrames || tc.q.peakBytes > tc.q.maxBytes || tc.q.frames != nil || tc.q.bytes != 0 {
				t.Fatalf("unbounded or silently incomplete backlog: %+v err=%v", tc.q, err)
			}
		})
	}
	t.Run("view-item-count", func(t *testing.T) {
		limited := &lateJoinView{session: "session"}
		for index := 1; index <= 64; index++ {
			current := item
			current.Sequence = sessionstore.Sequence(index)
			current.Data = inbox.Input{ID: inbox.ID(fmt.Sprintf("input-%d", index)), Kind: inbox.InputExternal, Payload: []byte(`"bounded"`)}
			if added, err := limited.apply(lateJoinEncode(t, "session", current)); err != nil || !added {
				t.Fatal(added, err)
			}
		}
		item := item
		item.Sequence = 65
		if _, err := limited.apply(lateJoinEncode(t, "session", item)); !errors.Is(err, errLateJoinResync) || len(limited.items) != 64 {
			t.Fatal("view item count exceeded bound:", err)
		}
	})
	// The view, not just its transport queue, has an explicit fixture bound.
	large := sessionstore.Item{Sequence: 2, Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "large", Kind: inbox.InputExternal, Payload: append(append([]byte{'"'}, bytes.Repeat([]byte{'x'}, lateJoinViewLimit)...), '"')}}
	if _, err := view.apply(lateJoinEncode(t, "session", large)); !errors.Is(err, errLateJoinResync) || len(view.items) != 1 {
		t.Fatal("view byte bound did not preserve accepted prefix:", err)
	}
}

func TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint(t *testing.T) {
	directory := t.TempDir()
	const id session.ID = "watch-gap"
	lease, err := localfile.AcquireLease(directory, id)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	notifications := 0
	observer := store.AddObserver(func(session.ID, sessionstore.Item) { notifications++ })
	defer store.RemoveObserver(observer)
	spec, err := operation.NewShellSpec(operation.ShellInput{Command: "true", Shell: "/bin/sh"}, filepath.Join(directory, "artifacts"), operation.DefaultMaxOutputLength)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "operation", Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, State: spec.State.Clone(), MaxOutputLength: spec.MaxOutputLength}
	if err := store.AppendTurn(t.Context(), id, session.Turn{ID: "turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendModelResponse(t.Context(), id, sessionstore.ModelResponse{TurnID: "turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "Bash", Arguments: `{"command":"true"}`}}}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendToolCallStatus(t.Context(), id, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
		t.Fatal(err)
	}
	before := lateJoinSnapshot(t, t.Context(), store, id)
	if notifications != len(before.items) {
		t.Fatal("history observer did not observe committed Items")
	}
	state, err := operation.DecodeShellState(op)
	if err != nil {
		t.Fatal(err)
	}
	state.Phase = operation.ShellPhaseCreateDirectory
	op.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	op.Status = operation.StatusAwaiting
	if err := store.SaveOperation(t.Context(), id, op); err != nil {
		t.Fatal(err)
	}
	fresh, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	after := lateJoinSnapshot(t, t.Context(), fresh, id)
	restored, err := fresh.Resume(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.items, after.items) || notifications != len(before.items) || len(restored.Operations) != 1 || !reflect.DeepEqual(restored.Operations[0], op) {
		t.Fatal("expected operation-only commit without Item/history notification was not observed")
	}
	head := sessionstore.Sequence(len(after.items))
	page, err := fresh.Items(t.Context(), id, head+1, 1)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("existing ahead cursor behavior changed: %+v %v", page, err)
	}
	if err := validateLateJoinCursor(id, id, head+1, head); !errors.Is(err, errLateJoinCursor) {
		t.Fatal("attach driver silently accepted ahead cursor:", err)
	}
	if err := validateLateJoinCursor(id, "wrong-session", 0, head); !errors.Is(err, errLateJoinCursor) {
		t.Fatal("attach driver accepted different session:", err)
	}
	if err := validateLateJoinCursor(id, id, head, head); err != nil {
		t.Fatal(err)
	}
	t.Log("fresh store exposes newer operation while Item history/watch stay unchanged; ahead cursor needs explicit rejection")
}
