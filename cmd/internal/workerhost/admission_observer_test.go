package workerhost

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
)

func TestAdmissionObserverRecoveryRetryAndPayloadOwnership(t *testing.T) {
	directory := t.TempDir()
	journal, err := ingress.Open(filepath.Join(directory, "ingress"), "fixture-session")
	if err != nil {
		t.Fatal(err)
	}
	seed := inbox.Input{ID: "recovered", Kind: inbox.InputExternal, Payload: []byte(`"recovered message"`)}
	if err := journal.Submit(t.Context(), seed); err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	observed := make(chan inbox.Input, 8)
	h, records, _ := testHostWith(t, directory, func(c *Config) {
		c.OnAdmission = func(in inbox.Input) {
			snapshot := in
			snapshot.Payload = in.Payload.Clone()
			observed <- snapshot
			in.Payload[0] = '!'
		}
	}, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	next := func(want inbox.ID) {
		t.Helper()
		select {
		case in := <-observed:
			if in.ID != want || in.Validate() != nil {
				t.Fatalf("bad confirmed admission: %+v", in)
			}
		case <-ctx.Done():
			t.Fatal("missing admission", want)
		}
	}
	next(seed.ID)
	waitInput(t, records, string(seed.ID))
	// Fence delivery to prove admission observation does not imply session input
	// persistence. The observer is complete before Submit returns.
	if err := h.lockDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			h.unlockDelivery()
		}
	}()
	input := inbox.Input{ID: "new-admission", Kind: inbox.InputExternal, Payload: []byte(`"new message"`)}
	if err := h.Submit(ctx, input); err != nil {
		t.Fatal(err)
	}
	next(input.ID)
	if err := h.Submit(ctx, input); err != nil {
		t.Fatal(err)
	}
	next(input.ID)
	pending := h.journal.Pending()
	if len(pending) != 1 || pending[0].ID != input.ID || string(pending[0].Payload) != `"new message"` {
		t.Fatalf("observer mutated canonical payload: %+v", pending)
	}
	h.unlockDelivery()
	locked = false
	waitInput(t, records, string(input.ID))
	input.Payload = []byte(`"conflicting message"`)
	if err := h.Submit(ctx, input); err == nil {
		t.Fatal("conflicting payload accepted")
	}
	select {
	case <-observed:
		t.Fatal("failed admission was published as confirmed")
	default:
	}
}
