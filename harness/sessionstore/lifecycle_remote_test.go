package sessionstore

import (
	"context"
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

type lifecycleItems struct {
	items []Item
}

func (s lifecycleItems) Items(_ context.Context, _ session.ID, after Sequence, _ int) (Page, error) {
	var items []Item
	for _, item := range s.items {
		if item.Sequence > after {
			items = append(items, item)
		}
	}
	return Page{Items: items}, nil
}

func TestLifecycleTerminalRemoteOwnershipNeedsLaterActualDrainReceipt(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusCompleted, operation.StatusCanceled, operation.StatusFailed, operation.StatusUnsupported} {
		t.Run(string(status), func(t *testing.T) {
			source := lifecycleItems{items: []Item{{Sequence: 1, Kind: ItemToolCallStatus, Data: ToolCallStatus{Operations: []operation.Operation{{ID: "retained-owner", Type: operation.TypeRemoteJob, Status: status}}}}}}
			history, err := ReadLifecycle(t.Context(), source, "s")
			if err != nil {
				t.Fatal(err)
			}
			if !errors.Is(history.ValidateResume(), ErrLifecycleRecoveryRequired) {
				t.Fatal("terminal root incorrectly proved retained-owner cleanup")
			}
			receipt := lifecycleControl(t, "receipt", inbox.ExecutionDrained, inbox.ExecutionDrainReceipt{Version: 1, Scope: "previous/1"})
			source.items = append(source.items, Item{Sequence: 2, Kind: ItemInput, Data: receipt})
			history, err = ReadLifecycle(t.Context(), source, "s")
			if err != nil {
				t.Fatal(err)
			}
			if err = history.ValidateResume(); err != nil {
				t.Fatalf("trusted later whole-execution receipt not recognized: %v", err)
			}
			// A subsequent remote event invalidates the old whole-execution barrier.
			source.items = append(source.items, Item{Sequence: 3, Kind: ItemToolCallStatus, Data: ToolCallStatus{Operations: []operation.Operation{{ID: "retained-owner", Type: operation.TypeRemoteJob, Status: status}}}})
			history, err = ReadLifecycle(t.Context(), source, "s")
			if err != nil {
				t.Fatal(err)
			}
			if history.ValidateResume() == nil {
				t.Fatal("old receipt covered later remote activity")
			}
			source.items = append(source.items, Item{Sequence: 4, Kind: ItemFork})
			history, err = ReadLifecycle(t.Context(), source, "fork")
			if err != nil {
				t.Fatal(err)
			}
			if len(history.RemoteOperations) != 0 || history.ValidateResume() != nil {
				t.Fatal("fork inherited source execution authority")
			}
		})
	}
}
