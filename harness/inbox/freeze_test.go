package inbox_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func TestFreezeTransfersQueuedFIFOAndPreservesDeduplication(t *testing.T) {
	inputs := newInbox(t)
	for _, id := range []inbox.ID{"first", "second", "first", "third"} {
		if err := inputs.Submit(t.Context(), testInput(id)); err != nil {
			t.Fatal(err)
		}
	}
	queued, err := inputs.Freeze(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 3 || queued[0].ID != "first" || queued[1].ID != "second" || queued[2].ID != "third" {
		t.Fatalf("drained FIFO = %v", queued)
	}
	assertNoInput(t, inputs.Output())
	if again, err := inputs.Freeze(t.Context()); err != nil || len(again) != 0 {
		t.Fatalf("second Freeze = %v, %v", again, err)
	}
	if err := inputs.Thaw(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := inputs.Submit(t.Context(), testInput("first")); err != nil {
		t.Fatal(err)
	}
	if got := submitAndReceive(t, inputs, testInput("fourth")); got.ID != "fourth" {
		t.Fatalf("after thaw = %v", got)
	}
}

func TestFrozenInboxBlocksSubmitUntilThawOrCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inputs := newInbox(t)
		if _, err := inputs.Freeze(t.Context()); err != nil {
			t.Fatal(err)
		}
		canceledCtx, cancel := context.WithCancel(t.Context())
		canceled := make(chan error, 1)
		accepted := make(chan error, 1)
		go func() { canceled <- inputs.Submit(canceledCtx, testInput("canceled")) }()
		go func() { accepted <- inputs.Submit(t.Context(), testInput("accepted")) }()
		synctest.Wait()
		select {
		case err := <-accepted:
			t.Fatalf("Submit crossed fence: %v", err)
		default:
		}
		cancel()
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked Submit cancellation = %v", err)
		}
		if err := inputs.Thaw(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := <-accepted; err != nil {
			t.Fatal(err)
		}
		if got := receiveInput(t, inputs.Output()); got.ID != "accepted" {
			t.Fatalf("after thaw = %v", got)
		}
		if got := submitAndReceive(t, inputs, testInput("canceled")); got.ID != "canceled" {
			t.Fatal("canceled producer's input was marked seen")
		}
	})
}

func TestFreezeRacingSubmitNeverDropsAcceptedInputs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inputs := newInbox(t)
		const count = 30
		submitted := make(chan error, count)
		for i := range count {
			go func() { submitted <- inputs.Submit(t.Context(), testInput(inbox.ID(fmt.Sprint(i)))) }()
		}
		queued, err := inputs.Freeze(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := inputs.Thaw(t.Context()); err != nil {
			t.Fatal(err)
		}
		seen := make(map[inbox.ID]bool)
		for _, input := range queued {
			seen[input.ID] = true
		}
		for range count - len(queued) {
			input := receiveInput(t, inputs.Output())
			if seen[input.ID] {
				t.Fatalf("duplicate %q", input.ID)
			}
			seen[input.ID] = true
		}
		for range count {
			if err := <-submitted; err != nil {
				t.Fatal(err)
			}
		}
		if len(seen) != count {
			t.Fatalf("received %d inputs, want %d", len(seen), count)
		}
	})
}

func TestFreezeCancellationDoesNotFenceLiveInbox(t *testing.T) {
	inputs := newInbox(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := inputs.Freeze(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Freeze = %v", err)
	}
	if got := submitAndReceive(t, inputs, testInput("live")); got.ID != "live" {
		t.Fatal("canceled Freeze fenced inbox")
	}
}

func TestFrozenInboxShutdownReleasesProducersAndControls(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		inputs, err := inbox.New(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := inputs.Freeze(t.Context()); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- inputs.Submit(t.Context(), testInput("blocked")) }()
		synctest.Wait()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown Submit = %v", err)
		}
		if err := inputs.Thaw(t.Context()); !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown Thaw = %v", err)
		}
	})
}
