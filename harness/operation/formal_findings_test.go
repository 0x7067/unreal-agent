package operation_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// Counterexample AtMostOnceSpawn in docs/formal/operations/OperationCheckpoint.tla.
// acceptLocalStep queues the phase=process checkpoint and starts the process in
// the same actor step, so the process runs before the coordinator can persist
// that checkpoint. A crash in that window resumes from create_err; file creation
// is idempotent, so the command runs again instead of failing as Shell.resume
// does for a recorded process phase.
func TestShellCrashBeforeProcessCheckpointRunsCommandOnce(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runs")
	t.Setenv("RUN_MARKER", marker)
	current := newShellOperation(t, "crash-before-process-checkpoint", operation.ShellInput{
		Shell:     testShellPath,
		Command:   `echo run >> "$RUN_MARKER"; sleep 1`,
		Directory: t.TempDir(),
	}, t.TempDir(), 64)

	firstContext, crash := context.WithCancel(t.Context())
	first := operation.NewLocalOperationManager(firstContext)
	if err := first.Add(current); err != nil {
		t.Fatal(err)
	}
	// The coordinator persists updates in order; it has persisted create_err and
	// has not yet received the process checkpoint.
	durable := receiveOperation(t, first.Updates(), current.ID, func(update operation.Operation) bool {
		return shellState(t, update).Phase == operation.ShellPhaseCreateErr
	})
	waitForFileContents(t, marker, "run\n")
	crash()
	if err := first.AwaitStopped(t.Context()); err != nil {
		t.Fatal(err)
	}

	second := operation.NewLocalOperationManager(t.Context())
	if err := second.Add(durable); err != nil {
		t.Fatal(err)
	}
	resumed := receiveTerminalOperation(t, second.Updates(), current.ID)
	runs, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(runs) != "run\n" {
		t.Fatalf("command ran %q after resume (status %s); want exactly one run", runs, resumed.Status)
	}
	if state := shellState(t, resumed); resumed.Status != operation.StatusFailed ||
		!strings.HasPrefix(state.TerminalError, "shell execution outcome is unk") {
		t.Fatalf("resumed operation = %s %q, want failed with unknown outcome", resumed.Status, state.TerminalError)
	}
}

// Counterexample SingleTerminalCheckpoint in OperationCheckpoint.tla. A failed
// remote cancellation emits a terminal Failed update but retains the operation
// (remoteUnknown). The handler-closed branch then fails it again, emitting a
// second terminal update that rewrites the checkpoint the model already saw.
func TestRemoteJobHandlerStopAfterCancellationFailureEmitsOneTerminalUpdate(t *testing.T) {
	handler := newTestRemoteJobHandler()
	handler.cancelErr = errors.New("cannot cancel")
	manager := operation.NewLocalOperationManager(t.Context(), handler)
	current := newTestRemoteJobOperation(t, "cancel-failed-then-handler-stopped")
	if err := manager.Add(current); err != nil {
		t.Fatal(err)
	}
	<-handler.adds
	if err := manager.Cancel(current.ID, "requested"); err != nil {
		t.Fatal(err)
	}
	first := receiveRemoteJobUpdate(t, manager.Updates())
	if first.Status != operation.StatusFailed {
		t.Fatalf("first update = %#v, want failed", first)
	}
	close(handler.updates)

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case second := <-manager.Updates():
		firstState, _ := operation.DecodeRemoteJobState(first)
		secondState, _ := operation.DecodeRemoteJobState(second)
		t.Fatalf("operation %q got a second terminal update %q (%q) after %q (%q)",
			second.ID, second.Status, secondState.TerminalError, first.Status, firstState.TerminalError)
	case <-timer.C:
	}
}
