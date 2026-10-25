package inbox_test

// Adversarial coverage for the Freeze/Thaw admission fence used by the worker
// handoff (harness/inbox/local.go:61-158). The invariant under test: across any
// number of freezes, every accepted input is transferred exactly once -- either in
// the frozen batch or on Output, never both, never neither -- and deduplication by
// input ID survives the fence.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

// TestRepeatedFreezeThawUnderConcurrentAdmissionDeliversExactlyOnce races a stream
// of unique admissions against repeated Freeze/Thaw cycles and a live consumer.
func TestRepeatedFreezeThawUnderConcurrentAdmissionDeliversExactlyOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	inputs, err := inbox.New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	const producers = 4
	const perProducer = 50
	const cycles = 25
	total := producers * perProducer

	var mu sync.Mutex
	delivered := make(map[inbox.ID]int)
	record := func(id inbox.ID) {
		mu.Lock()
		delivered[id]++
		mu.Unlock()
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered)
	}

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for input := range inputs.Output() {
			record(input.ID)
		}
	}()

	var producersWG sync.WaitGroup
	for producer := range producers {
		producersWG.Add(1)
		go func() {
			defer producersWG.Done()
			for index := range perProducer {
				id := inbox.ID(fmt.Sprintf("p%d-i%d", producer, index))
				if err := inputs.Submit(ctx, testInput(id)); err != nil {
					t.Errorf("submit %s: %v", id, err)
					return
				}
			}
		}()
	}

	// Freeze/Thaw cycles run concurrently with admission; each frozen batch is the
	// caller's responsibility, exactly as the coordinator's quiescence path treats it.
	for range cycles {
		queued, err := inputs.Freeze(ctx)
		if err != nil {
			t.Fatalf("freeze: %v", err)
		}
		for _, input := range queued {
			record(input.ID)
		}
		if err := inputs.Thaw(ctx); err != nil {
			t.Fatalf("thaw: %v", err)
		}
	}
	producersWG.Wait()

	deadline := time.Now().Add(15 * time.Second)
	for count() < total {
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d inputs crossed the fence", count(), total)
		}
		time.Sleep(time.Millisecond)
	}

	// Fence further output and collect any queued duplicate before taking the
	// final snapshot. Joining the consumer also includes a receive that raced
	// with Freeze but had not yet called record.
	queued, err := inputs.Freeze(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range queued {
		record(input.ID)
	}
	cancel()
	select {
	case <-consumerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not exit after inbox cancellation")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != total {
		t.Fatalf("delivered %d distinct inputs, want %d", len(delivered), total)
	}
	for id, times := range delivered {
		if times != 1 {
			t.Errorf("input %s crossed the fence %d times", id, times)
		}
	}
	// Exactly-once is only meaningful if the union covers every admitted ID.
	for producer := range producers {
		for index := range perProducer {
			id := inbox.ID(fmt.Sprintf("p%d-i%d", producer, index))
			if delivered[id] == 0 {
				t.Fatalf("admitted input %s never crossed the fence", id)
			}
		}
	}
}

// TestFreezeThawPreservesDeduplicationAcrossTheFence proves the fence does not
// reset the seen set: an ID admitted, frozen, thawed, and resubmitted must not be
// delivered a second time (local.go:143-148).
func TestFreezeThawPreservesDeduplicationAcrossTheFence(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	inputs, err := inbox.New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := inputs.Submit(ctx, testInput("once")); err != nil {
		t.Fatal(err)
	}
	queued, err := inputs.Freeze(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 || queued[0].ID != "once" {
		t.Fatalf("frozen batch = %v", queued)
	}
	if err := inputs.Thaw(ctx); err != nil {
		t.Fatal(err)
	}
	// A resubmission after the fence is accepted (idempotent) but already seen.
	if err := inputs.Submit(ctx, testInput("once")); err != nil {
		t.Fatalf("identical resubmission after the fence: %v", err)
	}
	if err := inputs.Submit(ctx, testInput("later")); err != nil {
		t.Fatal(err)
	}
	if got := receiveInput(t, inputs.Output()); got.ID != "later" {
		t.Fatalf("after the fence = %v", got)
	}
	assertNoInput(t, inputs.Output())
}
