package coordinator

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

const defaultCompactionFailureLimit = 3

func (current *coordinator) compactionFailureLimit() int {
	if current.state.compactionFailureLimit > 0 {
		return current.state.compactionFailureLimit
	}
	if current.dependencies.CompactionFailureLimit > 0 {
		return current.dependencies.CompactionFailureLimit
	}
	return defaultCompactionFailureLimit
}

func (current *coordinator) recordControl(ctx context.Context, request inbox.ControlMessage) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode compaction control: %w", err)
	}
	input := inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload}
	// Controls changing retry policy must be durable before taking effect.
	item := sessionstore.Item{Kind: sessionstore.ItemInput, Data: input}
	if err := current.storeItemInSessionStore(ctx, item); err != nil {
		return err
	}
	_, err = current.addItemToLocalState(item)
	return err
}

func (current *coordinator) disableUnusableCompaction(ctx context.Context) error {
	if current.state.compactionDisabled || current.state.compactionFailures < current.compactionFailureLimit() {
		return nil
	}
	return current.recordControl(ctx, inbox.ControlMessage{
		Mode:   inbox.CompactionDisabled,
		Reason: fmt.Sprintf("Automatic compaction disabled after %d unusable summaries. Original context preserved; continuing ordinary work. Use /compact to retry manually.", current.state.compactionFailures),
	})
}

func (current *coordinator) applyCompactionResponse(response llm.Response) {
	if response.Stop != llm.StopComplete || response.Failure != nil {
		current.state.compactionFailures++
		return
	}
	rebuilt, compacted := current.dependencies.ContextBuilder.Compact(response)
	if compacted {
		current.dependencies.ContextBuilder = rebuilt
		current.state.compactionFailures = 0
		current.state.compactionDisabled = false
	} else {
		current.state.compactionFailures++
	}
}
