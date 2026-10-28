package localfile

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestReadOnlyLifecycleUsesCanonicalCodecAndNeverCreatesState(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), "s"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []struct {
		id         inbox.ID
		mode       inbox.ControlMode
		parameters any
	}{
		{"stop", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: "host/1"}},
		{"complete", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "stop", Scope: "host/1", Phase: "completed"}},
	} {
		payload, err := json.Marshal(inbox.ControlMessage{Mode: value.mode, Parameters: value.parameters})
		if err != nil {
			t.Fatal(err)
		}
		if err = store.AppendInput(t.Context(), "s", inbox.Input{ID: value.id, Kind: inbox.InputControl, Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(store.sessionPath("s"))
	if err != nil {
		t.Fatal(err)
	}
	history, err := ReadLifecycle(t.Context(), bytes.NewReader(data), "s")
	if err != nil {
		t.Fatal(err)
	}
	if history.ShutdownResults["stop"].Phase != "completed" {
		t.Fatal("canonical outcome missing")
	}
	if _, err = ReadLifecycle(t.Context(), bytes.NewReader(data), "other"); err == nil {
		t.Fatal("wrong identity accepted")
	}
	torn := append(bytes.Clone(data), []byte(`{"type":"item","data":`)...)
	history, err = ReadLifecycle(t.Context(), bytes.NewReader(torn), "s")
	if err != nil || history.ShutdownResults["stop"].Phase != "completed" {
		t.Fatalf("committed tail handling differs from store: %v", err)
	}
	future := bytes.Replace(data, []byte(`"Version":2`), []byte(`"Version":99`), 1)
	if _, err = ReadLifecycle(t.Context(), bytes.NewReader(future), "s"); err == nil {
		t.Fatal("future header accepted")
	}
	after, err := os.ReadFile(store.sessionPath("s"))
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("read-only query changed canonical bytes")
	}
}

func TestReadOnlyLifecycleItemsPagingAndCancellation(t *testing.T) {
	items := make([]sessionstore.Item, 513)
	for i := range items {
		items[i] = sessionstore.Item{Sequence: sessionstore.Sequence(i + 1)}
	}
	source := readOnlyItems{items: items}
	first, err := source.Items(t.Context(), "s", sessionstore.BeforeFirst, 512)
	if err != nil || len(first.Items) != 512 || first.NextAfter != 512 || !first.More {
		t.Fatalf("unexpected first page: items=%d after=%d more=%v err=%v", len(first.Items), first.NextAfter, first.More, err)
	}
	last, err := source.Items(t.Context(), "s", first.NextAfter, 512)
	if err != nil || len(last.Items) != 1 || last.Items[0].Sequence != 513 || last.NextAfter != 513 || last.More {
		t.Fatalf("unexpected last page: %+v, err=%v", last, err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("diagnostic canceled")
	cancel(cause)
	if _, err := source.Items(ctx, "s", sessionstore.BeforeFirst, 512); !errors.Is(err, cause) {
		t.Fatalf("Items lost cancellation cause: %v", err)
	}
}

type lifecycleReaderFunc func([]byte) (int, error)

func (f lifecycleReaderFunc) Read(p []byte) (int, error) { return f(p) }

func TestReadOnlyLifecycleCancellationAndReaderErrors(t *testing.T) {
	cause := errors.New("diagnostic canceled")
	t.Run("before reading", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(cause)
		reader := lifecycleReaderFunc(func([]byte) (int, error) {
			t.Fatal("read after cancellation")
			return 0, io.EOF
		})
		if _, err := ReadLifecycle(ctx, reader, "s"); !errors.Is(err, cause) {
			t.Fatalf("lost cancellation cause: %v", err)
		}
	})
	t.Run("after reading before decoding", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		reader := lifecycleReaderFunc(func([]byte) (int, error) {
			cancel(cause)
			return 0, io.EOF
		})
		if _, err := ReadLifecycle(ctx, reader, "s"); !errors.Is(err, cause) {
			t.Fatalf("decoded after cancellation: %v", err)
		}
	})
	t.Run("reader failure", func(t *testing.T) {
		readErr := errors.New("reader failed")
		reader := lifecycleReaderFunc(func([]byte) (int, error) { return 0, readErr })
		if _, err := ReadLifecycle(t.Context(), reader, "s"); !errors.Is(err, readErr) {
			t.Fatalf("lost reader error: %v", err)
		}
	})
}
