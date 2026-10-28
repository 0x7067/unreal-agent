package localfile

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
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
