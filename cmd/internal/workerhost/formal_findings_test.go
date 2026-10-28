package workerhost

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Drift from docs/formal/handoff: the model only delivers to a ready owner, but
// launch publishes h.current before the hello/start handshake and the reload
// has already released the delivery fence. An input submitted while the
// replacement is starting is written as the child's first request, which the
// worker rejects because it must be start.
func TestInputSubmittedDuringReplacementLaunchWaitsForStart(t *testing.T) {
	h, r, _ := testHost(t, "")
	send(t, h, "before", "hello")
	waitInput(t, r, "before")

	gate := filepath.Join(t.TempDir(), "worker-gate")
	t.Setenv("UA_HOST_TEST_WORKER_GATE", gate) // inherited by the replacement only
	reloaded := make(chan error, 1)
	go func() { reloaded <- h.Reload(t.Context(), nil) }()
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(gate + ".ready"); err == nil {
			break
		}
		select {
		case err := <-reloaded:
			t.Fatalf("reload finished before replacement launch: %v", err)
		case <-deadline:
			t.Fatal("replacement never launched")
		case <-time.After(time.Millisecond):
		}
	}

	send(t, h, "during-launch", "world")
	time.Sleep(100 * time.Millisecond) // let the dispatcher run
	if err := os.WriteFile(gate, nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-reloaded:
		if err != nil {
			t.Fatalf("replacement failed after an input was submitted during its launch: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("reload did not finish")
	}
	waitInput(t, r, "during-launch")
}
